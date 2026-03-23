package hostapi

import (
	"context"
	"fmt"
	"time"

	"github.com/tetratelabs/wazero/api"
)

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

func readModMemoryAndDetect(mod api.Module, offset, length uint32) ([]byte, EncodingType, error) {
	raw, err := readModMemory(mod, offset, length)
	if err != nil {
		return nil, EncodingJSON, err
	}
	enc, payload := detectEncoding(raw)
	return payload, enc, nil
}

func unmarshalPayload(data []byte, enc EncodingType, v any) error {
	codec, err := CodecFor(enc)
	if err != nil {
		return err
	}
	return codec.Unmarshal(data, v)
}

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
	if offset == 0 {
		return 0, 0, fmt.Errorf("alloc(%d): plugin arena exhausted (returned null)", length)
	}
	mem := mod.Memory()
	if mem == nil {
		return 0, 0, fmt.Errorf("module has no memory")
	}
	if !mem.Write(offset, data) {
		return 0, 0, fmt.Errorf("memory write out of bounds: offset=%d, length=%d", offset, length)
	}
	return offset, length, nil
}

func errDepNotAvailable(name string) error {
	return fmt.Errorf("dependency %q is not available", name)
}

func contextAwareTimeout(ctx context.Context, maxTimeout time.Duration) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return maxTimeout
	}
	remaining := time.Until(deadline)
	if remaining < maxTimeout {
		return remaining
	}
	return maxTimeout
}
