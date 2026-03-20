package hostapi

import (
	"context"
	"encoding/json"
	"fmt"

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
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readModMemory(mod, offset, length)
		if err != nil {
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		var payload callPluginPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		perm := fmt.Sprintf("plugins:call:%s", payload.Target)
		if err := h.perms.CheckPermission(pluginID, perm); err != nil {
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		if h.deps.PluginRegistry == nil {
			writeErrorResult(ctx, mod, stack, errDepNotAvailable("PluginRegistry"))
			return
		}

		result, err := h.deps.PluginRegistry.CallPlugin(ctx, payload.Target, payload.Method, payload.Params)
		if err != nil {
			writeErrorResult(ctx, mod, stack, fmt.Errorf("call_plugin %q.%s: %w", payload.Target, payload.Method, err))
			return
		}

		offset2, length2, err := writeModMemory(ctx, mod, result)
		if err != nil {
			writeErrorResult(ctx, mod, stack, err)
			return
		}
		stack[0] = uint64(offset2)<<32 | uint64(length2)
	}
}

func (h *HostAPI) publishEventFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "plugins:events"); err != nil {
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		data, err := readModMemory(mod, offset, length)
		if err != nil {
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		var payload publishEventPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		if h.deps.Events == nil {
			writeErrorResult(ctx, mod, stack, errDepNotAvailable("EventBus"))
			return
		}

		if err := h.deps.Events.Publish(ctx, payload.Topic, payload.Payload); err != nil {
			writeErrorResult(ctx, mod, stack, fmt.Errorf("publish event %q: %w", payload.Topic, err))
			return
		}

		stack[0] = 0
	}
}
