package auth

import (
	"net/http"

	"github.com/fastfso/fastfso/backend/internal/audit"
	"github.com/fastfso/fastfso/backend/internal/env"
	"github.com/gin-gonic/gin"
)

type passwordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// RequestPasswordReset initiates a self-service password reset.
// Always returns 200 to avoid revealing whether the email exists.
func (h *Handler) RequestPasswordReset(c *gin.Context) {
	var req passwordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")

	ident, err := h.identities.GetByEmail(c.Request.Context(), req.Email)
	if err != nil {
		// Don't reveal whether the email exists.
		c.JSON(http.StatusOK, gin.H{"message": "If that email exists, a reset link has been sent."})
		return
	}

	if ident.PasswordHash == nil {
		// SSO-only account — no password to reset, silently succeed.
		c.JSON(http.StatusOK, gin.H{"message": "If that email exists, a reset link has been sent."})
		return
	}

	token, err := h.resets.GeneratePasswordReset(c.Request.Context(), ident.ID)
	if err != nil {
		h.logger.Error("failed to generate password reset token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	appURL := env.FrontendURL()
	if err := h.email.SendPasswordReset(c.Request.Context(), ident.Email, appURL, token); err != nil {
		h.logger.Error("failed to send password reset email", "error", err, "identity_id", ident.ID)
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &ident.ID,
		Action:     audit.ActionPasswordResetRequested,
		IPAddress:  ip,
		UserAgent:  ua,
	})

	c.JSON(http.StatusOK, gin.H{"message": "If that email exists, a reset link has been sent."})
}

type confirmPasswordResetRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required"`
	TOTPCode string `json:"totp_code"`
}

// ConfirmPasswordReset validates a reset token and sets the new password.
// If the account has TOTP 2FA enrolled, a valid TOTP code is also required —
// otherwise possession of the reset token alone would bypass 2FA.
func (h *Handler) ConfirmPasswordReset(c *gin.Context) {
	var req confirmPasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	tokenID, identityID, err := h.resets.ValidatePasswordReset(c.Request.Context(), req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reset link is invalid or has expired"})
		return
	}

	hasTOTP, err := h.totpStore.HasVerifiedTOTP(c.Request.Context(), identityID)
	if err != nil {
		h.logger.Error("failed to check 2FA status during reset", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if hasTOTP {
		if req.TOTPCode == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":        "2FA code required",
				"requires_2fa": true,
			})
			return
		}
		valid, err := h.totpStore.ValidateCode(c.Request.Context(), identityID, req.TOTPCode)
		if err != nil {
			h.logger.Error("failed to validate TOTP during reset", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if !valid {
			h.audit.Log(c.Request.Context(), audit.LogParams{
				IdentityID: &identityID,
				Action:     audit.Action2FAFailure,
				IPAddress:  c.ClientIP(),
				UserAgent:  c.GetHeader("User-Agent"),
				Metadata:   map[string]any{"method": "totp", "context": "password_reset"},
			})
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":        "invalid 2FA code",
				"requires_2fa": true,
			})
			return
		}
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.identities.UpdatePassword(c.Request.Context(), identityID, hash); err != nil {
		h.logger.Error("failed to update password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.resets.MarkUsed(c.Request.Context(), tokenID); err != nil {
		h.logger.Error("failed to mark reset token used", "error", err)
	}

	// Revoke all existing sessions so old credentials can't be replayed.
	if _, err := h.sessions.RevokeByIdentity(c.Request.Context(), identityID, "password_reset"); err != nil {
		h.logger.Error("failed to revoke sessions after password reset", "error", err)
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &identityID,
		Action:     audit.ActionPasswordReset,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, gin.H{"message": "password reset successfully"})
}
