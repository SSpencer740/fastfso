package sso

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/auth"
	"github.com/SSpencer740/fastfso/backend/internal/env"
	"github.com/SSpencer740/fastfso/backend/internal/identity"
	"github.com/SSpencer740/fastfso/backend/internal/session"
	"github.com/SSpencer740/fastfso/backend/internal/telemetry"
)

type ssoStore interface {
	GetByTenantSlug(ctx context.Context, slug string) (*Config, uuid.UUID, error)
	GetByTenantID(ctx context.Context, tenantID uuid.UUID) (*Config, error)
	Upsert(ctx context.Context, p UpsertParams) (*Config, error)
	ProvisionUser(ctx context.Context, identityID, tenantID uuid.UUID, role string) error
	IsTenantSuspended(ctx context.Context, tenantID uuid.UUID) (bool, error)
	ListEmailDomains(ctx context.Context, ssoConfigID uuid.UUID) ([]EmailDomain, error)
	AddEmailDomain(ctx context.Context, ssoConfigID uuid.UUID, domain string) (EmailDomain, error)
	RemoveEmailDomain(ctx context.Context, ssoConfigID uuid.UUID, domain string) error
}

type identityProvider interface {
	GetByEmail(ctx context.Context, email string) (*identity.Identity, error)
	Create(ctx context.Context, email, name string, passwordHash *string) (*identity.Identity, error)
}

type sessionCreator interface {
	Create(ctx context.Context, p session.CreateParams) (*session.Session, error)
}

type auditLogger interface {
	Log(ctx context.Context, p audit.LogParams)
}

type Handler struct {
	store      ssoStore
	identities identityProvider
	sessions   sessionCreator
	audit      auditLogger
	logger     *slog.Logger
}

func NewHandler(store ssoStore, identities identityProvider, sessions sessionCreator, auditStore auditLogger, logger *slog.Logger) *Handler {
	return &Handler{
		store:      store,
		identities: identities,
		sessions:   sessions,
		audit:      auditStore,
		logger:     logger,
	}
}

func (h *Handler) frontendURL() string {
	return env.FrontendURL()
}

