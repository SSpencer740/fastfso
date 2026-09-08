//go:build integration

package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/fastfso/fastfso/backend/internal/session"
	"github.com/fastfso/fastfso/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Create(t *testing.T) {
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	sess, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePre_Auth,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
		AuthMethod: "password",
		CSRFToken:  "csrf-token-123",
	})
	require.NoError(t, err)
	assert.NotZero(t, sess.ID)
	assert.Equal(t, identityID, sess.IdentityID)
	assert.Equal(t, session.StatePre_Auth, sess.State)
	assert.Equal(t, "password", sess.AuthMethod)
	assert.Equal(t, "csrf-token-123", sess.CSRFToken)
}

func TestStore_GetValid(t *testing.T) {
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	created, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePre_Auth,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
		AuthMethod: "password",
		CSRFToken:  "csrf-token-123",
	})
	require.NoError(t, err)

	got, err := store.GetValid(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, identityID, got.IdentityID)
}

func TestStore_UpdateState(t *testing.T) {
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	sess, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePre_Auth,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
		AuthMethod: "password",
		CSRFToken:  "csrf-123",
	})
	require.NoError(t, err)

	err = store.UpdateState(ctx, sess.ID, session.StatePreTenant)
	require.NoError(t, err)

	got, err := store.GetValid(ctx, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, session.StatePreTenant, got.State)
}

func TestStore_SetSecondFactor(t *testing.T) {
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	sess, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePre_Auth,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
		AuthMethod: "password",
		CSRFToken:  "csrf-123",
	})
	require.NoError(t, err)

	err = store.SetSecondFactor(ctx, sess.ID, "totp")
	require.NoError(t, err)

	got, err := store.GetValid(ctx, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, session.StatePreTenant, got.State)
	require.NotNil(t, got.SecondFactor)
	assert.Equal(t, "totp", *got.SecondFactor)
}

func TestStore_Revoke(t *testing.T) {
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	sess, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePre_Auth,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
		AuthMethod: "password",
		CSRFToken:  "csrf-123",
	})
	require.NoError(t, err)

	err = store.Revoke(ctx, sess.ID, nil, "test revoke")
	require.NoError(t, err)

	_, err = store.GetValid(ctx, sess.ID)
	assert.ErrorIs(t, err, session.ErrNotFound)
}

func TestStore_ListByIdentity(t *testing.T) {
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	_, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePre_Auth,
		IPAddress:  "127.0.0.1",
		UserAgent:  "agent-1",
		AuthMethod: "password",
		CSRFToken:  "csrf-1",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePreTenant,
		IPAddress:  "127.0.0.1",
		UserAgent:  "agent-2",
		AuthMethod: "passkey",
		CSRFToken:  "csrf-2",
	})
	require.NoError(t, err)

	sessions, err := store.ListByIdentity(ctx, identityID)
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
}

func TestStore_RevokeByTenant(t *testing.T) {
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	tenantID := testutil.CreateTenant(t, db, "Acme Corp")

	// Create a user in the tenant
	var userID string
	err := db.QueryRow(ctx, "test.createUser",
		`INSERT INTO users (identity_id, tenant_id, role) VALUES ($1, $2, 'administrator') RETURNING id`,
		identityID, tenantID,
	).Scan(&userID)
	require.NoError(t, err)

	// Create two sessions for this tenant
	sess1, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePreTenant,
		IPAddress:  "127.0.0.1",
		UserAgent:  "agent-1",
		AuthMethod: "password",
		CSRFToken:  "csrf-1",
	})
	require.NoError(t, err)

	// Set the tenant on the session
	err = store.SetTenant(ctx, sess1.ID, tenantID, uuid.MustParse(userID), nil, "fso")
	require.NoError(t, err)

	sess2, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePreTenant,
		IPAddress:  "127.0.0.1",
		UserAgent:  "agent-2",
		AuthMethod: "password",
		CSRFToken:  "csrf-2",
	})
	require.NoError(t, err)
	err = store.SetTenant(ctx, sess2.ID, tenantID, uuid.MustParse(userID), nil, "fso")
	require.NoError(t, err)

	// Bulk revoke
	count, err := store.RevokeByTenant(ctx, tenantID, "tenant_suspended")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Verify both sessions are invalid
	_, err = store.GetValid(ctx, sess1.ID)
	assert.ErrorIs(t, err, session.ErrNotFound)

	_, err = store.GetValid(ctx, sess2.ID)
	assert.ErrorIs(t, err, session.ErrNotFound)
}

