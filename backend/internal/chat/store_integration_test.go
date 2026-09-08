//go:build integration

package chat

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SSpencer740/fastfso/backend/internal/testutil"
)

func TestStore_SaveAndGetHistory(t *testing.T) {
	db := testutil.DB(t)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "alice@acme.com")
	tenantID := testutil.CreateTenant(t, db, "Acme")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "individual_contributor")

	s := NewStore(db)

	require.NoError(t, s.SaveMessage(ctx, tenantID, userID, "user", "Hello"))
	require.NoError(t, s.SaveMessage(ctx, tenantID, userID, "assistant", "Hi there"))
	require.NoError(t, s.SaveMessage(ctx, tenantID, userID, "user", "How are you?"))

	msgs, err := s.GetHistory(ctx, tenantID, userID, 40)
	require.NoError(t, err)
	require.Len(t, msgs, 3)

	// Oldest first.
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "Hello", msgs[0].Content)
	assert.Equal(t, "assistant", msgs[1].Role)
	assert.Equal(t, "Hi there", msgs[1].Content)
	assert.Equal(t, "user", msgs[2].Role)
	assert.Equal(t, "How are you?", msgs[2].Content)
}

func TestStore_GetHistory_Limit(t *testing.T) {
	db := testutil.DB(t)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "bob@acme.com")
	tenantID := testutil.CreateTenant(t, db, "Acme")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "individual_contributor")

	s := NewStore(db)

	for i := range 5 {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		require.NoError(t, s.SaveMessage(ctx, tenantID, userID, role, "msg"))
	}

	msgs, err := s.GetHistory(ctx, tenantID, userID, 3)
	require.NoError(t, err)
	assert.Len(t, msgs, 3)
}

func TestStore_GetHistory_WrongTenant(t *testing.T) {
	db := testutil.DB(t)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "carol@acme.com")
	tenantID := testutil.CreateTenant(t, db, "Acme")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "individual_contributor")

	s := NewStore(db)
	require.NoError(t, s.SaveMessage(ctx, tenantID, userID, "user", "secret"))

	otherIdentityID := testutil.CreateIdentity(t, db, "dave@other.com")
	otherTenantID := testutil.CreateTenant(t, db, "Other Corp")
	otherUserID := testutil.CreateUser(t, db, otherTenantID, otherIdentityID, "individual_contributor")

	msgs, err := s.GetHistory(ctx, otherTenantID, otherUserID, 40)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestStore_CleanupOld(t *testing.T) {
	db := testutil.DB(t)
	ctx := context.Background()

	identityID := testutil.CreateIdentity(t, db, "eve@acme.com")
	tenantID := testutil.CreateTenant(t, db, "Acme")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "individual_contributor")

	s := NewStore(db)

	// Insert a recent message.
	require.NoError(t, s.SaveMessage(ctx, tenantID, userID, "user", "recent"))

	// Insert an old message directly (bypassing SaveMessage default NOW()).
	_, err := db.Exec(ctx, "test.insert_old",
		`INSERT INTO chat_messages (tenant_id, user_id, role, content, created_at)
		 VALUES ($1, $2, 'user', 'old message', NOW() - INTERVAL '31 days')`,
		tenantID, userID,
	)
	require.NoError(t, err)

	n, err := s.CleanupOld(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	msgs, err := s.GetHistory(ctx, tenantID, userID, 40)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "recent", msgs[0].Content)
}
