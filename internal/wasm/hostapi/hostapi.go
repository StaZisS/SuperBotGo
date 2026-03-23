package hostapi

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"SuperBotGo/internal/metrics"
	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type HostAPI struct {
	deps       Dependencies
	perms      *permissionStore
	metrics    *metrics.Metrics
	kvStore    *KVStore
	rateLimits map[string]int
	provider   *HostFunctionProvider
}

func NewHostAPI(deps Dependencies) *HostAPI {
	return &HostAPI{
		deps:    deps,
		perms:   newPermissionStore(),
		kvStore: NewKVStore(),
	}
}

func (h *HostAPI) KVStore() *KVStore {
	return h.kvStore
}

func (h *HostAPI) SetRateLimits(limits map[string]int) {
	h.rateLimits = limits
}

func (h *HostAPI) NewRateLimiterForPlugin(pluginID string) *RateLimiter {
	return NewRateLimiter(pluginID, h.rateLimits)
}

func (h *HostAPI) ContextWithRateLimiter(ctx context.Context, pluginID string) context.Context {
	rl := h.NewRateLimiterForPlugin(pluginID)
	return context.WithValue(ctx, rateLimiterKey{}, rl)
}

func (h *HostAPI) SetMetrics(m *metrics.Metrics) {
	h.metrics = m
}

func (h *HostAPI) SetEventBus(eb EventBus) {
	h.deps.Events = eb
}

func (h *HostAPI) Provider() *HostFunctionProvider {
	return h.provider
}

func (h *HostAPI) RegisterHostModule(ctx context.Context, rt *wasmrt.Runtime) error {
	rt.AddContextHook(func(ctx context.Context, pluginID string) context.Context {
		ctx = h.ContextWithRateLimiter(ctx, pluginID)
		ctx = ContextWithTraceID(ctx, GenerateTraceID())
		return ctx
	})

	h.provider = h.buildProvider()

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

	builder = h.registerFunc(builder, "kv_get", h.kvGetFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "kv_set", h.kvSetFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "kv_delete", h.kvDeleteFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "kv_list", h.kvListFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	_, err := builder.Instantiate(ctx)
	return err
}

func (h *HostAPI) buildProvider() *HostFunctionProvider {
	p := NewHostFunctionProvider()

	p.Register(HostFunction{
		Name:        "db_query",
		Description: "Query the plugin-scoped database.",
		Permissions: []string{"db:read"},
		Handler:     h.noopHandler(),
	})
	p.Register(HostFunction{
		Name:        "db_save",
		Description: "Save a record to the plugin-scoped database.",
		Permissions: []string{"db:write"},
		Handler:     h.noopHandler(),
	})

	p.Register(HostFunction{
		Name:        "http_request",
		Description: "Make an outbound HTTP request.",
		Permissions: []string{},
		Handler:     h.noopHandler(),
	})

	p.Register(HostFunction{
		Name:        "call_plugin",
		Description: "Call a method on another plugin.",
		Permissions: []string{},
		Handler:     h.noopHandler(),
	})
	p.Register(HostFunction{
		Name:        "publish_event",
		Description: "Publish an event to the event bus.",
		Permissions: []string{"plugins:events"},
		Handler:     h.noopHandler(),
	})

	p.Register(HostFunction{
		Name:        "kv_get",
		Description: "Get a value from the plugin-scoped key-value store.",
		Permissions: []string{"kv:read"},
		Handler:     h.noopHandler(),
	})
	p.Register(HostFunction{
		Name:        "kv_set",
		Description: "Set a value in the plugin-scoped key-value store.",
		Permissions: []string{"kv:write"},
		Handler:     h.noopHandler(),
	})
	p.Register(HostFunction{
		Name:        "kv_delete",
		Description: "Delete a key from the plugin-scoped key-value store.",
		Permissions: []string{"kv:write"},
		Handler:     h.noopHandler(),
	})
	p.Register(HostFunction{
		Name:        "kv_list",
		Description: "List keys in the plugin-scoped key-value store.",
		Permissions: []string{"kv:read"},
		Handler:     h.noopHandler(),
	})

	return p
}

func (h *HostAPI) noopHandler() HostFunctionHandler {
	return func(ctx context.Context, pluginID string, request []byte) ([]byte, error) {
		return nil, fmt.Errorf("this handler is a placeholder; the wazero-specific binding handles the call")
	}
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

type hostCallStatus struct{ status string }

type hostCallStatusKey struct{}

func withHostCallStatus(ctx context.Context) (context.Context, *hostCallStatus) {
	s := &hostCallStatus{status: "ok"}
	return context.WithValue(ctx, hostCallStatusKey{}, s), s
}

func SetHostCallStatus(ctx context.Context, status string) {
	if s, ok := ctx.Value(hostCallStatusKey{}).(*hostCallStatus); ok {
		s.status = status
	}
}

func (h *HostAPI) registerFunc(builder wazero.HostModuleBuilder, name string, fn api.GoModuleFunc, params, results []api.ValueType) wazero.HostModuleBuilder {
	wrapped := func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		traceID := TraceIDFromContext(ctx)
		callChain := callChainFromContext(ctx)
		start := time.Now()

		ctx, callStatus := withHostCallStatus(ctx)

		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered in host function",
					"trace_id", traceID,
					"plugin_id", pluginID,
					"function", name,
					"panic", fmt.Sprintf("%v", r),
				)
				callStatus.status = "error"
				func() {
					defer func() {
						if r2 := recover(); r2 != nil {
							slog.Error("panic in recovery handler, returning zero result",
								"trace_id", traceID,
								"plugin_id", pluginID,
								"function", name,
								"panic", fmt.Sprintf("%v", r2),
							)
							stack[0] = 0
						}
					}()
					writeErrorResult(ctx, mod, stack,
						fmt.Errorf("host function %q panicked: %v", name, r))
				}()
			}

			dur := time.Since(start)
			status := callStatus.status

			if h.metrics != nil {
				h.metrics.PluginHostCallTotal.WithLabelValues(pluginID, name, status).Inc()
				h.metrics.PluginHostCallDuration.WithLabelValues(pluginID, name).Observe(dur.Seconds())
				h.metrics.HostAPITotal.WithLabelValues(pluginID, name, status).Inc()
				h.metrics.HostAPIDuration.WithLabelValues(pluginID, name).Observe(dur.Seconds())
			}

			logAttrs := []any{
				"trace_id", traceID,
				"plugin_id", pluginID,
				"function", name,
				"duration_ms", dur.Milliseconds(),
				"status", status,
			}
			if len(callChain) > 0 {
				logAttrs = append(logAttrs, "call_chain", strings.Join(callChain, " -> "))
			}
			slog.Info("host api call", logAttrs...)
		}()

		if rl, ok := ctx.Value(rateLimiterKey{}).(*RateLimiter); ok {
			if err := rl.Allow(name); err != nil {
				callStatus.status = "rate_limited"
				slog.Warn("host function rate limit hit",
					"trace_id", traceID,
					"plugin_id", pluginID,
					"function", name,
				)
				writeErrorResult(ctx, mod, stack, err)
				return
			}
		}

		fn(ctx, mod, stack)
	}

	return builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(wrapped), params, results).
		WithName(name).
		Export(name)
}
