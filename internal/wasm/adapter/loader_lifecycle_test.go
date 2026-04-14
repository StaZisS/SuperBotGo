package adapter

import (
	"testing"

	"SuperBotGo/internal/wasm/registry"
)

func TestUnregisterPluginRegistry_RemovesEntry(t *testing.T) {
	reg := registry.NewPluginRegistry()
	reg.Register(registry.PluginEntry{
		ID:   "lostandfound",
		Name: "Lost & Found",
		Versions: []registry.VersionEntry{{
			Version:  "1.0.0",
			WasmHash: "old-hash",
		}},
	})

	loader := &Loader{registry: reg}
	loader.unregisterPluginRegistry("lostandfound")

	if reg.Has("lostandfound") {
		t.Fatal("expected registry entry to be removed")
	}
}
