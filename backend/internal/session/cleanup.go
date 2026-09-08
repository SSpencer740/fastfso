package session

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CleanupHandler returns a Gin handler that cleans up expired sessions.
// Additional cleanup funcs (e.g. webauthn sessions) run after the session cleanup.
// Designed to be called by Cloud Scheduler or similar cron trigger.
func CleanupHandler(store *Store, logger *slog.Logger, extra ...func(context.Context) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		revoked, err := store.CleanupExpired(c.Request.Context())
		if err != nil {
			logger.Error("session cleanup failed", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cleanup failed"})
			return
		}
		logger.Info("session cleanup complete", "revoked", revoked)

		for _, fn := range extra {
			if err := fn(c.Request.Context()); err != nil {
				logger.Error("extra cleanup failed", "error", err)
			}
		}

		c.JSON(http.StatusOK, gin.H{"revoked": revoked})
	}
}
