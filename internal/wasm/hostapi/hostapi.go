package hostapi

import (
	"context"

	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type HostAPI struct {
	deps  Dependencies
	perms *permissionStore
}

func NewHostAPI(deps Dependencies) *HostAPI {
	return &HostAPI{
		deps:  deps,
		perms: newPermissionStore(),
	}
}

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

func (h *HostAPI) GrantPermissions(pluginID string, permissions []string) {
	h.perms.Grant(pluginID, permissions)
}

func (h *HostAPI) RevokePermissions(pluginID string) {
	h.perms.Revoke(pluginID)
}

func (h *HostAPI) ForPlugin(pluginID string, permissions []string) {
	h.perms.Grant(pluginID, permissions)
}

func (h *HostAPI) GetGrantedPermissions(pluginID string) []string {
	return h.perms.List(pluginID)
}

func pluginIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(wasmrt.PluginIDKey{}).(string); ok {
		return id
	}
	return "_unknown"
}

func (h *HostAPI) registerFunc(builder wazero.HostModuleBuilder, name string, fn api.GoModuleFunc, params, results []api.ValueType) wazero.HostModuleBuilder {
	return builder.NewFunctionBuilder().
		WithGoModuleFunction(fn, params, results).
		WithName(name).
		Export(name)
}
