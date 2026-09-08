//go:build integration

package aiusage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SSpencer740/fastfso/backend/internal/aiusage"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
)

func TestStore_RecordAndCount(t *testing.T) {
	db := testutil.DB(t)
	store := aiusage.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "administrator")

	since := time.Now().Add(-1 * time.Hour)

	count, err := store.CountUserSince(ctx, userID, "chat", since)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	require.NoError(t, store.Record(ctx, tenantID, userID, "chat", 120, 340))
	require.NoError(t, store.Record(ctx, tenantID, userID, "chat", 80, 200))

	count, err = store.CountUserSince(ctx, userID, "chat", since)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestStore_CountUserSince_FeatureIsolation(t *testing.T) {
	db := testutil.DB(t)
	store := aiusage.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "bob@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "administrator")

	require.NoError(t, store.Record(ctx, tenantID, userID, "chat", 10, 20))
	require.NoError(t, store.Record(ctx, tenantID, userID, "travel_summary", 30, 40))

	since := time.Now().Add(-1 * time.Hour)

	chatCount, err := store.CountUserSince(ctx, userID, "chat", since)
	require.NoError(t, err)
	assert.Equal(t, 1, chatCount)

	summaryCount, err := store.CountUserSince(ctx, userID, "travel_summary", since)
	require.NoError(t, err)
	assert.Equal(t, 1, summaryCount)
}

func TestStore_CountUserSince_RespectsWindow(t *testing.T) {
	db := testutil.DB(t)
	store := aiusage.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "carol@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "administrator")

	require.NoError(t, store.Record(ctx, tenantID, userID, "chat", 10, 20))

	future := time.Now().Add(1 * time.Hour)
	count, err := store.CountUserSince(ctx, userID, "chat", future)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestStore_TenantUsageByFeature(t *testing.T) {
	db := testutil.DB(t)
	store := aiusage.NewStore(db)
	ctx := context.Background()

	tenantA := testutil.CreateTenant(t, db, "Tenant A")
	tenantB := testutil.CreateTenant(t, db, "Tenant B")
	idA := testutil.CreateIdentity(t, db, "a@example.com")
	idB := testutil.CreateIdentity(t, db, "b@example.com")
	userA := testutil.CreateUser(t, db, tenantA, idA, "administrator")
	userB := testutil.CreateUser(t, db, tenantB, idB, "administrator")

	require.NoError(t, store.Record(ctx, tenantA, userA, "chat", 100, 200))
	require.NoError(t, store.Record(ctx, tenantA, userA, "chat", 50, 75))
	require.NoError(t, store.Record(ctx, tenantA, userA, "travel_summary", 1000, 500))
	require.NoError(t, store.Record(ctx, tenantB, userB, "chat", 999, 999))

	since := time.Now().Add(-1 * time.Hour)
	rollup, err := store.TenantUsageByFeature(ctx, tenantA, since)
	require.NoError(t, err)
	require.Len(t, rollup, 2)

	assert.Equal(t, "chat", rollup[0].Feature)
	assert.Equal(t, int64(2), rollup[0].Calls)
	assert.Equal(t, int64(150), rollup[0].PromptTokens)
	assert.Equal(t, int64(275), rollup[0].ResponseTokens)

	assert.Equal(t, "travel_summary", rollup[1].Feature)
	assert.Equal(t, int64(1), rollup[1].Calls)
	assert.Equal(t, int64(1000), rollup[1].PromptTokens)
	assert.Equal(t, int64(500), rollup[1].ResponseTokens)
}

func TestStore_CountUserSince_UserIsolation(t *testing.T) {
	db := testutil.DB(t)
	store := aiusage.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityA := testutil.CreateIdentity(t, db, "a@example.com")
	identityB := testutil.CreateIdentity(t, db, "b@example.com")
	userA := testutil.CreateUser(t, db, tenantID, identityA, "administrator")
	userB := testutil.CreateUser(t, db, tenantID, identityB, "administrator")

	require.NoError(t, store.Record(ctx, tenantID, userA, "chat", 10, 20))
	require.NoError(t, store.Record(ctx, tenantID, userA, "chat", 10, 20))
	require.NoError(t, store.Record(ctx, tenantID, userB, "chat", 10, 20))

	since := time.Now().Add(-1 * time.Hour)

	countA, err := store.CountUserSince(ctx, userA, "chat", since)
	require.NoError(t, err)
	assert.Equal(t, 2, countA)

	countB, err := store.CountUserSince(ctx, userB, "chat", since)
	require.NoError(t, err)
	assert.Equal(t, 1, countB)
}
