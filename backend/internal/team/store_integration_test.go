//go:build integration

package team_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SSpencer740/fastfso/backend/internal/team"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
)

func ptr[T any](v T) *T { return &v }

func TestStore_ListAndStats(t *testing.T) {
	db := testutil.DB(t)
	store := team.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	subOrgEng := testutil.CreateSubOrg(t, db, tenantID, "Engineering")
	subOrgOps := testutil.CreateSubOrg(t, db, tenantID, "Operations")

	idAlice := testutil.CreateIdentity(t, db, "alice@acme.com")
	idBob := testutil.CreateIdentity(t, db, "bob@acme.com")
	idCarol := testutil.CreateIdentity(t, db, "carol@acme.com")

	userAlice := testutil.CreateUser(t, db, tenantID, idAlice, "fso")
	userBob := testutil.CreateUser(t, db, tenantID, idBob, "individual_contributor")
	userCarol := testutil.CreateUser(t, db, tenantID, idCarol, "individual_contributor")

	testutil.AssignUserToSubOrg(t, db, userAlice, subOrgEng)
	testutil.AssignUserToSubOrg(t, db, userBob, subOrgEng)
	testutil.AssignUserToSubOrg(t, db, userCarol, subOrgOps)

	// Give Alice a TS clearance, Bob a Secret with investigation due in 30 days,
	// leave Carol with no record.
	_, err := store.SetClearance(ctx, tenantID, userAlice, userAlice, team.SetClearanceParams{
		Clearance:             "top_secret",
		InvestigationType:     ptr("T5"),
		LastInvestigationDate: ptr("2023-01-01"),
		NextInvestigationDate: ptr("2028-01-01"),
	})
	require.NoError(t, err)

	soon := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	_, err = store.SetClearance(ctx, tenantID, userBob, userAlice, team.SetClearanceParams{
		Clearance:             "secret",
		InvestigationType:     ptr("T3"),
		NextInvestigationDate: ptr(soon),
	})
	require.NoError(t, err)

	t.Run("list returns all members", func(t *testing.T) {
		members, _, err := store.List(ctx, team.ListFilters{TenantID: tenantID})
		require.NoError(t, err)
		assert.Len(t, members, 3)
	})

	t.Run("sub-org scope filters to one sub-org", func(t *testing.T) {
		members, _, err := store.List(ctx, team.ListFilters{TenantID: tenantID, SubOrgScope: &subOrgEng})
		require.NoError(t, err)
		assert.Len(t, members, 2)
	})

	t.Run("clearance filter exact match", func(t *testing.T) {
		members, _, err := store.List(ctx, team.ListFilters{TenantID: tenantID, Clearance: "secret"})
		require.NoError(t, err)
		require.Len(t, members, 1)
		assert.Equal(t, "bob@acme.com", members[0].Email)
	})

	t.Run("none_or_missing matches users without a record", func(t *testing.T) {
		members, _, err := store.List(ctx, team.ListFilters{TenantID: tenantID, Clearance: "none_or_missing"})
		require.NoError(t, err)
		require.Len(t, members, 1)
		assert.Equal(t, "carol@acme.com", members[0].Email)
		assert.Empty(t, members[0].Clearance)
	})

	t.Run("due within days picks up bob", func(t *testing.T) {
		days := 90
		members, _, err := store.List(ctx, team.ListFilters{TenantID: tenantID, DueWithinDays: &days})
		require.NoError(t, err)
		require.Len(t, members, 1)
		assert.Equal(t, "bob@acme.com", members[0].Email)
	})

	t.Run("search by name", func(t *testing.T) {
		members, _, err := store.List(ctx, team.ListFilters{TenantID: tenantID, Search: "ali"})
		require.NoError(t, err)
		require.Len(t, members, 1)
		assert.Equal(t, "alice@acme.com", members[0].Email)
	})

	t.Run("pagination bounds the page and reports full total", func(t *testing.T) {
		// Members are ordered by name: alice, bob, carol.
		page1, total, err := store.List(ctx, team.ListFilters{TenantID: tenantID, Limit: 2, Offset: 0})
		require.NoError(t, err)
		assert.Equal(t, 3, total, "total counts all members, not just the page")
		require.Len(t, page1, 2)
		assert.Equal(t, "alice@acme.com", page1[0].Email)
		assert.Equal(t, "bob@acme.com", page1[1].Email)

		page2, total, err := store.List(ctx, team.ListFilters{TenantID: tenantID, Limit: 2, Offset: 2})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		require.Len(t, page2, 1)
		assert.Equal(t, "carol@acme.com", page2[0].Email)
	})

	t.Run("stats", func(t *testing.T) {
		stats, err := store.Stats(ctx, tenantID, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 3, stats.Members)
		assert.Equal(t, 1, stats.DueWithin90) // bob
		assert.Equal(t, 0, stats.Overdue)
	})
}

