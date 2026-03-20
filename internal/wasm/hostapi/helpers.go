package hostapi

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// readModMemory reads bytes from a wasm module's memory.
func readModMemory(mod api.Module, offset, length uint32) ([]byte, error) {
	if length == 0 {
		return nil, nil
	}
	mem := mod.Memory()
	if mem == nil {
		return nil, fmt.Errorf("module has no memory")
	}
	data, ok := mem.Read(offset, length)
	if !ok {
		return nil, fmt.Errorf("memory read out of bounds: offset=%d, length=%d", offset, length)
	}
	result := make([]byte, length)
	copy(result, data)
	return result, nil
}

// writeModMemory allocates memory in the module and writes data to it.
func writeModMemory(ctx context.Context, mod api.Module, data []byte) (uint32, uint32, error) {
	length := uint32(len(data))
	if length == 0 {
		return 0, 0, nil
	}
	alloc := mod.ExportedFunction("alloc")
	if alloc == nil {
		return 0, 0, fmt.Errorf("module does not export 'alloc'")
	}
	results, err := alloc.Call(ctx, uint64(length))
	if err != nil {
		return 0, 0, fmt.Errorf("alloc(%d): %w", length, err)
	}
	offset := uint32(results[0])
	mem := mod.Memory()
	if mem == nil {
		return 0, 0, fmt.Errorf("module has no memory")
	}
	if !mem.Write(offset, data) {
		return 0, 0, fmt.Errorf("memory write out of bounds: offset=%d, length=%d", offset, length)
	}
	return offset, length, nil
}

// errDepNotAvailable returns an error indicating a dependency is not configured.
func errDepNotAvailable(name string) error {
	return fmt.Errorf("dependency %q is not available", name)
}
