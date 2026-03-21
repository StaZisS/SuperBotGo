package hostapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/tetratelabs/wazero/api"
)

type callPluginPayload struct {
	Target string          `json:"target"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type publishEventPayload struct {
	Topic   string          `json:"topic"`
	Payload json.RawMessage `json:"payload"`
}

func (h *HostAPI) callPluginFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		start := time.Now()
		status := "ok"
		defer func() {
			dur := time.Since(start)
			if h.metrics != nil {
				h.metrics.HostAPIDuration.WithLabelValues(pluginID, "call_plugin").Observe(dur.Seconds())
				h.metrics.HostAPITotal.WithLabelValues(pluginID, "call_plugin", status).Inc()
			}
			slog.Info("host api call", "plugin_id", pluginID, "function", "call_plugin", "duration_ms", dur.Milliseconds(), "status", status)
		}()

		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readModMemory(mod, offset, length)
		if err != nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		var payload callPluginPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		perm := fmt.Sprintf("plugins:call:%s", payload.Target)
		if err := h.perms.CheckPermission(pluginID, perm); err != nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		if h.deps.PluginRegistry == nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, errDepNotAvailable("PluginRegistry"))
			return
		}

		result, err := h.deps.PluginRegistry.CallPlugin(ctx, payload.Target, payload.Method, payload.Params)
		if err != nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, fmt.Errorf("call_plugin %q.%s: %w", payload.Target, payload.Method, err))
			return
		}

		offset2, length2, err := writeModMemory(ctx, mod, result)
		if err != nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, err)
			return
		}
		stack[0] = uint64(offset2)<<32 | uint64(length2)
	}
}

func (h *HostAPI) publishEventFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		start := time.Now()
		status := "ok"
		defer func() {
			dur := time.Since(start)
			if h.metrics != nil {
				h.metrics.HostAPIDuration.WithLabelValues(pluginID, "publish_event").Observe(dur.Seconds())
				h.metrics.HostAPITotal.WithLabelValues(pluginID, "publish_event", status).Inc()
			}
			slog.Info("host api call", "plugin_id", pluginID, "function", "publish_event", "duration_ms", dur.Milliseconds(), "status", status)
		}()

		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "plugins:events"); err != nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		data, err := readModMemory(mod, offset, length)
		if err != nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		var payload publishEventPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		if h.deps.Events == nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, errDepNotAvailable("EventBus"))
			return
		}

		if err := h.deps.Events.Publish(ctx, payload.Topic, payload.Payload); err != nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, fmt.Errorf("publish event %q: %w", payload.Topic, err))
			return
		}

		stack[0] = 0
	}
}