// InitiateSSO redirects to the SSO provider for the given tenant.
func (h *Handler) InitiateSSO(c *gin.Context) {
	slug := c.Param("tenant_slug")
	cfg, _, err := h.store.GetByTenantSlug(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO not configured for this organization"})
		return
	}

	if !cfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SSO is not enabled for this organization"})
		return
	}

	// Check if the tenant is suspended
	suspended, err := h.store.IsTenantSuspended(c.Request.Context(), cfg.TenantID)
	if err != nil {
		h.logger.Error("failed to check tenant suspension", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if suspended {
		c.JSON(http.StatusForbidden, gin.H{"error": "this organization is suspended"})
		return
	}

	switch cfg.Protocol {
	case "oidc":
		h.initiateOIDC(c, cfg)
	case "saml":
		// SAML initiation is more complex and requires XML processing.
		// For now, return a placeholder.
		c.JSON(http.StatusNotImplemented, gin.H{"error": "SAML SSO initiation is not yet implemented"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unknown SSO protocol"})
	}
}

func (h *Handler) initiateOIDC(c *gin.Context, cfg *Config) {
	if cfg.IssuerURL == nil || cfg.ClientID == nil || cfg.ClientSecret == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OIDC configuration is incomplete"})
		return
	}

	provider, err := oidc.NewProvider(c.Request.Context(), *cfg.IssuerURL)
	if err != nil {
		h.logger.Error("failed to create OIDC provider", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to identity provider"})
		return
	}

	oauth2Config := &oauth2.Config{
		ClientID:     *cfg.ClientID,
		ClientSecret: *cfg.ClientSecret,
		RedirectURL:  h.backendURL() + "/api/auth/sso/callback/oidc",
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	// Use tenant_id as state parameter to identify which tenant this is for
	state := cfg.TenantID.String()
	c.Redirect(http.StatusFound, oauth2Config.AuthCodeURL(state))
}

// OIDCCallback handles the OIDC callback from the identity provider.
func (h *Handler) OIDCCallback(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")

	if state == "" || code == "" {
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_failed")
		return
	}

	tenantID, err := uuid.Parse(state)
	if err != nil {
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_failed")
		return
	}

	cfg, err := h.store.GetByTenantID(c.Request.Context(), tenantID)
	if err != nil || !cfg.Enabled || cfg.Protocol != "oidc" {
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_failed")
		return
	}

	provider, err := oidc.NewProvider(c.Request.Context(), *cfg.IssuerURL)
	if err != nil {
		h.logger.Error("OIDC provider error in callback", "error", err)
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_failed")
		return
	}

	oauth2Config := &oauth2.Config{
		ClientID:     *cfg.ClientID,
		ClientSecret: *cfg.ClientSecret,
		RedirectURL:  h.backendURL() + "/api/auth/sso/callback/oidc",
		Endpoint:     provider.Endpoint(),
	}

	token, err := oauth2Config.Exchange(c.Request.Context(), code)
	if err != nil {
		h.logger.Error("OIDC token exchange failed", "error", err)
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_failed")
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_failed")
		return
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: *cfg.ClientID})
	idToken, err := verifier.Verify(c.Request.Context(), rawIDToken)
	if err != nil {
		h.logger.Error("OIDC token verification failed", "error", err)
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_failed")
		return
	}

	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil || claims.Email == "" {
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_failed")
		return
	}

	h.completeSSO(c, cfg, claims.Email, claims.Name, "oidc")
}

// SAMLCallback handles SAML ACS (Assertion Consumer Service) responses.
func (h *Handler) SAMLCallback(c *gin.Context) {
	// SAML response parsing requires more complex XML handling.
	// This is a placeholder that returns an error.
	c.JSON(http.StatusNotImplemented, gin.H{"error": "SAML callback is not yet implemented"})
}

// completeSSO creates/finds the identity and session after successful SSO.
func (h *Handler) completeSSO(c *gin.Context, cfg *Config, email, name, protocol string) {
	ctx := c.Request.Context()
	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")

	// Check if the tenant is suspended
	suspended, err := h.store.IsTenantSuspended(ctx, cfg.TenantID)
	if err != nil {
		h.logger.Error("failed to check tenant suspension in SSO callback", "error", err)
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_failed")
		return
	}
	if suspended {
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=org_suspended")
		return
	}

	// Email-domain allowlist: when the tenant has configured any allowed
	// domains, the IdP-asserted email must match one of them. An empty list
	// means "no restriction" so existing tenants without an allowlist are
	// not affected. Without this check, a tenant with auto_provision=true
	// would accept any IdP-asserted email, including personal accounts at
	// the same provider.
	allowedDomains, err := h.store.ListEmailDomains(ctx, cfg.ID)
	if err != nil {
		h.logger.Error("failed to list SSO email domains", "error", err)
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_failed")
		return
	}
	if !emailDomainAllowed(email, allowedDomains) {
		h.audit.Log(ctx, audit.LogParams{
			TenantID:  &cfg.TenantID,
			Action:    audit.ActionLoginFailure,
			IPAddress: ip,
			UserAgent: ua,
			Metadata:  map[string]any{"auth_method": protocol, "reason": "sso_domain_not_allowed", "email": email},
		})
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_domain_not_allowed")
		return
	}

	// Find or create identity
	ident, err := h.identities.GetByEmail(ctx, email)
	if err != nil {
		if cfg.AutoProvision {
			ident, err = h.identities.Create(ctx, email, name, nil)
			if err != nil {
				h.logger.Error("failed to create identity via SSO", "error", err)
				c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_failed")
				return
			}
			// Auto-provision user in the tenant
			if err := h.store.ProvisionUser(ctx, ident.ID, cfg.TenantID, cfg.DefaultRole); err != nil {
				h.logger.Error("failed to provision user via SSO", "error", err)
			}
		} else {
			c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_no_account")
			return
		}
	}

	if ident.IsSuspended() {
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=account_suspended")
		return
	}

	csrfToken, err := auth.GenerateCSRFToken()
	if err != nil {
		h.logger.Error("failed to generate CSRF token", "error", err)
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_failed")
		return
	}

	// SSO skips 2FA → pre_tenant
	sess, err := h.sessions.Create(ctx, session.CreateParams{
		IdentityID: ident.ID,
		State:      session.StatePreTenant,
		IPAddress:  ip,
		UserAgent:  ua,
		AuthMethod: protocol,
		CSRFToken:  csrfToken,
	})
	if err != nil {
		h.logger.Error("failed to create session via SSO", "error", err)
		c.Redirect(http.StatusFound, h.frontendURL()+"/login?error=sso_failed")
		return
	}

	telemetry.RecordLogin(ctx, "sso", true)
	telemetry.RecordSessionCreated(ctx)

	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &ident.ID,
		SessionID:  &sess.ID,
		TenantID:   &cfg.TenantID,
		Action:     audit.ActionLoginSuccess,
		IPAddress:  ip,
		UserAgent:  ua,
		Metadata:   map[string]any{"auth_method": protocol, "sso_tenant": cfg.TenantID},
	})

	auth.SetSessionCookie(c, sess.ID)
	auth.SetCSRFCookiePublic(c, csrfToken)
	c.Redirect(http.StatusFound, h.frontendURL()+"/select-tenant")
}

func (h *Handler) backendURL() string {
	return env.BackendURL()
}

// emailDomainAllowed reports whether the given email is permitted by the
// tenant's SSO email-domain allowlist. An empty allowlist means "no
// restriction" (legacy behavior). A malformed email is always rejected.
func emailDomainAllowed(email string, allowed []EmailDomain) bool {
	if len(allowed) == 0 {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return false
	}
	domain := strings.ToLower(strings.TrimSpace(email[at+1:]))
	if domain == "" {
		return false
	}
	for _, d := range allowed {
		if strings.EqualFold(strings.TrimSpace(d.Domain), domain) {
			return true
		}
	}
	return false
}

// GetSSOConfig returns the SSO configuration for a tenant (admin use).
func (h *Handler) GetSSOConfig(c *gin.Context) {
	sess, ok := auth.GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	var tenantID uuid.UUID
	if sess.IsSuperAdmin {
		// Super admin: tenant_id from URL param
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
			return
		}
		tenantID = id
	} else if sess.TenantID != nil {
		tenantID = *sess.TenantID
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "no tenant"})
		return
	}

	cfg, err := h.store.GetByTenantID(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no SSO configuration"})
		return
	}

	c.JSON(http.StatusOK, cfg)
}

