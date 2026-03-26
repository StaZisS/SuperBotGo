package hostapi

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/tetratelabs/wazero/api"
)

// returnError sets the host call status to "error" and writes a JSON error result.
func returnError(ctx context.Context, mod api.Module, stack []uint64, err error) {
	SetHostCallStatus(ctx, "error")
	writeErrorResult(ctx, mod, stack, err)
}

// returnErrorEnc sets the host call status to "error" and writes an encoded error result.
func returnErrorEnc(ctx context.Context, mod api.Module, stack []uint64, err error, enc EncodingType) {
	SetHostCallStatus(ctx, "error")
	writeErrorResultWithEnc(ctx, mod, stack, err, enc)
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
