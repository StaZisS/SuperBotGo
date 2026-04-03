package trigger

import (
	"encoding/hex"
	"regexp"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests for generateID
// ---------------------------------------------------------------------------

func TestGenerateID_Format(t *testing.T) {
	id := generateID()

	// UUID v4 format: 8-4-4-4-12 hex characters.
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	if !uuidPattern.MatchString(id) {
		t.Errorf("generateID() = %q, does not match UUID v4 format", id)
	}
}

func TestGenerateID_VersionNibble(t *testing.T) {
	id := generateID()

	// The version nibble is the high nibble of the 7th byte (0-indexed byte 6).
	// In the UUID string "xxxxxxxx-xxxx-Vxxx-xxxx-xxxxxxxxxxxx", V is at
	// position 14 (the first char of the third group).
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Fatalf("expected 5 UUID parts, got %d", len(parts))
	}

	// Third group: "Vxxx" — first char is the version nibble.
	versionChar := parts[2][0]
	if versionChar != '4' {
		t.Errorf("expected version nibble '4', got %q in UUID %q", string(versionChar), id)
	}
}

func TestGenerateID_VariantBits(t *testing.T) {
	id := generateID()

	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Fatalf("expected 5 UUID parts, got %d", len(parts))
	}

	// Fourth group: "Nxxx" — the high bits of byte 8 (first byte of this group)
	// must be 10xx in binary, meaning the hex digit is 8, 9, a, or b.
	variantHex := parts[3][0]
	validVariants := "89ab"
	if !strings.ContainsRune(validVariants, rune(variantHex)) {
		t.Errorf("expected variant nibble in [8,9,a,b], got %q in UUID %q", string(variantHex), id)
	}
}

func TestGenerateID_Uniqueness(t *testing.T) {
	const iterations = 100
	seen := make(map[string]bool, iterations)

	for range iterations {
		id := generateID()
		if seen[id] {
			t.Fatalf("generateID() produced duplicate ID: %q", id)
		}
		seen[id] = true
	}
}

func TestGenerateID_ConsistentLength(t *testing.T) {
	// UUID v4 string length: 8+1+4+1+4+1+4+1+12 = 36 characters.
	const expectedLen = 36

	for range 50 {
		id := generateID()
		if len(id) != expectedLen {
			t.Errorf("generateID() length = %d, want %d (id=%q)", len(id), expectedLen, id)
		}
	}
}

func TestGenerateID_VersionAndVariant_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		checkVersion    bool
		checkVariant    bool
		wantVersionByte byte
		validVariants   []byte
	}{
		{
			name:            "version byte has high nibble 0x40",
			checkVersion:    true,
			wantVersionByte: 0x40,
		},
		{
			name:          "variant byte has high bits 10xx",
			checkVariant:  true,
			validVariants: []byte{0x80, 0x90, 0xa0, 0xb0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate multiple IDs to exercise randomness.
			for range 20 {
				id := generateID()
				raw := strings.ReplaceAll(id, "-", "")
				b, err := hex.DecodeString(raw)
				if err != nil {
					t.Fatalf("failed to decode UUID hex %q: %v", raw, err)
				}
				if len(b) != 16 {
					t.Fatalf("expected 16 bytes, got %d", len(b))
				}

				if tt.checkVersion {
					// Byte 6: high nibble should be 0x4.
					got := b[6] & 0xf0
					if got != tt.wantVersionByte {
						t.Errorf("byte[6] high nibble = 0x%02x, want 0x%02x (id=%q)", got, tt.wantVersionByte, id)
					}
				}

				if tt.checkVariant {
					// Byte 8: high 2 bits should be 10 (i.e., 0x80..0xBF).
					got := b[8] & 0xc0
					if got != 0x80 {
						t.Errorf("byte[8] variant bits = 0x%02x, want 0x80 (10xxxxxx) (id=%q)", got, id)
					}
				}
			}
		})
	}
}

func TestGenerateID_TwoCallsProduceDifferentIDs(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == id2 {
		t.Errorf("two consecutive generateID() calls returned the same value: %q", id1)
	}
}

func TestGenerateID_AllHexCharacters(t *testing.T) {
	id := generateID()
	raw := strings.ReplaceAll(id, "-", "")

	for i, ch := range raw {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("non-hex character %q at position %d in UUID %q", string(ch), i, id)
		}
	}
}
