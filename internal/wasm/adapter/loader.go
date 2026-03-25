package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/trigger"
	"SuperBotGo/internal/wasm/hostapi"
	"SuperBotGo/internal/wasm/registry"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

const pluginDrainTimeout = 10 * time.Second

type Loader struct {
	mu              sync.RWMutex
	rt              *wasmrt.Runtime
	hostAPI         *hostapi.HostAPI
	send            SendFunc
	localizedSend   LocalizedSendFunc
	plugins         map[string]*loadedPlugin
	triggerRegistry *trigger.Registry
	metrics         *metrics.Metrics
	registry        *registry.PluginRegistry
}

func (l *Loader) SetLocalizedSend(fn LocalizedSendFunc) {
	l.localizedSend = fn
}

func (l *Loader) SetMetrics(m *metrics.Metrics) {
	l.metrics = m
}

type loadedPlugin struct {
	plugin         *WasmPlugin
	compiled       *wasmrt.CompiledModule
	config         json.RawMessage
	draining       atomic.Bool
	activeRequests atomic.Int64
	drained        chan struct{}
}

func NewLoader(rt *wasmrt.Runtime, hostAPI *hostapi.HostAPI, send SendFunc) *Loader {
	return &Loader{
		rt:      rt,
		hostAPI: hostAPI,
		send:    send,
		plugins: make(map[string]*loadedPlugin),
	}
}

func (l *Loader) LoadPlugin(ctx context.Context, wasmPath string, config json.RawMessage) (*WasmPlugin, error) {
	data, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("read wasm file %q: %w", wasmPath, err)
	}
	return l.LoadPluginFromBytes(ctx, data, config)
}

