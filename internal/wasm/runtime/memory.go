package runtime

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

func writeToMemory(ctx context.Context, mod api.Module, data []byte) (uint32, uint32, error) {
	length := uint32(len(data))
	if length == 0 {
		return 0, 0, nil
	}

	alloc := mod.ExportedFunction("alloc")
	if alloc == nil {
		return 0, 0, fmt.Errorf("wasm module does not export 'alloc' function")
	}

	results, err := alloc.Call(ctx, uint64(length))
	if err != nil {
		return 0, 0, fmt.Errorf("alloc(%d) failed: %w", length, err)
	}
	if len(results) == 0 {
		return 0, 0, fmt.Errorf("alloc(%d) returned no results", length)
	}
	offset := uint32(results[0])

	mem := mod.Memory()
	if mem == nil {
		return 0, 0, fmt.Errorf("wasm module has no memory")
	}

	if !mem.Write(offset, data) {
		return 0, 0, fmt.Errorf("memory write out of bounds at offset %d, length %d", offset, length)
	}

	return offset, length, nil
}

func readFromMemory(mod api.Module, offset, length uint32) ([]byte, error) {
	if length == 0 {
		return nil, nil
	}

	mem := mod.Memory()
	if mem == nil {
		return nil, fmt.Errorf("wasm module has no memory")
	}

	data, ok := mem.Read(offset, length)
	if !ok {
		return nil, fmt.Errorf("memory read out of bounds at offset %d, length %d", offset, length)
	}

	result := make([]byte, length)
	copy(result, data)
	return result, nil
}
