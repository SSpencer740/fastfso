package auth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPasswordResetter struct {
	token       string
	tokenID     uuid.UUID
	identityID  uuid.UUID
	validateErr error
	markUsedIDs []uuid.UUID
}

func (m *mockPasswordResetter) GeneratePasswordReset(_ context.Context, _ uuid.UUID) (string, error) {
	return m.token, nil
}

func (m *mockPasswordResetter) ValidatePasswordReset(_ context.Context, _ string) (uuid.UUID, uuid.UUID, error) {
	if m.validateErr != nil {
		return uuid.Nil, uuid.Nil, m.validateErr
	}
	return m.tokenID, m.identityID, nil
}

func (m *mockPasswordResetter) MarkUsed(_ context.Context, id uuid.UUID) error {
	m.markUsedIDs = append(m.markUsedIDs, id)
	return nil
}

func setupResetHandler(resets passwordResetter, sessions sessionManager, identities identityReader, totpStore totpChecker, audit auditLogger) *Handler {
	h := setupTestHandler(identities, audit, totpStore, nil)
	h.resets = resets
	h.sessions = sessions
	return h
}

func TestConfirmPasswordReset_NoTOTP_Succeeds(t *testing.T) {
	identityID := uuid.New()
	tokenID := uuid.New()

	resets := &mockPasswordResetter{tokenID: tokenID, identityID: identityID}
	identities := &mockIdentityReader{}
	totpStore := &mockTOTPChecker{hasVerified: false}
	audit := &mockAuditLogger{}
	sessions := &mockSessionManager{}
	h := setupResetHandler(resets, sessions, identities, totpStore, audit)

	c, w := setupGinContext(t, "POST", "/reset/confirm", map[string]string{
		"token":    "raw-token",
		"password": "new-password",
	})
	h.ConfirmPasswordReset(c)

	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, []uuid.UUID{tokenID}, resets.markUsedIDs)
	assert.NotEmpty(t, identities.updatedPassword)
}

func TestConfirmPasswordReset_TOTPEnrolled_MissingCode_Requires2FA(t *testing.T) {
	identityID := uuid.New()
	tokenID := uuid.New()

	resets := &mockPasswordResetter{tokenID: tokenID, identityID: identityID}
	identities := &mockIdentityReader{}
	totpStore := &mockTOTPChecker{hasVerified: true, validCode: "123456"}
	audit := &mockAuditLogger{}
	sessions := &mockSessionManager{}
	h := setupResetHandler(resets, sessions, identities, totpStore, audit)

	c, w := setupGinContext(t, "POST", "/reset/confirm", map[string]string{
		"token":    "raw-token",
		"password": "new-password",
	})
	h.ConfirmPasswordReset(c)

	require.Equal(t, 401, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["requires_2fa"])
	assert.Empty(t, resets.markUsedIDs, "token must not be consumed without 2FA")
	assert.Empty(t, identities.updatedPassword, "password must not be changed without 2FA")
}

func TestConfirmPasswordReset_TOTPEnrolled_InvalidCode_Rejects(t *testing.T) {
	identityID := uuid.New()
	tokenID := uuid.New()

	resets := &mockPasswordResetter{tokenID: tokenID, identityID: identityID}
	identities := &mockIdentityReader{}
	totpStore := &mockTOTPChecker{hasVerified: true, validCode: "123456"}
	audit := &mockAuditLogger{}
	sessions := &mockSessionManager{}
	h := setupResetHandler(resets, sessions, identities, totpStore, audit)

	c, w := setupGinContext(t, "POST", "/reset/confirm", map[string]string{
		"token":     "raw-token",
		"password":  "new-password",
		"totp_code": "000000",
	})
	h.ConfirmPasswordReset(c)

	require.Equal(t, 401, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["requires_2fa"])
	assert.Empty(t, resets.markUsedIDs)
	assert.Empty(t, identities.updatedPassword)

	// An audit event should have been recorded for the failed attempt.
	var sawFailure bool
	for _, entry := range audit.logged {
		if entry.Action == "2fa_failure" {
			sawFailure = true
		}
	}
	assert.True(t, sawFailure, "expected a 2fa_failure audit entry")
}

func TestConfirmPasswordReset_TOTPEnrolled_ValidCode_Succeeds(t *testing.T) {
	identityID := uuid.New()
	tokenID := uuid.New()

	resets := &mockPasswordResetter{tokenID: tokenID, identityID: identityID}
	identities := &mockIdentityReader{}
	totpStore := &mockTOTPChecker{hasVerified: true, validCode: "123456"}
	audit := &mockAuditLogger{}
	sessions := &mockSessionManager{}
	h := setupResetHandler(resets, sessions, identities, totpStore, audit)

	c, w := setupGinContext(t, "POST", "/reset/confirm", map[string]string{
		"token":     "raw-token",
		"password":  "new-password",
		"totp_code": "123456",
	})
	h.ConfirmPasswordReset(c)

	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, []uuid.UUID{tokenID}, resets.markUsedIDs)
	assert.NotEmpty(t, identities.updatedPassword)
}

func TestConfirmPasswordReset_InvalidToken_Rejects(t *testing.T) {
	resets := &mockPasswordResetter{validateErr: errors.New("expired")}
	identities := &mockIdentityReader{}
	totpStore := &mockTOTPChecker{}
	audit := &mockAuditLogger{}
	sessions := &mockSessionManager{}
	h := setupResetHandler(resets, sessions, identities, totpStore, audit)

	c, w := setupGinContext(t, "POST", "/reset/confirm", map[string]string{
		"token":    "bad-token",
		"password": "new-password",
	})
	h.ConfirmPasswordReset(c)

	assert.Equal(t, 400, w.Code)
	assert.Empty(t, identities.updatedPassword)
}

func TestConfirmPasswordReset_ShortPassword_Rejects(t *testing.T) {
	resets := &mockPasswordResetter{}
	identities := &mockIdentityReader{}
	totpStore := &mockTOTPChecker{}
	audit := &mockAuditLogger{}
	sessions := &mockSessionManager{}
	h := setupResetHandler(resets, sessions, identities, totpStore, audit)

	c, w := setupGinContext(t, "POST", "/reset/confirm", map[string]string{
		"token":    "raw",
		"password": "short",
	})
	h.ConfirmPasswordReset(c)

	assert.Equal(t, 400, w.Code)
}
