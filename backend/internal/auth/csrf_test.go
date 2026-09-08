package auth

import (
	"testing"
)

func TestGenerateCSRFToken(t *testing.T) {
	token1, err := GenerateCSRFToken()
	if err != nil {
		t.Fatalf("GenerateCSRFToken failed: %v", err)
	}
	if token1 == "" {
		t.Fatal("expected non-empty token")
	}

	token2, err := GenerateCSRFToken()
	if err != nil {
		t.Fatalf("GenerateCSRFToken failed: %v", err)
	}

	if token1 == token2 {
		t.Error("expected unique tokens")
	}
}

func TestGenerateCSRFTokenLength(t *testing.T) {
	token, err := GenerateCSRFToken()
	if err != nil {
		t.Fatalf("GenerateCSRFToken failed: %v", err)
	}
	// SHA-256 HMAC = 32 bytes = 64 hex chars
	if len(token) != 64 {
		t.Errorf("expected token length 64, got %d", len(token))
	}
}
