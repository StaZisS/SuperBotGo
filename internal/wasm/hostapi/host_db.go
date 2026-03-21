package hostapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/tetratelabs/wazero/api"
)

func (h *HostAPI) dbQueryFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		start := time.Now()
		status := "ok"
		defer func() {
			dur := time.Since(start)
			if h.metrics != nil {
				h.metrics.HostAPIDuration.WithLabelValues(pluginID, "db_query").Observe(dur.Seconds())
				h.metrics.HostAPITotal.WithLabelValues(pluginID, "db_query", status).Inc()
			}
			slog.Info("host api call", "plugin_id", pluginID, "function", "db_query", "duration_ms", dur.Milliseconds(), "status", status)
		}()

		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "db:read"); err != nil {
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

		var query map[string]interface{}
		if err := json.Unmarshal(data, &query); err != nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		if h.deps.DB == nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, errDepNotAvailable("DB"))
			return
		}

		results, err := h.deps.DB.Query(ctx, pluginID, query)
		if err != nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		writeJSONResult(ctx, mod, stack, results)
	}
}

func (h *HostAPI) dbSaveFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		start := time.Now()
		status := "ok"
		defer func() {
			dur := time.Since(start)
			if h.metrics != nil {
				h.metrics.HostAPIDuration.WithLabelValues(pluginID, "db_save").Observe(dur.Seconds())
				h.metrics.HostAPITotal.WithLabelValues(pluginID, "db_save", status).Inc()
			}
			slog.Info("host api call", "plugin_id", pluginID, "function", "db_save", "duration_ms", dur.Milliseconds(), "status", status)
		}()

		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "db:write"); err != nil {
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

		var record map[string]interface{}
		if err := json.Unmarshal(data, &record); err != nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		if h.deps.DB == nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, errDepNotAvailable("DB"))
			return
		}

		if err := h.deps.DB.Save(ctx, pluginID, record); err != nil {
			status = "error"
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		stack[0] = 0
	}
}

func writeErrorResult(ctx context.Context, mod api.Module, stack []uint64, err error) {
	slog.Error("wasm host function error", "error", err)
	resp := map[string]string{"error": err.Error()}
	writeJSONResult(ctx, mod, stack, resp)
}

func writeJSONResult(ctx context.Context, mod api.Module, stack []uint64, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("wasm: marshal result failed", "error", err)
		stack[0] = 0
		return
	}
	offset, length, err := writeModMemory(ctx, mod, data)
	if err != nil {
		slog.Error("wasm: write result to memory failed", "error", err)
		stack[0] = 0
		return
	}
	stack[0] = uint64(offset)<<32 | uint64(length)
}
