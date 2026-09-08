//go:build integration

package auth_test

import (
	"context"
	"testing"

	"github.com/fastfso/fastfso/backend/internal/auth"
	"github.com/fastfso/fastfso/backend/internal/database"
	"github.com/fastfso/fastfso/backend/internal/session"
	"github.com/fastfso/fastfso/backend/internal/sso"
	"github.com/fastfso/fastfso/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createUser inserts a user row linking an identity to a tenant.
func createUser(t *testing.T, db database.DB, identityID, tenantID uuid.UUID, role string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRow(context.Background(), "test.createUser",
		`INSERT INTO users (identity_id, tenant_id, role)
		 VALUES ($1, $2, $3) RETURNING id`,
		identityID, tenantID, role,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func TestStore_CreateTenant(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Acme Corp")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, tenant.ID)
	assert.Equal(t, "Acme Corp", tenant.Name)
	assert.Equal(t, 0, tenant.UserCount)
}

func TestStore_ListAllTenants(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	_, err := store.CreateTenant(ctx, "Alpha")
	require.NoError(t, err)
	_, err = store.CreateTenant(ctx, "Beta")
	require.NoError(t, err)

	tenants, err := store.ListAllTenants(ctx)
	require.NoError(t, err)
	assert.Len(t, tenants, 2)
}

func TestStore_GetTenantDetail(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Acme Corp")
	require.NoError(t, err)

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	createUser(t, db, identityID, tenant.ID, "administrator")

	detail, err := store.GetTenantDetail(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, "Acme Corp", detail.Name)
	assert.Len(t, detail.Users, 1)
	assert.Equal(t, "alice@example.com", detail.Users[0].Email)
}

func TestStore_UpdateTenant(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Old Name")
	require.NoError(t, err)

	err = store.UpdateTenant(ctx, tenant.ID, "New Name")
	require.NoError(t, err)

	detail, err := store.GetTenantDetail(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", detail.Name)
}

func TestStore_DeleteTenant(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Doomed Corp")
	require.NoError(t, err)

	err = store.DeleteTenant(ctx, tenant.ID)
	require.NoError(t, err)

	// Deleted tenants should not appear in list.
	tenants, err := store.ListAllTenants(ctx)
	require.NoError(t, err)
	assert.Len(t, tenants, 0)
}

func TestStore_GetUserWithTenant(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	userID := createUser(t, db, identityID, tenantID, "fso")

	u, err := store.GetUserWithTenant(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, userID, u.UserID)
	assert.Equal(t, "Alice", u.UserName)
	assert.Equal(t, "fso", u.UserRole)
	assert.Equal(t, "Acme Corp", u.TenantName)
}

func TestStore_ListTenantsByIdentity(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	t1 := testutil.CreateTenant(t, db, "Alpha")
	t2 := testutil.CreateTenant(t, db, "Beta")
	createUser(t, db, identityID, t1, "administrator")
	createUser(t, db, identityID, t2, "fso")

	memberships, err := store.ListTenantsByIdentity(ctx, identityID)
	require.NoError(t, err)
	assert.Len(t, memberships, 2)
}

func TestStore_GetUserInTenant(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	userID := createUser(t, db, identityID, tenantID, "fso")

	u, err := store.GetUserInTenant(ctx, identityID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, userID, u.UserID)
}

func TestStore_GetUserRole(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	userID := createUser(t, db, identityID, tenantID, "administrator")

	role, err := store.GetUserRole(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "administrator", role)
}

func TestStore_AddIdentityToTenant(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	tenantID := testutil.CreateTenant(t, db, "Acme Corp")

	userID, err := store.AddIdentityToTenant(ctx, identityID, tenantID, "fso")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, userID, "should return the new user_id")

	memberships, err := store.ListTenantsByIdentity(ctx, identityID)
	require.NoError(t, err)
	assert.Len(t, memberships, 1)
	assert.Equal(t, "fso", memberships[0].Role)
}

func TestStore_ListAllIdentities(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	testutil.CreateIdentity(t, db, "alice@example.com")
	testutil.CreateIdentity(t, db, "bob@example.com")

	identities, err := store.ListAllIdentities(ctx, 100, 0)
	require.NoError(t, err)
	assert.Len(t, identities, 2)
}

func TestStore_GetIdentityDetail(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	createUser(t, db, identityID, tenantID, "administrator")

	detail, err := store.GetIdentityDetail(ctx, identityID)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", detail.Email)
	assert.Len(t, detail.Tenants, 1)
	assert.Equal(t, "Acme Corp", detail.Tenants[0].Name)
}

func TestStore_SuspendTenant(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Acme Corp")
	require.NoError(t, err)
	assert.Nil(t, tenant.SuspendedAt)

	err = store.SuspendTenant(ctx, tenant.ID)
	require.NoError(t, err)

	detail, err := store.GetTenantDetail(ctx, tenant.ID)
	require.NoError(t, err)
	assert.NotNil(t, detail.SuspendedAt)
}

func TestStore_UnsuspendTenant(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Acme Corp")
	require.NoError(t, err)

	err = store.SuspendTenant(ctx, tenant.ID)
	require.NoError(t, err)

	err = store.UnsuspendTenant(ctx, tenant.ID)
	require.NoError(t, err)

	detail, err := store.GetTenantDetail(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Nil(t, detail.SuspendedAt)
}

func TestStore_IsTenantSuspended(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Acme Corp")
	require.NoError(t, err)

	suspended, err := store.IsTenantSuspended(ctx, tenant.ID)
	require.NoError(t, err)
	assert.False(t, suspended)

	err = store.SuspendTenant(ctx, tenant.ID)
	require.NoError(t, err)

	suspended, err = store.IsTenantSuspended(ctx, tenant.ID)
	require.NoError(t, err)
	assert.True(t, suspended)
}

func TestStore_ListAllTenants_IncludesSuspended(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	_, err := store.CreateTenant(ctx, "Active Corp")
	require.NoError(t, err)
	suspended, err := store.CreateTenant(ctx, "Suspended Corp")
	require.NoError(t, err)

	err = store.SuspendTenant(ctx, suspended.ID)
	require.NoError(t, err)

	tenants, err := store.ListAllTenants(ctx)
	require.NoError(t, err)
	assert.Len(t, tenants, 2)

	for _, tenant := range tenants {
		if tenant.ID == suspended.ID {
			assert.NotNil(t, tenant.SuspendedAt)
		} else {
			assert.Nil(t, tenant.SuspendedAt)
		}
	}
}

func TestStore_ListTenantsByIdentity_ShowsSuspended(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	t1 := testutil.CreateTenant(t, db, "Active Corp")
	t2 := testutil.CreateTenant(t, db, "Suspended Corp")
	createUser(t, db, identityID, t1, "administrator")
	createUser(t, db, identityID, t2, "fso")

	// Suspend one tenant
	err := store.SuspendTenant(ctx, t2)
	require.NoError(t, err)

	memberships, err := store.ListTenantsByIdentity(ctx, identityID)
	require.NoError(t, err)
	assert.Len(t, memberships, 2)

	for _, m := range memberships {
		if m.TenantID == t2 {
			assert.True(t, m.Suspended)
		} else {
			assert.False(t, m.Suspended)
		}
	}
}

func TestStore_ListSSOByIdentity(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ssoStore := sso.NewStore(db)
	ctx := context.Background()

	// Create tenant with SSO enabled
	tenantID := testutil.CreateTenant(t, db, "acme-corp")
	identityID := testutil.CreateIdentity(t, db, "alice@acme.com")
	createUser(t, db, identityID, tenantID, "fso")

	_, err := ssoStore.Upsert(ctx, sso.UpsertParams{
		TenantID:    tenantID,
		Protocol:    "oidc",
		Enabled:     true,
		DefaultRole: "fso",
	})
	require.NoError(t, err)

	opts, err := store.ListSSOByIdentity(ctx, identityID)
	require.NoError(t, err)
	assert.Len(t, opts, 1)
	assert.Equal(t, "acme-corp", opts[0].TenantName)
	assert.Equal(t, "/api/auth/sso/acme-corp", opts[0].SSOURL)
}

func TestStore_ListSSOByIdentity_DisabledNotIncluded(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ssoStore := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "acme-corp")
	identityID := testutil.CreateIdentity(t, db, "alice@acme.com")
	createUser(t, db, identityID, tenantID, "fso")

	_, err := ssoStore.Upsert(ctx, sso.UpsertParams{
		TenantID:    tenantID,
		Protocol:    "oidc",
		Enabled:     false,
		DefaultRole: "fso",
	})
	require.NoError(t, err)

	opts, err := store.ListSSOByIdentity(ctx, identityID)
	require.NoError(t, err)
	assert.Empty(t, opts)
}

func TestStore_ListSSOByIdentity_SuspendedNotIncluded(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ssoStore := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "acme-corp")
	identityID := testutil.CreateIdentity(t, db, "alice@acme.com")
	createUser(t, db, identityID, tenantID, "fso")

	_, err := ssoStore.Upsert(ctx, sso.UpsertParams{
		TenantID:    tenantID,
		Protocol:    "oidc",
		Enabled:     true,
		DefaultRole: "fso",
	})
	require.NoError(t, err)

	err = store.SuspendTenant(ctx, tenantID)
	require.NoError(t, err)

	opts, err := store.ListSSOByIdentity(ctx, identityID)
	require.NoError(t, err)
	assert.Empty(t, opts)
}

func TestStore_ListSSOByEmailDomain(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ssoStore := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "acme-corp")
	cfg, err := ssoStore.Upsert(ctx, sso.UpsertParams{
		TenantID:    tenantID,
		Protocol:    "oidc",
		Enabled:     true,
		DefaultRole: "fso",
	})
	require.NoError(t, err)

	_, err = ssoStore.AddEmailDomain(ctx, cfg.ID, "acme.com")
	require.NoError(t, err)

	opts, err := store.ListSSOByEmailDomain(ctx, "acme.com")
	require.NoError(t, err)
	assert.Len(t, opts, 1)
	assert.Equal(t, "acme-corp", opts[0].TenantName)
	assert.Equal(t, "/api/auth/sso/acme-corp", opts[0].SSOURL)
}

func TestStore_ListSSOByEmailDomain_NoMatch(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	opts, err := store.ListSSOByEmailDomain(ctx, "unknown.com")
	require.NoError(t, err)
	assert.Empty(t, opts)
}

func TestStore_ListSSOByEmailDomain_DisabledNotIncluded(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ssoStore := sso.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "acme-corp")
	cfg, err := ssoStore.Upsert(ctx, sso.UpsertParams{
		TenantID:    tenantID,
		Protocol:    "oidc",
		Enabled:     false,
		DefaultRole: "fso",
	})
	require.NoError(t, err)

	_, err = ssoStore.AddEmailDomain(ctx, cfg.ID, "acme.com")
	require.NoError(t, err)

	opts, err := store.ListSSOByEmailDomain(ctx, "acme.com")
	require.NoError(t, err)
	assert.Empty(t, opts)
}

