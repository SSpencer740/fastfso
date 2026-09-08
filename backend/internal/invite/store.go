package invite

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fastfso/fastfso/backend/internal/database"
)

var (
	ErrNotFound = errors.New("invite token not found")
	ErrExpired  = errors.New("invite token expired")
)

const tokenExpiry = 7 * 24 * time.Hour // 7 days
const resetExpiry = 1 * time.Hour      // 1 hour for password reset

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// Generate creates a secure invite token for the given identity.
// It invalidates any previous unused invite tokens for the same identity,
// inserts a new row in email_codes with purpose='invite', and returns the
// plaintext token (base64url, no padding).
func (s *Store) Generate(ctx context.Context, identityID uuid.UUID) (string, error) {
	raw, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate invite token: %w", err)
	}

	hash := hashToken(raw)

	// Invalidate any existing unused invite tokens for the same identity.
	_, _ = s.db.Exec(ctx, "invite.Generate.invalidate",
		`UPDATE email_codes SET used_at = now()
		 WHERE identity_id = $1 AND purpose = 'invite' AND used_at IS NULL`,
		identityID,
	)

	_, err = s.db.Exec(ctx, "invite.Generate.insert",
		`INSERT INTO email_codes (identity_id, code_hash, purpose, expires_at)
		 VALUES ($1, $2, 'invite', $3)`,
		identityID, hash, time.Now().Add(tokenExpiry),
	)
	if err != nil {
		return "", fmt.Errorf("store invite token: %w", err)
	}

	return raw, nil
}

// Validate looks up an invite token by its hash and checks validity.
// Returns the Token if valid, or ErrNotFound/ErrExpired.
func (s *Store) Validate(ctx context.Context, rawToken string) (*Token, error) {
	hash := hashToken(rawToken)

	var t Token
	err := s.db.QueryRow(ctx, "invite.Validate",
		`SELECT id, identity_id, expires_at, used_at FROM email_codes
		 WHERE code_hash = $1 AND purpose = 'invite' AND used_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		hash,
	).Scan(&t.ID, &t.IdentityID, &t.ExpiresAt, &t.UsedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("validate invite token: %w", err)
	}

	if time.Now().After(t.ExpiresAt) {
		return nil, ErrExpired
	}

	return &t, nil
}

// MarkUsed marks an invite token as used.
func (s *Store) MarkUsed(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, "invite.MarkUsed",
		`UPDATE email_codes SET used_at = now() WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("mark invite token used: %w", err)
	}
	return nil
}

// GeneratePasswordReset creates a secure token for a self-service password reset (1hr expiry).
func (s *Store) GeneratePasswordReset(ctx context.Context, identityID uuid.UUID) (string, error) {
	raw, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate password reset token: %w", err)
	}
	hash := hashToken(raw)

	_, _ = s.db.Exec(ctx, "invite.GeneratePasswordReset.invalidate",
		`UPDATE email_codes SET used_at = now()
		 WHERE identity_id = $1 AND purpose = 'password_reset' AND used_at IS NULL`,
		identityID,
	)

	_, err = s.db.Exec(ctx, "invite.GeneratePasswordReset.insert",
		`INSERT INTO email_codes (identity_id, code_hash, purpose, expires_at)
		 VALUES ($1, $2, 'password_reset', $3)`,
		identityID, hash, time.Now().Add(resetExpiry),
	)
	if err != nil {
		return "", fmt.Errorf("store password reset token: %w", err)
	}
	return raw, nil
}

// ValidatePasswordReset looks up a password reset token and returns (tokenID, identityID).
// Returns ErrNotFound or ErrExpired if invalid.
func (s *Store) ValidatePasswordReset(ctx context.Context, rawToken string) (uuid.UUID, uuid.UUID, error) {
	hash := hashToken(rawToken)

	var t Token
	err := s.db.QueryRow(ctx, "invite.ValidatePasswordReset",
		`SELECT id, identity_id, expires_at FROM email_codes
		 WHERE code_hash = $1 AND purpose = 'password_reset' AND used_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		hash,
	).Scan(&t.ID, &t.IdentityID, &t.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, uuid.Nil, ErrNotFound
		}
		return uuid.Nil, uuid.Nil, fmt.Errorf("validate password reset token: %w", err)
	}

	if time.Now().After(t.ExpiresAt) {
		return uuid.Nil, uuid.Nil, ErrExpired
	}
	return t.ID, t.IdentityID, nil
}

// generateToken creates a 32-byte random token and encodes it as base64url (no padding).
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken returns the SHA-256 hex digest of the token string.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
