package hostapi

import (
	"context"
	"time"

	"github.com/tetratelabs/wazero/api"
	"github.com/vmihailenco/msgpack/v5"
)

// kvPreamble extracts common fields, checks permissions, reads and unmarshals the payload,
// and verifies that kvStore is available. Returns nil on failure (error already written to stack).
func kvPreamble[T any](h *HostAPI, ctx context.Context, mod api.Module, stack []uint64) (string, *T) {
	pluginID := pluginIDFromContext(ctx)
	offset := uint32(stack[0])
	length := uint32(stack[1])

	if err := h.perms.CheckPermission(pluginID, "kv"); err != nil {
		returnError(ctx, mod, stack, err)
		return "", nil
	}

	data, err := readPayload(mod, offset, length)
	if err != nil {
		returnError(ctx, mod, stack, err)
		return "", nil
	}

	var req T
	if err := msgpack.Unmarshal(data, &req); err != nil {
		returnError(ctx, mod, stack, err)
		return "", nil
	}

	if h.kvStore == nil {
		returnError(ctx, mod, stack, errDepNotAvailable("KVStore"))
		return "", nil
	}

	return pluginID, &req
}

type kvGetRequest struct {
	Key string `msgpack:"key"`
}

type kvGetResponse struct {
	Value *string `msgpack:"value"`
	Found bool    `msgpack:"found"`
}

func (h *HostAPI) kvGetFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID, req := kvPreamble[kvGetRequest](h, ctx, mod, stack)
		if req == nil {
			return
		}

		value, found, err := h.kvStore.Get(pluginID, req.Key)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		resp := kvGetResponse{Found: found}
		if found {
			resp.Value = &value
		}
		writeResult(ctx, mod, stack, resp)
	}
}

type kvSetRequest struct {
	Key        string `msgpack:"key"`
	Value      string `msgpack:"value"`
	TTLSeconds *int   `msgpack:"ttl_seconds,omitempty"`
}

type kvSetResponse struct {
	OK    bool   `msgpack:"ok"`
	Error string `msgpack:"error,omitempty"`
}

func (h *HostAPI) kvSetFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID, req := kvPreamble[kvSetRequest](h, ctx, mod, stack)
		if req == nil {
			return
		}

		var ttl time.Duration
		if req.TTLSeconds != nil && *req.TTLSeconds > 0 {
			ttl = time.Duration(*req.TTLSeconds) * time.Second
		}

		if err := h.kvStore.Set(pluginID, req.Key, req.Value, ttl); err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, kvSetResponse{OK: false, Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, kvSetResponse{OK: true})
	}
}

type kvDeleteRequest struct {
	Key string `msgpack:"key"`
}

type kvDeleteResponse struct {
	OK    bool   `msgpack:"ok"`
	Error string `msgpack:"error,omitempty"`
}

func (h *HostAPI) kvDeleteFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID, req := kvPreamble[kvDeleteRequest](h, ctx, mod, stack)
		if req == nil {
			return
		}

		if err := h.kvStore.Delete(pluginID, req.Key); err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, kvDeleteResponse{OK: false, Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, kvDeleteResponse{OK: true})
	}
}

type kvListRequest struct {
	Prefix string `msgpack:"prefix,omitempty"`
}

type kvListResponse struct {
	Keys  []string `msgpack:"keys"`
	Error string   `msgpack:"error,omitempty"`
}

func (h *HostAPI) kvListFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID, req := kvPreamble[kvListRequest](h, ctx, mod, stack)
		if req == nil {
			return
		}

		keys, err := h.kvStore.List(pluginID, req.Prefix)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, kvListResponse{Error: err.Error()})
			return
		}
		if keys == nil {
			keys = []string{}
		}

		writeResult(ctx, mod, stack, kvListResponse{Keys: keys})
	}
}
