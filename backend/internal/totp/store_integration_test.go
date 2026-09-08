//go:build integration

package totp_test

import (
	"context"
	"testing"

	"github.com/SSpencer740/fastfso/backend/internal/testutil"
	"github.com/SSpencer740/fastfso/backend/internal/totp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Create(t *testing.T) {
	db := testutil.DB(t)
	store := totp.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	secret, err := store.Create(ctx, identityID, "JBSWY3DPEHPK3PXP")
	require.NoError(t, err)
	assert.Equal(t, identityID, secret.IdentityID)
	assert.Equal(t, "JBSWY3DPEHPK3PXP", secret.Secret)
	assert.False(t, secret.Verified)
}

func TestStore_Create_Upsert(t *testing.T) {
	db := testutil.DB(t)
	store := totp.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	_, err := store.Create(ctx, identityID, "OLD_SECRET")
	require.NoError(t, err)

	// Verify the old one.
	err = store.Verify(ctx, identityID)
	require.NoError(t, err)

	// Create again should reset verification.
	secret, err := store.Create(ctx, identityID, "NEW_SECRET")
	require.NoError(t, err)
	assert.Equal(t, "NEW_SECRET", secret.Secret)
	assert.False(t, secret.Verified)
}

func TestStore_GetByIdentity(t *testing.T) {
	db := testutil.DB(t)
	store := totp.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	_, err := store.Create(ctx, identityID, "JBSWY3DPEHPK3PXP")
	require.NoError(t, err)

	got, err := store.GetByIdentity(ctx, identityID)
	require.NoError(t, err)
	assert.Equal(t, "JBSWY3DPEHPK3PXP", got.Secret)
}

func TestStore_GetByIdentity_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := totp.NewStore(db)
	ctx := context.Background()

	_, err := store.GetByIdentity(ctx, uuid.New())
	assert.ErrorIs(t, err, totp.ErrNotFound)
}

func TestStore_HasVerifiedTOTP(t *testing.T) {
	db := testutil.DB(t)
	store := totp.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	has, err := store.HasVerifiedTOTP(ctx, identityID)
	require.NoError(t, err)
	assert.False(t, has)

	_, err = store.Create(ctx, identityID, "JBSWY3DPEHPK3PXP")
	require.NoError(t, err)

	has, err = store.HasVerifiedTOTP(ctx, identityID)
	require.NoError(t, err)
	assert.False(t, has)

	err = store.Verify(ctx, identityID)
	require.NoError(t, err)

	has, err = store.HasVerifiedTOTP(ctx, identityID)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestStore_Delete(t *testing.T) {
	db := testutil.DB(t)
	store := totp.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	_, err := store.Create(ctx, identityID, "JBSWY3DPEHPK3PXP")
	require.NoError(t, err)

	err = store.Delete(ctx, identityID)
	require.NoError(t, err)

	_, err = store.GetByIdentity(ctx, identityID)
	assert.ErrorIs(t, err, totp.ErrNotFound)
}

func TestStore_Delete_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := totp.NewStore(db)
	ctx := context.Background()

	err := store.Delete(ctx, uuid.New())
	assert.ErrorIs(t, err, totp.ErrNotFound)
}
