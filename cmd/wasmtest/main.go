// Command wasmtest is a standalone CLI utility for testing .wasm plugins
// in the SuperBotGo one-shot execution model.
//
// Usage:
//
//	go run ./cmd/wasmtest meta   <path.wasm>
//	go run ./cmd/wasmtest configure <path.wasm> '<json>'
//	go run ./cmd/wasmtest handle    <path.wasm> '<json>'
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n!!! Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 3 {
		printUsage()
		return fmt.Errorf("not enough arguments")
	}

	action := os.Args[1]
	wasmPath := os.Args[2]

	var jsonData string
	if len(os.Args) >= 4 {
		jsonData = os.Args[3]
	}

	switch action {
	case "meta":

	case "configure":

	case "handle", "handle_command":
		action = "handle_command"
	default:
		printUsage()
		return fmt.Errorf("unknown action %q (expected: meta, configure, handle)", action)
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("read wasm file: %w", err)
	}

	fileSizeMB := float64(len(wasmBytes)) / (1024 * 1024)

	fmt.Fprintf(os.Stdout, "=== Plugin Test: %s ===\n", action)
	fmt.Fprintf(os.Stdout, "File: %s (%.1f MB)\n", wasmPath, fileSizeMB)

	ctx := context.Background()

	rtCfg := wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true).
		WithMemoryLimitPages(256)
	rt := wazero.NewRuntimeWithConfig(ctx, rtCfg)
	defer rt.Close(ctx)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		return fmt.Errorf("instantiate wasi: %w", err)
	}

	if err := registerEnvModule(ctx, rt); err != nil {
		return fmt.Errorf("register env module: %w", err)
	}

	compileStart := time.Now()
	compiled, err := rt.CompileModule(ctx, wasmBytes)
	compileDuration := time.Since(compileStart)
	if err != nil {
		return fmt.Errorf("compile wasm: %w", err)
	}
	defer compiled.Close(ctx)

	exports := compiled.ExportedFunctions()
	names := make([]string, 0, len(exports))
	for name := range exports {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Fprintf(os.Stdout, "Exports: %s\n", strings.Join(names, ", "))

	if _, ok := exports["alloc"]; !ok {
		fmt.Fprintf(os.Stderr, "\nWARNING: module does not export 'alloc' — host functions that write to memory will fail\n")
	}

	fmt.Fprintf(os.Stdout, "\n--- Running action: %s ---\n", action)

	// Prepare stdin.
	var stdin *bytes.Reader
	if jsonData != "" {
		stdin = bytes.NewReader([]byte(jsonData))
	} else {
		stdin = bytes.NewReader(nil)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	modCfg := wazero.NewModuleConfig().
		WithEnv("PLUGIN_ACTION", action).
		WithStdin(stdin).
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithName("")

	execStart := time.Now()
	_, execErr := rt.InstantiateModule(ctx, compiled, modCfg)
	execDuration := time.Since(execStart)

	if stderr.Len() > 0 {
		fmt.Fprintf(os.Stderr, "%s", stderr.String())
	}

	if execErr != nil {
		if exitErr, ok := execErr.(*sys.ExitError); ok {
			if exitErr.ExitCode() != 0 {
				return fmt.Errorf("wasm module exited with code %d", exitErr.ExitCode())
			}

		} else {
			return fmt.Errorf("wasm execution failed: %w", execErr)
		}
	}

	fmt.Fprintf(os.Stdout, "\n--- Result (stdout) ---\n")
	result := stdout.Bytes()
	if len(result) > 0 {
		// Try to pretty-print JSON.
		var prettyBuf bytes.Buffer
		if json.Indent(&prettyBuf, result, "", "  ") == nil {
			fmt.Fprintln(os.Stdout, prettyBuf.String())
		} else {

			fmt.Fprintln(os.Stdout, string(result))
		}
	} else {
		fmt.Fprintln(os.Stdout, "(empty)")
	}

	fmt.Fprintf(os.Stdout, "\n--- Timing ---\n")
	fmt.Fprintf(os.Stdout, "Compilation: %s\n", formatDuration(compileDuration))
	fmt.Fprintf(os.Stdout, "Execution:   %s\n", formatDuration(execDuration))

	return nil
}

// registerEnvModule registers the "env" host module with stub implementations
// of all host functions. Each stub reads data from module memory (when applicable)
// and logs the call to stderr.
func registerEnvModule(ctx context.Context, rt wazero.Runtime) error {
	builder := rt.NewHostModuleBuilder("env")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			offset := uint32(stack[0])
			length := uint32(stack[1])
			data := readMem(mod, offset, length)
			fmt.Fprintf(os.Stderr, "[DB_QUERY] %s\n", formatPayload(data))

			result := writeStubResponse(ctx, mod, []byte("[]"))
			stack[0] = result
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI64}).
		WithName("db_query").Export("db_query")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			offset := uint32(stack[0])
			length := uint32(stack[1])
			data := readMem(mod, offset, length)
			fmt.Fprintf(os.Stderr, "[DB_SAVE] %s\n", formatPayload(data))
			stack[0] = 0
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI64}).
		WithName("db_save").Export("db_save")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			offset := uint32(stack[0])
			length := uint32(stack[1])
			data := readMem(mod, offset, length)
			fmt.Fprintf(os.Stderr, "[HTTP_REQUEST] %s\n", formatPayload(data))

			stubResp := `{"status_code":200,"headers":{},"body":"{}"}`
			result := writeStubResponse(ctx, mod, []byte(stubResp))
			stack[0] = result
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI64}).
		WithName("http_request").Export("http_request")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			offset := uint32(stack[0])
			length := uint32(stack[1])
			data := readMem(mod, offset, length)
			fmt.Fprintf(os.Stderr, "[CALL_PLUGIN] %s\n", formatPayload(data))

			result := writeStubResponse(ctx, mod, []byte("{}"))
			stack[0] = result
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI64}).
		WithName("call_plugin").Export("call_plugin")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			offset := uint32(stack[0])
			length := uint32(stack[1])
			data := readMem(mod, offset, length)
			fmt.Fprintf(os.Stderr, "[PUBLISH_EVENT] %s\n", formatPayload(data))
			stack[0] = 0
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI64}).
		WithName("publish_event").Export("publish_event")

	_, err := builder.Instantiate(ctx)
	return err
}

