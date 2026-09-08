package emailcode

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fastfso/fastfso/backend/internal/database"
)

var (
	ErrNotFound    = errors.New("email code not found")
	ErrExpired     = errors.New("email code expired")
	ErrAlreadyUsed = errors.New("email code already used")
	ErrInvalid     = errors.New("invalid code")
)

const codeExpiry = 10 * time.Minute

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// Generate creates a 6-digit code, stores its hash, and returns the plaintext code.
func (s *Store) Generate(ctx context.Context, identityID uuid.UUID, purpose string) (string, error) {
	code, err := generateCode()
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}

	hash := hashCode(code)

	// Invalidate any existing unused codes for the same identity and purpose
	_, _ = s.db.Exec(ctx, "emailcode.Generate.invalidate",
		`UPDATE email_codes SET used_at = now()
		 WHERE identity_id = $1 AND purpose = $2 AND used_at IS NULL`,
		identityID, purpose,
	)

	_, err = s.db.Exec(ctx, "emailcode.Generate.insert",
		`INSERT INTO email_codes (identity_id, code_hash, purpose, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		identityID, hash, purpose, time.Now().Add(codeExpiry),
	)
	if err != nil {
		return "", fmt.Errorf("store email code: %w", err)
	}

	return code, nil
}

// Verify validates a code against stored hashes and marks it as used.
func (s *Store) Verify(ctx context.Context, identityID uuid.UUID, purpose, code string) error {
	hash := hashCode(code)

	var ec Code
	err := s.db.QueryRow(ctx, "emailcode.Verify",
		`SELECT id, expires_at, used_at FROM email_codes
		 WHERE identity_id = $1 AND purpose = $2 AND code_hash = $3
		 ORDER BY created_at DESC LIMIT 1`,
		identityID, purpose, hash,
	).Scan(&ec.ID, &ec.ExpiresAt, &ec.UsedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalid
		}
		return fmt.Errorf("verify email code: %w", err)
	}

	if ec.UsedAt != nil {
		return ErrAlreadyUsed
	}
	if time.Now().After(ec.ExpiresAt) {
		return ErrExpired
	}

	// Mark as used
	_, err = s.db.Exec(ctx, "emailcode.Verify.markUsed",
		`UPDATE email_codes SET used_at = now() WHERE id = $1`, ec.ID,
	)
	if err != nil {
		return fmt.Errorf("mark email code used: %w", err)
	}

	return nil
}

func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hashCode(code string) string {
	h := sha256.Sum256([]byte(code))
	return hex.EncodeToString(h[:])
}