func TestStore_ListTenantMembers(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	alice := testutil.CreateIdentity(t, db, "alice@example.com")
	bob := testutil.CreateIdentity(t, db, "bob@example.com")
	createUser(t, db, alice, tenantID, "administrator")
	createUser(t, db, bob, tenantID, "fso")

	members, err := store.ListTenantMembers(ctx, tenantID, "")
	require.NoError(t, err)
	assert.Len(t, members, 2)

	// Search by email
	members, err = store.ListTenantMembers(ctx, tenantID, "alice")
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, "alice@example.com", members[0].Email)
	assert.Equal(t, "administrator", members[0].Role)
}

func TestStore_UpdateMemberRole(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	userID := createUser(t, db, identityID, tenantID, "fso")

	err := store.UpdateMemberRole(ctx, userID, tenantID, "administrator")
	require.NoError(t, err)

	role, err := store.GetUserRole(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "administrator", role)
}

func TestStore_RemoveMember(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	userID := createUser(t, db, identityID, tenantID, "fso")

	err := store.RemoveMember(ctx, userID, tenantID)
	require.NoError(t, err)

	members, err := store.ListTenantMembers(ctx, tenantID, "")
	require.NoError(t, err)
	assert.Empty(t, members)
}

func TestStore_ListAllActiveSessions(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	sessionStore := session.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	_, err := sessionStore.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePreTenant,
		IPAddress:  "10.0.0.1",
		UserAgent:  "test-agent",
		AuthMethod: "password",
		CSRFToken:  "csrf-1",
	})
	require.NoError(t, err)

	_, err = sessionStore.Create(ctx, session.CreateParams{
		IdentityID: identityID,
		State:      session.StatePreTenant,
		IPAddress:  "10.0.0.2",
		UserAgent:  "test-agent-2",
		AuthMethod: "passkey",
		CSRFToken:  "csrf-2",
	})
	require.NoError(t, err)

	sessions, err := store.ListAllActiveSessions(ctx, 50, 0)
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
	assert.Equal(t, identityID, sessions[0].IdentityID)
	assert.False(t, sessions[0].CreatedAt.IsZero())
	assert.False(t, sessions[0].LastActiveAt.IsZero())
	assert.False(t, sessions[0].ExpiresAt.IsZero())
}

