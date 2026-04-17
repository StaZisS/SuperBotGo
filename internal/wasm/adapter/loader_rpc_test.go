package adapter

import (
	"context"
	"strings"
	"testing"

	"SuperBotGo/internal/wasm/hostapi"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

func TestLoaderCallPlugin_TargetNotLoaded(t *testing.T) {
	t.Parallel()

	loader := &Loader{plugins: map[string]*loadedPlugin{}}
	_, err := loader.CallPlugin(context.Background(), "missing", "ping", nil)
	if err == nil {
		t.Fatal("expected error for missing target plugin")
	}
	if !strings.Contains(err.Error(), `plugin "missing" is not loaded`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoaderCallPlugin_MethodNotExposed(t *testing.T) {
	t.Parallel()

	loader := &Loader{
		plugins: map[string]*loadedPlugin{
			"callee": {
				plugin: &WasmPlugin{
					meta: wasmrt.PluginMeta{
						ID: "callee",
						RPCMethods: []wasmrt.RPCMethodDef{
							{Name: "ping"},
						},
					},
				},
			},
		},
	}

	_, err := loader.CallPlugin(context.Background(), "callee", "unknown", nil)
	if err == nil {
		t.Fatal("expected error for unknown rpc method")
	}
	if !strings.Contains(err.Error(), `does not expose rpc method "unknown"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDatabaseRequirements_RejectsPluginRequirementWhenRPCDisabled(t *testing.T) {
	t.Parallel()

	loader := &Loader{hostAPI: hostapi.NewHostAPI(hostapi.Dependencies{})}
	err := loader.validateDatabaseRequirements(wasmrt.PluginMeta{
		ID: "caller",
		Requirements: []wasmrt.RequirementDef{
			{Type: "plugin", Target: "callee"},
		},
	})
	if err == nil {
		t.Fatal("expected error when plugin requirement is declared but rpc is disabled")
	}
	if !strings.Contains(err.Error(), "inter-plugin RPC") {
		t.Fatalf("unexpected error: %v", err)
	}
}
