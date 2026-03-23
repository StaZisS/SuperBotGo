package hostapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/tetratelabs/wazero/api"
)

var wasmDBMaxTimeout = time.Duration(wasmrt.DefaultHostDBTimeoutSeconds) * time.Second

func (h *HostAPI) dbQueryFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)

		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "db:read"); err != nil {
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

		var query map[string]interface{}
		if err := unmarshalPayload(data, enc, &query); err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResultWithEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.deps.DB == nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResultWithEnc(ctx, mod, stack, errDepNotAvailable("DB"), enc)
			return
		}

		dbCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmDBMaxTimeout))
		defer cancel()

		results, err := h.deps.DB.Query(dbCtx, pluginID, query)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			if dbCtx.Err() == context.DeadlineExceeded {
				writeErrorResultWithEnc(ctx, mod, stack, fmt.Errorf("db_query timed out after %s", wasmDBMaxTimeout), enc)
			} else {
				writeErrorResultWithEnc(ctx, mod, stack, err, enc)
			}
			return
		}

		writeEncodedResult(ctx, mod, stack, results, enc)
	}
}

func (h *HostAPI) dbSaveFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)

		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "db:write"); err != nil {
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

		var record map[string]interface{}
		if err := unmarshalPayload(data, enc, &record); err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResultWithEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.deps.DB == nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResultWithEnc(ctx, mod, stack, errDepNotAvailable("DB"), enc)
			return
		}

		dbCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmDBMaxTimeout))
		defer cancel()

		if err := h.deps.DB.Save(dbCtx, pluginID, record); err != nil {
			SetHostCallStatus(ctx, "error")
			if dbCtx.Err() == context.DeadlineExceeded {
				writeErrorResultWithEnc(ctx, mod, stack, fmt.Errorf("db_save timed out after %s", wasmDBMaxTimeout), enc)
			} else {
				writeErrorResultWithEnc(ctx, mod, stack, err, enc)
			}
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

func writeErrorResultWithEnc(ctx context.Context, mod api.Module, stack []uint64, err error, enc EncodingType) {
	slog.Error("wasm host function error", "error", err)
	resp := map[string]string{"error": err.Error()}
	writeEncodedResult(ctx, mod, stack, resp, enc)
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

func writeEncodedResult(ctx context.Context, mod api.Module, stack []uint64, v interface{}, enc EncodingType) {
	if enc == EncodingJSON {
		writeJSONResult(ctx, mod, stack, v)
		return
	}

	codec, err := CodecFor(enc)
	if err != nil {
		slog.Error("wasm: unknown encoding, falling back to JSON", "encoding", enc, "error", err)
		writeJSONResult(ctx, mod, stack, v)
		return
	}

	data, err := prefixedEncode(codec, v)
	if err != nil {
		slog.Error("wasm: marshal result failed", "encoding", enc, "error", err)
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