// TestStore_RemoveMember_CascadesAndNullifies asserts that a tenant admin can
// remove a user even when that user has touched lots of related records:
// reviewed a task submission, was assigned an action item, was the creator of
// an active task that other ICs are working on, and has an active session.
// Before the FK cleanup migration these all blocked the DELETE; the handler
// then collapsed the FK violation into a misleading "member not found" 404.
func TestStore_RemoveMember_CascadesAndNullifies(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	adminIdent := testutil.CreateIdentity(t, db, "admin@acme.com")
	adminUser := testutil.CreateUser(t, db, tenantID, adminIdent, "administrator")
	icIdent := testutil.CreateIdentity(t, db, "ic@acme.com")
	icUser := testutil.CreateUser(t, db, tenantID, icIdent, "individual_contributor")

	// Active session for the user we're about to remove → must cascade.
	_, err := db.Exec(ctx, "test.createSession",
		`INSERT INTO sessions (identity_id, user_id, tenant_id, state, ip_address, expires_at, csrf_token, auth_method)
		 VALUES ($1, $2, $3, 'authenticated', '127.0.0.1', NOW() + INTERVAL '1 hour', 'csrf', 'password')`,
		adminIdent, adminUser, tenantID,
	)
	require.NoError(t, err)

	// A task created by the admin → SET NULL on remove (task survives).
	var taskID uuid.UUID
	err = db.QueryRow(ctx, "test.createTask",
		`INSERT INTO tasks (tenant_id, created_by_user_id, title, description, priority, status)
		 VALUES ($1, $2, 'Quarterly review', '', 'medium', 'active') RETURNING id`,
		tenantID, adminUser,
	).Scan(&taskID)
	require.NoError(t, err)

	// Task completion the admin reviewed → reviewed_by gets nulled.
	_, err = db.Exec(ctx, "test.createCompletion",
		`INSERT INTO task_completions (task_id, user_id, status, reviewed_by, reviewed_at)
		 VALUES ($1, $2, 'approved', $3, NOW())`,
		taskID, icUser, adminUser,
	)
	require.NoError(t, err)

	// Action item assigned to the admin → assigned_to gets nulled.
	var actionItemID uuid.UUID
	err = db.QueryRow(ctx, "test.createActionItem",
		`INSERT INTO action_items (tenant_id, source_type, title, description, priority, status, assigned_to)
		 VALUES ($1, 'task_submission', 'Review submission', '', 'medium', 'pending', $2)
		 RETURNING id`,
		tenantID, adminUser,
	).Scan(&actionItemID)
	require.NoError(t, err)

	// Now actually remove. This is the operation that previously failed.
	require.NoError(t, store.RemoveMember(ctx, adminUser, tenantID))

	// User row gone.
	var count int
	require.NoError(t, db.QueryRow(ctx, "test.countUser",
		`SELECT COUNT(*) FROM users WHERE id = $1`, adminUser).Scan(&count))
	assert.Equal(t, 0, count)

	// Session cascaded.
	require.NoError(t, db.QueryRow(ctx, "test.countSessions",
		`SELECT COUNT(*) FROM sessions WHERE user_id = $1`, adminUser).Scan(&count))
	assert.Equal(t, 0, count)

	// Task survives, creator nulled.
	var creator *uuid.UUID
	require.NoError(t, db.QueryRow(ctx, "test.getTaskCreator",
		`SELECT created_by_user_id FROM tasks WHERE id = $1`, taskID).Scan(&creator))
	assert.Nil(t, creator)

	// Completion survives, reviewer nulled.
	var reviewer *uuid.UUID
	require.NoError(t, db.QueryRow(ctx, "test.getCompletionReviewer",
		`SELECT reviewed_by FROM task_completions WHERE task_id = $1 AND user_id = $2`, taskID, icUser).Scan(&reviewer))
	assert.Nil(t, reviewer)

	// Action item survives, assignee nulled.
	var assignee *uuid.UUID
	require.NoError(t, db.QueryRow(ctx, "test.getActionItemAssignee",
		`SELECT assigned_to FROM action_items WHERE id = $1`, actionItemID).Scan(&assignee))
	assert.Nil(t, assignee)
}

