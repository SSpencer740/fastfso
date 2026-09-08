package errorreport

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimiterStore struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
}

func newRateLimiterStore() *rateLimiterStore {
	s := &rateLimiterStore{
		limiters: make(map[string]*ipLimiter),
	}
	go s.cleanup()
	return s
}

func (s *rateLimiterStore) get(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, ok := s.limiters[ip]
	if !ok {
		// 10 requests per minute, burst of 5
		l = &ipLimiter{
			limiter: rate.NewLimiter(rate.Every(time.Minute/10), 5),
		}
		s.limiters[ip] = l
	}
	l.lastSeen = time.Now()
	return l.limiter
}

func (s *rateLimiterStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		for ip, l := range s.limiters {
			if time.Since(l.lastSeen) > 5*time.Minute {
				delete(s.limiters, ip)
			}
		}
		s.mu.Unlock()
	}
}

// RateLimitMiddleware returns Gin middleware that rate-limits requests by client IP.
// This is intentionally in-memory rather than reusing auth.RateLimiter (which is
// database-backed via the audit log). A DB query per error report would add
// latency to a fire-and-forget endpoint, and per-instance approximation is
// sufficient here — we're only preventing spam, not enforcing security-critical
// limits like login attempt throttling.
func RateLimitMiddleware() gin.HandlerFunc {
	store := newRateLimiterStore()
	return func(c *gin.Context) {
		limiter := store.get(c.ClientIP())
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
