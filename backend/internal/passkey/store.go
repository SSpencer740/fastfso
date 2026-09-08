package passkey

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fastfso/fastfso/backend/internal/database"
)

var ErrNotFound = errors.New("passkey not found")

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

type CreateParams struct {
	IdentityID     uuid.UUID
	CredentialID   []byte
	PublicKey      []byte
	AAGUID         []byte
	SignCount      int64
	BackupEligible bool
	BackupState    bool
	Transports     []string
	FriendlyName   *string
}

func (s *Store) Create(ctx context.Context, p CreateParams) (*Passkey, error) {
	pk := &Passkey{}
	err := s.db.QueryRow(ctx, "passkey.Create",
		`INSERT INTO passkeys (identity_id, credential_id, public_key, aaguid, sign_count, backup_eligible, backup_state, transports, friendly_name)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, identity_id, credential_id, public_key, aaguid, sign_count, backup_eligible, backup_state, transports, friendly_name, last_used_at, created_at`,
		p.IdentityID, p.CredentialID, p.PublicKey, p.AAGUID, p.SignCount, p.BackupEligible, p.BackupState, p.Transports, p.FriendlyName,
	).Scan(
		&pk.ID, &pk.IdentityID, &pk.CredentialID, &pk.PublicKey, &pk.AAGUID,
		&pk.SignCount, &pk.BackupEligible, &pk.BackupState, &pk.Transports, &pk.FriendlyName, &pk.LastUsedAt, &pk.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("passkey create: %w", err)
	}
	return pk, nil
}

func (s *Store) ListByIdentity(ctx context.Context, identityID uuid.UUID) ([]Passkey, error) {
	rows, err := s.db.Query(ctx, "passkey.ListByIdentity",
		`SELECT id, identity_id, credential_id, public_key, aaguid, sign_count, backup_eligible, backup_state, transports, friendly_name, last_used_at, created_at
		 FROM passkeys WHERE identity_id = $1 ORDER BY created_at`,
		identityID,
	)
	if err != nil {
		return nil, fmt.Errorf("passkey list: %w", err)
	}
	defer rows.Close()

	var passkeys []Passkey
	for rows.Next() {
		var pk Passkey
		if err := rows.Scan(
			&pk.ID, &pk.IdentityID, &pk.CredentialID, &pk.PublicKey, &pk.AAGUID,
			&pk.SignCount, &pk.BackupEligible, &pk.BackupState, &pk.Transports, &pk.FriendlyName, &pk.LastUsedAt, &pk.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("passkey list scan: %w", err)
		}
		passkeys = append(passkeys, pk)
	}
	return passkeys, rows.Err()
}

func (s *Store) GetByCredentialID(ctx context.Context, credentialID []byte) (*Passkey, error) {
	pk := &Passkey{}
	err := s.db.QueryRow(ctx, "passkey.GetByCredentialID",
		`SELECT id, identity_id, credential_id, public_key, aaguid, sign_count, backup_eligible, backup_state, transports, friendly_name, last_used_at, created_at
		 FROM passkeys WHERE credential_id = $1`,
		credentialID,
	).Scan(
		&pk.ID, &pk.IdentityID, &pk.CredentialID, &pk.PublicKey, &pk.AAGUID,
		&pk.SignCount, &pk.BackupEligible, &pk.BackupState, &pk.Transports, &pk.FriendlyName, &pk.LastUsedAt, &pk.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("passkey get by credential: %w", err)
	}
	return pk, nil
}

func (s *Store) UpdateSignCount(ctx context.Context, id uuid.UUID, signCount int64) error {
	_, err := s.db.Exec(ctx, "passkey.UpdateSignCount",
		`UPDATE passkeys SET sign_count = $1, last_used_at = now() WHERE id = $2`,
		signCount, id,
	)
	if err != nil {
		return fmt.Errorf("passkey update sign count: %w", err)
	}
	return nil
}

func (s *Store) CountByIdentity(ctx context.Context, identityID uuid.UUID) (int, error) {
	var count int
	err := s.db.QueryRow(ctx, "passkey.CountByIdentity",
		`SELECT COUNT(*) FROM passkeys WHERE identity_id = $1`, identityID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("passkey count: %w", err)
	}
	return count, nil
}

func (s *Store) Delete(ctx context.Context, id uuid.UUID, identityID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "passkey.Delete",
		`DELETE FROM passkeys WHERE id = $1 AND identity_id = $2`,
		id, identityID,
	)
	if err != nil {
		return fmt.Errorf("passkey delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
