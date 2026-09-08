//go:build integration

package passkey_test

import (
	"context"
	"testing"

	"github.com/SSpencer740/fastfso/backend/internal/passkey"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Create(t *testing.T) {
	db := testutil.DB(t)
	store := passkey.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	name := "My Key"
	pk, err := store.Create(ctx, passkey.CreateParams{
		IdentityID:     identityID,
		CredentialID:   []byte("cred-id-1"),
		PublicKey:      []byte("pub-key-1"),
		AAGUID:         []byte("aaguid-1"),
		SignCount:      0,
		BackupEligible: true,
		BackupState:    true,
		Transports:     []string{"usb", "nfc"},
		FriendlyName:   &name,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, pk.ID)
	assert.Equal(t, identityID, pk.IdentityID)
	assert.Equal(t, []byte("cred-id-1"), pk.CredentialID)
	assert.Equal(t, []byte("pub-key-1"), pk.PublicKey)
	assert.True(t, pk.BackupEligible)
	assert.True(t, pk.BackupState)
	require.NotNil(t, pk.FriendlyName)
	assert.Equal(t, "My Key", *pk.FriendlyName)
}

func TestStore_GetByCredentialID(t *testing.T) {
	db := testutil.DB(t)
	store := passkey.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	created, err := store.Create(ctx, passkey.CreateParams{
		IdentityID:     identityID,
		CredentialID:   []byte("cred-id-1"),
		PublicKey:      []byte("pub-key-1"),
		SignCount:      0,
		BackupEligible: true,
		BackupState:    false,
	})
	require.NoError(t, err)

	got, err := store.GetByCredentialID(ctx, []byte("cred-id-1"))
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.True(t, got.BackupEligible)
	assert.False(t, got.BackupState)
}

func TestStore_GetByCredentialID_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := passkey.NewStore(db)
	ctx := context.Background()

	_, err := store.GetByCredentialID(ctx, []byte("nonexistent"))
	assert.ErrorIs(t, err, passkey.ErrNotFound)
}

func TestStore_ListByIdentity(t *testing.T) {
	db := testutil.DB(t)
	store := passkey.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	_, err := store.Create(ctx, passkey.CreateParams{
		IdentityID:     identityID,
		CredentialID:   []byte("cred-1"),
		PublicKey:      []byte("key-1"),
		SignCount:      0,
		BackupEligible: true,
		BackupState:    true,
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, passkey.CreateParams{
		IdentityID:     identityID,
		CredentialID:   []byte("cred-2"),
		PublicKey:      []byte("key-2"),
		SignCount:      0,
		BackupEligible: false,
		BackupState:    false,
	})
	require.NoError(t, err)

	passkeys, err := store.ListByIdentity(ctx, identityID)
	require.NoError(t, err)
	assert.Len(t, passkeys, 2)
	assert.True(t, passkeys[0].BackupEligible)
	assert.True(t, passkeys[0].BackupState)
	assert.False(t, passkeys[1].BackupEligible)
	assert.False(t, passkeys[1].BackupState)
}

func TestStore_UpdateSignCount(t *testing.T) {
	db := testutil.DB(t)
	store := passkey.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	pk, err := store.Create(ctx, passkey.CreateParams{
		IdentityID:   identityID,
		CredentialID: []byte("cred-1"),
		PublicKey:    []byte("key-1"),
		SignCount:    0,
	})
	require.NoError(t, err)

	err = store.UpdateSignCount(ctx, pk.ID, 42)
	require.NoError(t, err)

	got, err := store.GetByCredentialID(ctx, []byte("cred-1"))
	require.NoError(t, err)
	assert.Equal(t, int64(42), got.SignCount)
	assert.NotNil(t, got.LastUsedAt)
}

func TestStore_CountByIdentity(t *testing.T) {
	db := testutil.DB(t)
	store := passkey.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	count, err := store.CountByIdentity(ctx, identityID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	_, err = store.Create(ctx, passkey.CreateParams{
		IdentityID:   identityID,
		CredentialID: []byte("cred-1"),
		PublicKey:    []byte("key-1"),
		SignCount:    0,
	})
	require.NoError(t, err)

	count, err = store.CountByIdentity(ctx, identityID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestStore_Delete(t *testing.T) {
	db := testutil.DB(t)
	store := passkey.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	pk, err := store.Create(ctx, passkey.CreateParams{
		IdentityID:   identityID,
		CredentialID: []byte("cred-1"),
		PublicKey:    []byte("key-1"),
		SignCount:    0,
	})
	require.NoError(t, err)

	err = store.Delete(ctx, pk.ID, identityID)
	require.NoError(t, err)

	_, err = store.GetByCredentialID(ctx, []byte("cred-1"))
	assert.ErrorIs(t, err, passkey.ErrNotFound)
}

func TestStore_Delete_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := passkey.NewStore(db)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@example.com")

	err := store.Delete(ctx, uuid.New(), identityID)
	assert.ErrorIs(t, err, passkey.ErrNotFound)
}
