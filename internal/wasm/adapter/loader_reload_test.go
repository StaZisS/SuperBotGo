package adapter

import (
	"encoding/json"
	"strings"
	"testing"

	wasmrt "SuperBotGo/internal/wasm/runtime"
)

func TestLoaderBeginReload_UsesExistingConfigWhenNewConfigIsNil(t *testing.T) {
	t.Parallel()

	oldConfig := json.RawMessage(`{"token":"old"}`)
	loader := &Loader{
		plugins: map[string]*loadedPlugin{
			"demo": {
				plugin: &WasmPlugin{
					meta: wasmrt.PluginMeta{
						ID:      "demo",
						Version: "1.0.0",
					},
				},
				config: cloneRawMessage(oldConfig),
			},
		},
	}

	plan, err := loader.beginReload("demo", nil)
	if err != nil {
		t.Fatalf("beginReload() error = %v", err)
	}

	if got, want := string(plan.config), string(oldConfig); got != want {
		t.Fatalf("plan.config = %s, want %s", got, want)
	}
	plan.config[0] = '['
	if string(loader.plugins["demo"].config) != string(oldConfig) {
		t.Fatalf("old config mutated: got %s, want %s", loader.plugins["demo"].config, oldConfig)
	}
	if !loader.plugins["demo"].draining.Load() {
		t.Fatal("expected beginReload() to mark plugin as draining")
	}
}

func TestLoaderBeginReload_MissingPlugin(t *testing.T) {
	t.Parallel()

	loader := &Loader{plugins: map[string]*loadedPlugin{}}
	_, err := loader.beginReload("missing", nil)
	if err == nil {
		t.Fatal("beginReload() error = nil, want missing plugin error")
	}
	if !strings.Contains(err.Error(), `plugin "missing" not loaded`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoaderMaybeRunReloadMigration_SkipsWhenVersionUnchanged(t *testing.T) {
	t.Parallel()

	loader := &Loader{strictMigrate: true}
	err := loader.maybeRunReloadMigration(nil, &reloadPlan{
		pluginID:   "demo",
		oldVersion: "1.0.0",
	}, &preparedPlugin{
		plugin: &WasmPlugin{
			meta: wasmrt.PluginMeta{
				ID:      "demo",
				Version: "1.0.0",
			},
		},
	})
	if err != nil {
		t.Fatalf("maybeRunReloadMigration() error = %v, want nil", err)
	}
}
