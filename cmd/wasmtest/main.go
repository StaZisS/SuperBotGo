package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

type hostCallEntry struct {
	Function string          `json:"function"`
	Input    json.RawMessage `json:"input"`
	Output   json.RawMessage `json:"output"`
	Time     time.Time       `json:"time"`
}

type hostCallLog struct {
	mu      sync.Mutex
	entries []hostCallEntry
}

func (l *hostCallLog) record(name string, input, output []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, hostCallEntry{
		Function: name,
		Input:    normalizeJSON(input),
		Output:   normalizeJSON(output),
		Time:     time.Now(),
	})
}

func (l *hostCallLog) all() []hostCallEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := make([]hostCallEntry, len(l.entries))
	copy(cp, l.entries)
	return cp
}

func (l *hostCallLog) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = nil
}

func normalizeJSON(data []byte) json.RawMessage {
	if len(data) == 0 {
		return json.RawMessage("null")
	}
	if json.Valid(data) {
		return json.RawMessage(data)
	}
	b, _ := json.Marshal(string(data))
	return json.RawMessage(b)
}

type mockStore map[string]json.RawMessage

func loadMocks(path string) (mockStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read mocks file: %w", err)
	}
	var raw map[string]map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse mocks JSON: %w", err)
	}
	store := make(mockStore, len(raw))
	for fn, variants := range raw {
		if def, ok := variants["default"]; ok {
			store[fn] = def
		}
	}
	return store, nil
}

func (m mockStore) get(name string) ([]byte, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[name]
	if !ok {
		return nil, false
	}
	return []byte(v), true
}

var (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

func disableColors() {
	colorReset = ""
	colorRed = ""
	colorGreen = ""
	colorYellow = ""
	colorCyan = ""
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

type options struct {
	verbose   bool
	raw       bool
	mocksFile string
	inputFile string
	suiteFile string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s!!! Error: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printUsage()
		return fmt.Errorf("not enough arguments")
	}

	if os.Args[1] == "test" {
		return runTestSuite()
	}

	if len(os.Args) < 3 {
		printUsage()
		return fmt.Errorf("not enough arguments")
	}

	action := os.Args[1]
	wasmPath := os.Args[2]

	opts := parseFlags(os.Args[3:])

	if opts.raw || !isTerminal() {
		disableColors()
	}

	action, err := resolveAction(action)
	if err != nil {
		printUsage()
		return err
	}

	jsonData, err := resolveInput(opts)
	if err != nil {
		return err
	}

	var mocks mockStore
	if opts.mocksFile != "" {
		mocks, err = loadMocks(opts.mocksFile)
		if err != nil {
			return err
		}
	}

	callLog := &hostCallLog{}

	result, timing, err := executePlugin(context.Background(), wasmPath, action, jsonData, mocks, callLog, opts.verbose)
	if err != nil {
		return err
	}

	printResult(result, timing, opts, callLog)
	return nil
}

var validActions = map[string]string{
	"meta":           "meta",
	"configure":      "configure",
	"handle":         "handle_command",
	"handle_command": "handle_command",
	"handle_event":   "handle_event",
	"event":          "handle_event",
	"step_callback":  "step_callback",
	"callback":       "step_callback",
	"migrate":        "migrate",
}

func resolveAction(action string) (string, error) {
	if resolved, ok := validActions[action]; ok {
		return resolved, nil
	}
	names := make([]string, 0, len(validActions))
	seen := map[string]bool{}
	for _, v := range validActions {
		if !seen[v] {
			names = append(names, v)
			seen[v] = true
		}
	}
	sort.Strings(names)
	return "", fmt.Errorf("unknown action %q (valid: %s)", action, strings.Join(names, ", "))
}

func parseFlags(args []string) options {
	var opts options
	var positionalJSON string

	var flagArgs []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && positionalJSON == "" && opts.inputFile == "" {
			positionalJSON = a
		} else {
			flagArgs = append(flagArgs, a)
		}
	}

	fs := flag.NewFlagSet("wasmtest", flag.ContinueOnError)
	fs.BoolVar(&opts.verbose, "verbose", false, "")
	fs.BoolVar(&opts.verbose, "v", false, "")
	fs.BoolVar(&opts.raw, "raw", false, "")
	fs.StringVar(&opts.mocksFile, "mocks", "", "")
	fs.StringVar(&opts.inputFile, "input", "", "")
	fs.StringVar(&opts.suiteFile, "suite", "", "")
	_ = fs.Parse(flagArgs)

	if positionalJSON != "" {
		opts.inputFile = ":" + positionalJSON
	}

	return opts
}

