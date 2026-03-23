package hostapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"SuperBotGo/internal/wasm/eventbus"
	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/tetratelabs/wazero/api"
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
	Target string          `json:"target" msgpack:"target"`
	Method string          `json:"method" msgpack:"method"`
	Params json.RawMessage `json:"params,omitempty" msgpack:"params,omitempty"`
}

type publishEventPayload struct {
	Topic   string          `json:"topic" msgpack:"topic"`
	Payload json.RawMessage `json:"payload" msgpack:"payload"`
}

func (h *HostAPI) callPluginFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		traceID := TraceIDFromContext(ctx)

		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		var payload callPluginPayload
		if err := unmarshalPayload(data, enc, &payload); err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResultWithEnc(ctx, mod, stack, err, enc)
			return
		}

		perm := fmt.Sprintf("plugins:call:%s", payload.Target)
		if err := h.perms.CheckPermission(pluginID, perm); err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResultWithEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.deps.PluginRegistry == nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResultWithEnc(ctx, mod, stack, errDepNotAvailable("PluginRegistry"), enc)
			return
		}

		chain := callChainFromContext(ctx)
		if len(chain) == 0 {
			chain = []string{pluginID}
			ctx = context.WithValue(ctx, callChainKey{}, chain)
		}

		if err := checkCallCycle(chain, payload.Target); err != nil {
			SetHostCallStatus(ctx, "error")
			slog.Warn("inter-plugin cycle blocked",
				"trace_id", traceID,
				"caller", pluginID,
				"target", payload.Target,
				"chain", chain,
			)
			writeErrorResultWithEnc(ctx, mod, stack, err, enc)
			return
		}

		depth := callDepthFromContext(ctx)
		if depth >= MaxCallDepth {
			SetHostCallStatus(ctx, "error")
			slog.Warn("inter-plugin call depth exceeded",
				"trace_id", traceID,
				"caller", pluginID,
				"target", payload.Target,
				"depth", depth,
			)
			writeErrorResultWithEnc(ctx, mod, stack, fmt.Errorf("inter-plugin call depth exceeded (max %d)", MaxCallDepth), enc)
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
			SetHostCallStatus(ctx, "error")
			if callCtx.Err() == context.DeadlineExceeded {
				writeErrorResultWithEnc(ctx, mod, stack, fmt.Errorf("call_plugin %q.%s timed out after %s", payload.Target, payload.Method, wasmCallPluginMaxTimeout), enc)
			} else {
				writeErrorResultWithEnc(ctx, mod, stack, fmt.Errorf("call_plugin %q.%s: %w", payload.Target, payload.Method, err), enc)
			}
			return
		}

		offset2, length2, err := writeModMemory(ctx, mod, result)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResultWithEnc(ctx, mod, stack, err, enc)
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
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		var payload publishEventPayload
		if err := unmarshalPayload(data, enc, &payload); err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResultWithEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.deps.Events == nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResultWithEnc(ctx, mod, stack, errDepNotAvailable("EventBus"), enc)
			return
		}

		pubCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmPublishEventMaxTimeout))
		defer cancel()

		pubCtx = eventbus.ContextWithPluginID(pubCtx, pluginID)

		if err := h.deps.Events.Publish(pubCtx, payload.Topic, payload.Payload); err != nil {
			SetHostCallStatus(ctx, "error")
			if pubCtx.Err() == context.DeadlineExceeded {
				writeErrorResultWithEnc(ctx, mod, stack, fmt.Errorf("publish_event %q timed out after %s", payload.Topic, wasmPublishEventMaxTimeout), enc)
			} else {
				writeErrorResultWithEnc(ctx, mod, stack, fmt.Errorf("publish event %q: %w", payload.Topic, err), enc)
			}
			return
		}

		stack[0] = 0
	}
}
