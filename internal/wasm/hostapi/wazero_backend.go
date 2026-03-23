package hostapi

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type WazeroBackend struct {
	hostAPI  *HostAPI
	provider *HostFunctionProvider
}

func NewWazeroBackend(hostAPI *HostAPI, provider *HostFunctionProvider) *WazeroBackend {
	return &WazeroBackend{
		hostAPI:  hostAPI,
		provider: provider,
	}
}

func (b *WazeroBackend) Register(builder wazero.HostModuleBuilder) wazero.HostModuleBuilder {
	for _, fn := range b.provider.Functions() {
		goModFunc := b.wrapHandler(fn)
		builder = b.hostAPI.registerFunc(
			builder,
			fn.Name,
			goModFunc,
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI64},
		)
	}
	return builder
}

func (b *WazeroBackend) wrapHandler(fn HostFunction) api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)

		offset := uint32(stack[0])
		length := uint32(stack[1])

		for _, perm := range fn.Permissions {
			if err := b.hostAPI.perms.CheckPermission(pluginID, perm); err != nil {
				SetHostCallStatus(ctx, "error")
				writeErrorResult(ctx, mod, stack, err)
				return
			}
		}

		data, err := readModMemory(mod, offset, length)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		response, err := fn.Handler(ctx, pluginID, data)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			errResp, _ := json.Marshal(map[string]string{"error": err.Error()})
			slog.Error("wasm host function error", "function", fn.Name, "error", err)
			off, ln, wErr := writeModMemory(ctx, mod, errResp)
			if wErr != nil {
				slog.Error("wasm: write error result to memory failed", "error", wErr)
				stack[0] = 0
				return
			}
			stack[0] = uint64(off)<<32 | uint64(ln)
			return
		}

		if response == nil {
			stack[0] = 0
			return
		}

		off, ln, wErr := writeModMemory(ctx, mod, response)
		if wErr != nil {
			SetHostCallStatus(ctx, "error")
			slog.Error("wasm: write result to memory failed", "error", wErr)
			stack[0] = 0
			return
		}
		stack[0] = uint64(off)<<32 | uint64(ln)
	}
}
