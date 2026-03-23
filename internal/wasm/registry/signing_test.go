package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestSignModule(t *testing.T) {
	data := []byte("hello wasm module")
	hash := SignModule(data)

	expected := sha256.Sum256(data)
	want := hex.EncodeToString(expected[:])

	if hash != want {
		t.Errorf("SignModule returned %q, want %q", hash, want)
	}
}

func TestVerifyModule(t *testing.T) {
	data := []byte("hello wasm module")
	hash := SignModule(data)

	if !VerifyModule(data, hash) {
		t.Error("VerifyModule returned false for matching hash")
	}

	if VerifyModule(data, "0000000000000000000000000000000000000000000000000000000000000000") {
		t.Error("VerifyModule returned true for mismatched hash")
	}
}

func TestVerifyOrError(t *testing.T) {
	data := []byte("module bytes")
	hash := SignModule(data)

	if err := VerifyOrError(data, hash); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := VerifyOrError(data, "badhash")
	if err == nil {
		t.Fatal("expected error for bad hash")
	}
}

func TestSignModuleEmpty(t *testing.T) {
	hash := SignModule(nil)
	expected := sha256.Sum256(nil)
	want := hex.EncodeToString(expected[:])
	if hash != want {
		t.Errorf("SignModule(nil) = %q, want %q", hash, want)
	}
}