func resolveInput(opts options) (string, error) {
	if opts.inputFile == "" {
		return "", nil
	}
	if strings.HasPrefix(opts.inputFile, ":") {
		return opts.inputFile[1:], nil
	}
	data, err := os.ReadFile(opts.inputFile)
	if err != nil {
		return "", fmt.Errorf("read input file %q: %w", opts.inputFile, err)
	}
	return string(data), nil
}

type timingInfo struct {
	Compile     time.Duration
	Instantiate time.Duration
}

func executePlugin(ctx context.Context, wasmPath, action, jsonData string, mocks mockStore, callLog *hostCallLog, verbose bool) ([]byte, timingInfo, error) {
	var timing timingInfo

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, timing, fmt.Errorf("read wasm file: %w", err)
	}

	fileSizeMB := float64(len(wasmBytes)) / (1024 * 1024)
	fmt.Fprintf(os.Stdout, "%s=== Plugin Test: %s ===%s\n", colorCyan, action, colorReset)
	fmt.Fprintf(os.Stdout, "File: %s (%.1f MB)\n", wasmPath, fileSizeMB)

	rtCfg := wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true).
		WithMemoryLimitPages(256)
	rt := wazero.NewRuntimeWithConfig(ctx, rtCfg)
	defer rt.Close(ctx)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		return nil, timing, fmt.Errorf("instantiate wasi: %w", err)
	}

	if err := registerEnvModule(ctx, rt, mocks, callLog, verbose); err != nil {
		return nil, timing, fmt.Errorf("register env module: %w", err)
	}

	compileStart := time.Now()
	compiled, err := rt.CompileModule(ctx, wasmBytes)
	timing.Compile = time.Since(compileStart)
	if err != nil {
		return nil, timing, fmt.Errorf("compile wasm: %w", err)
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
		fmt.Fprintf(os.Stderr, "%sWARNING: module does not export 'alloc' — host functions that write to memory will fail%s\n", colorYellow, colorReset)
	}

	fmt.Fprintf(os.Stdout, "\n--- Running action: %s ---\n", action)

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
		WithFSConfig(wazero.NewFSConfig()).
		WithName("")

	execStart := time.Now()
	_, execErr := rt.InstantiateModule(ctx, compiled, modCfg)
	timing.Instantiate = time.Since(execStart)

	if stderr.Len() > 0 {
		fmt.Fprintf(os.Stderr, "%s", stderr.String())
	}

	if execErr != nil {
		if exitErr, ok := execErr.(*sys.ExitError); ok {
			if exitErr.ExitCode() != 0 {
				return nil, timing, fmt.Errorf("wasm module exited with code %d", exitErr.ExitCode())
			}
		} else {
			return nil, timing, fmt.Errorf("wasm execution failed: %w", execErr)
		}
	}

	return stdout.Bytes(), timing, nil
}

