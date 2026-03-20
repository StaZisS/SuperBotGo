package api

import (
	"context"
	"io"
	"log/slog"

	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/wasm/adapter"
)

// AutoloadPlugins loads all enabled Wasm plugins from the store at startup.
func AutoloadPlugins(ctx context.Context, store PluginStore, blobs BlobStore, loader *adapter.Loader, manager *plugin.Manager) error {
	records, err := store.ListPlugins(ctx)
	if err != nil {
		return err
	}

	var loaded int
	for _, rec := range records {
		if !rec.Enabled {
			slog.Debug("wasm: skipping disabled plugin", "id", rec.ID)
			continue
		}

		rc, err := blobs.Get(ctx, rec.WasmKey)
		if err != nil {
			slog.Error("wasm: failed to read wasm blob for autoload", "id", rec.ID, "key", rec.WasmKey, "error", err)
			continue
		}
		wasmBytes, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			slog.Error("wasm: failed to read wasm blob bytes", "id", rec.ID, "error", err)
			continue
		}

		wp, err := loader.LoadPluginFromBytes(ctx, wasmBytes, rec.ConfigJSON, rec.Permissions)
		if err != nil {
			slog.Error("wasm: failed to autoload plugin", "id", rec.ID, "error", err)
			continue
		}

		manager.Register(wp)
		loaded++
		slog.Info("wasm: autoloaded plugin", "id", rec.ID, "name", wp.Name())
	}

	slog.Info("wasm: autoload complete", "loaded", loaded, "total", len(records))
	return nil
}
