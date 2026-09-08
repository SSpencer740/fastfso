package actionitem

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/SSpencer740/fastfso/backend/internal/database"
)

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new action item and returns it with all database-generated fields.
// When AssignedTo is nil and SubOrgID is set, the sub-org's primary FSO is used automatically.
// AssigneeName and AssigneeEmail are populated if an assignee was resolved.
func (s *Store) Create(ctx context.Context, p CreateParams) (*ActionItem, error) {
	var ai ActionItem
	err := s.db.QueryRow(ctx, "actionitem.Create",
		`WITH ins AS (
		    INSERT INTO action_items (tenant_id, source_type, source_id, title, description, priority, assigned_to, due_date, sub_org_id)
		    VALUES ($1, $2, $3, $4, $5, $6,
		        COALESCE($7, (SELECT primary_fso_user_id FROM suborganizations WHERE id = $9 AND tenant_id = $1)),
		        $8, $9)
		    RETURNING *
		)
		SELECT ins.id, ins.tenant_id, ins.source_type, ins.source_id, ins.title, ins.description,
		       ins.priority, ins.status, ins.assigned_to, ins.notes, ins.due_date, ins.created_at, ins.updated_at,
		       i.name, i.email
		FROM ins
		LEFT JOIN users u ON u.id = ins.assigned_to
		LEFT JOIN identities i ON i.id = u.identity_id`,
		p.TenantID, p.SourceType, p.SourceID, p.Title, p.Description, p.Priority, p.AssignedTo, p.DueDate, p.SubOrgID,
	).Scan(&ai.ID, &ai.TenantID, &ai.SourceType, &ai.SourceID, &ai.Title, &ai.Description,
		&ai.Priority, &ai.Status, &ai.AssignedTo, &ai.Notes, &ai.DueDate, &ai.CreatedAt, &ai.UpdatedAt,
		&ai.AssigneeName, &ai.AssigneeEmail)
	if err != nil {
		return nil, fmt.Errorf("create action item: %w", err)
	}
	return &ai, nil
}

// Get retrieves a single action item by ID, scoped to the given tenant.
func (s *Store) Get(ctx context.Context, tenantID, id uuid.UUID) (*ActionItem, error) {
	var ai ActionItem
	err := s.db.QueryRow(ctx, "actionitem.Get",
		`SELECT ai.id, ai.tenant_id, ai.source_type, ai.source_id, ai.title, ai.description,
		        ai.priority, ai.status, ai.assigned_to, ai.notes, ai.due_date, ai.created_at, ai.updated_at,
		        i.name, i.email
		 FROM action_items ai
		 LEFT JOIN users u ON u.id = ai.assigned_to
		 LEFT JOIN identities i ON i.id = u.identity_id
		 WHERE ai.id = $1 AND ai.tenant_id = $2`,
		id, tenantID,
	).Scan(&ai.ID, &ai.TenantID, &ai.SourceType, &ai.SourceID, &ai.Title, &ai.Description,
		&ai.Priority, &ai.Status, &ai.AssignedTo, &ai.Notes, &ai.DueDate, &ai.CreatedAt, &ai.UpdatedAt,
		&ai.AssigneeName, &ai.AssigneeEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get action item: %w", err)
	}
	return &ai, nil
}

// ErrAssigneeWrongTenant is returned when reassigning to a user that does not belong to the tenant.
var ErrAssigneeWrongTenant = errors.New("assignee does not belong to tenant")

