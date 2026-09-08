package chat

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/SSpencer740/fastfso/backend/internal/database"
)

type Message struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	UserID    uuid.UUID
	Role      string
	Content   string
	CreatedAt time.Time
}

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

func (s *Store) SaveMessage(ctx context.Context, tenantID, userID uuid.UUID, role, content string) error {
	_, err := s.db.Exec(ctx, "chat.SaveMessage",
		`INSERT INTO chat_messages (tenant_id, user_id, role, content) VALUES ($1, $2, $3, $4)`,
		tenantID, userID, role, content,
	)
	return err
}

// GetHistory returns the most recent messages within the last 30 days, oldest first.
func (s *Store) GetHistory(ctx context.Context, tenantID, userID uuid.UUID, limit int) ([]Message, error) {
	rows, err := s.db.Query(ctx, "chat.GetHistory",
		`SELECT id, tenant_id, user_id, role, content, created_at
		 FROM chat_messages
		 WHERE tenant_id = $1 AND user_id = $2 AND created_at > NOW() - INTERVAL '30 days'
		 ORDER BY created_at DESC
		 LIMIT $3`,
		tenantID, userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	msgs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Message, error) {
		var m Message
		return m, row.Scan(&m.ID, &m.TenantID, &m.UserID, &m.Role, &m.Content, &m.CreatedAt)
	})
	if err != nil {
		return nil, err
	}

	// Reverse so oldest is first.
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

func (s *Store) CleanupOld(ctx context.Context) (int64, error) {
	tag, err := s.db.Exec(ctx, "chat.CleanupOld",
		`DELETE FROM chat_messages WHERE created_at < NOW() - INTERVAL '30 days'`,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
