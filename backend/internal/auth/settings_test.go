package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/identity"
	"github.com/SSpencer740/fastfso/backend/internal/session"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing

type mockIdentityReader struct {
	identity        *identity.Identity
	err             error
	updatedPassword string
	updateErr       error
}

func (m *mockIdentityReader) GetByEmail(_ context.Context, _ string) (*identity.Identity, error) {
	return m.identity, m.err
}

func (m *mockIdentityReader) GetByID(_ context.Context, _ uuid.UUID) (*identity.Identity, error) {
	return m.identity, m.err
}

func (m *mockIdentityReader) Create(_ context.Context, _, _ string, _ *string) (*identity.Identity, error) {
	return m.identity, m.err
}

func (m *mockIdentityReader) UpdatePassword(_ context.Context, _ uuid.UUID, hash string) error {
	m.updatedPassword = hash
	return m.updateErr
}

func (m *mockIdentityReader) SetSuperAdmin(_ context.Context, _ uuid.UUID, _ bool) error {
	return m.updateErr
}

func (m *mockIdentityReader) Suspend(_ context.Context, _ uuid.UUID) error {
	return m.updateErr
}

func (m *mockIdentityReader) Unsuspend(_ context.Context, _ uuid.UUID) error {
	return m.updateErr
}

func (m *mockIdentityReader) CountActiveSuperAdmins(_ context.Context) (int, error) {
	return 0, nil
}

func (m *mockIdentityReader) Delete(_ context.Context, _ uuid.UUID) error {
	return m.updateErr
}

func (m *mockIdentityReader) UpdateName(_ context.Context, _ uuid.UUID, _ string) error {
	return m.updateErr
}

func (m *mockIdentityReader) UpdateEmail(_ context.Context, _ uuid.UUID, _ string) error {
	return m.updateErr
}

func (m *mockIdentityReader) Activate(_ context.Context, _ uuid.UUID) error {
	return m.updateErr
}

func (m *mockIdentityReader) RecordFailedLogin(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockIdentityReader) ClearFailedLogin(_ context.Context, _ uuid.UUID) error {
	return nil
}

type mockAuditLogger struct {
	logged []audit.LogParams
}

func (m *mockAuditLogger) Log(_ context.Context, p audit.LogParams) {
	m.logged = append(m.logged, p)
}

func (m *mockAuditLogger) Query(_ context.Context, _ *uuid.UUID, _, _ int) ([]audit.Entry, error) {
	return nil, nil
}

func (m *mockAuditLogger) QueryByTenant(_ context.Context, _ uuid.UUID, _, _ string, _, _ int) ([]audit.TenantEntry, int, error) {
	return nil, 0, nil
}

type mockTOTPChecker struct {
	hasVerified bool
	err         error
	validCode   string
}

func (m *mockTOTPChecker) HasVerifiedTOTP(_ context.Context, _ uuid.UUID) (bool, error) {
	return m.hasVerified, m.err
}

func (m *mockTOTPChecker) ValidateCode(_ context.Context, _ uuid.UUID, code string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	if !m.hasVerified {
		return false, nil
	}
	return code == m.validCode, nil
}

type mockPasskeyCounter struct {
	count int
	err   error
}

func (m *mockPasskeyCounter) CountByIdentity(_ context.Context, _ uuid.UUID) (int, error) {
	return m.count, m.err
}

type mockSessionManager struct {
	resetContextErr error
	resetContextID  uuid.UUID
}

func (m *mockSessionManager) Create(_ context.Context, _ session.CreateParams) (*session.Session, error) {
	return nil, nil
}
func (m *mockSessionManager) GetValid(_ context.Context, _ uuid.UUID) (*session.Session, error) {
	return nil, nil
}
func (m *mockSessionManager) SetTenant(_ context.Context, _, _, _ uuid.UUID, _ *uuid.UUID, _ string) error {
	return nil
}
func (m *mockSessionManager) SetSuperAdmin(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockSessionManager) ResetContext(_ context.Context, id uuid.UUID) error {
	m.resetContextID = id
	return m.resetContextErr
}
func (m *mockSessionManager) Revoke(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _ string) error {
	return nil
}
func (m *mockSessionManager) RevokeByTenant(_ context.Context, _ uuid.UUID, _ string) (int64, error) {
	return 0, nil
}
func (m *mockSessionManager) RevokeByIdentity(_ context.Context, _ uuid.UUID, _ string) (int64, error) {
	return 0, nil
}
func (m *mockSessionManager) RevokeByIdentityExcept(_ context.Context, _, _ uuid.UUID, _ string) (int64, error) {
	return 0, nil
}
func (m *mockSessionManager) ListByIdentity(_ context.Context, _ uuid.UUID) ([]session.Session, error) {
	return nil, nil
}

type mockAuthStore struct {
	tenants []TenantMembership
}

