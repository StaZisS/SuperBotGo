package api

import "testing"

func TestGenerateTemporaryPassword(t *testing.T) {
	first, err := generateTemporaryPassword()
	if err != nil {
		t.Fatalf("generateTemporaryPassword() error = %v", err)
	}
	if len(first) < 20 {
		t.Fatalf("temporary password length = %d, want at least 20", len(first))
	}

	second, err := generateTemporaryPassword()
	if err != nil {
		t.Fatalf("generateTemporaryPassword() second error = %v", err)
	}
	if first == second {
		t.Fatal("two generated temporary passwords are equal")
	}
}
