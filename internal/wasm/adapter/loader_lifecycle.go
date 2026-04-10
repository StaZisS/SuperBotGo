package adapter

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// DropPluginData reverts every goose migration declared by the loaded plugin
// and removes its per-plugin goose tracking table. The plugin must still be
// loaded when this is called — callers that need to clean up a disabled
// plugin should load it temporarily first.
func (l *Loader) DropPluginData(ctx context.Context, pluginID string) error {
	l.mu.RLock()
	lp, ok := l.plugins[pluginID]
	l.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin %q not loaded", pluginID)
	}

	meta := lp.plugin.Meta()
	if len(meta.Migrations) == 0 {
		return nil
	}

	sqlStore := l.hostAPI.SQLStore()
	if sqlStore == nil {
		return nil
	}

	dsn := sqlStore.DSN(pluginID, "default")
	if dsn == "" {
		return nil
	}

	return dropPluginMigrations(ctx, pluginID, dsn, meta.Migrations)
}

func (l *Loader) UnloadPlugin(ctx context.Context, pluginID string) error {
	l.mu.Lock()
	lp, ok := l.plugins[pluginID]
	if !ok {
		l.mu.Unlock()
		return fmt.Errorf("plugin %q not loaded", pluginID)
	}
	delete(l.plugins, pluginID)
	l.mu.Unlock()

	if l.metrics != nil {
		l.metrics.LoadedPluginsGauge.Dec()
	}

	l.hostAPI.RevokePermissions(pluginID)

	if sqlStore := l.hostAPI.SQLStore(); sqlStore != nil {
		sqlStore.UnregisterPlugin(pluginID)
	}

	if l.triggerRegistry != nil {
		l.triggerRegistry.UnregisterTriggers(pluginID)
	}

	l.drainPlugin(lp, pluginID)

	if err := lp.compiled.Close(ctx); err != nil {
		slog.Error("wasm: error closing compiled module", "plugin", pluginID, "error", err)
	}

	slog.Info("wasm: plugin unloaded", "id", pluginID)
	return nil
}

func (l *Loader) AcquirePlugin(pluginID string) (*WasmPlugin, func()) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	lp, ok := l.plugins[pluginID]
	if !ok || lp.draining.Load() {
		return nil, func() {}
	}
	lp.activeRequests.Add(1)
	var once sync.Once
	return lp.plugin, func() {
		once.Do(func() {
			if lp.activeRequests.Add(-1) <= 0 && lp.draining.Load() {
				select {
				case <-lp.drained:
				default:
					close(lp.drained)
				}
			}
		})
	}
}

func (l *Loader) GetPlugin(pluginID string) (*WasmPlugin, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	lp, ok := l.plugins[pluginID]
	if !ok {
		return nil, false
	}
	return lp.plugin, true
}

func (l *Loader) AllPlugins() []*WasmPlugin {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]*WasmPlugin, 0, len(l.plugins))
	for _, lp := range l.plugins {
		result = append(result, lp.plugin)
	}
	return result
}

func (l *Loader) drainPlugin(lp *loadedPlugin, pluginID string) {
	lp.draining.Store(true)

	if lp.activeRequests.Load() <= 0 {
		select {
		case <-lp.drained:
		default:
			close(lp.drained)
		}
	}

	select {
	case <-lp.drained:
		slog.Info("wasm: plugin drained gracefully", "plugin", pluginID)
	case <-time.After(pluginDrainTimeout):
		slog.Warn("wasm: plugin drain timed out, force closing",
			"plugin", pluginID,
			"active_requests", lp.activeRequests.Load())
	}
}

func (l *Loader) Close(ctx context.Context) error {
	l.mu.Lock()
	snapshot := l.plugins
	l.plugins = make(map[string]*loadedPlugin)
	l.mu.Unlock()

	for _, lp := range snapshot {
		lp.draining.Store(true)
	}

	var firstErr error
	for id, lp := range snapshot {
		l.drainPlugin(lp, id)
		if err := lp.compiled.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		l.hostAPI.RevokePermissions(id)
		if sqlStore := l.hostAPI.SQLStore(); sqlStore != nil {
			sqlStore.UnregisterPlugin(id)
		}
	}

	if l.metrics != nil {
		l.metrics.LoadedPluginsGauge.Set(0)
	}
	return firstErr
}
