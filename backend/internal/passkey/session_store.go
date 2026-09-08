package passkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
)

const webAuthnSessionTTL = 5 * time.Minute

var ErrSessionNotFound = errors.New("webauthn session not found or expired")

// saveWebAuthnSession serializes and stores a WebAuthn ceremony session.
func (s *Store) saveWebAuthnSession(ctx context.Context, id string, data *webauthn.SessionData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal webauthn session: %w", err)
	}
	_, err = s.db.Exec(ctx, "passkey.saveWebAuthnSession",
		`INSERT INTO webauthn_sessions (id, data, expires_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data, expires_at = EXCLUDED.expires_at`,
		id, b, time.Now().Add(webAuthnSessionTTL),
	)
	if err != nil {
		return fmt.Errorf("store webauthn session: %w", err)
	}
	return nil
}

// loadAndDeleteWebAuthnSession atomically retrieves and removes a session.
func (s *Store) loadAndDeleteWebAuthnSession(ctx context.Context, id string) (*webauthn.SessionData, error) {
	var b []byte
	err := s.db.QueryRow(ctx, "passkey.loadAndDeleteWebAuthnSession",
		`DELETE FROM webauthn_sessions
		 WHERE id = $1 AND expires_at > now()
		 RETURNING data`,
		id,
	).Scan(&b)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("load webauthn session: %w", err)
	}

	var sd webauthn.SessionData
	if err := json.Unmarshal(b, &sd); err != nil {
		return nil, fmt.Errorf("unmarshal webauthn session: %w", err)
	}
	return &sd, nil
}

// CleanupExpiredWebAuthnSessions removes expired ceremony sessions.
func (s *Store) CleanupExpiredWebAuthnSessions(ctx context.Context) error {
	_, err := s.db.Exec(ctx, "passkey.CleanupExpiredWebAuthnSessions",
		`DELETE FROM webauthn_sessions WHERE expires_at <= now()`,
	)
	if err != nil {
		return fmt.Errorf("cleanup webauthn sessions: %w", err)
	}
	return nil
}