func printResult(result []byte, timing timingInfo, opts options, callLog *hostCallLog) {
	if opts.verbose {
		entries := callLog.all()
		if len(entries) > 0 {
			fmt.Fprintf(os.Stdout, "\n%s--- Host Function Calls (%d) ---%s\n", colorCyan, len(entries), colorReset)
			for i, e := range entries {
				fmt.Fprintf(os.Stdout, "  %d. %s%s%s\n", i+1, colorYellow, e.Function, colorReset)
				fmt.Fprintf(os.Stdout, "     Input:  %s\n", prettyJSON(e.Input))
				fmt.Fprintf(os.Stdout, "     Output: %s\n", prettyJSON(e.Output))
			}
		}
	}

	fmt.Fprintf(os.Stdout, "\n--- Result (stdout) ---\n")
	if len(result) > 0 {
		if opts.raw {
			fmt.Fprintln(os.Stdout, string(result))
		} else {
			var prettyBuf bytes.Buffer
			if json.Indent(&prettyBuf, result, "", "  ") == nil {
				fmt.Fprintln(os.Stdout, prettyBuf.String())
			} else {
				fmt.Fprintln(os.Stdout, string(result))
			}
		}
	} else {
		fmt.Fprintln(os.Stdout, "(empty)")
	}

	fmt.Fprintf(os.Stdout, "\n--- Timing ---\n")
	fmt.Fprintf(os.Stdout, "Compilation: %s\n", formatDuration(timing.Compile))
	fmt.Fprintf(os.Stdout, "Execution:   %s\n", formatDuration(timing.Instantiate))

	if opts.verbose {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		fmt.Fprintf(os.Stdout, "\n--- Memory ---\n")
		fmt.Fprintf(os.Stdout, "Alloc:      %.1f MB\n", float64(m.Alloc)/(1024*1024))
		fmt.Fprintf(os.Stdout, "TotalAlloc: %.1f MB\n", float64(m.TotalAlloc)/(1024*1024))
		fmt.Fprintf(os.Stdout, "Sys:        %.1f MB\n", float64(m.Sys)/(1024*1024))
	}
}

type testCase struct {
	Name   string          `json:"name"`
	Action string          `json:"action"`
	Input  json.RawMessage `json:"input"`
	Expect json.RawMessage `json:"expect"`
	Mocks  json.RawMessage `json:"mocks,omitempty"`
}

type testSuiteResult struct {
	passed int
	failed int
	errors []string
}

