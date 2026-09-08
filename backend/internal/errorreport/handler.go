package errorreport

import (
	"log/slog"
	"net/http"

	"github.com/fastfso/fastfso/backend/internal/env"
	"github.com/fastfso/fastfso/backend/internal/telemetry"
	"github.com/gin-gonic/gin"
)

type lastAPICall struct {
	Method       string `json:"method"`
	URL          string `json:"url"`
	Status       int    `json:"status"`
	ResponseBody string `json:"response_body"`
}

type reportRequest struct {
	Message       string       `json:"message"       binding:"required,max=2048"`
	Stack         *string      `json:"stack"          binding:"omitempty,max=8192"`
	URL           string       `json:"url"            binding:"required,max=2048"`
	Timestamp     string       `json:"timestamp"      binding:"required"`
	Source        string       `json:"source"         binding:"required,oneof=window.onerror unhandledrejection error_boundary manual"`
	IdentityID    *string      `json:"identity_id"    binding:"omitempty,max=36"`
	UserID        *string      `json:"user_id"        binding:"omitempty,max=36"`
	TenantID      *string      `json:"tenant_id"      binding:"omitempty,max=36"`
	LastAPICall   *lastAPICall `json:"last_api_call"`
	ComponentName *string      `json:"component_name" binding:"omitempty,max=256"`
}

// Handler returns a Gin handler that accepts frontend error reports and logs
// them in GCP ReportedErrorEvent format for Cloud Error Reporting auto-ingestion.
func Handler(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req reportRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Build the message field: error message + stack trace.
		// Error Reporting uses this for grouping.
		msg := req.Message
		if req.Stack != nil {
			msg = msg + "\n" + *req.Stack
		}

		// Determine user identifier for Error Reporting context.
		user := "anonymous"
		if req.IdentityID != nil {
			user = *req.IdentityID
		}

		// Log as ReportedErrorEvent for Cloud Error Reporting auto-detection.
		attrs := []any{
			slog.String("@type", "type.googleapis.com/google.devtools.clouderrorreporting.v1beta1.ReportedErrorEvent"),
			slog.String("message", msg),
			slog.Group("serviceContext",
				slog.String("service", "fastfso-frontend"),
				slog.String("version", env.AppEnv()),
			),
			slog.Group("context",
				slog.Group("httpRequest",
					slog.String("url", req.URL),
					slog.String("userAgent", c.GetHeader("User-Agent")),
					slog.String("remoteIp", c.ClientIP()),
				),
				slog.String("user", user),
			),
			slog.String("error_source", req.Source),
			slog.String("client_timestamp", req.Timestamp),
		}

		if req.IdentityID != nil {
			attrs = append(attrs, slog.String("identity_id", *req.IdentityID))
		}
		if req.UserID != nil {
			attrs = append(attrs, slog.String("user_id", *req.UserID))
		}
		if req.TenantID != nil {
			attrs = append(attrs, slog.String("tenant_id", *req.TenantID))
		}
		if req.ComponentName != nil {
			attrs = append(attrs, slog.String("component_name", *req.ComponentName))
		}
		if req.LastAPICall != nil {
			attrs = append(attrs, slog.Group("last_api_call",
				slog.String("method", req.LastAPICall.Method),
				slog.String("url", req.LastAPICall.URL),
				slog.Int("status", req.LastAPICall.Status),
				slog.String("response_body", req.LastAPICall.ResponseBody),
			))
		}

		logger.ErrorContext(c.Request.Context(), "frontend error", attrs...)

		telemetry.RecordFrontendError(c.Request.Context(), req.Source)

		c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})
	}
}
