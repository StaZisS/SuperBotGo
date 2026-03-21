package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/trigger"
	"SuperBotGo/internal/wasm/hostapi"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

type Loader struct {
	mu              sync.RWMutex
	rt              *wasmrt.Runtime
	hostAPI         *hostapi.HostAPI
	send            SendFunc
	plugins         map[string]*loadedPlugin
	triggerRegistry *trigger.Registry
	metrics         *metrics.Metrics
}

func (l *Loader) SetMetrics(m *metrics.Metrics) {
	l.metrics = m
}

type loadedPlugin struct {
	plugin   *WasmPlugin
	compiled *wasmrt.CompiledModule
	config   json.RawMessage
	perms    []string
}

func NewLoader(rt *wasmrt.Runtime, hostAPI *hostapi.HostAPI, send SendFunc) *Loader {
	return &Loader{
		rt:      rt,
		hostAPI: hostAPI,
		send:    send,
		plugins: make(map[string]*loadedPlugin),
	}
}

func (l *Loader) LoadPlugin(ctx context.Context, wasmPath string, config json.RawMessage, permissions []string) (*WasmPlugin, error) {
	data, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("read wasm file %q: %w", wasmPath, err)
	}
	return l.LoadPluginFromBytes(ctx, data, config, permissions)
}

func (l *Loader) LoadPluginFromBytes(ctx context.Context, wasmBytes []byte, config json.RawMessage, permissions []string) (*WasmPlugin, error) {

	compiled, err := l.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile wasm plugin: %w", err)
	}

	const probeID = "_temp_probe"
	l.hostAPI.ForPlugin(probeID, permissions)
	compiled.ID = probeID

	meta, err := compiled.CallMeta(ctx)
	l.hostAPI.RevokePermissions(probeID)
	if err != nil {
		_ = compiled.Close(ctx)
		return nil, fmt.Errorf("call meta: %w", err)
	}

	l.hostAPI.ForPlugin(meta.ID, permissions)
	compiled.ID = meta.ID
	compiled.Version = meta.Version

	if len(config) > 0 {
		if err := compiled.CallConfigure(ctx, config); err != nil {
			_ = compiled.Close(ctx)
			l.hostAPI.RevokePermissions(meta.ID)
			return nil, fmt.Errorf("configure plugin %q: %w", meta.ID, err)
		}
	}

	wp := &WasmPlugin{
		compiled: compiled,
		meta:     meta,
		config:   config,
		send:     l.send,
	}

	l.mu.Lock()
	l.plugins[meta.ID] = &loadedPlugin{
		plugin:   wp,
		compiled: compiled,
		config:   config,
		perms:    permissions,
	}
	l.mu.Unlock()

	if l.metrics != nil {
		l.metrics.LoadedPluginsGauge.Inc()
	}

	if l.triggerRegistry != nil && len(meta.Triggers) > 0 {
		l.triggerRegistry.RegisterTriggers(meta.ID, meta.Triggers)
		slog.Info("wasm: registered triggers", "plugin", meta.ID, "count", len(meta.Triggers))
	}

	slog.Info("wasm: plugin loaded", "id", meta.ID, "name", meta.Name, "version", meta.Version)
	return wp, nil
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

	if l.triggerRegistry != nil {
		l.triggerRegistry.UnregisterTriggers(pluginID)
	}

	if err := lp.compiled.Close(ctx); err != nil {
		slog.Error("wasm: error closing compiled module", "plugin", pluginID, "error", err)
	}

	slog.Info("wasm: plugin unloaded", "id", pluginID)
	return nil
}

func (l *Loader) SetTriggerRegistry(registry *trigger.Registry) {
	l.triggerRegistry = registry
}

func (l *Loader) ReloadPlugin(ctx context.Context, pluginID string, newWasmPath string, newConfig json.RawMessage) error {
	data, err := os.ReadFile(newWasmPath)
	if err != nil {
		return fmt.Errorf("read wasm file %q: %w", newWasmPath, err)
	}
	return l.ReloadPluginFromBytes(ctx, pluginID, data, newConfig)
}

func (l *Loader) ReloadPluginFromBytes(ctx context.Context, pluginID string, wasmBytes []byte, newConfig json.RawMessage) error {
	l.mu.RLock()
	old, ok := l.plugins[pluginID]
	if !ok {
		l.mu.RUnlock()
		return fmt.Errorf("plugin %q not loaded", pluginID)
	}
	perms := old.perms
	config := newConfig
	if config == nil {
		config = old.config
	}
	oldCompiled := old.compiled
	l.mu.RUnlock()

	newPlugin, err := l.LoadPluginFromBytes(ctx, wasmBytes, config, perms)
	if err != nil {
		return fmt.Errorf("reload plugin %q: load new version: %w", pluginID, err)
	}

	if newPlugin.ID() != pluginID {
		l.mu.Lock()
		delete(l.plugins, pluginID)
		l.mu.Unlock()
		l.hostAPI.RevokePermissions(pluginID)
	}

	if err := oldCompiled.Close(ctx); err != nil {
		slog.Error("wasm: error closing old compiled module during reload", "plugin", pluginID, "error", err)
	}

	slog.Info("wasm: plugin reloaded", "id", pluginID, "new_version", newPlugin.Version())
	return nil
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

func (l *Loader) UpdatePermissions(pluginID string, permissions []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if lp, ok := l.plugins[pluginID]; ok {
		lp.perms = permissions
	}
}

func (l *Loader) Close(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	var firstErr error
	for id, lp := range l.plugins {
		if err := lp.compiled.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		l.hostAPI.RevokePermissions(id)
	}
	l.plugins = make(map[string]*loadedPlugin)
	if l.metrics != nil {
		l.metrics.LoadedPluginsGauge.Set(0)
	}
	return firstErr
}
