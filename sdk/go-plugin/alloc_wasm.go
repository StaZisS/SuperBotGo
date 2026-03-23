//go:build wasip1

package wasmplugin

import "unsafe"

// ---------------------------------------------------------------------------
// Arena allocator — exported so the host can write into module memory.
//
// Each WASM execution is a fresh module instance, so the arena only needs to
// be safe within a single execution. The allocator:
//   - Uses a 256 KB heap (up from 64 KB).
//   - Aligns all allocations to 8 bytes.
//   - Returns 0 (null) when the heap is exhausted instead of silently
//     wrapping around — the host must check for a null return.
//   - Provides an explicit alloc_reset export that clears all allocations
//     (useful if instance pooling is ever implemented).
// ---------------------------------------------------------------------------

const heapSize = 262144 // 256 KB

var heapBuf [heapSize]byte
var heapOffset uint32

//go:wasmexport alloc
func alloc(size uint32) uint32 {
	if size == 0 {
		return 0
	}
	// Align to 8 bytes.
	aligned := (heapOffset + 7) & ^uint32(7)
	end := aligned + size
	if end > heapSize {
		// Heap exhausted — return null. The host must handle this.
		return 0
	}
	heapOffset = end
	return uint32(uintptr(unsafe.Pointer(&heapBuf[aligned])))
}

//go:wasmexport alloc_reset
func allocReset() {
	heapOffset = 0
}

// ptrToBytes returns a Go slice aliasing memory at (ptr, length).
func ptrToBytes(ptr uint32, length uint32) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length)
}

// bytesToPtr returns a pointer and length for a byte slice.
func bytesToPtr(b []byte) (uint32, uint32) {
	if len(b) == 0 {
		return 0, 0
	}
	return uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b))
}
