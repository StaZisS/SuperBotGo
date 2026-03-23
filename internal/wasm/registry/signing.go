package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func SignModule(wasmBytes []byte) string {
	h := sha256.Sum256(wasmBytes)
	return hex.EncodeToString(h[:])
}

func VerifyModule(wasmBytes []byte, expectedHash string) bool {
	actual := SignModule(wasmBytes)
	return actual == expectedHash
}

func VerifyOrError(wasmBytes []byte, expectedHash string) error {
	if !VerifyModule(wasmBytes, expectedHash) {
		actual := SignModule(wasmBytes)
		return fmt.Errorf("wasm integrity check failed: expected hash %s, got %s", expectedHash, actual)
	}
	return nil
}
