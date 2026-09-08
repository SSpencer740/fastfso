package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fastfso/fastfso/backend/internal/database"
)

var ErrNotFound = errors.New("identity not found")

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, email, name string, passwordHash *string) (*Identity, error) {
	ident := &Identity{}
	err := s.db.QueryRow(ctx, "identity.Create",
		`INSERT INTO identities (email, name, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id, email, name, password_hash, is_super_admin, activated, suspended_at, failed_login_attempts, locked_until, created_at, updated_at`,
		email, name, passwordHash,
	).Scan(
		&ident.ID, &ident.Email, &ident.Name, &ident.PasswordHash,
		&ident.IsSuperAdmin, &ident.Activated, &ident.SuspendedAt,
		&ident.FailedLoginAttempts, &ident.LockedUntil,
		&ident.CreatedAt, &ident.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("identity create: %w", err)
	}
	return ident, nil
}

func (s *Store) GetByEmail(ctx context.Context, email string) (*Identity, error) {
	ident := &Identity{}
	err := s.db.QueryRow(ctx, "identity.GetByEmail",
		`SELECT id, email, name, password_hash, is_super_admin, activated, suspended_at, failed_login_attempts, locked_until, created_at, updated_at
		 FROM identities WHERE lower(email) = lower($1)`,
		email,
	).Scan(
		&ident.ID, &ident.Email, &ident.Name, &ident.PasswordHash,
		&ident.IsSuperAdmin, &ident.Activated, &ident.SuspendedAt,
		&ident.FailedLoginAttempts, &ident.LockedUntil,
		&ident.CreatedAt, &ident.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("identity get by email: %w", err)
	}
	return ident, nil
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (*Identity, error) {
	ident := &Identity{}
	err := s.db.QueryRow(ctx, "identity.GetByID",
		`SELECT id, email, name, password_hash, is_super_admin, activated, suspended_at, failed_login_attempts, locked_until, created_at, updated_at
		 FROM identities WHERE id = $1`,
		id,
	).Scan(
		&ident.ID, &ident.Email, &ident.Name, &ident.PasswordHash,
		&ident.IsSuperAdmin, &ident.Activated, &ident.SuspendedAt,
		&ident.FailedLoginAttempts, &ident.LockedUntil,
		&ident.CreatedAt, &ident.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("identity get by id: %w", err)
	}
	return ident, nil
}

func (s *Store) SetSuperAdmin(ctx context.Context, id uuid.UUID, isSuperAdmin bool) error {
	tag, err := s.db.Exec(ctx, "identity.SetSuperAdmin",
		`UPDATE identities SET is_super_admin = $1, updated_at = now() WHERE id = $2`,
		isSuperAdmin, id,
	)
	if err != nil {
		return fmt.Errorf("identity set super admin: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Suspend(ctx context.Context, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "identity.Suspend",
		`UPDATE identities SET suspended_at = now(), updated_at = now()
		 WHERE id = $1 AND suspended_at IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("identity suspend: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Unsuspend(ctx context.Context, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "identity.Unsuspend",
		`UPDATE identities SET suspended_at = NULL, updated_at = now()
		 WHERE id = $1 AND suspended_at IS NOT NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("identity unsuspend: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "identity.Delete",
		`DELETE FROM identities WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("identity delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CountActiveSuperAdmins(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRow(ctx, "identity.CountActiveSuperAdmins",
		`SELECT COUNT(*) FROM identities WHERE is_super_admin = true AND suspended_at IS NULL`,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("identity count active super admins: %w", err)
	}
	return count, nil
}

func (s *Store) UpdateName(ctx context.Context, id uuid.UUID, name string) error {
	tag, err := s.db.Exec(ctx, "identity.UpdateName",
		`UPDATE identities SET name = $1, updated_at = now() WHERE id = $2`,
		name, id,
	)
	if err != nil {
		return fmt.Errorf("identity update name: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Activate(ctx context.Context, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "identity.Activate",
		`UPDATE identities SET activated = true, updated_at = now() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("identity activate: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateEmail(ctx context.Context, id uuid.UUID, email string) error {
	tag, err := s.db.Exec(ctx, "identity.UpdateEmail",
		`UPDATE identities SET email = $1, updated_at = now() WHERE id = $2`,
		email, id,
	)
	if err != nil {
		return fmt.Errorf("identity update email: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	tag, err := s.db.Exec(ctx, "identity.UpdatePassword",
		`UPDATE identities SET password_hash = $1, updated_at = now() WHERE id = $2`,
		passwordHash, id,
	)
	if err != nil {
		return fmt.Errorf("identity update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RecordFailedLogin increments the failed attempt counter and locks the account
// once MaxFailedLogins is reached.
func (s *Store) RecordFailedLogin(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, "identity.RecordFailedLogin",
		`UPDATE identities
		 SET
		   failed_login_attempts = failed_login_attempts + 1,
		   locked_until = CASE
		     WHEN failed_login_attempts + 1 >= $2 THEN now() + make_interval(secs => $3)
		     ELSE locked_until
		   END,
		   updated_at = now()
		 WHERE id = $1`,
		id, MaxFailedLogins, int(LockDuration.Seconds()),
	)
	if err != nil {
		return fmt.Errorf("identity record failed login: %w", err)
	}
	return nil
}

// ClearFailedLogin resets the lockout state after a successful login.
func (s *Store) ClearFailedLogin(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, "identity.ClearFailedLogin",
		`UPDATE identities SET failed_login_attempts = 0, locked_until = NULL, updated_at = now() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("identity clear failed login: %w", err)
	}
	return nil
}
