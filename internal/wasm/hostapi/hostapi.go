package hostapi

import (
	"context"

	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// HostAPI aggregates all host functions and provides them to Wasm plugins
// based on their granted permissions.
type HostAPI struct {
	deps  Dependencies
	perms *permissionStore
}

// NewHostAPI creates a new HostAPI with the given dependencies.
func NewHostAPI(deps Dependencies) *HostAPI {
	return &HostAPI{
		deps:  deps,
		perms: newPermissionStore(),
	}
}

// RegisterHostModule registers the "env" host module once on the runtime.
// Must be called at startup, before any plugins are loaded.
func (h *HostAPI) RegisterHostModule(ctx context.Context, rt *wasmrt.Runtime) error {
	builder := rt.Engine().NewHostModuleBuilder("env")

	builder = h.registerFunc(builder, "db_query", h.dbQueryFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "db_save", h.dbSaveFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "http_request", h.httpRequestFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "call_plugin", h.callPluginFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "publish_event", h.publishEventFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	_, err := builder.Instantiate(ctx)
	return err
}

// GrantPermissions registers the given permissions for a plugin.
func (h *HostAPI) GrantPermissions(pluginID string, permissions []string) {
	h.perms.Grant(pluginID, permissions)
}

// RevokePermissions removes all permissions for a plugin.
func (h *HostAPI) RevokePermissions(pluginID string) {
	h.perms.Revoke(pluginID)
}

// ForPlugin grants permissions for a plugin.
func (h *HostAPI) ForPlugin(pluginID string, permissions []string) {
	h.perms.Grant(pluginID, permissions)
}

// pluginIDFromContext extracts the plugin ID from the context.
// In the one-shot model, CompiledModule.RunAction sets this before instantiation.
func pluginIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(wasmrt.PluginIDKey{}).(string); ok {
		return id
	}
	return "_unknown"
}

// registerFunc is a helper that adds a single function to the host module builder.
func (h *HostAPI) registerFunc(builder wazero.HostModuleBuilder, name string, fn api.GoModuleFunc, params, results []api.ValueType) wazero.HostModuleBuilder {
	return builder.NewFunctionBuilder().
		WithGoModuleFunction(fn, params, results).
		WithName(name).
		Export(name)
}