func runTestSuite() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: wasmtest test <path.wasm> --suite <test_suite.json> [--verbose] [--raw] [--mocks mocks.json]")
	}
	wasmPath := os.Args[2]

	opts := parseFlags(os.Args[3:])
	if opts.suiteFile == "" {
		return fmt.Errorf("--suite flag is required for the test subcommand")
	}

	if opts.raw || !isTerminal() {
		disableColors()
	}

	suiteData, err := os.ReadFile(opts.suiteFile)
	if err != nil {
		return fmt.Errorf("read test suite file: %w", err)
	}

	var cases []testCase
	if err := json.Unmarshal(suiteData, &cases); err != nil {
		return fmt.Errorf("parse test suite JSON: %w", err)
	}

	var globalMocks mockStore
	if opts.mocksFile != "" {
		globalMocks, err = loadMocks(opts.mocksFile)
		if err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stdout, "%s=== Test Suite: %s (%d tests) ===%s\n\n", colorCyan, opts.suiteFile, len(cases), colorReset)

	res := testSuiteResult{}

	for i, tc := range cases {
		action, err := resolveAction(tc.Action)
		if err != nil {
			res.failed++
			msg := fmt.Sprintf("  %s[FAIL]%s %s: invalid action %q", colorRed, colorReset, tc.Name, tc.Action)
			res.errors = append(res.errors, msg)
			fmt.Fprintln(os.Stdout, msg)
			continue
		}

		mocks := globalMocks
		if len(tc.Mocks) > 0 {
			var raw map[string]map[string]json.RawMessage
			if json.Unmarshal(tc.Mocks, &raw) == nil {
				mocks = make(mockStore, len(raw))
				for fn, variants := range raw {
					if def, ok := variants["default"]; ok {
						mocks[fn] = def
					}
				}
			}
		}

		callLog := &hostCallLog{}

		inputStr := ""
		if len(tc.Input) > 0 {
			inputStr = string(tc.Input)
		}

		origStdout := os.Stdout
		if !opts.verbose {
			os.Stdout, _ = os.Open(os.DevNull)
		}

		result, _, execErr := executePlugin(context.Background(), wasmPath, action, inputStr, mocks, callLog, opts.verbose)

		if !opts.verbose {
			os.Stdout = origStdout
		}

		testNum := i + 1

		if execErr != nil {
			res.failed++
			msg := fmt.Sprintf("  %s[FAIL]%s #%d %s: execution error: %v", colorRed, colorReset, testNum, tc.Name, execErr)
			res.errors = append(res.errors, msg)
			fmt.Fprintln(os.Stdout, msg)
			continue
		}

		if len(tc.Expect) > 0 {
			pass, detail := assertSubset(tc.Expect, result)
			if pass {
				res.passed++
				fmt.Fprintf(os.Stdout, "  %s[PASS]%s #%d %s\n", colorGreen, colorReset, testNum, tc.Name)
			} else {
				res.failed++
				msg := fmt.Sprintf("  %s[FAIL]%s #%d %s: %s", colorRed, colorReset, testNum, tc.Name, detail)
				res.errors = append(res.errors, msg)
				fmt.Fprintln(os.Stdout, msg)
			}
		} else {
			res.passed++
			fmt.Fprintf(os.Stdout, "  %s[PASS]%s #%d %s\n", colorGreen, colorReset, testNum, tc.Name)
		}
	}

	fmt.Fprintf(os.Stdout, "\n%s--- Summary ---%s\n", colorCyan, colorReset)
	fmt.Fprintf(os.Stdout, "Total: %d  Passed: %s%d%s  Failed: %s%d%s\n",
		res.passed+res.failed,
		colorGreen, res.passed, colorReset,
		colorRed, res.failed, colorReset,
	)

	if res.failed > 0 {
		fmt.Fprintln(os.Stdout)
		for _, e := range res.errors {
			fmt.Fprintln(os.Stdout, e)
		}
		return fmt.Errorf("%d test(s) failed", res.failed)
	}

	return nil
}

func assertSubset(expectedRaw json.RawMessage, actualBytes []byte) (bool, string) {
	if len(actualBytes) == 0 {
		return false, "actual output is empty"
	}

	var expected interface{}
	if err := json.Unmarshal(expectedRaw, &expected); err != nil {
		return false, fmt.Sprintf("invalid expected JSON: %v", err)
	}

	var actual interface{}
	if err := json.Unmarshal(actualBytes, &actual); err != nil {
		return false, fmt.Sprintf("actual output is not valid JSON: %v", err)
	}

	return deepSubset("$", expected, actual)
}

func deepSubset(path string, expected, actual interface{}) (bool, string) {
	switch exp := expected.(type) {
	case map[string]interface{}:
		act, ok := actual.(map[string]interface{})
		if !ok {
			return false, fmt.Sprintf("at %s: expected object, got %T", path, actual)
		}
		for key, expVal := range exp {
			actVal, exists := act[key]
			if !exists {
				return false, fmt.Sprintf("at %s.%s: missing field", path, key)
			}
			if ok, detail := deepSubset(path+"."+key, expVal, actVal); !ok {
				return false, detail
			}
		}
		return true, ""

	case []interface{}:
		act, ok := actual.([]interface{})
		if !ok {
			return false, fmt.Sprintf("at %s: expected array, got %T", path, actual)
		}
		if len(exp) != len(act) {
			return false, fmt.Sprintf("at %s: expected array length %d, got %d", path, len(exp), len(act))
		}
		for i := range exp {
			if ok, detail := deepSubset(fmt.Sprintf("%s[%d]", path, i), exp[i], act[i]); !ok {
				return false, detail
			}
		}
		return true, ""

	default:
		if !reflect.DeepEqual(expected, actual) {
			expJSON, _ := json.Marshal(expected)
			actJSON, _ := json.Marshal(actual)
			return false, fmt.Sprintf("at %s: expected %s, got %s", path, expJSON, actJSON)
		}
		return true, ""
	}
}

