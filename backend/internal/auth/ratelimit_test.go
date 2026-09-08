package auth

import (
	"context"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(nil, 5, 15*time.Minute)
	if rl.max != 5 {
		t.Errorf("expected max 5, got %d", rl.max)
	}
	if rl.window != 15*time.Minute {
		t.Errorf("expected window 15m, got %v", rl.window)
	}
}

func TestRateLimiterAllowFailsOpen(t *testing.T) {
	// With nil counter, Allow should fail open (return true)
	rl := NewRateLimiter(nil, 1, time.Minute)
	if !rl.Allow(context.Background(), "127.0.0.1") {
		t.Error("expected Allow to fail open with nil counter")
	}
}
