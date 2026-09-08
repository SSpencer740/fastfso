package suborg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fastfso/fastfso/backend/internal/database"
)

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// List returns all sub-orgs for a tenant with member counts and primary FSO info.
func (s *Store) List(ctx context.Context, tenantID uuid.UUID) ([]SubOrg, error) {
	rows, err := s.db.Query(ctx, "suborg.List",
		`SELECT s.id, s.tenant_id, s.name,
		        COUNT(us.user_id) AS member_count,
		        s.primary_fso_user_id,
		        fso_i.name  AS primary_fso_name,
		        fso_i.email AS primary_fso_email,
		        s.created_at, s.updated_at
		 FROM suborganizations s
		 LEFT JOIN user_suborganizations us ON us.suborganization_id = s.id
		 LEFT JOIN users fso_u ON fso_u.id = s.primary_fso_user_id
		 LEFT JOIN identities fso_i ON fso_i.id = fso_u.identity_id
		 WHERE s.tenant_id = $1
		 GROUP BY s.id, fso_i.name, fso_i.email
		 ORDER BY s.name = 'Default' DESC, s.name`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("suborg list: %w", err)
	}
	defer rows.Close()

	var result []SubOrg
	for rows.Next() {
		var o SubOrg
		if err := rows.Scan(&o.ID, &o.TenantID, &o.Name, &o.MemberCount,
			&o.PrimaryFSOUserID, &o.PrimaryFSOName, &o.PrimaryFSOEmail,
			&o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("suborg list scan: %w", err)
		}
		result = append(result, o)
	}
	return result, nil
}

