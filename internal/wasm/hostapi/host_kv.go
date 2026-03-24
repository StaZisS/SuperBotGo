package hostapi

import (
	"context"
	"time"

	"github.com/tetratelabs/wazero/api"
)

type kvGetRequest struct {
	Key string `json:"key" msgpack:"key"`
}

type kvGetResponse struct {
	Value *string `json:"value" msgpack:"value"`
	Found bool    `json:"found" msgpack:"found"`
}

func (h *HostAPI) kvGetFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "kv:read"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req kvGetRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.kvStore == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("KVStore"), enc)
			return
		}

		value, found, err := h.kvStore.Get(pluginID, req.Key)
		if err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		resp := kvGetResponse{Found: found}
		if found {
			resp.Value = &value
		}
		writeEncodedResult(ctx, mod, stack, resp, enc)
	}
}

type kvSetRequest struct {
	Key        string `json:"key" msgpack:"key"`
	Value      string `json:"value" msgpack:"value"`
	TTLSeconds *int   `json:"ttl_seconds,omitempty" msgpack:"ttl_seconds,omitempty"`
}

type kvSetResponse struct {
	OK    bool   `json:"ok" msgpack:"ok"`
	Error string `json:"error,omitempty" msgpack:"error,omitempty"`
}

func (h *HostAPI) kvSetFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "kv:write"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req kvSetRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.kvStore == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("KVStore"), enc)
			return
		}

		var ttl time.Duration
		if req.TTLSeconds != nil && *req.TTLSeconds > 0 {
			ttl = time.Duration(*req.TTLSeconds) * time.Second
		}

		if err := h.kvStore.Set(pluginID, req.Key, req.Value, ttl); err != nil {
			SetHostCallStatus(ctx, "error")
			writeEncodedResult(ctx, mod, stack, kvSetResponse{OK: false, Error: err.Error()}, enc)
			return
		}

		writeEncodedResult(ctx, mod, stack, kvSetResponse{OK: true}, enc)
	}
}

type kvDeleteRequest struct {
	Key string `json:"key" msgpack:"key"`
}

type kvDeleteResponse struct {
	OK    bool   `json:"ok" msgpack:"ok"`
	Error string `json:"error,omitempty" msgpack:"error,omitempty"`
}

func (h *HostAPI) kvDeleteFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "kv:write"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req kvDeleteRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.kvStore == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("KVStore"), enc)
			return
		}

		if err := h.kvStore.Delete(pluginID, req.Key); err != nil {
			SetHostCallStatus(ctx, "error")
			writeEncodedResult(ctx, mod, stack, kvDeleteResponse{OK: false, Error: err.Error()}, enc)
			return
		}

		writeEncodedResult(ctx, mod, stack, kvDeleteResponse{OK: true}, enc)
	}
}

type kvListRequest struct {
	Prefix string `json:"prefix,omitempty" msgpack:"prefix,omitempty"`
}

type kvListResponse struct {
	Keys  []string `json:"keys" msgpack:"keys"`
	Error string   `json:"error,omitempty" msgpack:"error,omitempty"`
}

func (h *HostAPI) kvListFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "kv:read"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req kvListRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.kvStore == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("KVStore"), enc)
			return
		}

		keys, err := h.kvStore.List(pluginID, req.Prefix)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeEncodedResult(ctx, mod, stack, kvListResponse{Error: err.Error()}, enc)
			return
		}
		if keys == nil {
			keys = []string{}
		}

		writeEncodedResult(ctx, mod, stack, kvListResponse{Keys: keys}, enc)
	}
}
