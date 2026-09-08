//go:build integration

package invite_test

import (
	"context"
	"testing"
	"time"

	"github.com/SSpencer740/fastfso/backend/internal/invite"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Generate(t *testing.T) {
	db := testutil.DB(t)
	store := invite.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	token, err := store.Generate(ctx, identityID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Len(t, token, 43) // 32 bytes base64url, no padding
}

func TestStore_Validate(t *testing.T) {
	db := testutil.DB(t)
	store := invite.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	token, err := store.Generate(ctx, identityID)
	require.NoError(t, err)

	tok, err := store.Validate(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, identityID, tok.IdentityID)
	assert.Nil(t, tok.UsedAt)
	assert.True(t, tok.ExpiresAt.After(time.Now()))
}

func TestStore_Validate_InvalidToken(t *testing.T) {
	db := testutil.DB(t)
	store := invite.NewStore(db)
	ctx := context.Background()

	_, err := store.Validate(ctx, "bogus-token")
	assert.ErrorIs(t, err, invite.ErrNotFound)
}

func TestStore_MarkUsed(t *testing.T) {
	db := testutil.DB(t)
	store := invite.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	token, err := store.Generate(ctx, identityID)
	require.NoError(t, err)

	tok, err := store.Validate(ctx, token)
	require.NoError(t, err)

	err = store.MarkUsed(ctx, tok.ID)
	require.NoError(t, err)

	// Token should no longer validate (used_at IS NULL filter)
	_, err = store.Validate(ctx, token)
	assert.ErrorIs(t, err, invite.ErrNotFound)
}

func TestStore_Generate_InvalidatesPrevious(t *testing.T) {
	db := testutil.DB(t)
	store := invite.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	token1, err := store.Generate(ctx, identityID)
	require.NoError(t, err)

	// Generate a second token — should invalidate the first
	token2, err := store.Generate(ctx, identityID)
	require.NoError(t, err)
	assert.NotEqual(t, token1, token2)

	// First token should be invalid
	_, err = store.Validate(ctx, token1)
	assert.ErrorIs(t, err, invite.ErrNotFound)

	// Second token should work
	tok, err := store.Validate(ctx, token2)
	require.NoError(t, err)
	assert.Equal(t, identityID, tok.IdentityID)
}

func TestStore_Validate_Expired(t *testing.T) {
	db := testutil.DB(t)
	store := invite.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	token, err := store.Generate(ctx, identityID)
	require.NoError(t, err)

	// Manually expire the token
	_, err = db.Exec(ctx, "test.ExpireToken",
		`UPDATE email_codes SET expires_at = now() - interval '1 hour'
		 WHERE identity_id = $1 AND purpose = 'invite' AND used_at IS NULL`,
		identityID,
	)
	require.NoError(t, err)

	_, err = store.Validate(ctx, token)
	assert.ErrorIs(t, err, invite.ErrExpired)
}
