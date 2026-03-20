package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"SuperBotGo/internal/wasm/hostapi"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

// Loader manages the lifecycle of Wasm plugins.
type Loader struct {
	mu      sync.RWMutex
	rt      *wasmrt.Runtime
	hostAPI *hostapi.HostAPI
	reply   ReplyFunc
	send    SendFunc
	plugins map[string]*loadedPlugin
}

type loadedPlugin struct {
	plugin   *WasmPlugin
	compiled *wasmrt.CompiledModule
	config   json.RawMessage
	perms    []string
}

// NewLoader creates a new Wasm plugin loader.
func NewLoader(rt *wasmrt.Runtime, hostAPI *hostapi.HostAPI, reply ReplyFunc, send SendFunc) *Loader {
	return &Loader{
		rt:      rt,
		hostAPI: hostAPI,
		reply:   reply,
		send:    send,
		plugins: make(map[string]*loadedPlugin),
	}
}

// LoadPlugin compiles and loads a Wasm plugin from the given file path.
// It returns a WasmPlugin that implements the Plugin interface.
//
// Flow: read file -> LoadPluginFromBytes.
func (l *Loader) LoadPlugin(ctx context.Context, wasmPath string, config json.RawMessage, permissions []string) (*WasmPlugin, error) {
	data, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("read wasm file %q: %w", wasmPath, err)
	}
	return l.LoadPluginFromBytes(ctx, data, config, permissions)
}

// LoadPluginFromBytes compiles and loads a Wasm plugin from raw bytes.
// It returns a WasmPlugin that implements the Plugin interface.
//
// Flow: compile -> CallMeta -> CallConfigure -> create WasmPlugin with CompiledModule.
func (l *Loader) LoadPluginFromBytes(ctx context.Context, wasmBytes []byte, config json.RawMessage, permissions []string) (*WasmPlugin, error) {

	compiled, err := l.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile wasm plugin: %w", err)
	}

	// Grant temporary permissions so host functions work during meta call.
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
		reply:    l.reply,
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

	slog.Info("wasm: plugin loaded", "id", meta.ID, "name", meta.Name, "version", meta.Version)
	return wp, nil
}

// UnloadPlugin stops and removes a loaded plugin.
func (l *Loader) UnloadPlugin(ctx context.Context, pluginID string) error {
	l.mu.Lock()
	lp, ok := l.plugins[pluginID]
	if !ok {
		l.mu.Unlock()
		return fmt.Errorf("plugin %q not loaded", pluginID)
	}
	delete(l.plugins, pluginID)
	l.mu.Unlock()

	l.hostAPI.RevokePermissions(pluginID)

	if err := lp.compiled.Close(ctx); err != nil {
		slog.Error("wasm: error closing compiled module", "plugin", pluginID, "error", err)
	}

	slog.Info("wasm: plugin unloaded", "id", pluginID)
	return nil
}

// ReloadPlugin loads a new version of a plugin from a file path and gracefully switches traffic.
func (l *Loader) ReloadPlugin(ctx context.Context, pluginID string, newWasmPath string, newConfig json.RawMessage) error {
	data, err := os.ReadFile(newWasmPath)
	if err != nil {
		return fmt.Errorf("read wasm file %q: %w", newWasmPath, err)
	}
	return l.ReloadPluginFromBytes(ctx, pluginID, data, newConfig)
}

// ReloadPluginFromBytes loads a new version of a plugin from raw bytes and gracefully switches traffic.
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

// GetPlugin returns a loaded Wasm plugin by ID.
func (l *Loader) GetPlugin(pluginID string) (*WasmPlugin, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	lp, ok := l.plugins[pluginID]
	if !ok {
		return nil, false
	}
	return lp.plugin, true
}

// AllPlugins returns all loaded Wasm plugins.
func (l *Loader) AllPlugins() []*WasmPlugin {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]*WasmPlugin, 0, len(l.plugins))
	for _, lp := range l.plugins {
		result = append(result, lp.plugin)
	}
	return result
}

// Close stops all loaded plugins.
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
	return firstErr
}
