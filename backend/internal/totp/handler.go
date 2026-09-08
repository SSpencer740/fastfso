package totp

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"

	"github.com/fastfso/fastfso/backend/internal/audit"
	"github.com/fastfso/fastfso/backend/internal/auth"
	"github.com/fastfso/fastfso/backend/internal/identity"
	"github.com/fastfso/fastfso/backend/internal/session"
)

type Handler struct {
	store      *Store
	identities *identity.Store
	sessions   *session.Store
	audit      *audit.Store
	logger     *slog.Logger
}

func NewHandler(store *Store, identities *identity.Store, sessions *session.Store, auditStore *audit.Store, logger *slog.Logger) *Handler {
	return &Handler{
		store:      store,
		identities: identities,
		sessions:   sessions,
		audit:      auditStore,
		logger:     logger,
	}
}

// Enroll generates a new TOTP secret and returns the provisioning URI.
func (h *Handler) Enroll(c *gin.Context) {
	sess, ok := auth.GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	ident, err := h.identities.GetByID(c.Request.Context(), sess.IdentityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "fastFSO",
		AccountName: ident.Email,
	})
	if err != nil {
		h.logger.Error("failed to generate TOTP key", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if _, err := h.store.Create(c.Request.Context(), sess.IdentityID, key.Secret()); err != nil {
		h.logger.Error("failed to store TOTP secret", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"secret": key.Secret(),
		"url":    key.URL(),
	})
}

type verifyRequest struct {
	Code string `json:"code" binding:"required"`
}

// Verify validates a TOTP code during 2FA challenge.
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

	secret, err := h.store.GetByIdentity(c.Request.Context(), sess.IdentityID)
	if err != nil {
		h.audit.Log(c.Request.Context(), audit.LogParams{
			IdentityID: &sess.IdentityID,
			SessionID:  &sess.ID,
			Action:     audit.Action2FAFailure,
			IPAddress:  c.ClientIP(),
			UserAgent:  c.GetHeader("User-Agent"),
			Metadata:   map[string]any{"method": "totp", "reason": "no_secret"},
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": "TOTP not configured"})
		return
	}

	if !totp.Validate(req.Code, secret.Secret) {
		h.audit.Log(c.Request.Context(), audit.LogParams{
			IdentityID: &sess.IdentityID,
			SessionID:  &sess.ID,
			Action:     audit.Action2FAFailure,
			IPAddress:  c.ClientIP(),
			UserAgent:  c.GetHeader("User-Agent"),
			Metadata:   map[string]any{"method": "totp", "reason": "invalid_code"},
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid code"})
		return
	}

	// Mark secret as verified if it wasn't already
	if !secret.Verified {
		if err := h.store.Verify(c.Request.Context(), sess.IdentityID); err != nil {
			h.logger.Error("failed to verify TOTP secret", "error", err)
		}
	}

	// Upgrade session to pre_tenant
	if err := h.sessions.SetSecondFactor(c.Request.Context(), sess.ID, "totp"); err != nil {
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
		Metadata:   map[string]any{"method": "totp"},
	})

	c.JSON(http.StatusOK, gin.H{"state": session.StatePreTenant})
}

// ConfirmEnroll verifies a TOTP code during enrollment (authenticated state).
func (h *Handler) ConfirmEnroll(c *gin.Context) {
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

	secret, err := h.store.GetByIdentity(c.Request.Context(), sess.IdentityID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "TOTP not configured"})
		return
	}

	if !totp.Validate(req.Code, secret.Secret) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid code"})
		return
	}

	if err := h.store.Verify(c.Request.Context(), sess.IdentityID); err != nil {
		h.logger.Error("failed to verify TOTP", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionTOTPEnrolled,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	})

	// After mandatory enrollment, promote session so user can select a tenant.
	if sess.State == session.StateSetup2FA {
		if err := h.sessions.SetSecondFactor(c.Request.Context(), sess.ID, "totp"); err != nil {
			h.logger.Error("failed to promote session after enrollment", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"state": session.StatePreTenant})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "TOTP enrolled"})
}

// Remove deletes the TOTP secret for the current identity.
func (h *Handler) Remove(c *gin.Context) {
	sess, ok := auth.GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	if err := h.store.Delete(c.Request.Context(), sess.IdentityID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "TOTP not configured"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionTOTPRemoved,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, gin.H{"message": "TOTP removed"})
}
