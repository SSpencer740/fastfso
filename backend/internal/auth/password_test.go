package auth

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("mypassword123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == "mypassword123" {
		t.Fatal("hash should not equal plaintext")
	}
}

func TestCheckPassword(t *testing.T) {
	hash, err := HashPassword("correctpassword")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if !CheckPassword(hash, "correctpassword") {
		t.Error("expected correct password to match")
	}

	if CheckPassword(hash, "wrongpassword") {
		t.Error("expected wrong password to not match")
	}
}

func TestCheckPasswordEmpty(t *testing.T) {
	hash, err := HashPassword("")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if !CheckPassword(hash, "") {
		t.Error("expected empty password to match its hash")
	}

	if CheckPassword(hash, "notempty") {
		t.Error("expected non-empty password to not match empty hash")
	}
}
