package hostapi

import (
	"context"
	"log/slog"

	"github.com/tetratelabs/wazero/api"
)

// returnError sets the host call status to "error" and writes a msgpack error result.
func returnError(ctx context.Context, mod api.Module, stack []uint64, err error) {
	SetHostCallStatus(ctx, "error")
	slog.Error("wasm host function error", "error", err)
	resp := map[string]string{"error": err.Error()}
	writeResult(ctx, mod, stack, resp)
}

func writeResult(ctx context.Context, mod api.Module, stack []uint64, v interface{}) {
	data, err := marshalWire(v)
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
