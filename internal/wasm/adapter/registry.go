package adapter

import (
	"log/slog"

	"SuperBotGo/internal/plugin"
)

// RegisterWasmPlugins registers all loaded Wasm plugins in the existing plugin manager.
func RegisterWasmPlugins(manager *plugin.Manager, loader *Loader) {
	for _, wp := range loader.AllPlugins() {
		manager.Register(wp)
		slog.Info("wasm: registered plugin in manager", "id", wp.ID(), "name", wp.Name())
	}
}

// UnregisterWasmPlugin removes a Wasm plugin from the plugin manager.
func UnregisterWasmPlugin(manager *plugin.Manager, pluginID string) {
	manager.Remove(pluginID)
	slog.Info("wasm: unregistered plugin from manager", "id", pluginID)
}