// UpdateSSOConfig updates the SSO configuration for a tenant (admin use).
func (h *Handler) UpdateSSOConfig(c *gin.Context) {
	sess, ok := auth.GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	var tenantID uuid.UUID
	if sess.IsSuperAdmin {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
			return
		}
		tenantID = id
	} else if sess.TenantID != nil {
		tenantID = *sess.TenantID
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "no tenant"})
		return
	}

	var req UpsertParams
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.TenantID = tenantID

	cfg, err := h.store.Upsert(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("failed to upsert SSO config", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionSSOConfigured,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"protocol": cfg.Protocol},
	})

	c.JSON(http.StatusOK, cfg)
}

// resolveTenantID returns the tenant ID from the URL param (super admin) or session (tenant admin).
func (h *Handler) resolveTenantID(c *gin.Context) (uuid.UUID, bool) {
	sess, ok := auth.GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return uuid.Nil, false
	}

	if sess.IsSuperAdmin {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
			return uuid.Nil, false
		}
		return id, true
	}

	if sess.TenantID != nil {
		return *sess.TenantID, true
	}

	c.JSON(http.StatusForbidden, gin.H{"error": "no tenant"})
	return uuid.Nil, false
}

// ListEmailDomains returns SSO email domains for a tenant's SSO configuration.
func (h *Handler) ListEmailDomains(c *gin.Context) {
	tenantID, ok := h.resolveTenantID(c)
	if !ok {
		return
	}

	cfg, err := h.store.GetByTenantID(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no SSO configuration"})
		return
	}

	domains, err := h.store.ListEmailDomains(c.Request.Context(), cfg.ID)
	if err != nil {
		h.logger.Error("failed to list email domains", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if domains == nil {
		domains = []EmailDomain{}
	}

	c.JSON(http.StatusOK, gin.H{"domains": domains})
}

// AddEmailDomain adds an email domain to a tenant's SSO configuration.
func (h *Handler) AddEmailDomain(c *gin.Context) {
	tenantID, ok := h.resolveTenantID(c)
	if !ok {
		return
	}

	var req struct {
		Domain string `json:"domain" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	cfg, err := h.store.GetByTenantID(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no SSO configuration"})
		return
	}

	domain, err := h.store.AddEmailDomain(c.Request.Context(), cfg.ID, req.Domain)
	if err != nil {
		h.logger.Error("failed to add email domain", "error", err)
		c.JSON(http.StatusConflict, gin.H{"error": "domain already exists or invalid"})
		return
	}

	c.JSON(http.StatusCreated, domain)
}

// RemoveEmailDomain removes an email domain from a tenant's SSO configuration.
func (h *Handler) RemoveEmailDomain(c *gin.Context) {
	tenantID, ok := h.resolveTenantID(c)
	if !ok {
		return
	}

	domainParam := c.Param("domain")
	if domainParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is required"})
		return
	}

	cfg, err := h.store.GetByTenantID(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no SSO configuration"})
		return
	}

	if err := h.store.RemoveEmailDomain(c.Request.Context(), cfg.ID, domainParam); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "domain removed"})
}
