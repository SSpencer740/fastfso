package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/email"
	"github.com/SSpencer740/fastfso/backend/internal/env"
	"github.com/SSpencer740/fastfso/backend/internal/identity"
	"github.com/SSpencer740/fastfso/backend/internal/session"
	"github.com/SSpencer740/fastfso/backend/internal/telemetry"
)

type authStore interface {
	GetUserWithTenant(ctx context.Context, userID uuid.UUID) (UserWithTenant, error)
	ListTenantsByIdentity(ctx context.Context, identityID uuid.UUID) ([]TenantMembership, error)
	GetUserInTenant(ctx context.Context, identityID, tenantID uuid.UUID) (UserWithTenant, error)
	GetUserRole(ctx context.Context, userID uuid.UUID) (string, error)
	ListSSOByIdentity(ctx context.Context, identityID uuid.UUID) ([]SSOOption, error)
	ListSSOByEmailDomain(ctx context.Context, domain string) ([]SSOOption, error)
	HasEnabledSSODomain(ctx context.Context, tenantID uuid.UUID, domain string) (bool, error)
	ListAllActiveSessions(ctx context.Context, limit, offset int) ([]AdminSessionRow, error)
	ListAllTenants(ctx context.Context) ([]AdminTenantRow, error)
	CreateTenant(ctx context.Context, name string) (AdminTenantRow, error)
	GetTenantDetail(ctx context.Context, tenantID uuid.UUID) (AdminTenantDetail, error)
	UpdateTenant(ctx context.Context, tenantID uuid.UUID, name string) error
	ListAllIdentities(ctx context.Context, limit, offset int) ([]AdminIdentityRow, error)
	GetIdentityDetail(ctx context.Context, identityID uuid.UUID) (AdminIdentityDetail, error)
	AddIdentityToTenant(ctx context.Context, identityID, tenantID uuid.UUID, role string) (uuid.UUID, error)
	ListTenantMembers(ctx context.Context, tenantID uuid.UUID, search string) ([]TenantMemberRow, error)
	UpdateMemberRole(ctx context.Context, userID, tenantID uuid.UUID, role string) error
	RemoveMember(ctx context.Context, userID, tenantID uuid.UUID) error
	DeleteTenant(ctx context.Context, tenantID uuid.UUID) error
	SuspendTenant(ctx context.Context, tenantID uuid.UUID) error
	UnsuspendTenant(ctx context.Context, tenantID uuid.UUID) error
	IsTenantSuspended(ctx context.Context, tenantID uuid.UUID) (bool, error)
}

