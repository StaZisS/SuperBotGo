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

var pluginDrainTimeout = wasmrt.PluginDrainTimeout

type Loader struct {
	mu              sync.RWMutex
	rt              *wasmrt.Runtime
	hostAPI         *hostapi.HostAPI
	send            SendFunc
	localizedSend   LocalizedSendFunc
	messageSend     MessageSendFunc
	plugins         map[string]*loadedPlugin
	triggerRegistry *trigger.Registry
	metrics         *metrics.Metrics
	registry        *registry.PluginRegistry
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

func (l *Loader) SetLocalizedSend(fn LocalizedSendFunc)    { l.localizedSend = fn }
func (l *Loader) SetMessageSend(fn MessageSendFunc)        { l.messageSend = fn }
func (l *Loader) SetMetrics(m *metrics.Metrics)            { l.metrics = m }
func (l *Loader) SetTriggerRegistry(tr *trigger.Registry)  { l.triggerRegistry = tr }
func (l *Loader) SetRegistry(reg *registry.PluginRegistry) { l.registry = reg }
func (l *Loader) Registry() *registry.PluginRegistry       { return l.registry }

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

	if err := l.checkDependencies(ctx, compiled, &meta, wasmBytes); err != nil {
		return nil, err
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

	l.registerDatabases(meta.ID, config)

	// Validate that all declared database requirements are fulfilled.
	for _, req := range meta.Requirements {
		if req.Type != "database" {
			continue
		}
		dbName := req.Name
		if dbName == "" {
			dbName = "default"
		}
		if sqlStore := l.hostAPI.SQLStore(); sqlStore == nil || !sqlStore.HasDSN(meta.ID, dbName) {
			_ = compiled.Close(ctx)
			l.hostAPI.RevokePermissions(meta.ID)
			return nil, fmt.Errorf("plugin %q requires database %q but its connection string is not configured", meta.ID, dbName)
		}
	}

	// Run plugin SQL migrations against the "default" database before configure.
	if len(meta.Migrations) > 0 {
		if sqlStore := l.hostAPI.SQLStore(); sqlStore != nil {
			if dsn := sqlStore.DSN(meta.ID, "default"); dsn != "" {
				if err := runPluginMigrations(ctx, meta.ID, dsn, meta.Migrations); err != nil {
					_ = compiled.Close(ctx)
					l.hostAPI.RevokePermissions(meta.ID)
					return nil, fmt.Errorf("plugin %q migrations: %w", meta.ID, err)
				}
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
		messageSend:   l.messageSend,
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

	l.registerInRegistry(&meta, wasmBytes)

	slog.Info("wasm: plugin loaded", "id", meta.ID, "name", meta.Name, "version", meta.Version)
	return wp, nil
}

// checkDependencies resolves plugin dependencies and verifies integrity if a registry is configured.
func (l *Loader) checkDependencies(ctx context.Context, compiled *wasmrt.CompiledModule, meta *wasmrt.PluginMeta, wasmBytes []byte) error {
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

		tempEntry := registry.PluginEntry{
			ID:           meta.ID,
			Name:         meta.Name,
			Dependencies: convertDependencies(meta.Dependencies),
			Versions:     []registry.VersionEntry{{Version: meta.Version}},
		}
		l.registry.Register(tempEntry)

		if err := registry.ResolveDependencies(l.registry, meta.ID, meta.Version, installedPlugins); err != nil {
			_ = compiled.Close(ctx)
			return fmt.Errorf("plugin %q dependency check failed: %w", meta.ID, err)
		}
	}

	if l.registry != nil {
		if ve, err := l.registry.GetVersion(meta.ID, meta.Version); err == nil && ve.WasmHash != "" {
			if verifyErr := registry.VerifyOrError(wasmBytes, ve.WasmHash); verifyErr != nil {
				_ = compiled.Close(ctx)
				return fmt.Errorf("plugin %q: %w", meta.ID, verifyErr)
			}
			slog.Debug("wasm: integrity check passed", "plugin", meta.ID, "version", meta.Version)
		}
	}
	return nil
}

// registerDatabases reads the "databases" map from plugin config and registers
// each named DSN with the SQL store.
func (l *Loader) registerDatabases(pluginID string, config json.RawMessage) {
	sqlStore := l.hostAPI.SQLStore()
	if sqlStore == nil || len(config) == 0 {
		return
	}
	var cfgMap map[string]any
	if json.Unmarshal(config, &cfgMap) != nil {
		return
	}
	dbs, ok := cfgMap["databases"].(map[string]any)
	if !ok {
		return
	}
	for name, v := range dbs {
		if dsn, ok := v.(string); ok && dsn != "" {
			sqlStore.RegisterDSN(pluginID, name, dsn)
		}
	}
}

func (l *Loader) registerInRegistry(meta *wasmrt.PluginMeta, wasmBytes []byte) {
	if l.registry == nil {
		return
	}
	hash := registry.SignModule(wasmBytes)
	l.registry.Register(registry.PluginEntry{
		ID:           meta.ID,
		Name:         meta.Name,
		Dependencies: convertDependencies(meta.Dependencies),
		Signature:    hash,
		Versions: []registry.VersionEntry{{
			Version:       meta.Version,
			WasmHash:      hash,
			UploadedAt:    time.Now(),
			MinSDKVersion: meta.SDKVersion,
		}},
	})
}

func convertDependencies(deps []wasmrt.DependencyDef) []registry.Dependency {
	result := make([]registry.Dependency, len(deps))
	for i, d := range deps {
		result[i] = registry.Dependency{
			PluginID:          d.PluginID,
			VersionConstraint: d.VersionConstraint,
		}
	}
	return result
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