// UpdateAssignedTo reassigns an action item and returns the updated item with assignee contact info.
// If userID is non-nil, the target user must belong to the same tenant.
func (s *Store) UpdateAssignedTo(ctx context.Context, tenantID, id uuid.UUID, userID *uuid.UUID) (*ActionItem, error) {
	if userID != nil {
		var ok bool
		err := s.db.QueryRow(ctx, "actionitem.UpdateAssignedTo.verifyUser",
			`SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND tenant_id = $2)`,
			*userID, tenantID,
		).Scan(&ok)
		if err != nil {
			return nil, fmt.Errorf("verify assignee tenant: %w", err)
		}
		if !ok {
			return nil, ErrAssigneeWrongTenant
		}
	}
	var ai ActionItem
	err := s.db.QueryRow(ctx, "actionitem.UpdateAssignedTo",
		`WITH upd AS (
		    UPDATE action_items SET assigned_to = $3, updated_at = now()
		    WHERE id = $1 AND tenant_id = $2
		    RETURNING *
		)
		SELECT upd.id, upd.tenant_id, upd.source_type, upd.source_id, upd.title, upd.description,
		       upd.priority, upd.status, upd.assigned_to, upd.notes, upd.due_date, upd.created_at, upd.updated_at,
		       i.name, i.email
		FROM upd
		LEFT JOIN users u ON u.id = upd.assigned_to
		LEFT JOIN identities i ON i.id = u.identity_id`,
		id, tenantID, userID,
	).Scan(&ai.ID, &ai.TenantID, &ai.SourceType, &ai.SourceID, &ai.Title, &ai.Description,
		&ai.Priority, &ai.Status, &ai.AssignedTo, &ai.Notes, &ai.DueDate, &ai.CreatedAt, &ai.UpdatedAt,
		&ai.AssigneeName, &ai.AssigneeEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update action item assignee: %w", err)
	}
	return &ai, nil
}

// ListAdmins returns all administrator users for a tenant. Used as a notification
// fallback when an action item cannot be routed to a primary FSO.
func (s *Store) ListAdmins(ctx context.Context, tenantID uuid.UUID) ([]FSOUser, error) {
	rows, err := s.db.Query(ctx, "actionitem.ListAdmins",
		`SELECT u.id, i.name, i.email
		 FROM users u
		 JOIN identities i ON i.id = u.identity_id
		 WHERE u.tenant_id = $1 AND u.role = 'administrator'
		 ORDER BY i.name`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list admins: %w", err)
	}
	defer rows.Close()

	var result []FSOUser
	for rows.Next() {
		var u FSOUser
		if err := rows.Scan(&u.UserID, &u.Name, &u.Email); err != nil {
			return nil, fmt.Errorf("scan admin: %w", err)
		}
		result = append(result, u)
	}
	return result, nil
}

// ResolveNotifyTargets returns the resolved assignee if one exists, otherwise all
// tenant administrators as a fallback so action items never route silently into the void.
func (s *Store) ResolveNotifyTargets(ctx context.Context, tenantID uuid.UUID, ai *ActionItem) []FSOUser {
	if ai != nil && ai.AssignedTo != nil && ai.AssigneeEmail != nil && ai.AssigneeName != nil && *ai.AssigneeEmail != "" {
		return []FSOUser{{UserID: ai.AssignedTo.String(), Name: *ai.AssigneeName, Email: *ai.AssigneeEmail}}
	}
	admins, err := s.ListAdmins(ctx, tenantID)
	if err != nil {
		return nil
	}
	return admins
}

// ListFSOUsers returns all FSO and administrator users for a tenant (used for the assignee picker).
func (s *Store) ListFSOUsers(ctx context.Context, tenantID uuid.UUID) ([]FSOUser, error) {
	rows, err := s.db.Query(ctx, "actionitem.ListFSOUsers",
		`SELECT u.id, i.name, i.email
		 FROM users u
		 JOIN identities i ON i.id = u.identity_id
		 WHERE u.tenant_id = $1 AND u.role IN ('fso', 'administrator')
		 ORDER BY i.name`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list fso users: %w", err)
	}
	defer rows.Close()

	var result []FSOUser
	for rows.Next() {
		var u FSOUser
		if err := rows.Scan(&u.UserID, &u.Name, &u.Email); err != nil {
			return nil, fmt.Errorf("scan fso user: %w", err)
		}
		result = append(result, u)
	}
	return result, nil
}

