package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/sys"
)

// requiredExports lists the function names that every Wasm plugin must export.
// In the one-shot model, meta/configure/handle_command are dispatched via
// PLUGIN_ACTION env var and stdin/stdout, so only "alloc" is required
// (host functions use it to write responses into module memory).
var requiredExports = []string{"alloc"}

// CompiledModule represents a validated and AOT-compiled Wasm module ready for instantiation.
type CompiledModule struct {
	compiled wazero.CompiledModule
	rt       *Runtime
	ID       string
	Version  string
	Hash     string
}

// CompileModule validates the Wasm binary and compiles it.
// It checks that all required exports (alloc) are present.
func (r *Runtime) CompileModule(ctx context.Context, wasmBytes []byte) (*CompiledModule, error) {
	compiled, err := r.engine.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile wasm module: %w", err)
	}

	exports := compiled.ExportedFunctions()
	for _, name := range requiredExports {
		if _, ok := exports[name]; !ok {
			_ = compiled.Close(ctx)
			return nil, fmt.Errorf("wasm module missing required export %q", name)
		}
	}

	hash := sha256.Sum256(wasmBytes)

	return &CompiledModule{
		compiled: compiled,
		rt:       r,
		Hash:     hex.EncodeToString(hash[:]),
	}, nil
}

// LoadModuleFromFile reads a .wasm file and compiles it.
func (r *Runtime) LoadModuleFromFile(ctx context.Context, path string) (*CompiledModule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wasm file %q: %w", path, err)
	}
	return r.CompileModule(ctx, data)
}

// Close releases the compiled module resources.
func (cm *CompiledModule) Close(ctx context.Context) error {
	return cm.compiled.Close(ctx)
}

// RunAction runs a one-shot module instance with the given action and input.
// The action is passed via PLUGIN_ACTION env var, input via stdin, result via stdout.
// Go wasip1 modules call proc_exit(0) after main(); this is treated as success.
func (cm *CompiledModule) RunAction(ctx context.Context, action string, input []byte) ([]byte, error) {
	return cm.RunActionWithConfig(ctx, action, input, nil)
}

// RunActionWithConfig is like RunAction but also passes plugin configuration
// via PLUGIN_CONFIG env var.
func (cm *CompiledModule) RunActionWithConfig(ctx context.Context, action string, input []byte, configJSON []byte) ([]byte, error) {
	timeout := time.Duration(cm.rt.config.DefaultTimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ctx = context.WithValue(ctx, PluginIDKey{}, cm.ID)

	var stdout bytes.Buffer
	var stdin *bytes.Reader
	if len(input) > 0 {
		stdin = bytes.NewReader(input)
	} else {
		stdin = bytes.NewReader(nil)
	}

	modCfg := wazero.NewModuleConfig().
		WithEnv("PLUGIN_ACTION", action).
		WithStdin(stdin).
		WithStdout(&stdout).
		WithName("")

	if len(configJSON) > 0 {
		modCfg = modCfg.WithEnv("PLUGIN_CONFIG", string(configJSON))
	}

	_, err := cm.rt.engine.InstantiateModule(ctx, cm.compiled, modCfg)
	if err != nil {

		if exitErr, ok := err.(*sys.ExitError); ok {
			if exitErr.ExitCode() == 0 {

				return stdout.Bytes(), nil
			}
			return nil, fmt.Errorf("wasm module exited with code %d", exitErr.ExitCode())
		}
		return nil, fmt.Errorf("instantiate wasm module: %w", err)
	}

	return stdout.Bytes(), nil
}

// CallMeta runs the "meta" action and returns parsed plugin metadata.
func (cm *CompiledModule) CallMeta(ctx context.Context) (PluginMeta, error) {
	data, err := cm.RunAction(ctx, "meta", nil)
	if err != nil {
		return PluginMeta{}, fmt.Errorf("call meta: %w", err)
	}

	var meta PluginMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return PluginMeta{}, fmt.Errorf("unmarshal meta JSON (%q): %w", string(data), err)
	}

	if meta.SDKVersion > MaxSupportedSDKVersion {
		return PluginMeta{}, fmt.Errorf(
			"plugin %q requires SDK protocol v%d, but host supports up to v%d — upgrade the host",
			meta.ID, meta.SDKVersion, MaxSupportedSDKVersion)
	}

	return meta, nil
}

// CallConfigure runs the "configure" action with the given config JSON.
func (cm *CompiledModule) CallConfigure(ctx context.Context, configJSON []byte) error {
	data, err := cm.RunAction(ctx, "configure", configJSON)
	if err != nil {
		return fmt.Errorf("call configure: %w", err)
	}

	if len(data) > 0 {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("configure error: %s", errResp.Error)
		}
	}

	return nil
}

// CallHandleCommand runs the "handle_command" action with the given request JSON.
func (cm *CompiledModule) CallHandleCommand(ctx context.Context, reqJSON []byte, configJSON []byte) ([]byte, error) {
	data, err := cm.RunActionWithConfig(ctx, "handle_command", reqJSON, configJSON)
	if err != nil {
		return nil, fmt.Errorf("call handle_command: %w", err)
	}
	return data, nil
}

// CallStepCallback runs the "step_callback" action with the given request JSON.
// This is used to invoke plugin-defined callback functions for validation,
// dynamic options, pagination, and condition evaluation.
func (cm *CompiledModule) CallStepCallback(ctx context.Context, reqJSON []byte, configJSON []byte) ([]byte, error) {
	data, err := cm.RunActionWithConfig(ctx, "step_callback", reqJSON, configJSON)
	if err != nil {
		return nil, fmt.Errorf("call step_callback: %w", err)
	}
	return data, nil
}
