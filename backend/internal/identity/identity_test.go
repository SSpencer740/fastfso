package identity

import (
	"testing"
)

func TestHasPassword(t *testing.T) {
	tests := []struct {
		name     string
		hash     *string
		expected bool
	}{
		{"nil hash", nil, false},
		{"empty hash", ptrStr(""), false},
		{"valid hash", ptrStr("$2a$12$somehash"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ident := Identity{PasswordHash: tt.hash}
			if got := ident.HasPassword(); got != tt.expected {
				t.Errorf("HasPassword() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func ptrStr(s string) *string {
	return &s
}