func TestStore_SetClearanceSupersedesPrior(t *testing.T) {
	db := testutil.DB(t)
	store := team.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	idAlice := testutil.CreateIdentity(t, db, "alice@acme.com")
	userAlice := testutil.CreateUser(t, db, tenantID, idAlice, "individual_contributor")

	// First record.
	r1, err := store.SetClearance(ctx, tenantID, userAlice, userAlice, team.SetClearanceParams{
		Clearance: "secret",
	})
	require.NoError(t, err)

	// Second record should supersede the first.
	r2, err := store.SetClearance(ctx, tenantID, userAlice, userAlice, team.SetClearanceParams{
		Clearance: "top_secret",
	})
	require.NoError(t, err)
	assert.NotEqual(t, r1.ID, r2.ID)

	detail, err := store.Get(ctx, tenantID, userAlice, nil)
	require.NoError(t, err)
	assert.Equal(t, "top_secret", detail.Clearance)
	require.Len(t, detail.History, 2)
	// Newest first
	assert.Equal(t, r2.ID, detail.History[0].ID)
	assert.Nil(t, detail.History[0].SupersededAt)
	assert.Equal(t, r1.ID, detail.History[1].ID)
	assert.NotNil(t, detail.History[1].SupersededAt)
}

func TestStore_SetClearanceInvalid(t *testing.T) {
	db := testutil.DB(t)
	store := team.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	idAlice := testutil.CreateIdentity(t, db, "alice@acme.com")
	userAlice := testutil.CreateUser(t, db, tenantID, idAlice, "individual_contributor")

	_, err := store.SetClearance(ctx, tenantID, userAlice, userAlice, team.SetClearanceParams{
		Clearance: "wat",
	})
	assert.ErrorIs(t, err, team.ErrInvalidClearance)

	_, err = store.SetClearance(ctx, tenantID, userAlice, userAlice, team.SetClearanceParams{
		Clearance:         "secret",
		InvestigationType: ptr("bogus"),
	})
	assert.ErrorIs(t, err, team.ErrInvalidInvestType)
}

func TestStore_GetScopeEnforcement(t *testing.T) {
	db := testutil.DB(t)
	store := team.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	subOrgEng := testutil.CreateSubOrg(t, db, tenantID, "Engineering")
	subOrgOps := testutil.CreateSubOrg(t, db, tenantID, "Operations")

	idA := testutil.CreateIdentity(t, db, "a@acme.com")
	idB := testutil.CreateIdentity(t, db, "b@acme.com")
	userA := testutil.CreateUser(t, db, tenantID, idA, "individual_contributor")
	userB := testutil.CreateUser(t, db, tenantID, idB, "individual_contributor")
	testutil.AssignUserToSubOrg(t, db, userA, subOrgEng)
	testutil.AssignUserToSubOrg(t, db, userB, subOrgOps)

	// In-scope read works.
	_, err := store.Get(ctx, tenantID, userA, &subOrgEng)
	require.NoError(t, err)

	// Out-of-scope read returns ErrNotFound (no info leak).
	_, err = store.Get(ctx, tenantID, userB, &subOrgEng)
	assert.ErrorIs(t, err, team.ErrNotFound)
}
