package totp

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pquerna/otp/totp"

	"github.com/fastfso/fastfso/backend/internal/database"
)

var ErrNotFound = errors.New("totp secret not found")

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, identityID uuid.UUID, secret string) (*Secret, error) {
	ts := &Secret{}
	err := s.db.QueryRow(ctx, "totp.Create",
		`INSERT INTO totp_secrets (identity_id, secret)
		 VALUES ($1, $2)
		 ON CONFLICT (identity_id) DO UPDATE SET secret = $2, verified = FALSE
		 RETURNING id, identity_id, secret, verified, created_at`,
		identityID, secret,
	).Scan(&ts.ID, &ts.IdentityID, &ts.Secret, &ts.Verified, &ts.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("totp create: %w", err)
	}
	return ts, nil
}

func (s *Store) GetByIdentity(ctx context.Context, identityID uuid.UUID) (*Secret, error) {
	ts := &Secret{}
	err := s.db.QueryRow(ctx, "totp.GetByIdentity",
		`SELECT id, identity_id, secret, verified, created_at
		 FROM totp_secrets WHERE identity_id = $1`,
		identityID,
	).Scan(&ts.ID, &ts.IdentityID, &ts.Secret, &ts.Verified, &ts.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("totp get: %w", err)
	}
	return ts, nil
}

func (s *Store) HasVerifiedTOTP(ctx context.Context, identityID uuid.UUID) (bool, error) {
	ts, err := s.GetByIdentity(ctx, identityID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return ts.Verified, nil
}

// ValidateCode checks a TOTP code against the stored secret for the identity.
// Returns false (without error) if no verified secret exists.
func (s *Store) ValidateCode(ctx context.Context, identityID uuid.UUID, code string) (bool, error) {
	ts, err := s.GetByIdentity(ctx, identityID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if !ts.Verified {
		return false, nil
	}
	return totp.Validate(code, ts.Secret), nil
}

func (s *Store) Verify(ctx context.Context, identityID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "totp.Verify",
		`UPDATE totp_secrets SET verified = TRUE WHERE identity_id = $1`,
		identityID,
	)
	if err != nil {
		return fmt.Errorf("totp verify: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, identityID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "totp.Delete",
		`DELETE FROM totp_secrets WHERE identity_id = $1`,
		identityID,
	)
	if err != nil {
		return fmt.Errorf("totp delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