type identityReader interface {
	GetByEmail(ctx context.Context, email string) (*identity.Identity, error)
	GetByID(ctx context.Context, id uuid.UUID) (*identity.Identity, error)
	Create(ctx context.Context, email, name string, passwordHash *string) (*identity.Identity, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	UpdateName(ctx context.Context, id uuid.UUID, name string) error
	UpdateEmail(ctx context.Context, id uuid.UUID, email string) error
	Activate(ctx context.Context, id uuid.UUID) error
	SetSuperAdmin(ctx context.Context, id uuid.UUID, isSuperAdmin bool) error
	Suspend(ctx context.Context, id uuid.UUID) error
	Unsuspend(ctx context.Context, id uuid.UUID) error
	CountActiveSuperAdmins(ctx context.Context) (int, error)
	Delete(ctx context.Context, id uuid.UUID) error
	RecordFailedLogin(ctx context.Context, id uuid.UUID) error
	ClearFailedLogin(ctx context.Context, id uuid.UUID) error
}

type inviteGenerator interface {
	Generate(ctx context.Context, identityID uuid.UUID) (string, error)
}

type passwordResetter interface {
	GeneratePasswordReset(ctx context.Context, identityID uuid.UUID) (string, error)
	ValidatePasswordReset(ctx context.Context, rawToken string) (tokenID uuid.UUID, identityID uuid.UUID, err error)
	MarkUsed(ctx context.Context, id uuid.UUID) error
}

type sessionManager interface {
	Create(ctx context.Context, p session.CreateParams) (*session.Session, error)
	GetValid(ctx context.Context, id uuid.UUID) (*session.Session, error)
	SetTenant(ctx context.Context, id, tenantID, userID uuid.UUID, subOrgID *uuid.UUID, userRole string) error
	SetSuperAdmin(ctx context.Context, id uuid.UUID) error
	ResetContext(ctx context.Context, id uuid.UUID) error
	Revoke(ctx context.Context, id uuid.UUID, revokedBy *uuid.UUID, reason string) error
	RevokeByTenant(ctx context.Context, tenantID uuid.UUID, reason string) (int64, error)
	RevokeByIdentity(ctx context.Context, identityID uuid.UUID, reason string) (int64, error)
	RevokeByIdentityExcept(ctx context.Context, identityID, exceptID uuid.UUID, reason string) (int64, error)
	ListByIdentity(ctx context.Context, identityID uuid.UUID) ([]session.Session, error)
}

type auditLogger interface {
	Log(ctx context.Context, p audit.LogParams)
	Query(ctx context.Context, identityID *uuid.UUID, limit, offset int) ([]audit.Entry, error)
	QueryByTenant(ctx context.Context, tenantID uuid.UUID, action, search string, limit, offset int) ([]audit.TenantEntry, int, error)
}

type totpChecker interface {
	HasVerifiedTOTP(ctx context.Context, identityID uuid.UUID) (bool, error)
	ValidateCode(ctx context.Context, identityID uuid.UUID, code string) (bool, error)
}

type passkeyCounter interface {
	CountByIdentity(ctx context.Context, identityID uuid.UUID) (int, error)
}

// SubOrgAssigner moves a user to a specific sub-org within a tenant. Used
// by the invite flow to honor the admin's sub-org choice; falls back to the
// "Default" sub-org auto-assignment trigger when not provided.
type SubOrgAssigner interface {
	SetUserSubOrg(ctx context.Context, tenantID, userID, subOrgID uuid.UUID) error
}

// Handler holds dependencies for auth route handlers.
type Handler struct {
	store        authStore
	identities   identityReader
	sessions     sessionManager
	audit        auditLogger
	totpStore    totpChecker
	passkeys     passkeyCounter
	invites      inviteGenerator
	resets       passwordResetter
	email        *email.Service
	userSettings *UserSettingsStore
	subOrgs      SubOrgAssigner
	logger       *slog.Logger
}

func NewHandler(
	store authStore,
	identities identityReader,
	sessions sessionManager,
	auditStore auditLogger,
	totpStore totpChecker,
	passkeys passkeyCounter,
	invites inviteGenerator,
	resets passwordResetter,
	emailService *email.Service,
	userSettings *UserSettingsStore,
	subOrgs SubOrgAssigner,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		store:        store,
		identities:   identities,
		sessions:     sessions,
		audit:        auditStore,
		totpStore:    totpStore,
		passkeys:     passkeys,
		invites:      invites,
		resets:       resets,
		email:        emailService,
		userSettings: userSettings,
		subOrgs:      subOrgs,
		logger:       logger,
	}
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")

	// Log the attempt for rate limiting
	h.audit.Log(c.Request.Context(), audit.LogParams{
		Action:    audit.ActionLoginAttempt,
		IPAddress: ip,
		UserAgent: ua,
		Metadata:  map[string]any{"email": req.Email},
	})

	ident, err := h.identities.GetByEmail(c.Request.Context(), req.Email)
	if err != nil {
		telemetry.RecordLogin(c.Request.Context(), "password", false)
		h.audit.Log(c.Request.Context(), audit.LogParams{
			Action:    audit.ActionLoginFailure,
			IPAddress: ip,
			UserAgent: ua,
			Metadata:  map[string]any{"email": req.Email, "reason": "identity_not_found"},
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if !ident.HasPassword() {
		telemetry.RecordLogin(c.Request.Context(), "password", false)
		h.audit.Log(c.Request.Context(), audit.LogParams{
			IdentityID: &ident.ID,
			Action:     audit.ActionLoginFailure,
			IPAddress:  ip,
			UserAgent:  ua,
			Metadata:   map[string]any{"reason": "no_password_set"},
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if ident.IsLocked() {
		telemetry.RecordLogin(c.Request.Context(), "password", false)
		h.audit.Log(c.Request.Context(), audit.LogParams{
			IdentityID: &ident.ID,
			Action:     audit.ActionLoginFailure,
			IPAddress:  ip,
			UserAgent:  ua,
			Metadata:   map[string]any{"reason": "account_locked"},
		})
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "account temporarily locked due to too many failed attempts"})
		return
	}

	if !CheckPassword(*ident.PasswordHash, req.Password) {
		telemetry.RecordLogin(c.Request.Context(), "password", false)
		if err := h.identities.RecordFailedLogin(c.Request.Context(), ident.ID); err != nil {
			h.logger.Warn("failed to record failed login", "error", err)
		}
		h.audit.Log(c.Request.Context(), audit.LogParams{
			IdentityID: &ident.ID,
			Action:     audit.ActionLoginFailure,
			IPAddress:  ip,
			UserAgent:  ua,
			Metadata:   map[string]any{"reason": "wrong_password"},
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Clear lockout state on successful password verification
	if err := h.identities.ClearFailedLogin(c.Request.Context(), ident.ID); err != nil {
		h.logger.Warn("failed to clear failed login counter", "error", err)
	}

	if ident.IsSuspended() {
		telemetry.RecordLogin(c.Request.Context(), "password", false)
		h.audit.Log(c.Request.Context(), audit.LogParams{
			IdentityID: &ident.ID,
			Action:     audit.ActionLoginFailure,
			IPAddress:  ip,
			UserAgent:  ua,
			Metadata:   map[string]any{"reason": "account_suspended"},
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "account is suspended"})
		return
	}

	// Check if 2FA is required
	has2FA, methods := h.get2FAMethods(c.Request.Context(), ident.ID)

	csrfToken, err := GenerateCSRFToken()
	if err != nil {
		h.logger.Error("failed to generate CSRF token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	state := session.StateSetup2FA
	if has2FA {
		state = session.StatePre_Auth
	}

	sess, err := h.sessions.Create(c.Request.Context(), session.CreateParams{
		IdentityID: ident.ID,
		State:      state,
		IPAddress:  ip,
		UserAgent:  ua,
		AuthMethod: "password",
		CSRFToken:  csrfToken,
	})
	if err != nil {
		h.logger.Error("failed to create session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	telemetry.RecordLogin(c.Request.Context(), "password", true)
	telemetry.RecordSessionCreated(c.Request.Context())

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &ident.ID,
		SessionID:  &sess.ID,
		Action:     audit.ActionLoginSuccess,
		IPAddress:  ip,
		UserAgent:  ua,
		Metadata:   map[string]any{"auth_method": "password"},
	})

	setSessionCookie(c, sess.ID)
	setCSRFCookie(c, csrfToken)

	resp := gin.H{"state": sess.State}
	if has2FA {
		resp["available_2fa"] = methods
	} else {
		resp["setup_2fa_required"] = true
	}
	c.JSON(http.StatusOK, resp)
}

type identifyRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type identifyResponse struct {
	Methods    []string    `json:"methods"`
	SSOOptions []SSOOption `json:"sso_options"`
}

// Identify determines the available authentication methods for a given email.
func (h *Handler) Identify(c *gin.Context) {
	var req identifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	domain := ""
	if i := strings.LastIndex(email, "@"); i >= 0 {
		domain = email[i+1:]
	}

	ctx := c.Request.Context()

	ident, err := h.identities.GetByEmail(ctx, email)
	if err != nil {
		// Identity not found — check email domain for SSO
		var ssoOptions []SSOOption
		if domain != "" {
			ssoOptions, _ = h.store.ListSSOByEmailDomain(ctx, domain)
		}
		if len(ssoOptions) > 0 {
			c.JSON(http.StatusOK, identifyResponse{
				Methods:    []string{},
				SSOOptions: ssoOptions,
			})
			return
		}
		// Anti-enumeration: return password method for unknown emails
		c.JSON(http.StatusOK, identifyResponse{
			Methods:    []string{"password"},
			SSOOptions: []SSOOption{},
		})
		return
	}

	var methods []string
	if ident.HasPassword() {
		methods = append(methods, "password")
	}

	// Check passkeys
	count, err := h.passkeys.CountByIdentity(ctx, ident.ID)
	if err == nil && count > 0 {
		methods = append(methods, "passkey")
	}

	// Check SSO via tenant memberships
	ssoOptions, err := h.store.ListSSOByIdentity(ctx, ident.ID)
	if err != nil {
		h.logger.Error("failed to list SSO options by identity", "error", err)
		ssoOptions = []SSOOption{}
	}

	if methods == nil {
		methods = []string{}
	}
	if ssoOptions == nil {
		ssoOptions = []SSOOption{}
	}

	c.JSON(http.StatusOK, identifyResponse{
		Methods:    methods,
		SSOOptions: ssoOptions,
	})
}

func (h *Handler) get2FAMethods(ctx context.Context, identityID uuid.UUID) (bool, []string) {
	var methods []string

	// Check TOTP
	if verified, err := h.totpStore.HasVerifiedTOTP(ctx, identityID); err == nil && verified {
		methods = append(methods, "totp")
	}

	// Check passkeys
	count, err := h.passkeys.CountByIdentity(ctx, identityID)
	if err == nil && count > 0 {
		methods = append(methods, "passkey")
	}

	return len(methods) > 0, methods
}

func (h *Handler) Logout(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	if err := h.sessions.Revoke(c.Request.Context(), sess.ID, &sess.IdentityID, "logout"); err != nil {
		h.logger.Error("failed to revoke session", "error", err)
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   sess.TenantID,
		Action:     audit.ActionLogout,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	})

	clearSessionCookie(c)
	clearCSRFCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *Handler) Me(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	ident, err := h.identities.GetByID(c.Request.Context(), sess.IdentityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	resp := gin.H{
		"state": sess.State,
		"identity": gin.H{
			"id":             ident.ID,
			"email":          ident.Email,
			"name":           ident.Name,
			"is_super_admin": ident.IsSuperAdmin,
		},
		"is_super_admin": sess.IsSuperAdmin,
	}

	if sess.UserID != nil && sess.TenantID != nil {
		u, err := h.store.GetUserWithTenant(c.Request.Context(), *sess.UserID)
		if err == nil {
			resp["user"] = gin.H{
				"id":          sess.UserID,
				"name":        u.UserName,
				"role":        u.UserRole,
				"tenant_id":   sess.TenantID,
				"tenant_name": u.TenantName,
			}
		}
	}

	// Compute can_switch_context
	tenants, err := h.store.ListTenantsByIdentity(c.Request.Context(), sess.IdentityID)
	if err != nil {
		h.logger.Error("failed to list tenants for switch context", "error", err)
		tenants = nil
	}
	contextCount := len(tenants)
	if ident.IsSuperAdmin {
		contextCount++
	}
	resp["can_switch_context"] = contextCount > 1

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) SwitchContext(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	if err := h.sessions.ResetContext(c.Request.Context(), sess.ID); err != nil {
		h.logger.Error("failed to reset session context", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   sess.TenantID,
		Action:     audit.ActionContextSwitch,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, gin.H{"state": session.StatePreTenant})
}

type tenantInfo struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Suspended bool      `json:"suspended"`
}

func (h *Handler) ListTenants(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	memberships, err := h.store.ListTenantsByIdentity(c.Request.Context(), sess.IdentityID)
	if err != nil {
		h.logger.Error("failed to list tenants", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Check if identity is super admin
	ident, err := h.identities.GetByID(c.Request.Context(), sess.IdentityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	tenants := make([]tenantInfo, 0, len(memberships))
	for _, m := range memberships {
		tenants = append(tenants, tenantInfo{
			ID:        m.TenantID,
			Name:      m.Name,
			Role:      m.Role,
			Suspended: m.Suspended,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"tenants":                tenants,
		"can_access_admin_panel": ident.IsSuperAdmin,
	})
}

type selectTenantRequest struct {
	TenantID uuid.UUID `json:"tenant_id" binding:"required"`
}

func (h *Handler) SelectTenant(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	var req selectTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Verify this identity has a user in the requested tenant
	u, err := h.store.GetUserInTenant(c.Request.Context(), sess.IdentityID, req.TenantID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this tenant"})
		return
	}

	// Check if the tenant is suspended
	suspended, err := h.store.IsTenantSuspended(c.Request.Context(), req.TenantID)
	if err != nil {
		h.logger.Error("failed to check tenant suspension", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if suspended {
		c.JSON(http.StatusForbidden, gin.H{"error": "this organization is suspended"})
		return
	}

	if err := h.sessions.SetTenant(c.Request.Context(), sess.ID, req.TenantID, u.UserID, u.SubOrgID, u.UserRole); err != nil {
		h.logger.Error("failed to set tenant", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   &req.TenantID,
		Action:     audit.ActionTenantSelected,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"tenant_name": u.TenantName},
	})

	c.JSON(http.StatusOK, gin.H{
		"state": session.StateAuthenticated,
		"user": gin.H{
			"id":          u.UserID,
			"name":        u.UserName,
			"role":        u.UserRole,
			"tenant_name": u.TenantName,
		},
	})
}

func (h *Handler) SelectAdmin(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	ident, err := h.identities.GetByID(c.Request.Context(), sess.IdentityID)
	if err != nil || !ident.IsSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a super admin"})
		return
	}

	if err := h.sessions.SetSuperAdmin(c.Request.Context(), sess.ID); err != nil {
		h.logger.Error("failed to set super admin", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionSuperAdminAccess,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, gin.H{
		"state":          session.StateAuthenticated,
		"is_super_admin": true,
	})
}

func (h *Handler) ListSessions(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	sessions, err := h.sessions.ListByIdentity(c.Request.Context(), sess.IdentityID)
	if err != nil {
		h.logger.Error("failed to list sessions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions":           sessions,
		"current_session_id": sess.ID,
	})
}

func (h *Handler) RevokeSession(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	targetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
		return
	}

	// Verify the target session belongs to the same identity
	target, err := h.sessions.GetValid(c.Request.Context(), targetID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	if target.IdentityID != sess.IdentityID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot revoke another identity's session"})
		return
	}

	if err := h.sessions.Revoke(c.Request.Context(), targetID, &sess.IdentityID, "user_revoked"); err != nil {
		h.logger.Error("failed to revoke session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &targetID,
		Action:     audit.ActionSessionRevoked,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"revoked_session_id": targetID},
	})

	c.JSON(http.StatusOK, gin.H{"message": "session revoked"})
}

// RevokeAllOtherSessions revokes every active session for the caller's
// identity except the one issuing the request. Powers the "sign out
// everywhere else" control in user settings — a quick incident-response
// lever after, e.g. losing a device or noticing an unfamiliar entry in
// the active sessions list.
func (h *Handler) RevokeAllOtherSessions(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	revoked, err := h.sessions.RevokeByIdentityExcept(c.Request.Context(), sess.IdentityID, sess.ID, "user_revoked_all")
	if err != nil {
		h.logger.Error("failed to revoke all other sessions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   sess.TenantID,
		Action:     audit.ActionSessionRevoked,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"scope": "all_other_sessions", "revoked_count": revoked},
	})

	c.JSON(http.StatusOK, gin.H{"message": "other sessions revoked", "revoked": revoked})
}

// Get2FAMethods returns the available 2FA methods for the current session's identity.
func (h *Handler) Get2FAMethods(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	_, methods := h.get2FAMethods(c.Request.Context(), sess.IdentityID)
	c.JSON(http.StatusOK, gin.H{"methods": methods})
}

// GetUserRole implements UserRoleGetter for use with RequireRole middleware.
func (h *Handler) GetUserRole(ctx context.Context, userID uuid.UUID) (string, error) {
	return h.store.GetUserRole(ctx, userID)
}

func setSessionCookie(c *gin.Context, sessionID uuid.UUID) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(sessionCookieName, sessionID.String(), 0, "/", "", env.IsCloud(), true)
}

// SetSessionCookie is exported for use by other packages (e.g. passkey, sso).
func SetSessionCookie(c *gin.Context, sessionID uuid.UUID) {
	setSessionCookie(c, sessionID)
}

func clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(sessionCookieName, "", -1, "/", "", env.IsCloud(), true)
}
