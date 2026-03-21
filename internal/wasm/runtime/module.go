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

var requiredExports = []string{"alloc"}

type CompiledModule struct {
	compiled wazero.CompiledModule
	rt       *Runtime
	ID       string
	Version  string
	Hash     string
}

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

func (r *Runtime) LoadModuleFromFile(ctx context.Context, path string) (*CompiledModule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wasm file %q: %w", path, err)
	}
	return r.CompileModule(ctx, data)
}

func (cm *CompiledModule) Close(ctx context.Context) error {
	return cm.compiled.Close(ctx)
}

func (cm *CompiledModule) RunAction(ctx context.Context, action string, input []byte) ([]byte, error) {
	return cm.RunActionWithConfig(ctx, action, input, nil)
}

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

func (cm *CompiledModule) CallHandleCommand(ctx context.Context, reqJSON []byte, configJSON []byte) ([]byte, error) {
	data, err := cm.RunActionWithConfig(ctx, "handle_command", reqJSON, configJSON)
	if err != nil {
		return nil, fmt.Errorf("call handle_command: %w", err)
	}
	return data, nil
}

func (cm *CompiledModule) CallStepCallback(ctx context.Context, reqJSON []byte, configJSON []byte) ([]byte, error) {
	data, err := cm.RunActionWithConfig(ctx, "step_callback", reqJSON, configJSON)
	if err != nil {
		return nil, fmt.Errorf("call step_callback: %w", err)
	}
	return data, nil
}

func (cm *CompiledModule) CallHandleEvent(ctx context.Context, eventJSON []byte, configJSON []byte) ([]byte, error) {
	data, err := cm.RunActionWithConfig(ctx, "handle_event", eventJSON, configJSON)
	if err != nil {
		return nil, fmt.Errorf("call handle_event: %w", err)
	}
	return data, nil
}
