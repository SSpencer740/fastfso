package emailcode

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fastfso/fastfso/backend/internal/audit"
	"github.com/fastfso/fastfso/backend/internal/auth"
	"github.com/fastfso/fastfso/backend/internal/session"
)

type Handler struct {
	store    *Store
	sessions *session.Store
	audit    *audit.Store
	logger   *slog.Logger
}

func NewHandler(store *Store, sessions *session.Store, auditStore *audit.Store, logger *slog.Logger) *Handler {
	return &Handler{
		store:    store,
		sessions: sessions,
		audit:    auditStore,
		logger:   logger,
	}
}

// Send generates an email code. Email delivery is not yet implemented.
func (h *Handler) Send(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "email sending is not yet implemented"})
}

type verifyRequest struct {
	Code string `json:"code" binding:"required"`
}

// Verify validates an email code during 2FA challenge.
func (h *Handler) Verify(c *gin.Context) {
	sess, ok := auth.GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	var req verifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := h.store.Verify(c.Request.Context(), sess.IdentityID, "2fa", req.Code); err != nil {
		h.audit.Log(c.Request.Context(), audit.LogParams{
			IdentityID: &sess.IdentityID,
			SessionID:  &sess.ID,
			Action:     audit.Action2FAFailure,
			IPAddress:  c.ClientIP(),
			UserAgent:  c.GetHeader("User-Agent"),
			Metadata:   map[string]any{"method": "email_code", "reason": err.Error()},
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired code"})
		return
	}

	if err := h.sessions.SetSecondFactor(c.Request.Context(), sess.ID, "email_code"); err != nil {
		h.logger.Error("failed to update session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.Action2FASuccess,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"method": "email_code"},
	})

	c.JSON(http.StatusOK, gin.H{"state": session.StatePreTenant})
}
