package passkey

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/auth"
	"github.com/SSpencer740/fastfso/backend/internal/identity"
	"github.com/SSpencer740/fastfso/backend/internal/session"
	"github.com/SSpencer740/fastfso/backend/internal/telemetry"
)

type Handler struct {
	store      *Store
	identities *identity.Store
	sessions   *session.Store
	audit      *audit.Store
	wa         *webauthn.WebAuthn
	logger     *slog.Logger
}

func NewHandler(store *Store, identities *identity.Store, sessions *session.Store, auditStore *audit.Store, logger *slog.Logger) (*Handler, error) {
	wa, err := newWebAuthn()
	if err != nil {
		return nil, err
	}
	return &Handler{
		store:      store,
		identities: identities,
		sessions:   sessions,
		audit:      auditStore,
		wa:         wa,
		logger:     logger,
	}, nil
}

// --- Registration (authenticated user adding a passkey) ---

func (h *Handler) BeginRegistration(c *gin.Context) {
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

	user, err := loadCredentials(c.Request.Context(), h.store, ident.ID, ident.Email, ident.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	creation, sessionData, err := h.wa.BeginRegistration(user)
	if err != nil {
		h.logger.Error("webauthn begin registration failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.store.saveWebAuthnSession(c.Request.Context(), "reg:"+sess.IdentityID.String(), sessionData); err != nil {
		h.logger.Error("failed to store webauthn registration session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, creation)
}

type finishRegistrationRequest struct {
	FriendlyName string `json:"friendly_name"`
}

func (h *Handler) FinishRegistration(c *gin.Context) {
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

	sessionData, err := h.store.loadAndDeleteWebAuthnSession(c.Request.Context(), "reg:"+sess.IdentityID.String())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no registration in progress"})
		return
	}

	user, err := loadCredentials(c.Request.Context(), h.store, ident.ID, ident.Email, ident.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	credential, err := h.wa.FinishRegistration(user, *sessionData, c.Request)
	if err != nil {
		h.logger.Error("webauthn finish registration failed", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "registration failed"})
		return
	}

	// Parse friendly_name from query or JSON body
	friendlyName := c.Query("friendly_name")
	if friendlyName == "" {
		var body finishRegistrationRequest
		// Try to parse from a wrapper, but the main body is the credential response,
		// so we get it from the query param instead.
		_ = json.NewDecoder(c.Request.Body).Decode(&body)
		friendlyName = body.FriendlyName
	}
	var namePtr *string
	if friendlyName != "" {
		namePtr = &friendlyName
	}

	transports := make([]string, len(credential.Transport))
	for i, t := range credential.Transport {
		transports[i] = string(t)
	}

	pk, err := h.store.Create(c.Request.Context(), CreateParams{
		IdentityID:     sess.IdentityID,
		CredentialID:   credential.ID,
		PublicKey:      credential.PublicKey,
		AAGUID:         credential.Authenticator.AAGUID,
		SignCount:      int64(credential.Authenticator.SignCount),
		BackupEligible: credential.Flags.BackupEligible,
		BackupState:    credential.Flags.BackupState,
		Transports:     transports,
		FriendlyName:   namePtr,
	})
	if err != nil {
		h.logger.Error("failed to store passkey", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionPasskeyRegistered,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, pk.Public())
}

// --- Login with passkey (primary auth) ---

func (h *Handler) BeginLogin(c *gin.Context) {
	assertion, sessionData, err := h.wa.BeginDiscoverableLogin()
	if err != nil {
		h.logger.Error("webauthn begin login failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	key := "login:" + string(sessionData.Challenge)
	if err := h.store.saveWebAuthnSession(c.Request.Context(), key, sessionData); err != nil {
		h.logger.Error("failed to store webauthn login session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, assertion)
}

func (h *Handler) FinishLogin(c *gin.Context) {
	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credential response"})
		return
	}

	// Find the credential in our store
	pk, err := h.store.GetByCredentialID(c.Request.Context(), parsedResponse.RawID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unknown credential"})
		return
	}

	ident, err := h.identities.GetByID(c.Request.Context(), pk.IdentityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if ident.IsSuspended() {
		c.JSON(http.StatusForbidden, gin.H{"error": "account is suspended"})
		return
	}

	user, err := loadCredentials(c.Request.Context(), h.store, ident.ID, ident.Email, ident.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	key := "login:" + string(parsedResponse.Response.CollectedClientData.Challenge)
	sd, err := h.store.loadAndDeleteWebAuthnSession(c.Request.Context(), key)
	if err != nil {
		telemetry.RecordLogin(c.Request.Context(), "passkey", false)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication failed"})
		return
	}

	matchedCredential, err := h.wa.ValidateDiscoverableLogin(
		func(_, userHandle []byte) (webauthn.User, error) {
			return user, nil
		},
		*sd,
		parsedResponse,
	)
	if err != nil {
		h.logger.Warn("passkey login validation failed", "error", err)
	}

	if matchedCredential == nil {
		telemetry.RecordLogin(c.Request.Context(), "passkey", false)
		ip := c.ClientIP()
		h.audit.Log(c.Request.Context(), audit.LogParams{
			IdentityID: &ident.ID,
			Action:     audit.ActionLoginFailure,
			IPAddress:  ip,
			UserAgent:  c.GetHeader("User-Agent"),
			Metadata:   map[string]any{"auth_method": "passkey", "reason": "validation_failed"},
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication failed"})
		return
	}

	// Update sign count
	if err := h.store.UpdateSignCount(c.Request.Context(), pk.ID, int64(matchedCredential.Authenticator.SignCount)); err != nil {
		h.logger.Warn("failed to update sign count", "error", err)
	}

	// Passkey login skips 2FA → go straight to pre_tenant
	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")

	csrfToken, err := auth.GenerateCSRFToken()
	if err != nil {
		h.logger.Error("failed to generate CSRF token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	sess, err := h.sessions.Create(c.Request.Context(), session.CreateParams{
		IdentityID: ident.ID,
		State:      session.StatePreTenant,
		IPAddress:  ip,
		UserAgent:  ua,
		AuthMethod: "passkey",
		CSRFToken:  csrfToken,
	})
	if err != nil {
		h.logger.Error("failed to create session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	telemetry.RecordLogin(c.Request.Context(), "passkey", true)
	telemetry.RecordSessionCreated(c.Request.Context())

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &ident.ID,
		SessionID:  &sess.ID,
		Action:     audit.ActionLoginSuccess,
		IPAddress:  ip,
		UserAgent:  ua,
		Metadata:   map[string]any{"auth_method": "passkey"},
	})

	auth.SetSessionCookie(c, sess.ID)
	auth.SetCSRFCookiePublic(c, csrfToken)

	c.JSON(http.StatusOK, gin.H{"state": session.StatePreTenant})
}

// --- 2FA with passkey ---

func (h *Handler) Begin2FA(c *gin.Context) {
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

	user, err := loadCredentials(c.Request.Context(), h.store, ident.ID, ident.Email, ident.Name)
	if err != nil || len(user.credentials) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no passkeys configured"})
		return
	}

	assertion, sessionData, err := h.wa.BeginLogin(user)
	if err != nil {
		h.logger.Error("webauthn begin 2fa failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.store.saveWebAuthnSession(c.Request.Context(), "2fa:"+sess.ID.String(), sessionData); err != nil {
		h.logger.Error("failed to store webauthn 2fa session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, assertion)
}

func (h *Handler) Finish2FA(c *gin.Context) {
	sess, ok := auth.GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	sd, err := h.store.loadAndDeleteWebAuthnSession(c.Request.Context(), "2fa:"+sess.ID.String())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no 2FA challenge in progress"})
		return
	}

	// Reload credentials to reconstruct the webauthn user
	ident, err := h.identities.GetByID(c.Request.Context(), sess.IdentityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	user, err := loadCredentials(c.Request.Context(), h.store, ident.ID, ident.Email, ident.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	credential, err := h.wa.FinishLogin(user, *sd, c.Request)
	if err != nil {
		h.audit.Log(c.Request.Context(), audit.LogParams{
			IdentityID: &sess.IdentityID,
			SessionID:  &sess.ID,
			Action:     audit.Action2FAFailure,
			IPAddress:  c.ClientIP(),
			UserAgent:  c.GetHeader("User-Agent"),
			Metadata:   map[string]any{"method": "passkey", "reason": "validation_failed"},
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication failed"})
		return
	}

	// Find and update the passkey sign count
	pk, err := h.store.GetByCredentialID(c.Request.Context(), credential.ID)
	if err == nil {
		if updateErr := h.store.UpdateSignCount(c.Request.Context(), pk.ID, int64(credential.Authenticator.SignCount)); updateErr != nil {
			h.logger.Warn("failed to update sign count", "error", updateErr)
		}
	}

	if err := h.sessions.SetSecondFactor(c.Request.Context(), sess.ID, "passkey"); err != nil {
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
		Metadata:   map[string]any{"method": "passkey"},
	})

	c.JSON(http.StatusOK, gin.H{"state": session.StatePreTenant})
}

// --- Admin passkey management (super admin) ---

func (h *Handler) AdminListPasskeys(c *gin.Context) {
	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid identity ID"})
		return
	}

	passkeys, err := h.store.ListByIdentity(c.Request.Context(), identityID)
	if err != nil {
		h.logger.Error("failed to list passkeys for identity", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	infos := make([]PublicInfo, len(passkeys))
	for i, pk := range passkeys {
		infos[i] = pk.Public()
	}
	c.JSON(http.StatusOK, gin.H{"passkeys": infos})
}

func (h *Handler) AdminDeletePasskey(c *gin.Context) {
	sess, ok := auth.GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid identity ID"})
		return
	}

	passkeyID, err := uuid.Parse(c.Param("passkey_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid passkey ID"})
		return
	}

	if err := h.store.Delete(c.Request.Context(), passkeyID, identityID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "passkey not found"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionPasskeyRemovedByAdmin,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"passkey_id": passkeyID, "target_identity_id": identityID},
	})

	c.JSON(http.StatusOK, gin.H{"message": "passkey removed"})
}

// --- Passkey management (authenticated) ---

func (h *Handler) List(c *gin.Context) {
	sess, ok := auth.GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	passkeys, err := h.store.ListByIdentity(c.Request.Context(), sess.IdentityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	infos := make([]PublicInfo, len(passkeys))
	for i, pk := range passkeys {
		infos[i] = pk.Public()
	}
	c.JSON(http.StatusOK, gin.H{"passkeys": infos})
}

func (h *Handler) Remove(c *gin.Context) {
	sess, ok := auth.GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	passkeyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid passkey ID"})
		return
	}

	if err := h.store.Delete(c.Request.Context(), passkeyID, sess.IdentityID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "passkey not found"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionPasskeyRemoved,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"passkey_id": passkeyID},
	})

	c.JSON(http.StatusOK, gin.H{"message": "passkey removed"})
}