func TestStore_RevokeByIdentity(t *testing.T) {
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	sess1, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePreTenant,
		IPAddress:  "127.0.0.1",
		UserAgent:  "agent-1",
		AuthMethod: "password",
		CSRFToken:  "csrf-1",
	})
	require.NoError(t, err)

	sess2, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePreTenant,
		IPAddress:  "127.0.0.1",
		UserAgent:  "agent-2",
		AuthMethod: "passkey",
		CSRFToken:  "csrf-2",
	})
	require.NoError(t, err)

	// Create a session for a different identity (should not be revoked)
	otherID := testutil.CreateIdentity(t, db, "bob@example.com")
	otherSess, err := store.Create(ctx, session.CreateParams{
		IdentityID: otherID,
		State:      session.StatePreTenant,
		IPAddress:  "127.0.0.1",
		UserAgent:  "agent-3",
		AuthMethod: "password",
		CSRFToken:  "csrf-3",
	})
	require.NoError(t, err)

	// Bulk revoke by identity
	count, err := store.RevokeByIdentity(ctx, identityID, "identity_suspended")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Verify both sessions are invalid
	_, err = store.GetValid(ctx, sess1.ID)
	assert.ErrorIs(t, err, session.ErrNotFound)

	_, err = store.GetValid(ctx, sess2.ID)
	assert.ErrorIs(t, err, session.ErrNotFound)

	// Other identity's session should still be valid
	_, err = store.GetValid(ctx, otherSess.ID)
	require.NoError(t, err)
}

func TestStore_ResetContext(t *testing.T) {
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	tenantID := testutil.CreateTenant(t, db, "Acme Corp")

	// Create a user in the tenant
	var userID uuid.UUID
	err := db.QueryRow(ctx, "test.createUser",
		`INSERT INTO users (identity_id, tenant_id, role) VALUES ($1, $2, 'administrator') RETURNING id`,
		identityID, tenantID,
	).Scan(&userID)
	require.NoError(t, err)

	// Create session and set tenant
	sess, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePreTenant,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
		AuthMethod: "password",
		CSRFToken:  "csrf-123",
	})
	require.NoError(t, err)

	err = store.SetTenant(ctx, sess.ID, tenantID, userID, nil, "fso")
	require.NoError(t, err)

	// Verify tenant is set
	got, err := store.GetValid(ctx, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, session.StateAuthenticated, got.State)
	assert.NotNil(t, got.TenantID)
	assert.NotNil(t, got.UserID)

	// Reset context
	err = store.ResetContext(ctx, sess.ID)
	require.NoError(t, err)

	// Verify reset
	got, err = store.GetValid(ctx, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, session.StatePreTenant, got.State)
	assert.Nil(t, got.TenantID)
	assert.Nil(t, got.UserID)
	assert.False(t, got.IsSuperAdmin)
}

func TestStore_ResetContext_SuperAdmin(t *testing.T) {
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "admin@example.com")

	// Create session and set super admin
	sess, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePreTenant,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
		AuthMethod: "password",
		CSRFToken:  "csrf-123",
	})
	require.NoError(t, err)

	err = store.SetSuperAdmin(ctx, sess.ID)
	require.NoError(t, err)

	// Verify super admin is set
	got, err := store.GetValid(ctx, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, session.StateAuthenticated, got.State)
	assert.True(t, got.IsSuperAdmin)

	// Reset context
	err = store.ResetContext(ctx, sess.ID)
	require.NoError(t, err)

	// Verify reset
	got, err = store.GetValid(ctx, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, session.StatePreTenant, got.State)
	assert.Nil(t, got.TenantID)
	assert.Nil(t, got.UserID)
	assert.False(t, got.IsSuperAdmin)
}

