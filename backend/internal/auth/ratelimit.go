package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/telemetry"
	"github.com/gin-gonic/gin"
)

type rateLimitCounter interface {
	CountRecentByIP(ctx context.Context, ip string, actions []string, since time.Time) (int, error)
}

// RateLimiter provides per-key rate limiting backed by the audit log.
// This works correctly across multiple Cloud Run instances since state
// is stored in PostgreSQL rather than in-memory.
type RateLimiter struct {
	counter rateLimitCounter
	max     int
	window  time.Duration
}

func NewRateLimiter(counter rateLimitCounter, max int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		counter: counter,
		max:     max,
		window:  window,
	}
}

// Allow checks if a request is within the rate limit for the given key.
// It counts recent audit log entries for login attempts from this IP.
func (rl *RateLimiter) Allow(ctx context.Context, key string) bool {
	if rl.counter == nil {
		return true
	}
	actions := []string{audit.ActionLoginAttempt, audit.ActionLoginFailure}
	count, err := rl.counter.CountRecentByIP(ctx, key, actions, time.Now().Add(-rl.window))
	if err != nil {
		// If we can't check, allow the request (fail open)
		return true
	}
	return count < rl.max
}

// Middleware returns a Gin middleware that rate limits by client IP.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.Allow(c.Request.Context(), c.ClientIP()) {
			telemetry.RecordRateLimitBlocked(c.Request.Context())
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests, please try again later",
			})
			return
		}
		c.Next()
	}
}
