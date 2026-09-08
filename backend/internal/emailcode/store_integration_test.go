//go:build integration

package emailcode_test

import (
	"context"
	"testing"

	"github.com/SSpencer740/fastfso/backend/internal/emailcode"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Generate(t *testing.T) {
	db := testutil.DB(t)
	store := emailcode.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	code, err := store.Generate(ctx, identityID, "2fa")
	require.NoError(t, err)
	assert.Len(t, code, 6)
}

func TestStore_Verify(t *testing.T) {
	db := testutil.DB(t)
	store := emailcode.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	code, err := store.Generate(ctx, identityID, "2fa")
	require.NoError(t, err)

	err = store.Verify(ctx, identityID, "2fa", code)
	assert.NoError(t, err)
}

func TestStore_Verify_Invalid(t *testing.T) {
	db := testutil.DB(t)
	store := emailcode.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	_, err := store.Generate(ctx, identityID, "2fa")
	require.NoError(t, err)

	err = store.Verify(ctx, identityID, "2fa", "000000")
	assert.ErrorIs(t, err, emailcode.ErrInvalid)
}

func TestStore_Verify_AlreadyUsed(t *testing.T) {
	db := testutil.DB(t)
	store := emailcode.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	code, err := store.Generate(ctx, identityID, "2fa")
	require.NoError(t, err)

	err = store.Verify(ctx, identityID, "2fa", code)
	require.NoError(t, err)

	err = store.Verify(ctx, identityID, "2fa", code)
	assert.ErrorIs(t, err, emailcode.ErrAlreadyUsed)
}

func TestStore_Generate_InvalidatesPrevious(t *testing.T) {
	db := testutil.DB(t)
	store := emailcode.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	firstCode, err := store.Generate(ctx, identityID, "2fa")
	require.NoError(t, err)

	secondCode, err := store.Generate(ctx, identityID, "2fa")
	require.NoError(t, err)

	// First code should be invalidated (marked as used).
	err = store.Verify(ctx, identityID, "2fa", firstCode)
	assert.Error(t, err)

	// Second code should still be valid.
	err = store.Verify(ctx, identityID, "2fa", secondCode)
	assert.NoError(t, err)
}
