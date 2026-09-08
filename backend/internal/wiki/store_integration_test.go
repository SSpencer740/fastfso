//go:build integration

package wiki_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SSpencer740/fastfso/backend/internal/database"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
	"github.com/SSpencer740/fastfso/backend/internal/wiki"
)

func setPrimaryFSO(t *testing.T, db database.DB, subOrgID, userID uuid.UUID) {
	t.Helper()
	_, err := db.Exec(context.Background(), "test.setPrimary",
		`UPDATE suborganizations SET primary_fso_user_id = $1 WHERE id = $2`,
		userID, subOrgID,
	)
	require.NoError(t, err)
}

// TestStore_GetFSO_PrefersSubOrgPrimary asserts that the wiki "Your FSO" tile
// resolves through the viewer's sub-org primary FSO, not just the oldest
// tenant-wide FSO. Before this fix, every IC saw the same name regardless of
// which sub-org they belonged to.
func TestStore_GetFSO_PrefersSubOrgPrimary(t *testing.T) {
	db := testutil.DB(t)
	store := wiki.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	subA := testutil.CreateSubOrg(t, db, tenantID, "SubA")
	subB := testutil.CreateSubOrg(t, db, tenantID, "SubB")

	// Two FSOs in the tenant. fsoA is older — pre-fix, every IC would see them.
	fsoAIdent := testutil.CreateIdentity(t, db, "fso-a@acme.com")
	fsoA := testutil.CreateUser(t, db, tenantID, fsoAIdent, "fso")
	fsoBIdent := testutil.CreateIdentity(t, db, "fso-b@acme.com")
	fsoB := testutil.CreateUser(t, db, tenantID, fsoBIdent, "fso")

	setPrimaryFSO(t, db, subA, fsoA)
	setPrimaryFSO(t, db, subB, fsoB)

	// Viewer in SubA sees fsoA.
	got, err := store.GetFSO(ctx, tenantID, &subA)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "fso-a@acme.com", got.Email)

	// Viewer in SubB sees fsoB — this is the case that was broken before.
	got, err = store.GetFSO(ctx, tenantID, &subB)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "fso-b@acme.com", got.Email)
}

// TestStore_GetFSO_FallsBackWhenNoPrimary asserts that a sub-org with no
// primary FSO assigned still returns *some* FSO — the oldest tenant FSO —
// rather than nil. We don't want the "Your FSO" tile to read "None" just
// because primary-FSO assignment was skipped during onboarding.
func TestStore_GetFSO_FallsBackWhenNoPrimary(t *testing.T) {
	db := testutil.DB(t)
	store := wiki.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	subA := testutil.CreateSubOrg(t, db, tenantID, "SubA")

	fsoIdent := testutil.CreateIdentity(t, db, "lone-fso@acme.com")
	testutil.CreateUser(t, db, tenantID, fsoIdent, "fso")
	// SubA has no primary_fso_user_id set.

	got, err := store.GetFSO(ctx, tenantID, &subA)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "lone-fso@acme.com", got.Email)
}

// TestStore_GetFSO_NilSubOrgUsesFallback asserts that callers without a
// session sub-org (admin context, weird state) still get a sensible answer.
func TestStore_GetFSO_NilSubOrgUsesFallback(t *testing.T) {
	db := testutil.DB(t)
	store := wiki.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	fsoIdent := testutil.CreateIdentity(t, db, "fso@acme.com")
	testutil.CreateUser(t, db, tenantID, fsoIdent, "fso")

	got, err := store.GetFSO(ctx, tenantID, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "fso@acme.com", got.Email)
}

// TestStore_GetFSO_NoFSOReturnsNil asserts that a tenant with no FSO at all
// (and no primary set on the viewer's sub-org) returns nil rather than an
// error. The frontend renders an "no FSO assigned" message in that case.
func TestStore_GetFSO_NoFSOReturnsNil(t *testing.T) {
	db := testutil.DB(t)
	store := wiki.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	subA := testutil.CreateSubOrg(t, db, tenantID, "SubA")
	// Tenant has no FSO-role users at all.

	got, err := store.GetFSO(ctx, tenantID, &subA)
	require.NoError(t, err)
	assert.Nil(t, got)
}