var defaultMockResponses = map[string][]byte{
	"db_query":      []byte("[]"),
	"db_save":       nil,
	"http_request":  []byte(`{"status_code":200,"headers":{},"body":"{}"}`),
	"call_plugin":   []byte("{}"),
	"publish_event": nil,
	"kv_get":        []byte("{}"),
	"kv_set":        nil,
	"kv_delete":     nil,
	"kv_list":       []byte("[]"),
}

func registerEnvModule(ctx context.Context, rt wazero.Runtime, mocks mockStore, callLog *hostCallLog, verbose bool) error {
	builder := rt.NewHostModuleBuilder("env")

	paramTypes := []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}
	resultTypes := []api.ValueType{api.ValueTypeI64}

	hostFunctions := []string{
		"db_query", "db_save", "http_request", "call_plugin",
		"publish_event", "kv_get", "kv_set", "kv_delete", "kv_list",
	}

	for _, fnName := range hostFunctions {
		name := fnName
		builder.NewFunctionBuilder().
			WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				offset := uint32(stack[0])
				length := uint32(stack[1])
				inputData := readMem(mod, offset, length)

				var responseData []byte
				if mockResp, ok := mocks.get(name); ok {
					responseData = mockResp
				} else if def, ok := defaultMockResponses[name]; ok {
					responseData = def
				}

				if verbose {
					tag := strings.ToUpper(name)
					fmt.Fprintf(os.Stderr, "[%s] %s\n", tag, formatPayload(inputData))
				}

				callLog.record(name, inputData, responseData)

				if responseData == nil {
					stack[0] = 0
					return
				}
				result := writeStubResponse(ctx, mod, responseData)
				stack[0] = result
			}), paramTypes, resultTypes).
			WithName(name).Export(name)
	}

	_, err := builder.Instantiate(ctx)
	return err
}

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

func prettyJSON(data []byte) string {
	var buf bytes.Buffer
	if json.Indent(&buf, data, "", "  ") == nil {
		return buf.String()
	}
	return string(data)
}

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
	fmt.Fprintln(os.Stderr, `Usage: wasmtest <action> <path.wasm> [input_json] [flags]
       wasmtest test <path.wasm> --suite <test_suite.json> [flags]

Actions:
  meta             Show plugin metadata
  configure        Send configuration JSON to the plugin
  handle           Send a handle_command request (alias: handle_command)
  handle_event     Send a handle_event request (alias: event)
  step_callback    Send a step_callback request (alias: callback)
  migrate          Run the migrate action

Flags:
  --mocks <file>   JSON file with mock host function responses
  --input <file>   Read input JSON from a file instead of positional arg
  --verbose, -v    Log all host function calls, show memory stats
  --raw            Output raw JSON without pretty-printing or colours
  --suite <file>   (test subcommand) JSON file with test cases

Test suite format:
  [
    {
      "name": "test name",
      "action": "handle_event",
      "input": { ... },
      "expect": { "status": "ok" }
    }
  ]

Mock file format:
  {
    "db_query": { "default": [{"id": 1, "name": "test"}] },
    "http_request": { "default": {"status_code": 200, "body": "ok"} }
  }

Examples:
  wasmtest meta plugin.wasm
  wasmtest handle plugin.wasm '{"command_name":"test"}' --verbose
  wasmtest handle_event plugin.wasm --input event.json --mocks mocks.json
  wasmtest test plugin.wasm --suite tests.json --mocks mocks.json`)
}