// List returns action items matching the given filters, along with the total count for pagination.
func (s *Store) List(ctx context.Context, f ListFilters) ([]ActionItemRow, int, error) {
	where := "ai.tenant_id = $1"
	args := []any{f.TenantID}
	n := 2

	if f.SourceType != "" {
		where += fmt.Sprintf(" AND ai.source_type = $%d", n)
		args = append(args, f.SourceType)
		n++
	}
	if f.Status != "" {
		where += fmt.Sprintf(" AND ai.status = $%d", n)
		args = append(args, f.Status)
		n++
	}
	if f.Search != "" {
		where += fmt.Sprintf(" AND (ai.title ILIKE '%%' || $%d || '%%' OR ai.description ILIKE '%%' || $%d || '%%')", n, n)
		args = append(args, f.Search)
		n++
	}
	if f.SubOrgScope != nil {
		where += fmt.Sprintf(" AND (ai.sub_org_id IS NULL OR ai.sub_org_id = $%d)", n)
		args = append(args, f.SubOrgScope)
		n++
	}
	if f.AssignedTo != nil {
		where += fmt.Sprintf(" AND ai.assigned_to = $%d", n)
		args = append(args, f.AssignedTo)
		n++
	}

	// Count
	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := s.db.QueryRow(ctx, "actionitem.List.count",
		fmt.Sprintf("SELECT COUNT(*) FROM action_items ai WHERE %s", where),
		countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count action items: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit, f.Offset)

	query := fmt.Sprintf(`
		SELECT ai.id, ai.source_type, ai.title, ai.description, ai.priority, ai.status,
		       i.name, i.email, so.name,
		       ai.due_date, ai.created_at
		FROM action_items ai
		LEFT JOIN users u ON u.id = ai.assigned_to
		LEFT JOIN identities i ON i.id = u.identity_id
		LEFT JOIN suborganizations so ON so.id = ai.sub_org_id
		WHERE %s
		ORDER BY ai.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, n, n+1)

	rows, err := s.db.Query(ctx, "actionitem.List", query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list action items: %w", err)
	}
	defer rows.Close()

	var result []ActionItemRow
	for rows.Next() {
		var r ActionItemRow
		if err := rows.Scan(&r.ID, &r.SourceType, &r.Title, &r.Description, &r.Priority, &r.Status, &r.AssigneeName, &r.AssigneeEmail, &r.SubOrgName, &r.DueDate, &r.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan action item row: %w", err)
		}
		result = append(result, r)
	}

	return result, total, nil
}

// UpdateNotes sets the notes field without changing status.
func (s *Store) UpdateNotes(ctx context.Context, tenantID, id uuid.UUID, notes string) error {
	tag, err := s.db.Exec(ctx, "actionitem.UpdateNotes",
		`UPDATE action_items SET notes = $3, updated_at = now()
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID, notes,
	)
	if err != nil {
		return fmt.Errorf("update action item notes: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatus changes the status and notes of an action item.
func (s *Store) UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status, notes string) error {
	tag, err := s.db.Exec(ctx, "actionitem.UpdateStatus",
		`UPDATE action_items SET status = $3, notes = $4, updated_at = now()
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID, status, notes,
	)
	if err != nil {
		return fmt.Errorf("update action item status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ResolveBySource marks all pending/under_review action items for a given source as processed.
func (s *Store) ResolveBySource(ctx context.Context, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID) error {
	_, err := s.db.Exec(ctx, "actionitem.ResolveBySource",
		`UPDATE action_items SET status = 'processed', updated_at = now()
		 WHERE tenant_id = $1 AND source_type = $2 AND source_id = $3
		   AND status IN ('pending', 'under_review')`,
		tenantID, sourceType, sourceID,
	)
	if err != nil {
		return fmt.Errorf("resolve action items by source: %w", err)
	}
	return nil
}

// SummaryStats returns aggregate counts of action items by status for a tenant.
func (s *Store) SummaryStats(ctx context.Context, tenantID uuid.UUID, subOrgScope *uuid.UUID) (*SummaryStats, error) {
	where := "tenant_id = $1"
	args := []any{tenantID}
	if subOrgScope != nil {
		where += " AND (sub_org_id IS NULL OR sub_org_id = $2)"
		args = append(args, subOrgScope)
	}
	var stats SummaryStats
	err := s.db.QueryRow(ctx, "actionitem.SummaryStats",
		fmt.Sprintf(`SELECT
		    COUNT(*),
		    COUNT(*) FILTER (WHERE status = 'pending'),
		    COUNT(*) FILTER (WHERE status = 'under_review'),
		    COUNT(*) FILTER (WHERE status = 'processed')
		 FROM action_items WHERE %s`, where),
		args...,
	).Scan(&stats.Total, &stats.Pending, &stats.UnderReview, &stats.Processed)
	if err != nil {
		return nil, fmt.Errorf("action item summary stats: %w", err)
	}
	return &stats, nil
}
