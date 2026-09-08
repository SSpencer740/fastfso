//go:build integration

package identity_test

import (
	"context"
	"testing"

	"log/slog"

	"github.com/fastfso/fastfso/backend/internal/audit"
	"github.com/fastfso/fastfso/backend/internal/identity"
	"github.com/fastfso/fastfso/backend/internal/session"
	"github.com/fastfso/fastfso/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Create(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	ident, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", ident.Email)
	assert.Equal(t, "Alice", ident.Name)
	assert.Nil(t, ident.PasswordHash)
	assert.False(t, ident.IsSuperAdmin)
	assert.False(t, ident.Activated)
	assert.NotEqual(t, uuid.Nil, ident.ID)
	assert.NotZero(t, ident.CreatedAt)
}

func TestStore_Create_WithPassword(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	hash := "$2a$12$testhashvalue"
	ident, err := store.Create(ctx, "bob@example.com", "Bob", &hash)
	require.NoError(t, err)
	require.NotNil(t, ident.PasswordHash)
	assert.Equal(t, hash, *ident.PasswordHash)
}

func TestStore_Create_DuplicateEmail(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	_, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)

	_, err = store.Create(ctx, "alice@example.com", "Alice 2", nil)
	assert.Error(t, err)
}

func TestStore_Create_DuplicateEmailCaseInsensitive(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	_, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)

	_, err = store.Create(ctx, "Alice@Example.com", "Alice 2", nil)
	assert.Error(t, err)
}

func TestStore_GetByEmail_CaseInsensitive(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	created, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)

	got, err := store.GetByEmail(ctx, "Alice@Example.com")
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestStore_GetByEmail(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	created, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)

	got, err := store.GetByEmail(ctx, "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "alice@example.com", got.Email)
}

func TestStore_GetByEmail_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	_, err := store.GetByEmail(ctx, "nobody@example.com")
	assert.ErrorIs(t, err, identity.ErrNotFound)
}

func TestStore_GetByID(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	created, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)

	got, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "Alice", got.Name)
}

func TestStore_GetByID_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	_, err := store.GetByID(ctx, uuid.New())
	assert.ErrorIs(t, err, identity.ErrNotFound)
}

func TestStore_UpdatePassword(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	created, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)

	err = store.UpdatePassword(ctx, created.ID, "$2a$12$newhash")
	require.NoError(t, err)

	got, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, got.PasswordHash)
	assert.Equal(t, "$2a$12$newhash", *got.PasswordHash)
}

func TestStore_UpdatePassword_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	err := store.UpdatePassword(ctx, uuid.New(), "$2a$12$hash")
	assert.ErrorIs(t, err, identity.ErrNotFound)
}

func TestStore_SetSuperAdmin(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	created, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)
	assert.False(t, created.IsSuperAdmin)

	err = store.SetSuperAdmin(ctx, created.ID, true)
	require.NoError(t, err)

	got, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, got.IsSuperAdmin)

	// Revoke
	err = store.SetSuperAdmin(ctx, created.ID, false)
	require.NoError(t, err)

	got, err = store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, got.IsSuperAdmin)
}

func TestStore_SetSuperAdmin_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	err := store.SetSuperAdmin(ctx, uuid.New(), true)
	assert.ErrorIs(t, err, identity.ErrNotFound)
}

func TestStore_Suspend(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	created, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)
	assert.Nil(t, created.SuspendedAt)
	assert.False(t, created.IsSuspended())

	err = store.Suspend(ctx, created.ID)
	require.NoError(t, err)

	got, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.NotNil(t, got.SuspendedAt)
	assert.True(t, got.IsSuspended())
}

func TestStore_Suspend_AlreadySuspended(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	created, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)

	err = store.Suspend(ctx, created.ID)
	require.NoError(t, err)

	// Suspending again returns not found (0 rows affected)
	err = store.Suspend(ctx, created.ID)
	assert.ErrorIs(t, err, identity.ErrNotFound)
}