// readMem reads bytes from the module's memory. Returns empty slice on failure.
func readMem(mod api.Module, offset, length uint32) []byte {
	if length == 0 {
		return nil
	}
	mem := mod.Memory()
	if mem == nil {
		fmt.Fprintf(os.Stderr, "[WASMTEST] WARNING: module has no memory\n")
		return nil
	}
	data, ok := mem.Read(offset, length)
	if !ok {
		fmt.Fprintf(os.Stderr, "[WASMTEST] WARNING: memory read out of bounds (offset=%d, length=%d)\n", offset, length)
		return nil
	}
	result := make([]byte, length)
	copy(result, data)
	return result
}

// writeStubResponse allocates memory in the module via "alloc" and writes data.
// Returns the packed offset<<32|length value, or 0 on failure.
func writeStubResponse(ctx context.Context, mod api.Module, data []byte) uint64 {
	length := uint32(len(data))
	if length == 0 {
		return 0
	}
	alloc := mod.ExportedFunction("alloc")
	if alloc == nil {
		fmt.Fprintf(os.Stderr, "[WASMTEST] WARNING: module does not export 'alloc', cannot write stub response\n")
		return 0
	}
	results, err := alloc.Call(ctx, uint64(length))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WASMTEST] WARNING: alloc(%d) failed: %v\n", length, err)
		return 0
	}
	if len(results) == 0 {
		fmt.Fprintf(os.Stderr, "[WASMTEST] WARNING: alloc(%d) returned no results\n", length)
		return 0
	}
	offset := uint32(results[0])
	mem := mod.Memory()
	if mem == nil {
		return 0
	}
	if !mem.Write(offset, data) {
		fmt.Fprintf(os.Stderr, "[WASMTEST] WARNING: memory write failed at offset=%d, length=%d\n", offset, length)
		return 0
	}
	return uint64(offset)<<32 | uint64(length)
}

// formatPayload tries to pretty-print data as JSON; falls back to showing raw bytes.
func formatPayload(data []byte) string {
	if len(data) == 0 {
		return "(empty)"
	}

	pretty := prettyJSON(data)
	if pretty != string(data) {
		return pretty
	}

	if isPrintable(data) {
		return string(data)
	}
	if len(data) > 64 {
		return fmt.Sprintf("(%d bytes) %x...", len(data), data[:64])
	}
	return fmt.Sprintf("(%d bytes) %x", len(data), data)
}

// prettyJSON attempts to format raw bytes as indented JSON.
// Returns the original string if it is not valid JSON.
func prettyJSON(data []byte) string {
	var buf bytes.Buffer
	if json.Indent(&buf, data, "", "  ") == nil {
		return buf.String()
	}
	return string(data)
}

// isPrintable returns true if all bytes are printable ASCII or common whitespace.
func isPrintable(data []byte) bool {
	for _, b := range data {
		if b < 0x20 && b != '\n' && b != '\r' && b != '\t' {
			return false
		}
		if b > 0x7e {
			return false
		}
	}
	return true
}

// formatDuration formats a duration as a human-friendly string.
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%d\u00b5s", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: wasmtest <action> <path.wasm> [json_data]

Actions:
  meta        Show plugin metadata
  configure   Send configuration JSON to the plugin
  handle      Send a handle_command request to the plugin

Examples:
  wasmtest meta plugins/example/plugin.wasm
  wasmtest configure plugins/example/plugin.wasm '{"key":"value"}'
  wasmtest handle plugins/example/plugin.wasm '{"user_id":{"platform":"TELEGRAM","platform_id":"123"},"channel_type":"TELEGRAM","chat_id":"456","command_name":"test","params":{},"locale":"ru"}'`)
}
