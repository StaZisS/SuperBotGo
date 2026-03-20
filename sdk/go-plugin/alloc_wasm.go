//go:build wasip1

package wasmplugin

import "unsafe"

// ---------------------------------------------------------------------------
// Bump allocator — exported so the host can write into module memory.
// ---------------------------------------------------------------------------

var heapBuf [65536]byte
var heapOffset uint32

//go:wasmexport alloc
func alloc(size uint32) uint32 {
	// Align to 4 bytes.
	aligned := (heapOffset + 3) & ^uint32(3)
	if aligned+size > uint32(len(heapBuf)) {
		// Reset allocator when out of space.
		// Safe because the host copies data immediately after alloc.
		aligned = 0
		heapOffset = 0
	}
	ptr := aligned
	heapOffset = aligned + size
	return uint32(uintptr(unsafe.Pointer(&heapBuf[ptr])))
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