func TestStore_RemoveMember_NotFoundReturnsTypedError(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	err := store.RemoveMember(ctx, uuid.New(), tenantID)
	assert.ErrorIs(t, err, auth.ErrMemberNotFound)
}

// TestStore_RemoveMember_OrphansIdentity asserts that removing the only
// tenant membership for a non-super-admin identity also wipes the identity
// itself (and cascades passkeys/totp/sessions/etc.). Without this, removed
// users could still log in but belong to no tenant — a phantom account.
func TestStore_RemoveMember_OrphansIdentity(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	identID := testutil.CreateIdentity(t, db, "single-tenant@acme.com")
	userID := testutil.CreateUser(t, db, tenantID, identID, "individual_contributor")

	require.NoError(t, store.RemoveMember(ctx, userID, tenantID))

	var count int
	require.NoError(t, db.QueryRow(ctx, "test.countIdentity",
		`SELECT COUNT(*) FROM identities WHERE id = $1`, identID).Scan(&count))
	assert.Equal(t, 0, count, "single-tenant identity should be deleted alongside the membership")
}

// TestStore_RemoveMember_PreservesMultiTenantIdentity asserts that an
// identity who belongs to multiple tenants stays around when removed from
// just one — they still need to log in and access the others.
func TestStore_RemoveMember_PreservesMultiTenantIdentity(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenantA := testutil.CreateTenant(t, db, "TenantA")
	tenantB := testutil.CreateTenant(t, db, "TenantB")
	identID := testutil.CreateIdentity(t, db, "consultant@example.com")
	userA := testutil.CreateUser(t, db, tenantA, identID, "individual_contributor")
	_ = testutil.CreateUser(t, db, tenantB, identID, "individual_contributor")

	require.NoError(t, store.RemoveMember(ctx, userA, tenantA))

	var count int
	require.NoError(t, db.QueryRow(ctx, "test.countIdentity",
		`SELECT COUNT(*) FROM identities WHERE id = $1`, identID).Scan(&count))
	assert.Equal(t, 1, count, "identity should survive — still has TenantB membership")

	require.NoError(t, db.QueryRow(ctx, "test.countOtherTenant",
		`SELECT COUNT(*) FROM users WHERE identity_id = $1 AND tenant_id = $2`,
		identID, tenantB).Scan(&count))
	assert.Equal(t, 1, count, "TenantB membership should be untouched")
}