// SetPrimaryFSO assigns a user as the primary FSO for a sub-org.
// Pass nil userID to clear the assignment.
func (s *Store) SetPrimaryFSO(ctx context.Context, tenantID, subOrgID uuid.UUID, userID *uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "suborg.SetPrimaryFSO",
		`UPDATE suborganizations SET primary_fso_user_id = $3, updated_at = now()
		 WHERE id = $1 AND tenant_id = $2`,
		subOrgID, tenantID, userID,
	)
	if err != nil {
		return fmt.Errorf("suborg set primary fso: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetPrimaryFSO returns the primary FSO contact for the sub-org containing userID.
// Returns ErrNotFound if the user has no sub-org or the sub-org has no primary FSO set.
func (s *Store) GetPrimaryFSO(ctx context.Context, tenantID, userID uuid.UUID) (*FSOContact, error) {
	var c FSOContact
	err := s.db.QueryRow(ctx, "suborg.GetPrimaryFSO",
		`SELECT fso_i.name, fso_i.email
		 FROM user_suborganizations us
		 JOIN suborganizations s ON s.id = us.suborganization_id
		 JOIN users fso_u ON fso_u.id = s.primary_fso_user_id
		 JOIN identities fso_i ON fso_i.id = fso_u.identity_id
		 WHERE us.user_id = $1 AND s.tenant_id = $2
		 LIMIT 1`,
		userID, tenantID,
	).Scan(&c.Name, &c.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("suborg get primary fso: %w", err)
	}
	return &c, nil
}

// Create adds a new sub-org.
func (s *Store) Create(ctx context.Context, tenantID uuid.UUID, name string) (*SubOrg, error) {
	var o SubOrg
	err := s.db.QueryRow(ctx, "suborg.Create",
		`INSERT INTO suborganizations (tenant_id, name)
		 VALUES ($1, $2)
		 RETURNING id, tenant_id, name, created_at, updated_at`,
		tenantID, name,
	).Scan(&o.ID, &o.TenantID, &o.Name, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrNameTaken
		}
		return nil, fmt.Errorf("suborg create: %w", err)
	}
	return &o, nil
}

// Update renames a sub-org. The "Default" sub-org can be renamed.
func (s *Store) Update(ctx context.Context, tenantID, id uuid.UUID, name string) error {
	tag, err := s.db.Exec(ctx, "suborg.Update",
		`UPDATE suborganizations SET name = $3, updated_at = now()
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID, name,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrNameTaken
		}
		return fmt.Errorf("suborg update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a sub-org. The "Default" sub-org is protected.
// Members are moved to the tenant's Default sub-org before deletion.
func (s *Store) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	// Fetch name to protect Default
	var name string
	err := s.db.QueryRow(ctx, "suborg.Delete.getName",
		`SELECT name FROM suborganizations WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(&name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("suborg delete get name: %w", err)
	}
	if strings.EqualFold(name, "Default") {
		return ErrDefaultLocked
	}

	// Move members to the Default sub-org
	_, err = s.db.Exec(ctx, "suborg.Delete.moveMembers",
		`INSERT INTO user_suborganizations (user_id, suborganization_id)
		 SELECT us.user_id, def.id
		 FROM user_suborganizations us
		 JOIN suborganizations def ON def.tenant_id = $2 AND def.name = 'Default'
		 WHERE us.suborganization_id = $1
		 ON CONFLICT DO NOTHING`,
		id, tenantID,
	)
	if err != nil {
		return fmt.Errorf("suborg delete move members: %w", err)
	}

	// Delete the sub-org (cascades user_suborganizations rows)
	_, err = s.db.Exec(ctx, "suborg.Delete",
		`DELETE FROM suborganizations WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	if err != nil {
		return fmt.Errorf("suborg delete: %w", err)
	}
	return nil
}

// ListMembers returns all members of a sub-org.
func (s *Store) ListMembers(ctx context.Context, tenantID, subOrgID uuid.UUID) ([]SubOrgMember, error) {
	rows, err := s.db.Query(ctx, "suborg.ListMembers",
		`SELECT u.id, i.name, i.email, u.role
		 FROM user_suborganizations us
		 JOIN users u ON u.id = us.user_id
		 JOIN identities i ON i.id = u.identity_id
		 WHERE us.suborganization_id = $1 AND u.tenant_id = $2
		 ORDER BY i.name`,
		subOrgID, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("suborg list members: %w", err)
	}
	defer rows.Close()

	var result []SubOrgMember
	for rows.Next() {
		var m SubOrgMember
		if err := rows.Scan(&m.UserID, &m.Name, &m.Email, &m.Role); err != nil {
			return nil, fmt.Errorf("suborg list members scan: %w", err)
		}
		result = append(result, m)
	}
	return result, nil
}

// SetUserSubOrg moves a user to a specific sub-org within the tenant,
// removing them from all other sub-orgs in that tenant first.
func (s *Store) SetUserSubOrg(ctx context.Context, tenantID, userID, subOrgID uuid.UUID) error {
	// Verify the target sub-org belongs to this tenant
	var exists bool
	err := s.db.QueryRow(ctx, "suborg.SetUserSubOrg.check",
		`SELECT EXISTS(SELECT 1 FROM suborganizations WHERE id = $1 AND tenant_id = $2)`,
		subOrgID, tenantID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("suborg set user check: %w", err)
	}
	if !exists {
		return ErrNotFound
	}

	// Remove from all current sub-orgs in this tenant
	_, err = s.db.Exec(ctx, "suborg.SetUserSubOrg.remove",
		`DELETE FROM user_suborganizations
		 WHERE user_id = $1
		   AND suborganization_id IN (
		       SELECT id FROM suborganizations WHERE tenant_id = $2
		   )`,
		userID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("suborg set user remove: %w", err)
	}

	// Add to the new sub-org
	_, err = s.db.Exec(ctx, "suborg.SetUserSubOrg.add",
		`INSERT INTO user_suborganizations (user_id, suborganization_id)
		 VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		userID, subOrgID,
	)
	if err != nil {
		return fmt.Errorf("suborg set user add: %w", err)
	}
	return nil
}
