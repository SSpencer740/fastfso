//go:build integration

package suborg_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastfso/fastfso/backend/internal/suborg"
	"github.com/fastfso/fastfso/backend/internal/testutil"
)

func TestStore_SetPrimaryFSO(t *testing.T) {
	db := testutil.DB(t)
	store := suborg.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	subOrgID := testutil.CreateSubOrg(t, db, tenantID, "Engineering")
	identityID := testutil.CreateIdentity(t, db, "fso@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "fso")

	t.Run("set primary FSO", func(t *testing.T) {
		err := store.SetPrimaryFSO(ctx, tenantID, subOrgID, &userID)
		require.NoError(t, err)

		orgs, err := store.List(ctx, tenantID)
		require.NoError(t, err)
		require.Len(t, orgs, 2) // Engineering + Default (created by CreateTenant)

		var eng suborg.SubOrg
		for _, o := range orgs {
			if o.ID == subOrgID {
				eng = o
			}
		}
		assert.Equal(t, &userID, eng.PrimaryFSOUserID)
		assert.NotNil(t, eng.PrimaryFSOName)
		assert.NotNil(t, eng.PrimaryFSOEmail)
		assert.Equal(t, "fso@example.com", *eng.PrimaryFSOEmail)
	})

	t.Run("clear primary FSO", func(t *testing.T) {
		err := store.SetPrimaryFSO(ctx, tenantID, subOrgID, nil)
		require.NoError(t, err)

		orgs, err := store.List(ctx, tenantID)
		require.NoError(t, err)

		var eng suborg.SubOrg
		for _, o := range orgs {
			if o.ID == subOrgID {
				eng = o
			}
		}
		assert.Nil(t, eng.PrimaryFSOUserID)
	})

	t.Run("not found", func(t *testing.T) {
		otherTenantID := testutil.CreateTenant(t, db, "Other Corp")
		err := store.SetPrimaryFSO(ctx, otherTenantID, subOrgID, &userID)
		assert.ErrorIs(t, err, suborg.ErrNotFound)
	})
}

func TestStore_GetPrimaryFSO(t *testing.T) {
	db := testutil.DB(t)
	store := suborg.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	subOrgID := testutil.CreateSubOrg(t, db, tenantID, "Engineering")

	fsoIdentityID := testutil.CreateIdentity(t, db, "fso@example.com")
	fsoUserID := testutil.CreateUser(t, db, tenantID, fsoIdentityID, "fso")

	icIdentityID := testutil.CreateIdentity(t, db, "ic@example.com")
	icUserID := testutil.CreateUser(t, db, tenantID, icIdentityID, "individual_contributor")
	testutil.AssignUserToSubOrg(t, db, icUserID, subOrgID)

	t.Run("returns contact when FSO assigned", func(t *testing.T) {
		require.NoError(t, store.SetPrimaryFSO(ctx, tenantID, subOrgID, &fsoUserID))

		contact, err := store.GetPrimaryFSO(ctx, tenantID, icUserID)
		require.NoError(t, err)
		assert.Equal(t, "fso@example.com", contact.Email)
	})

	t.Run("not found when no FSO set", func(t *testing.T) {
		require.NoError(t, store.SetPrimaryFSO(ctx, tenantID, subOrgID, nil))

		_, err := store.GetPrimaryFSO(ctx, tenantID, icUserID)
		assert.ErrorIs(t, err, suborg.ErrNotFound)
	})

	t.Run("not found when user has no sub-org", func(t *testing.T) {
		unassignedIdentityID := testutil.CreateIdentity(t, db, "lone@example.com")
		unassignedUserID := testutil.CreateUser(t, db, tenantID, unassignedIdentityID, "individual_contributor")

		_, err := store.GetPrimaryFSO(ctx, tenantID, unassignedUserID)
		assert.ErrorIs(t, err, suborg.ErrNotFound)
	})
}