func TestStore_Unsuspend(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	created, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)

	err = store.Suspend(ctx, created.ID)
	require.NoError(t, err)

	err = store.Unsuspend(ctx, created.ID)
	require.NoError(t, err)

	got, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got.SuspendedAt)
	assert.False(t, got.IsSuspended())
}

func TestStore_Unsuspend_NotSuspended(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	created, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)

	err = store.Unsuspend(ctx, created.ID)
	assert.ErrorIs(t, err, identity.ErrNotFound)
}

func TestStore_Delete(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	created, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)

	err = store.Delete(ctx, created.ID)
	require.NoError(t, err)

	_, err = store.GetByID(ctx, created.ID)
	assert.ErrorIs(t, err, identity.ErrNotFound)
}

func TestStore_Delete_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	err := store.Delete(ctx, uuid.New())
	assert.ErrorIs(t, err, identity.ErrNotFound)
}

func TestStore_Delete_CascadesUsers(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	tenantID := testutil.CreateTenant(t, db, "Acme Corp")

	// Add identity as user in the tenant
	_, err := db.Exec(ctx, "test.AddUser",
		`INSERT INTO users (identity_id, tenant_id, role) VALUES ($1, $2, 'administrator')`,
		identityID, tenantID,
	)
	require.NoError(t, err)

	// Delete the identity
	err = store.Delete(ctx, identityID)
	require.NoError(t, err)

	// Verify the user row was cascaded
	var count int
	err = db.QueryRow(ctx, "test.CountUsers",
		`SELECT COUNT(*) FROM users WHERE identity_id = $1`, identityID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestStore_Delete_WithSessionAndAuditLog(t *testing.T) {
	db := testutil.DB(t)
	identityStore := identity.NewStore(db)
	sessionStore := session.NewStore(db)
	auditStore := audit.NewStore(db, slog.Default())
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	// Create a session for the identity.
	sess, err := sessionStore.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      "authenticated",
		IPAddress:  "127.0.0.1",
		UserAgent:  "test",
		AuthMethod: "password",
		CSRFToken:  "tok",
	})
	require.NoError(t, err)

	// Create an audit log entry referencing both the identity and session.
	auditStore.Log(ctx, audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		Action:     "login",
	})

	// Deleting the identity should succeed despite the audit log FK.
	err = identityStore.Delete(ctx, identityID)
	require.NoError(t, err)

	// Audit log row is preserved with NULLed references.
	var count int
	err = db.QueryRow(ctx, "test.CountAuditLogs",
		`SELECT COUNT(*) FROM auth_audit_log
		 WHERE identity_id IS NULL AND session_id IS NULL AND action = 'login'`,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestStore_Delete_EmailReusable(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	created, err := store.Create(ctx, "alice@example.com", "Alice", nil)
	require.NoError(t, err)

	err = store.Delete(ctx, created.ID)
	require.NoError(t, err)

	// Re-create with the same email should succeed
	reCreated, err := store.Create(ctx, "alice@example.com", "Alice Reborn", nil)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", reCreated.Email)
	assert.NotEqual(t, created.ID, reCreated.ID)
}

func TestStore_CountActiveSuperAdmins(t *testing.T) {
	db := testutil.DB(t)
	store := identity.NewStore(db)
	ctx := context.Background()

	// Initially zero
	count, err := store.CountActiveSuperAdmins(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Create two super admins
	a, err := store.Create(ctx, "admin1@example.com", "Admin1", nil)
	require.NoError(t, err)
	err = store.SetSuperAdmin(ctx, a.ID, true)
	require.NoError(t, err)

	b, err := store.Create(ctx, "admin2@example.com", "Admin2", nil)
	require.NoError(t, err)
	err = store.SetSuperAdmin(ctx, b.ID, true)
	require.NoError(t, err)

	count, err = store.CountActiveSuperAdmins(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Suspend one → count drops
	err = store.Suspend(ctx, a.ID)
	require.NoError(t, err)

	count, err = store.CountActiveSuperAdmins(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