func (m *mockAuthStore) GetUserWithTenant(_ context.Context, _ uuid.UUID) (UserWithTenant, error) {
	return UserWithTenant{}, nil
}
func (m *mockAuthStore) ListTenantsByIdentity(_ context.Context, _ uuid.UUID) ([]TenantMembership, error) {
	return m.tenants, nil
}
func (m *mockAuthStore) GetUserInTenant(_ context.Context, _, _ uuid.UUID) (UserWithTenant, error) {
	return UserWithTenant{}, nil
}
func (m *mockAuthStore) GetUserRole(_ context.Context, _ uuid.UUID) (string, error) {
	return "", nil
}
func (m *mockAuthStore) ListSSOByIdentity(_ context.Context, _ uuid.UUID) ([]SSOOption, error) {
	return nil, nil
}
func (m *mockAuthStore) ListSSOByEmailDomain(_ context.Context, _ string) ([]SSOOption, error) {
	return nil, nil
}
func (m *mockAuthStore) HasEnabledSSODomain(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	return false, nil
}
func (m *mockAuthStore) ListAllActiveSessions(_ context.Context, _, _ int) ([]AdminSessionRow, error) {
	return nil, nil
}
func (m *mockAuthStore) ListAllTenants(_ context.Context) ([]AdminTenantRow, error) {
	return nil, nil
}
func (m *mockAuthStore) CreateTenant(_ context.Context, _ string) (AdminTenantRow, error) {
	return AdminTenantRow{}, nil
}
func (m *mockAuthStore) GetTenantDetail(_ context.Context, _ uuid.UUID) (AdminTenantDetail, error) {
	return AdminTenantDetail{}, nil
}
func (m *mockAuthStore) UpdateTenant(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (m *mockAuthStore) ListAllIdentities(_ context.Context, _, _ int) ([]AdminIdentityRow, error) {
	return nil, nil
}
func (m *mockAuthStore) GetIdentityDetail(_ context.Context, _ uuid.UUID) (AdminIdentityDetail, error) {
	return AdminIdentityDetail{}, nil
}
func (m *mockAuthStore) AddIdentityToTenant(_ context.Context, _, _ uuid.UUID, _ string) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (m *mockAuthStore) DeleteTenant(_ context.Context, _ uuid.UUID) error    { return nil }
func (m *mockAuthStore) SuspendTenant(_ context.Context, _ uuid.UUID) error   { return nil }
func (m *mockAuthStore) UnsuspendTenant(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockAuthStore) IsTenantSuspended(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}
func (m *mockAuthStore) ListTenantMembers(_ context.Context, _ uuid.UUID, _ string) ([]TenantMemberRow, error) {
	return nil, nil
}
func (m *mockAuthStore) UpdateMemberRole(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockAuthStore) RemoveMember(_ context.Context, _, _ uuid.UUID) error { return nil }

func setupTestHandler(identities identityReader, auditLog auditLogger, totpStore totpChecker, passkeys passkeyCounter) *Handler {
	return &Handler{
		identities: identities,
		audit:      auditLog,
		totpStore:  totpStore,
		passkeys:   passkeys,
		logger:     slog.Default(),
	}
}

func setupGinContext(t *testing.T, method, path string, body any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	if body != nil {
		err := json.NewEncoder(&buf).Encode(body)
		require.NoError(t, err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, &buf)
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func setSession(c *gin.Context, sess *session.Session) {
	c.Set(sessionContextKey, sess)
}

func TestChangePassword_Success(t *testing.T) {
	hash, err := HashPassword("oldpassword")
	require.NoError(t, err)

	identityID := uuid.New()
	sessionID := uuid.New()

	mockIdent := &mockIdentityReader{
		identity: &identity.Identity{
			ID:           identityID,
			Email:        "user@test.com",
			PasswordHash: &hash,
		},
	}
	mockAudit := &mockAuditLogger{}
	h := setupTestHandler(mockIdent, mockAudit, &mockTOTPChecker{}, &mockPasskeyCounter{})

	c, w := setupGinContext(t, "POST", "/api/auth/change-password", changePasswordRequest{
		CurrentPassword: "oldpassword",
		NewPassword:     "newpassword123",
	})
	setSession(c, &session.Session{ID: sessionID, IdentityID: identityID})

	h.ChangePassword(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, mockIdent.updatedPassword)
	assert.True(t, CheckPassword(mockIdent.updatedPassword, "newpassword123"))
	require.Len(t, mockAudit.logged, 1)
	assert.Equal(t, audit.ActionPasswordChanged, mockAudit.logged[0].Action)
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	hash, err := HashPassword("realpassword")
	require.NoError(t, err)

	mockIdent := &mockIdentityReader{
		identity: &identity.Identity{
			ID:           uuid.New(),
			PasswordHash: &hash,
		},
	}
	h := setupTestHandler(mockIdent, &mockAuditLogger{}, &mockTOTPChecker{}, &mockPasskeyCounter{})

	c, w := setupGinContext(t, "POST", "/api/auth/change-password", changePasswordRequest{
		CurrentPassword: "wrongpassword",
		NewPassword:     "newpassword123",
	})
	setSession(c, &session.Session{ID: uuid.New(), IdentityID: uuid.New()})

	h.ChangePassword(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestChangePassword_TooShortNewPassword(t *testing.T) {
	h := setupTestHandler(&mockIdentityReader{}, &mockAuditLogger{}, &mockTOTPChecker{}, &mockPasskeyCounter{})

	c, w := setupGinContext(t, "POST", "/api/auth/change-password", map[string]string{
		"current_password": "oldpassword",
		"new_password":     "short",
	})
	setSession(c, &session.Session{ID: uuid.New(), IdentityID: uuid.New()})

	h.ChangePassword(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChangePassword_MissingFields(t *testing.T) {
	h := setupTestHandler(&mockIdentityReader{}, &mockAuditLogger{}, &mockTOTPChecker{}, &mockPasskeyCounter{})

	c, w := setupGinContext(t, "POST", "/api/auth/change-password", map[string]string{})
	setSession(c, &session.Session{ID: uuid.New(), IdentityID: uuid.New()})

	h.ChangePassword(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChangePassword_NoPasswordAccount(t *testing.T) {
	mockIdent := &mockIdentityReader{
		identity: &identity.Identity{
			ID:           uuid.New(),
			PasswordHash: nil,
		},
	}
	h := setupTestHandler(mockIdent, &mockAuditLogger{}, &mockTOTPChecker{}, &mockPasskeyCounter{})

	c, w := setupGinContext(t, "POST", "/api/auth/change-password", changePasswordRequest{
		CurrentPassword: "anything",
		NewPassword:     "newpassword123",
	})
	setSession(c, &session.Session{ID: uuid.New(), IdentityID: uuid.New()})

	h.ChangePassword(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSwitchContext_Success(t *testing.T) {
	sessionID := uuid.New()
	identityID := uuid.New()
	tenantID := uuid.New()

	mockSessions := &mockSessionManager{}
	mockAudit := &mockAuditLogger{}
	h := &Handler{
		sessions: mockSessions,
		audit:    mockAudit,
		logger:   slog.Default(),
	}

	c, w := setupGinContext(t, "POST", "/api/auth/switch-context", nil)
	setSession(c, &session.Session{
		ID:         sessionID,
		IdentityID: identityID,
		TenantID:   &tenantID,
	})

	h.SwitchContext(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, sessionID, mockSessions.resetContextID)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "pre_tenant", resp["state"])

	require.Len(t, mockAudit.logged, 1)
	assert.Equal(t, audit.ActionContextSwitch, mockAudit.logged[0].Action)
}

func TestMe_CanSwitchContext(t *testing.T) {
	identityID := uuid.New()
	sessionID := uuid.New()
	tenantID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name          string
		isSuperAdmin  bool
		tenantCount   int
		wantCanSwitch bool
	}{
		{"super admin with 1 tenant", true, 1, true},
		{"super admin with 0 tenants", true, 0, false},
		{"regular user with 2 tenants", false, 2, true},
		{"regular user with 1 tenant", false, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenants := make([]TenantMembership, tt.tenantCount)
			for i := range tenants {
				tenants[i] = TenantMembership{TenantID: uuid.New(), Name: "Tenant", Role: "user"}
			}

			mockStore := &mockAuthStore{tenants: tenants}
			mockIdent := &mockIdentityReader{
				identity: &identity.Identity{
					ID:           identityID,
					Email:        "user@test.com",
					IsSuperAdmin: tt.isSuperAdmin,
				},
			}
			h := &Handler{
				store:      mockStore,
				identities: mockIdent,
				audit:      &mockAuditLogger{},
				logger:     slog.Default(),
			}

			c, w := setupGinContext(t, "GET", "/api/auth/me", nil)
			setSession(c, &session.Session{
				ID:         sessionID,
				IdentityID: identityID,
				TenantID:   &tenantID,
				UserID:     &userID,
			})

			h.Me(c)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCanSwitch, resp["can_switch_context"])
		})
	}
}

func TestSecurityOverview(t *testing.T) {
	hash, err := HashPassword("password")
	require.NoError(t, err)

	identityID := uuid.New()
	mockIdent := &mockIdentityReader{
		identity: &identity.Identity{
			ID:           identityID,
			PasswordHash: &hash,
		},
	}
	h := setupTestHandler(mockIdent, &mockAuditLogger{}, &mockTOTPChecker{hasVerified: true}, &mockPasskeyCounter{count: 2})

	c, w := setupGinContext(t, "GET", "/api/auth/security", nil)
	setSession(c, &session.Session{ID: uuid.New(), IdentityID: identityID})

	h.SecurityOverview(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, true, resp["has_password"])
	assert.Equal(t, true, resp["has_totp"])
	assert.Equal(t, float64(2), resp["passkey_count"])
}
