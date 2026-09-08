//go:build integration

package audit_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Log(t *testing.T) {
	db := testutil.DB(t)
	logger := slog.Default()
	store := audit.NewStore(db, logger)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	// Log should not panic or error (it logs internally).
	store.Log(ctx, audit.LogParams{
		IdentityID: &identityID,
		Action:     audit.ActionLoginSuccess,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
		Metadata:   map[string]any{"method": "password"},
	})

	// Verify the entry was written.
	entries, err := store.Query(ctx, &identityID, 10, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, audit.ActionLoginSuccess, entries[0].Action)
	assert.Contains(t, entries[0].IPAddress, "127.0.0.1")
}

func TestStore_Query(t *testing.T) {
	db := testutil.DB(t)
	logger := slog.Default()
	store := audit.NewStore(db, logger)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	store.Log(ctx, audit.LogParams{
		IdentityID: &identityID,
		Action:     audit.ActionLoginSuccess,
		IPAddress:  "127.0.0.1",
	})
	store.Log(ctx, audit.LogParams{
		IdentityID: &identityID,
		Action:     audit.ActionLogout,
		IPAddress:  "127.0.0.1",
	})

	entries, err := store.Query(ctx, &identityID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestStore_Query_Pagination(t *testing.T) {
	db := testutil.DB(t)
	logger := slog.Default()
	store := audit.NewStore(db, logger)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	for i := 0; i < 5; i++ {
		store.Log(ctx, audit.LogParams{
			IdentityID: &identityID,
			Action:     audit.ActionLoginSuccess,
			IPAddress:  "127.0.0.1",
		})
	}

	entries, err := store.Query(ctx, &identityID, 2, 0)
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	entries, err = store.Query(ctx, &identityID, 10, 3)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestStore_Query_NoFilter(t *testing.T) {
	db := testutil.DB(t)
	logger := slog.Default()
	store := audit.NewStore(db, logger)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	store.Log(ctx, audit.LogParams{
		IdentityID: &identityID,
		Action:     audit.ActionLoginSuccess,
		IPAddress:  "127.0.0.1",
	})

	// Query with nil identityID returns all entries.
	entries, err := store.Query(ctx, nil, 10, 0)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestStore_Log_NilIdentity(t *testing.T) {
	db := testutil.DB(t)
	logger := slog.Default()
	store := audit.NewStore(db, logger)
	ctx := context.Background()

	store.Log(ctx, audit.LogParams{
		Action:    audit.ActionLoginAttempt,
		IPAddress: "127.0.0.1",
	})

	entries, err := store.Query(ctx, nil, 10, 0)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Nil(t, entries[0].IdentityID)
}

func TestStore_QueryByTenant(t *testing.T) {
	db := testutil.DB(t)
	logger := slog.Default()
	store := audit.NewStore(db, logger)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	tenantID := testutil.CreateTenant(t, db, "Acme")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "administrator")
	_ = userID

	store.Log(ctx, audit.LogParams{
		IdentityID: &identityID,
		TenantID:   &tenantID,
		Action:     audit.ActionLoginSuccess,
		IPAddress:  "10.1.1.1",
	})
	store.Log(ctx, audit.LogParams{
		IdentityID: &identityID,
		TenantID:   &tenantID,
		Action:     audit.ActionTaskSubmitted,
		IPAddress:  "10.1.1.1",
		Metadata:   map[string]any{"task_id": "abc"},
	})

	entries, total, err := store.QueryByTenant(ctx, tenantID, "", "", 50, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, entries, 2)
	assert.Equal(t, "Alice", entries[0].IdentityName)
	assert.Equal(t, "alice@example.com", entries[0].IdentityEmail)
}

func TestStore_QueryByTenant_ActionFilter(t *testing.T) {
	db := testutil.DB(t)
	logger := slog.Default()
	store := audit.NewStore(db, logger)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "bob@example.com")
	tenantID := testutil.CreateTenant(t, db, "Acme")

	store.Log(ctx, audit.LogParams{IdentityID: &identityID, TenantID: &tenantID, Action: audit.ActionLoginSuccess})
	store.Log(ctx, audit.LogParams{IdentityID: &identityID, TenantID: &tenantID, Action: audit.ActionLogout})

	entries, total, err := store.QueryByTenant(ctx, tenantID, audit.ActionLoginSuccess, "", 50, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, entries, 1)
	assert.Equal(t, audit.ActionLoginSuccess, entries[0].Action)
}

func TestStore_QueryByTenant_SearchFilter(t *testing.T) {
	db := testutil.DB(t)
	logger := slog.Default()
	store := audit.NewStore(db, logger)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "carol@example.com")
	tenantID := testutil.CreateTenant(t, db, "Acme")

	store.Log(ctx, audit.LogParams{IdentityID: &identityID, TenantID: &tenantID, Action: audit.ActionLoginSuccess, IPAddress: "192.168.1.1"})
	store.Log(ctx, audit.LogParams{IdentityID: &identityID, TenantID: &tenantID, Action: audit.ActionLogout, IPAddress: "10.0.0.1"})

	entries, total, err := store.QueryByTenant(ctx, tenantID, "", "carol", 50, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, entries, 2)

	entries, total, err = store.QueryByTenant(ctx, tenantID, "", "192.168", 50, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, entries, 1)
}

func TestStore_QueryByTenant_Isolation(t *testing.T) {
	db := testutil.DB(t)
	logger := slog.Default()
	store := audit.NewStore(db, logger)
	ctx := context.Background()

	idA := testutil.CreateIdentity(t, db, "alice@a.com")
	tenantA := testutil.CreateTenant(t, db, "Tenant A")
	idB := testutil.CreateIdentity(t, db, "bob@b.com")
	tenantB := testutil.CreateTenant(t, db, "Tenant B")

	store.Log(ctx, audit.LogParams{IdentityID: &idA, TenantID: &tenantA, Action: audit.ActionLoginSuccess})
	store.Log(ctx, audit.LogParams{IdentityID: &idB, TenantID: &tenantB, Action: audit.ActionLoginSuccess})

	entries, total, err := store.QueryByTenant(ctx, tenantA, "", "", 50, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, entries, 1)
	assert.Equal(t, "Alice", entries[0].IdentityName)
}

func TestStore_QueryByTenant_Pagination(t *testing.T) {
	db := testutil.DB(t)
	logger := slog.Default()
	store := audit.NewStore(db, logger)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "dave@example.com")
	tenantID := testutil.CreateTenant(t, db, "Acme")

	for range 5 {
		store.Log(ctx, audit.LogParams{IdentityID: &identityID, TenantID: &tenantID, Action: audit.ActionLoginSuccess})
	}

	entries, total, err := store.QueryByTenant(ctx, tenantID, "", "", 2, 0)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, entries, 2)

	entries, _, err = store.QueryByTenant(ctx, tenantID, "", "", 2, 4)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestStore_CountRecentByIP(t *testing.T) {
	db := testutil.DB(t)
	logger := slog.Default()
	store := audit.NewStore(db, logger)
	ctx := context.Background()

	// Log some entries from the same IP with different actions.
	store.Log(ctx, audit.LogParams{
		Action:    audit.ActionLoginAttempt,
		IPAddress: "10.0.0.1",
	})
	store.Log(ctx, audit.LogParams{
		Action:    audit.ActionLoginFailure,
		IPAddress: "10.0.0.1",
	})
	store.Log(ctx, audit.LogParams{
		Action:    audit.ActionLoginSuccess,
		IPAddress: "10.0.0.1",
	})
	// Different IP — should not be counted.
	store.Log(ctx, audit.LogParams{
		Action:    audit.ActionLoginAttempt,
		IPAddress: "10.0.0.2",
	})

	actions := []string{audit.ActionLoginAttempt, audit.ActionLoginFailure}
	count, err := store.CountRecentByIP(ctx, "10.0.0.1", actions, time.Now().Add(-1*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestStore_CountRecentByIP_WindowFiltering(t *testing.T) {
	db := testutil.DB(t)
	logger := slog.Default()
	store := audit.NewStore(db, logger)
	ctx := context.Background()

	store.Log(ctx, audit.LogParams{
		Action:    audit.ActionLoginAttempt,
		IPAddress: "10.0.0.1",
	})

	// With a "since" time in the future, nothing should match.
	actions := []string{audit.ActionLoginAttempt}
	count, err := store.CountRecentByIP(ctx, "10.0.0.1", actions, time.Now().Add(1*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestStore_QueryByTenant_IncludesTenantlessLogins verifies that login
// events written before tenant selection (login_attempt, login_success,
// login_failure — all of which have tenant_id=NULL) still surface in the
// tenant audit log when the identity belongs to that tenant. Without this,
// FSOs/admins reviewing their tenant's audit log would never see who logged
// in or who tried to.
func TestStore_QueryByTenant_IncludesTenantlessLogins(t *testing.T) {
	db := testutil.DB(t)
	logger := slog.Default()
	store := audit.NewStore(db, logger)
	ctx := context.Background()

	tenantA := testutil.CreateTenant(t, db, "TenantA")
	tenantB := testutil.CreateTenant(t, db, "TenantB")

	identityA := testutil.CreateIdentity(t, db, "alice@a.com")
	identityB := testutil.CreateIdentity(t, db, "bob@b.com")
	testutil.CreateUser(t, db, tenantA, identityA, "individual_contributor")
	testutil.CreateUser(t, db, tenantB, identityB, "individual_contributor")

	// login_success with tenant_id=NULL — but identity belongs to tenantA.
	store.Log(ctx, audit.LogParams{
		IdentityID: &identityA,
		Action:     audit.ActionLoginSuccess,
		IPAddress:  "10.0.0.1",
		Metadata:   map[string]any{"auth_method": "password"},
	})
	// Same shape but for tenantB's user — must NOT show up in tenantA's log.
	store.Log(ctx, audit.LogParams{
		IdentityID: &identityB,
		Action:     audit.ActionLoginSuccess,
		IPAddress:  "10.0.0.2",
	})
	// Anonymous login_attempt (no identity, no tenant) — must NOT show up.
	store.Log(ctx, audit.LogParams{
		Action:    audit.ActionLoginAttempt,
		IPAddress: "10.0.0.3",
		Metadata:  map[string]any{"email": "stranger@nowhere.com"},
	})
	// A regular tenant-scoped event for sanity.
	store.Log(ctx, audit.LogParams{
		IdentityID: &identityA,
		TenantID:   &tenantA,
		Action:     audit.ActionTaskCreated,
	})

	entries, total, err := store.QueryByTenant(ctx, tenantA, "", "", 50, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total, "tenantA should see its own login + task_created — not the other tenant's login or the anonymous attempt")

	got := map[string]bool{}
	for _, e := range entries {
		got[e.Action] = true
	}
	assert.True(t, got[audit.ActionLoginSuccess], "tenantless login_success for tenantA's user should surface")
	assert.True(t, got[audit.ActionTaskCreated], "regular tenant-scoped event still works")

	// Filtering by action should still work.
	entries, total, err = store.QueryByTenant(ctx, tenantA, audit.ActionLoginSuccess, "", 50, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, entries, 1)
	assert.Equal(t, audit.ActionLoginSuccess, entries[0].Action)
}
