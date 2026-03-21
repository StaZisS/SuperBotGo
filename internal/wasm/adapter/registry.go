package adapter

import (
	"log/slog"

	"SuperBotGo/internal/plugin"
)

func RegisterWasmPlugins(manager *plugin.Manager, loader *Loader) {
	for _, wp := range loader.AllPlugins() {
		manager.Register(wp)
		slog.Info("wasm: registered plugin in manager", "id", wp.ID(), "name", wp.Name())
	}
}
