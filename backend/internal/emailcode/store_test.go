package emailcode

import (
	"testing"
)

func TestGenerateCode(t *testing.T) {
	code, err := generateCode()
	if err != nil {
		t.Fatalf("generateCode failed: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("expected 6-digit code, got %q (len %d)", code, len(code))
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			t.Errorf("code contains non-digit character: %c", c)
		}
	}
}

func TestGenerateCodeUniqueness(t *testing.T) {
	codes := make(map[string]bool)
	for range 100 {
		code, err := generateCode()
		if err != nil {
			t.Fatalf("generateCode failed: %v", err)
		}
		codes[code] = true
	}
	// With 6-digit codes and 100 generations, we should have at least 50 unique values
	if len(codes) < 50 {
		t.Errorf("expected more unique codes, got %d out of 100", len(codes))
	}
}

func TestHashCode(t *testing.T) {
	hash1 := hashCode("123456")
	hash2 := hashCode("123456")
	hash3 := hashCode("654321")

	if hash1 != hash2 {
		t.Error("same input should produce same hash")
	}
	if hash1 == hash3 {
		t.Error("different input should produce different hash")
	}
	if len(hash1) != 64 {
		t.Errorf("expected SHA-256 hex hash length 64, got %d", len(hash1))
	}
}
