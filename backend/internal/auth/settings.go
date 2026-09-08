package auth

import (
	"net/http"

	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/gin-gonic/gin"
)

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// ChangePassword lets an authenticated user change their own password.
func (h *Handler) ChangePassword(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current_password and new_password (min 8 chars) are required"})
		return
	}

	ident, err := h.identities.GetByID(c.Request.Context(), sess.IdentityID)
	if err != nil {
		h.logger.Error("failed to get identity", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if !ident.HasPassword() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account does not have a password set"})
		return
	}

	if !CheckPassword(*ident.PasswordHash, req.CurrentPassword) {
		c.JSON(http.StatusForbidden, gin.H{"error": "current password is incorrect"})
		return
	}

	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.identities.UpdatePassword(c.Request.Context(), ident.ID, hash); err != nil {
		h.logger.Error("failed to update password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionPasswordChanged,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, gin.H{"message": "password changed"})
}

// SecurityOverview returns the security status of the current user's account.
func (h *Handler) SecurityOverview(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	ident, err := h.identities.GetByID(c.Request.Context(), sess.IdentityID)
	if err != nil {
		h.logger.Error("failed to get identity", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	hasTOTP, _ := h.totpStore.HasVerifiedTOTP(c.Request.Context(), sess.IdentityID)
	passkeyCount, _ := h.passkeys.CountByIdentity(c.Request.Context(), sess.IdentityID)

	c.JSON(http.StatusOK, gin.H{
		"has_password":  ident.HasPassword(),
		"has_totp":      hasTOTP,
		"passkey_count": passkeyCount,
	})
}