// TestStore_RemoveMember_PreservesSuperAdmin asserts that a tenant admin
// removing a super admin from their own tenant doesn't nuke the super
// admin's identity. Super admin deletion is gated separately.
func TestStore_RemoveMember_PreservesSuperAdmin(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	identID := testutil.CreateIdentity(t, db, "superuser@example.com")
	_, err := db.Exec(ctx, "test.markSuperAdmin",
		`UPDATE identities SET is_super_admin = TRUE WHERE id = $1`, identID)
	require.NoError(t, err)
	userID := testutil.CreateUser(t, db, tenantID, identID, "administrator")

	require.NoError(t, store.RemoveMember(ctx, userID, tenantID))

	var count int
	require.NoError(t, db.QueryRow(ctx, "test.countSuperAdminIdentity",
		`SELECT COUNT(*) FROM identities WHERE id = $1`, identID).Scan(&count))
	assert.Equal(t, 1, count, "super admin identity must survive even with no tenant memberships")
}

// TestStore_HasEnabledSSODomain confirms the lookup that drives the SSO
// invite branch: case-insensitive domain match, scoped to the right tenant,
// and only when the SSO config is enabled.
func TestStore_HasEnabledSSODomain(t *testing.T) {
	db := testutil.DB(t)
	store := auth.NewStore(db)
	ctx := context.Background()

	tenantA := testutil.CreateTenant(t, db, "TenantA")
	tenantB := testutil.CreateTenant(t, db, "TenantB")

	// Tenant A: enabled SSO config with one domain.
	var ssoA uuid.UUID
	require.NoError(t, db.QueryRow(ctx, "test.ssoA",
		`INSERT INTO sso_configurations (tenant_id, protocol, enabled, auto_provision)
		 VALUES ($1, 'oidc', TRUE, FALSE) RETURNING id`, tenantA).Scan(&ssoA))
	_, err := db.Exec(ctx, "test.dA",
		`INSERT INTO sso_email_domains (sso_configuration_id, domain) VALUES ($1, $2)`, ssoA, "acme.com")
	require.NoError(t, err)

	// Tenant B: SSO config exists but is disabled. Domain is unique per
	// table, so we use a different one.
	var ssoB uuid.UUID
	require.NoError(t, db.QueryRow(ctx, "test.ssoB",
		`INSERT INTO sso_configurations (tenant_id, protocol, enabled, auto_provision)
		 VALUES ($1, 'oidc', FALSE, FALSE) RETURNING id`, tenantB).Scan(&ssoB))
	_, err = db.Exec(ctx, "test.dB",
		`INSERT INTO sso_email_domains (sso_configuration_id, domain) VALUES ($1, $2)`, ssoB, "disabled.com")
	require.NoError(t, err)

	matched, err := store.HasEnabledSSODomain(ctx, tenantA, "acme.com")
	require.NoError(t, err)
	assert.True(t, matched, "tenant A should match its enabled SSO domain")

	// Case-insensitive — admins routinely type the wrong case.
	matched, err = store.HasEnabledSSODomain(ctx, tenantA, "ACME.COM")
	require.NoError(t, err)
	assert.True(t, matched, "domain match should be case-insensitive")

	// Domain registered but config disabled → no match.
	matched, err = store.HasEnabledSSODomain(ctx, tenantB, "disabled.com")
	require.NoError(t, err)
	assert.False(t, matched, "disabled SSO config should not match")

	// Different domain.
	matched, err = store.HasEnabledSSODomain(ctx, tenantA, "other.com")
	require.NoError(t, err)
	assert.False(t, matched)

	// Scoped to the right tenant — same domain registered under a
	// different tenant must not leak.
	matched, err = store.HasEnabledSSODomain(ctx, testutil.CreateTenant(t, db, "Stranger"), "acme.com")
	require.NoError(t, err)
	assert.False(t, matched, "tenant scoping must prevent cross-tenant matches")
}
