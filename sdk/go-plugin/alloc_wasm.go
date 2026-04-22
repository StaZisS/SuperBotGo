//go:build wasip1

package wasmplugin

import "unsafe"

const initialArenaSize = 64 * 1024 // grows on demand up to the module memory limit

var heapBuf []byte
var heapOffset uint32

//go:wasmexport alloc
func alloc(size uint32) uint32 {
	if size == 0 {
		return 0
	}
	aligned := (heapOffset + 7) & ^uint32(7)
	end := aligned + size
	if end < aligned {
		return 0
	}

	need := int(end)
	if cap(heapBuf) < need {
		newCap := cap(heapBuf)
		if newCap < initialArenaSize {
			newCap = initialArenaSize
		}
		for newCap < need {
			newCap *= 2
		}
		newBuf := make([]byte, need, newCap)
		copy(newBuf, heapBuf)
		heapBuf = newBuf
	} else if len(heapBuf) < need {
		heapBuf = heapBuf[:need]
	}

	heapOffset = end
	return uint32(uintptr(unsafe.Pointer(&heapBuf[aligned])))
}

//go:wasmexport alloc_reset
func allocReset() {
	heapBuf = heapBuf[:0]
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