func TestStore_TouchLastActive(t *testing.T) {
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	sess, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePre_Auth,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
		AuthMethod: "password",
		CSRFToken:  "csrf-123",
	})
	require.NoError(t, err)

	err = store.TouchLastActive(ctx, sess.ID)
	require.NoError(t, err)

	got, err := store.GetValid(ctx, sess.ID)
	require.NoError(t, err)
	assert.True(t, !got.LastActiveAt.Before(sess.LastActiveAt))
}

func TestStore_Create_PreAuthHasShortTTL(t *testing.T) {
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	preAuth, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePre_Auth,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test",
		AuthMethod: "password",
		CSRFToken:  "csrf",
	})
	require.NoError(t, err)

	setup, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StateSetup2FA,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test",
		AuthMethod: "password",
		CSRFToken:  "csrf",
	})
	require.NoError(t, err)

	preTenant, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePreTenant,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test",
		AuthMethod: "oidc",
		CSRFToken:  "csrf",
	})
	require.NoError(t, err)

	// Half-authenticated states get the short TTL (~15 min).
	assert.WithinDuration(t, time.Now().Add(15*time.Minute), preAuth.ExpiresAt, time.Minute,
		"pre_auth should expire ~15 minutes after creation")
	assert.WithinDuration(t, time.Now().Add(15*time.Minute), setup.ExpiresAt, time.Minute,
		"setup_2fa should expire ~15 minutes after creation")
	// Post-2FA states (pre_tenant, authenticated) keep the full 7-day TTL.
	assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), preTenant.ExpiresAt, time.Minute,
		"pre_tenant should retain the full 7-day TTL (already past 2FA)")
}

func TestStore_SetSecondFactor_ExtendsExpiry(t *testing.T) {
	// After 2FA, the session must extend its expiry past the short pre_auth
	// window. Without this, a user who completes 2FA at minute 14 would get
	// kicked out at minute 15 even though they just authenticated.
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	sess, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePre_Auth,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test",
		AuthMethod: "password",
		CSRFToken:  "csrf",
	})
	require.NoError(t, err)
	preAuthExpiry := sess.ExpiresAt

	require.NoError(t, store.SetSecondFactor(ctx, sess.ID, "totp"))

	got, err := store.GetValid(ctx, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, session.StatePreTenant, got.State)
	assert.True(t, got.ExpiresAt.After(preAuthExpiry.Add(time.Hour)),
		"expires_at must be extended well beyond the original pre_auth window")
	assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), got.ExpiresAt, time.Minute,
		"post-2FA session should have the full 7-day TTL")
}

func TestStore_RevokeByIdentityExcept(t *testing.T) {
	// Powers the "sign out everywhere else" flow — revokes every session
	// for the identity *except* the one issuing the request, so the user
	// stays logged in on the device they're currently using.
	db := testutil.DB(t)
	store := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	other := testutil.CreateIdentity(t, db, "bob@example.com")

	current, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePreTenant,
		IPAddress:  "127.0.0.1",
		UserAgent:  "current-device",
		AuthMethod: "password",
		CSRFToken:  "csrf-current",
	})
	require.NoError(t, err)

	otherDevice, err := store.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePreTenant,
		IPAddress:  "127.0.0.1",
		UserAgent:  "other-device",
		AuthMethod: "password",
		CSRFToken:  "csrf-other",
	})
	require.NoError(t, err)

	bobSession, err := store.Create(ctx, session.CreateParams{
		IdentityID: other,
		State:      session.StatePreTenant,
		IPAddress:  "127.0.0.1",
		UserAgent:  "bob-device",
		AuthMethod: "password",
		CSRFToken:  "csrf-bob",
	})
	require.NoError(t, err)

	count, err := store.RevokeByIdentityExcept(ctx, identityID, current.ID, "user_revoked_all")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Current session must still be valid.
	got, err := store.GetValid(ctx, current.ID)
	require.NoError(t, err)
	assert.Equal(t, current.ID, got.ID)

	// The other device of the same identity must be revoked.
	_, err = store.GetValid(ctx, otherDevice.ID)
	assert.ErrorIs(t, err, session.ErrNotFound)

	// A different identity's session must be untouched.
	got, err = store.GetValid(ctx, bobSession.ID)
	require.NoError(t, err)
	assert.Equal(t, bobSession.ID, got.ID)
}