func (l *Loader) LoadPluginFromBytes(ctx context.Context, wasmBytes []byte, config json.RawMessage) (*WasmPlugin, error) {

	compiled, err := l.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile wasm plugin: %w", err)
	}

	const probeID = "_temp_probe"
	l.hostAPI.GrantPermissions(probeID, nil)
	compiled.ID = probeID

	meta, err := compiled.CallMeta(ctx)
	l.hostAPI.RevokePermissions(probeID)
	if err != nil {
		_ = compiled.Close(ctx)
		return nil, fmt.Errorf("call meta: %w", err)
	}

	// Derive internal permissions from declared requirements.
	permissions := hostapi.PermissionsFromRequirements(meta.Requirements)

	l.mu.RLock()
	if existing, ok := l.plugins[meta.ID]; ok {
		oldVer := existing.plugin.Version()
		newVer := meta.Version
		if oldVer == newVer {
			slog.Warn("wasm: plugin with the same version is already loaded",
				"id", meta.ID, "version", newVer)
		} else if registry.CompareVersions(newVer, oldVer) < 0 {
			slog.Warn("wasm: loading older plugin version than currently loaded",
				"id", meta.ID, "current_version", oldVer, "new_version", newVer)
		}
	}
	l.mu.RUnlock()

	if l.registry != nil && len(meta.Dependencies) > 0 {
		installedPlugins := make([]registry.InstalledPlugin, 0, len(l.plugins))
		l.mu.RLock()
		for id, lp := range l.plugins {
			installedPlugins = append(installedPlugins, registry.InstalledPlugin{
				ID:      id,
				Version: lp.plugin.Version(),
			})
		}
		l.mu.RUnlock()

		deps := make([]registry.Dependency, len(meta.Dependencies))
		for i, d := range meta.Dependencies {
			deps[i] = registry.Dependency{
				PluginID:          d.PluginID,
				VersionConstraint: d.VersionConstraint,
			}
		}
		tempEntry := registry.PluginEntry{
			ID:           meta.ID,
			Name:         meta.Name,
			Dependencies: deps,
			Versions:     []registry.VersionEntry{{Version: meta.Version}},
		}
		l.registry.Register(tempEntry)

		if err := registry.ResolveDependencies(l.registry, meta.ID, meta.Version, installedPlugins); err != nil {
			_ = compiled.Close(ctx)
			return nil, fmt.Errorf("plugin %q dependency check failed: %w", meta.ID, err)
		}
	}

	if l.registry != nil {
		if ve, err := l.registry.GetVersion(meta.ID, meta.Version); err == nil && ve.WasmHash != "" {
			if verifyErr := registry.VerifyOrError(wasmBytes, ve.WasmHash); verifyErr != nil {
				_ = compiled.Close(ctx)
				return nil, fmt.Errorf("plugin %q: %w", meta.ID, verifyErr)
			}
			slog.Debug("wasm: integrity check passed", "plugin", meta.ID, "version", meta.Version)
		}
	}

	l.hostAPI.GrantPermissions(meta.ID, permissions)
	compiled.ID = meta.ID
	compiled.Version = meta.Version

	if len(config) > 0 {
		if err := ValidateConfigAgainstSchema(meta.ConfigSchema, config); err != nil {
			_ = compiled.Close(ctx)
			l.hostAPI.RevokePermissions(meta.ID)
			return nil, fmt.Errorf("plugin %q: %w", meta.ID, err)
		}
	}

	// Extract SQL DSN from plugin config and register with SQLStore.
	var hasSQLDSN bool
	if sqlStore := l.hostAPI.SQLStore(); sqlStore != nil && len(config) > 0 {
		var cfgMap map[string]any
		if json.Unmarshal(config, &cfgMap) == nil {
			if dsn, ok := cfgMap["sql_dsn"].(string); ok && dsn != "" {
				sqlStore.RegisterDSN(meta.ID, dsn)
				hasSQLDSN = true
			}
		}
	}

	// Validate that all declared requirements are fulfilled.
	for _, req := range meta.Requirements {
		switch req.Type {
		case "database":
			if !hasSQLDSN {
				_ = compiled.Close(ctx)
				l.hostAPI.RevokePermissions(meta.ID)
				return nil, fmt.Errorf("plugin %q requires database access but sql_dsn is not configured", meta.ID)
			}
		}
	}

	if len(config) > 0 {
		if err := compiled.CallConfigure(ctx, config); err != nil {
			_ = compiled.Close(ctx)
			l.hostAPI.RevokePermissions(meta.ID)
			return nil, fmt.Errorf("configure plugin %q: %w", meta.ID, err)
		}
	}

	compiled.EnablePool(l.rt.Config().PoolConfig())
	slog.Info("wasm: module pool enabled",
		"plugin", meta.ID,
		"max_concurrency", compiled.Pool().Stats().PoolSize)

	wp := &WasmPlugin{
		compiled:      compiled,
		meta:          meta,
		config:        config,
		send:          l.send,
		localizedSend: l.localizedSend,
	}

	l.mu.Lock()
	l.plugins[meta.ID] = &loadedPlugin{
		plugin:   wp,
		compiled: compiled,
		config:   config,
		drained:  make(chan struct{}),
	}
	l.mu.Unlock()

	if l.metrics != nil {
		l.metrics.LoadedPluginsGauge.Inc()
	}

	if l.triggerRegistry != nil && len(meta.Triggers) > 0 {
		l.triggerRegistry.RegisterTriggers(meta.ID, meta.Triggers)
		slog.Info("wasm: registered triggers", "plugin", meta.ID, "count", len(meta.Triggers))
	}

	if l.registry != nil {
		deps := make([]registry.Dependency, len(meta.Dependencies))
		for i, d := range meta.Dependencies {
			deps[i] = registry.Dependency{
				PluginID:          d.PluginID,
				VersionConstraint: d.VersionConstraint,
			}
		}
		l.registry.Register(registry.PluginEntry{
			ID:           meta.ID,
			Name:         meta.Name,
			Dependencies: deps,
			Signature:    registry.SignModule(wasmBytes),
			Versions: []registry.VersionEntry{{
				Version:       meta.Version,
				WasmHash:      registry.SignModule(wasmBytes),
				UploadedAt:    time.Now(),
				MinSDKVersion: meta.SDKVersion,
			}},
		})
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

func (l *Loader) SetTriggerRegistry(tr *trigger.Registry) {
	l.triggerRegistry = tr
}

func (l *Loader) SetRegistry(reg *registry.PluginRegistry) {
	l.registry = reg
}

func (l *Loader) Registry() *registry.PluginRegistry {
	return l.registry
}

func (l *Loader) ReloadPlugin(ctx context.Context, pluginID string, newWasmPath string, newConfig json.RawMessage) error {
	data, err := os.ReadFile(newWasmPath)
	if err != nil {
		return fmt.Errorf("read wasm file %q: %w", newWasmPath, err)
	}
	return l.ReloadPluginFromBytes(ctx, pluginID, data, newConfig)
}

func (l *Loader) ReloadPluginFromBytes(ctx context.Context, pluginID string, wasmBytes []byte, newConfig json.RawMessage) error {
	start := time.Now()
	reloadStatus := "ok"
	defer func() {
		dur := time.Since(start)
		if l.metrics != nil {
			l.metrics.PluginReloadTotal.WithLabelValues(pluginID, reloadStatus).Inc()
			l.metrics.PluginReloadDuration.WithLabelValues(pluginID).Observe(dur.Seconds())
		}
		slog.Info("wasm: plugin reload",
			"plugin_id", pluginID,
			"status", reloadStatus,
			"duration_ms", dur.Milliseconds(),
		)
	}()

	l.mu.RLock()
	old, ok := l.plugins[pluginID]
	if !ok {
		l.mu.RUnlock()
		reloadStatus = "error"
		return fmt.Errorf("plugin %q not loaded", pluginID)
	}
	config := newConfig
	if config == nil {
		config = old.config
	}
	l.mu.RUnlock()

	old.draining.Store(true)

	oldVersion := old.plugin.Version()

	newPlugin, err := l.LoadPluginFromBytes(ctx, wasmBytes, config)
	if err != nil {
		old.draining.Store(false)
		reloadStatus = "error"
		return fmt.Errorf("reload plugin %q: load new version: %w", pluginID, err)
	}

	newVersion := newPlugin.Version()
	if oldVersion != newVersion {
		slog.Info("wasm: plugin version changed, running migration",
			"plugin", pluginID,
			"old_version", oldVersion,
			"new_version", newVersion,
		)
		if migrateErr := newPlugin.compiled.CallMigrate(ctx, oldVersion, newVersion); migrateErr != nil {
			slog.Error("wasm: plugin migration failed (proceeding with reload)",
				"plugin", pluginID,
				"old_version", oldVersion,
				"new_version", newVersion,
				"error", migrateErr,
			)
		} else {
			slog.Info("wasm: plugin migration completed successfully",
				"plugin", pluginID,
				"old_version", oldVersion,
				"new_version", newVersion,
			)
		}
	}

	if newPlugin.ID() != pluginID {
		l.mu.Lock()
		delete(l.plugins, pluginID)
		l.mu.Unlock()
		l.hostAPI.RevokePermissions(pluginID)
	}

	l.drainPlugin(old, pluginID)

	if err := old.compiled.Close(ctx); err != nil {
		slog.Error("wasm: error closing old compiled module during reload", "plugin", pluginID, "error", err)
	}

	slog.Info("wasm: plugin reloaded", "id", pluginID, "new_version", newPlugin.Version())
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

func (l *Loader) AllPlugins() []*WasmPlugin {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]*WasmPlugin, 0, len(l.plugins))
	for _, lp := range l.plugins {
		result = append(result, lp.plugin)
	}
	return result
}

func (l *Loader) ValidateConfig(pluginID string, config json.RawMessage) error {
	l.mu.RLock()
	lp, ok := l.plugins[pluginID]
	l.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin %q not loaded", pluginID)
	}
	return ValidateConfigAgainstSchema(lp.plugin.meta.ConfigSchema, config)
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
