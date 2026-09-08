package email

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// HandleSendTask returns a Gin handler that delivers an email from a Cloud Tasks invocation.
func HandleSendTask(sender Sender, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var msg Message
		if err := c.ShouldBindJSON(&msg); err != nil {
			logger.Error("invalid email task payload", "error", err)
			c.Status(http.StatusOK) // don't retry bad payloads
			return
		}

		retryCount, _ := strconv.Atoi(c.GetHeader("X-CloudTasks-TaskRetryCount"))

		if err := sender.Send(c.Request.Context(), msg); err != nil {
			var sgErr *SendGridError
			if errors.As(err, &sgErr) && sgErr.IsTransient() {
				logger.Warn("transient email send failure",
					"to", msg.To,
					"subject", msg.Subject,
					"retry_count", retryCount,
					"error", err,
				)
				if retryCount >= 4 {
					logger.Error("email_dead_letter",
						"to", msg.To,
						"subject", msg.Subject,
						"retry_count", retryCount,
						"error", err,
					)
					c.Status(http.StatusOK) // stop retrying
					return
				}
				c.Status(http.StatusServiceUnavailable) // trigger retry
				return
			}

			// Non-transient error (4xx from SendGrid, etc.) — log and don't retry
			if retryCount >= 4 {
				logger.Error("email_dead_letter",
					"to", msg.To,
					"subject", msg.Subject,
					"retry_count", retryCount,
					"error", err,
				)
			} else {
				logger.Error("email send failed",
					"to", msg.To,
					"subject", msg.Subject,
					"retry_count", retryCount,
					"error", err,
				)
			}
			c.Status(http.StatusOK) // don't retry
			return
		}

		logger.Info("email task delivered", "to", msg.To, "subject", msg.Subject)
		c.Status(http.StatusOK)
	}
}
