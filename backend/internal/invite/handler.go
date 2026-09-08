package invite

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/auth"
	"github.com/SSpencer740/fastfso/backend/internal/identity"
	"github.com/SSpencer740/fastfso/backend/internal/session"
	"github.com/SSpencer740/fastfso/backend/internal/telemetry"
)

type identityStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*identity.Identity, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	UpdateName(ctx context.Context, id uuid.UUID, name string) error
	Activate(ctx context.Context, id uuid.UUID) error
}

type sessionCreator interface {
	Create(ctx context.Context, p session.CreateParams) (*session.Session, error)
}

type auditLogger interface {
	Log(ctx context.Context, p audit.LogParams)
}

// TenantNameLookup returns the primary tenant name for an identity (for the invite setup page).
type TenantNameLookup func(ctx context.Context, identityID uuid.UUID) string

// Handler serves the invite accept/info endpoints.
type Handler struct {
	store        *Store
	identities   identityStore
	sessions     sessionCreator
	audit        auditLogger
	tenantLookup TenantNameLookup
	logger       *slog.Logger
}

func NewHandler(
	store *Store,
	identities identityStore,
	sessions sessionCreator,
	auditStore auditLogger,
	tenantLookup TenantNameLookup,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		store:        store,
		identities:   identities,
		sessions:     sessions,
		audit:        auditStore,
		tenantLookup: tenantLookup,
		logger:       logger,
	}
}

type tokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// Info validates an invite token and returns identity/tenant info for the setup page.
func (h *Handler) Info(c *gin.Context) {
	var req tokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	ctx := c.Request.Context()

	tok, err := h.store.Validate(ctx, req.Token)
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrExpired) {
			c.JSON(http.StatusNotFound, gin.H{"error": "invalid or expired invite"})
			return
		}
		h.logger.Error("failed to validate invite token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	ident, err := h.identities.GetByID(ctx, tok.IdentityID)
	if err != nil {
		h.logger.Error("failed to get identity for invite", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid or expired invite"})
		return
	}

	tenantName := h.tenantLookup(ctx, tok.IdentityID)

	c.JSON(http.StatusOK, gin.H{
		"email":       ident.Email,
		"name":        ident.Name,
		"tenant_name": tenantName,
	})
}

type acceptRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name"`
}

// Accept validates the token, sets password/name, activates the identity,
// creates a session, and returns pre_tenant state.
func (h *Handler) Accept(c *gin.Context) {
	var req acceptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token and password (min 8 chars) are required"})
		return
	}

	ctx := c.Request.Context()
	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")

	tok, err := h.store.Validate(ctx, req.Token)
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrExpired) {
			c.JSON(http.StatusNotFound, gin.H{"error": "invalid or expired invite"})
			return
		}
		h.logger.Error("failed to validate invite token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	ident, err := h.identities.GetByID(ctx, tok.IdentityID)
	if err != nil {
		h.logger.Error("failed to get identity for invite accept", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid or expired invite"})
		return
	}

	if ident.Activated {
		c.JSON(http.StatusConflict, gin.H{"error": "account already activated"})
		return
	}

	if ident.IsSuspended() {
		c.JSON(http.StatusForbidden, gin.H{"error": "account is suspended"})
		return
	}

	// Hash password
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Set password
	if err := h.identities.UpdatePassword(ctx, ident.ID, hash); err != nil {
		h.logger.Error("failed to set password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Optionally update name
	if req.Name != "" && req.Name != ident.Name {
		if err := h.identities.UpdateName(ctx, ident.ID, req.Name); err != nil {
			h.logger.Error("failed to update name", "error", err)
			// Non-fatal, continue
		}
	}

	// Activate
	if err := h.identities.Activate(ctx, ident.ID); err != nil {
		h.logger.Error("failed to activate identity", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Mark token used
	if err := h.store.MarkUsed(ctx, tok.ID); err != nil {
		h.logger.Error("failed to mark invite token used", "error", err)
		// Non-fatal, continue
	}

	// Create session
	csrfToken, err := auth.GenerateCSRFToken()
	if err != nil {
		h.logger.Error("failed to generate CSRF token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	sess, err := h.sessions.Create(ctx, session.CreateParams{
		IdentityID: ident.ID,
		State:      session.StatePreTenant,
		IPAddress:  ip,
		UserAgent:  ua,
		AuthMethod: "invite",
		CSRFToken:  csrfToken,
	})
	if err != nil {
		h.logger.Error("failed to create session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	telemetry.RecordLogin(ctx, "invite", true)
	telemetry.RecordSessionCreated(ctx)

	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &ident.ID,
		SessionID:  &sess.ID,
		Action:     audit.ActionInviteAccepted,
		IPAddress:  ip,
		UserAgent:  ua,
	})

	auth.SetSessionCookie(c, sess.ID)
	auth.SetCSRFCookiePublic(c, csrfToken)

	c.JSON(http.StatusOK, gin.H{"state": session.StatePreTenant})
}
