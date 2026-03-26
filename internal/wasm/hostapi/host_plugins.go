package hostapi

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"SuperBotGo/internal/wasm/eventbus"
	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/tetratelabs/wazero/api"
	"github.com/vmihailenco/msgpack/v5"
)

var wasmCallPluginMaxTimeout = time.Duration(wasmrt.DefaultHostCallPluginTimeoutSeconds) * time.Second

var wasmPublishEventMaxTimeout = time.Duration(wasmrt.DefaultHostEventTimeoutSeconds) * time.Second

const MaxCallDepth = 5

type callDepthKey struct{}

type callChainKey struct{}

func callDepthFromContext(ctx context.Context) int {
	if depth, ok := ctx.Value(callDepthKey{}).(int); ok {
		return depth
	}
	return 0
}

func callChainFromContext(ctx context.Context) []string {
	if chain, ok := ctx.Value(callChainKey{}).([]string); ok {
		return chain
	}
	return nil
}

func withCallDepth(ctx context.Context, target string) context.Context {
	depth := callDepthFromContext(ctx)
	chain := callChainFromContext(ctx)

	newChain := make([]string, len(chain), len(chain)+1)
	copy(newChain, chain)
	newChain = append(newChain, target)

	ctx = context.WithValue(ctx, callDepthKey{}, depth+1)
	ctx = context.WithValue(ctx, callChainKey{}, newChain)
	return ctx
}

func checkCallCycle(chain []string, target string) error {
	for _, id := range chain {
		if id == target {
			cycle := make([]string, len(chain), len(chain)+1)
			copy(cycle, chain)
			cycle = append(cycle, target)
			return fmt.Errorf("circular inter-plugin call detected: %s", strings.Join(cycle, " -> "))
		}
	}
	return nil
}

type callPluginPayload struct {
	Target string `msgpack:"target"`
	Method string `msgpack:"method"`
	Params []byte `msgpack:"params,omitempty"`
}

type publishEventPayload struct {
	Topic   string `msgpack:"topic"`
	Payload []byte `msgpack:"payload"`
}

func (h *HostAPI) callPluginFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		traceID := TraceIDFromContext(ctx)

		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var payload callPluginPayload
		if err := msgpack.Unmarshal(data, &payload); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		perm := fmt.Sprintf("plugins:call:%s", payload.Target)
		if err := h.perms.CheckPermission(pluginID, perm); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.PluginRegistry == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("PluginRegistry"))
			return
		}

		chain := callChainFromContext(ctx)
		if len(chain) == 0 {
			chain = []string{pluginID}
			ctx = context.WithValue(ctx, callChainKey{}, chain)
		}

		if err := checkCallCycle(chain, payload.Target); err != nil {
			slog.Warn("inter-plugin cycle blocked",
				"trace_id", traceID,
				"caller", pluginID,
				"target", payload.Target,
				"chain", chain,
			)
			returnError(ctx, mod, stack, err)
			return
		}

		depth := callDepthFromContext(ctx)
		if depth >= MaxCallDepth {
			slog.Warn("inter-plugin call depth exceeded",
				"trace_id", traceID,
				"caller", pluginID,
				"target", payload.Target,
				"depth", depth,
			)
			returnError(ctx, mod, stack, fmt.Errorf("inter-plugin call depth exceeded (max %d)", MaxCallDepth))
			return
		}

		slog.Info("inter-plugin call",
			"trace_id", traceID,
			"caller", pluginID,
			"target", payload.Target,
			"method", payload.Method,
			"call_chain", strings.Join(append(chain, payload.Target), " -> "),
		)

		depthCtx := withCallDepth(ctx, payload.Target)

		callCtx, cancel := context.WithTimeout(depthCtx, contextAwareTimeout(ctx, wasmCallPluginMaxTimeout))
		defer cancel()

		result, err := h.deps.PluginRegistry.CallPlugin(callCtx, payload.Target, payload.Method, payload.Params)
		if err != nil {
			if callCtx.Err() == context.DeadlineExceeded {
				err = fmt.Errorf("call_plugin %q.%s timed out after %s", payload.Target, payload.Method, wasmCallPluginMaxTimeout)
			} else {
				err = fmt.Errorf("call_plugin %q.%s: %w", payload.Target, payload.Method, err)
			}
			returnError(ctx, mod, stack, err)
			return
		}

		offset2, length2, err := writeModMemory(ctx, mod, result)
		if err != nil {
			returnError(ctx, mod, stack, err)
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

		if err := h.perms.CheckPermission(pluginID, "events"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var payload publishEventPayload
		if err := msgpack.Unmarshal(data, &payload); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.Events == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("EventBus"))
			return
		}

		pubCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmPublishEventMaxTimeout))
		defer cancel()

		pubCtx = eventbus.ContextWithPluginID(pubCtx, pluginID)

		if err := h.deps.Events.Publish(pubCtx, payload.Topic, payload.Payload); err != nil {
			if pubCtx.Err() == context.DeadlineExceeded {
				err = fmt.Errorf("publish_event %q timed out after %s", payload.Topic, wasmPublishEventMaxTimeout)
			} else {
				err = fmt.Errorf("publish event %q: %w", payload.Topic, err)
			}
			returnError(ctx, mod, stack, err)
			return
		}

		stack[0] = 0
	}
}
