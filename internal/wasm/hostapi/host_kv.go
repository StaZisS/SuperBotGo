package hostapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tetratelabs/wazero/api"
)

type kvGetRequest struct {
	Key string `json:"key"`
}

type kvGetResponse struct {
	Value *string `json:"value"`
	Found bool    `json:"found"`
}

func (h *HostAPI) kvGetFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "kv:read"); err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		data, err := readModMemory(mod, offset, length)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		var req kvGetRequest
		if err := json.Unmarshal(data, &req); err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		if h.kvStore == nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, errDepNotAvailable("KVStore"))
			return
		}

		value, found, err := h.kvStore.Get(pluginID, req.Key)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		resp := kvGetResponse{Found: found}
		if found {
			resp.Value = &value
		}
		writeJSONResult(ctx, mod, stack, resp)
	}
}

type kvSetRequest struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	TTLSeconds *int   `json:"ttl_seconds,omitempty"`
}

type kvSetResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (h *HostAPI) kvSetFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "kv:write"); err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		data, err := readModMemory(mod, offset, length)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		var req kvSetRequest
		if err := json.Unmarshal(data, &req); err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		if h.kvStore == nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, errDepNotAvailable("KVStore"))
			return
		}

		var ttl time.Duration
		if req.TTLSeconds != nil && *req.TTLSeconds > 0 {
			ttl = time.Duration(*req.TTLSeconds) * time.Second
		}

		if err := h.kvStore.Set(pluginID, req.Key, req.Value, ttl); err != nil {
			SetHostCallStatus(ctx, "error")
			writeJSONResult(ctx, mod, stack, kvSetResponse{OK: false, Error: err.Error()})
			return
		}

		writeJSONResult(ctx, mod, stack, kvSetResponse{OK: true})
	}
}

type kvDeleteRequest struct {
	Key string `json:"key"`
}

type kvDeleteResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (h *HostAPI) kvDeleteFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "kv:write"); err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		data, err := readModMemory(mod, offset, length)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		var req kvDeleteRequest
		if err := json.Unmarshal(data, &req); err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		if h.kvStore == nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, errDepNotAvailable("KVStore"))
			return
		}

		if err := h.kvStore.Delete(pluginID, req.Key); err != nil {
			SetHostCallStatus(ctx, "error")
			writeJSONResult(ctx, mod, stack, kvDeleteResponse{OK: false, Error: err.Error()})
			return
		}

		writeJSONResult(ctx, mod, stack, kvDeleteResponse{OK: true})
	}
}

type kvListRequest struct {
	Prefix string `json:"prefix,omitempty"`
}

type kvListResponse struct {
	Keys  []string `json:"keys"`
	Error string   `json:"error,omitempty"`
}

func (h *HostAPI) kvListFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "kv:read"); err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		data, err := readModMemory(mod, offset, length)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		var req kvListRequest
		if err := json.Unmarshal(data, &req); err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		if h.kvStore == nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, errDepNotAvailable("KVStore"))
			return
		}

		keys, err := h.kvStore.List(pluginID, req.Prefix)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeJSONResult(ctx, mod, stack, kvListResponse{Error: err.Error()})
			return
		}
		if keys == nil {
			keys = []string{}
		}

		writeJSONResult(ctx, mod, stack, kvListResponse{Keys: keys})
	}
}
