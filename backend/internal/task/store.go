package task

import (
	"context"
	"errors"
	"fmt"
	"time"

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

// Create inserts a task with its requirements and assignment rules.
func (s *Store) Create(ctx context.Context, p CreateParams) (*Task, error) {
	var t Task
	err := s.db.QueryRow(ctx, "task.Create",
		`INSERT INTO tasks (tenant_id, created_by_user_id, title, description, priority, due_date, status, sub_org_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, tenant_id, created_by_user_id, title, description, priority, due_date, status, created_at, updated_at`,
		p.TenantID, p.CreatedByUserID, p.Title, p.Description, p.Priority, p.DueDate, p.Status, p.SubOrgID,
	).Scan(&t.ID, &t.TenantID, &t.CreatedByUserID, &t.Title, &t.Description, &t.Priority, &t.DueDate, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}

	for i, req := range p.Requirements {
		// AI verification only applies to file_upload requirements; ignore the
		// criteria field for other kinds rather than rejecting at the API
		// layer (forward-compat: if we add AI to other kinds later, the
		// stored value is already there).
		criteria := req.AIVerificationCriteria
		if req.Kind != "file_upload" {
			criteria = ""
		}
		_, err := s.db.Exec(ctx, "task.Create.requirement",
			`INSERT INTO task_requirements (task_id, kind, label, description, required, sort_order, ai_verification_criteria)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			t.ID, req.Kind, req.Label, req.Description, req.Required, i, criteria,
		)
		if err != nil {
			return nil, fmt.Errorf("insert requirement: %w", err)
		}
	}

	for _, rule := range p.Rules {
		_, err := s.db.Exec(ctx, "task.Create.rule",
			`INSERT INTO task_assignment_rules (task_id, rule_type, target_id)
			 VALUES ($1, $2, $3)`,
			t.ID, rule.RuleType, rule.TargetID,
		)
		if err != nil {
			return nil, fmt.Errorf("insert rule: %w", err)
		}
	}

	return &t, nil
}

// Get returns the admin detail view for a task.
func (s *Store) Get(ctx context.Context, tenantID, taskID uuid.UUID) (*TaskDetail, error) {
	var t Task
	err := s.db.QueryRow(ctx, "task.Get",
		`SELECT id, tenant_id, created_by_user_id, title, description, priority, due_date, status, created_at, updated_at
		 FROM tasks WHERE id = $1 AND tenant_id = $2`,
		taskID, tenantID,
	).Scan(&t.ID, &t.TenantID, &t.CreatedByUserID, &t.Title, &t.Description, &t.Priority, &t.DueDate, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get task: %w", err)
	}

	reqs, err := s.listRequirements(ctx, taskID)
	if err != nil {
		return nil, err
	}

	rules, err := s.listRules(ctx, taskID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx, "task.Get.assignees",
		`SELECT tus.completion_id, tus.user_id, i.name, i.email, tus.status, tus.viewed_at, tus.submitted_at
		 FROM task_user_status tus
		 JOIN users u ON u.id = tus.user_id
		 JOIN identities i ON i.id = u.identity_id
		 WHERE tus.task_id = $1
		 ORDER BY i.name`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("get assignees: %w", err)
	}
	defer rows.Close()

	assignees := make([]AssigneeStatus, 0)
	for rows.Next() {
		var a AssigneeStatus
		if err := rows.Scan(&a.CompletionID, &a.UserID, &a.UserName, &a.UserEmail, &a.Status, &a.ViewedAt, &a.SubmittedAt); err != nil {
			return nil, fmt.Errorf("scan assignee: %w", err)
		}
		assignees = append(assignees, a)
	}

	return &TaskDetail{
		Task:         t,
		Requirements: reqs,
		Rules:        rules,
		Assignees:    assignees,
	}, nil
}

// List returns a paginated list of tasks for the admin view.
func (s *Store) List(ctx context.Context, f ListFilters) ([]TaskRow, int, error) {
	where := "t.tenant_id = $1"
	args := []any{f.TenantID}
	n := 2

	if f.Status != "" {
		where += fmt.Sprintf(" AND t.status = $%d", n)
		args = append(args, f.Status)
		n++
	}
	if f.Priority != "" {
		where += fmt.Sprintf(" AND t.priority = $%d", n)
		args = append(args, f.Priority)
		n++
	}
	if f.Search != "" {
		where += fmt.Sprintf(" AND (t.title ILIKE '%%' || $%d || '%%' OR t.description ILIKE '%%' || $%d || '%%')", n, n)
		args = append(args, f.Search)
		n++
	}
	if f.SubOrgScope != nil {
		where += fmt.Sprintf(" AND (t.sub_org_id IS NULL OR t.sub_org_id = $%d)", n)
		args = append(args, f.SubOrgScope)
		n++
	}

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := s.db.QueryRow(ctx, "task.List.count",
		fmt.Sprintf("SELECT COUNT(*) FROM tasks t WHERE %s", where),
		countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := f.Offset
	args = append(args, limit, offset)

	query := fmt.Sprintf(`
		SELECT t.id, t.title, t.description, t.priority, t.due_date, t.status,
		       COALESCE(i.name, '') AS creator_name,
		       COALESCE(agg.total, 0) AS total_assigned,
		       COALESCE(agg.completed, 0) AS completed,
		       t.created_at
		FROM tasks t
		LEFT JOIN users u ON u.id = t.created_by_user_id
		LEFT JOIN identities i ON i.id = u.identity_id
		LEFT JOIN (
		    SELECT task_id, COUNT(*) AS total,
		           COUNT(*) FILTER (WHERE status IN ('approved')) AS completed
		    FROM task_user_status GROUP BY task_id
		) agg ON agg.task_id = t.id
		WHERE %s
		ORDER BY t.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, n, n+1)

	rows, err := s.db.Query(ctx, "task.List", query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var result []TaskRow
	for rows.Next() {
		var r TaskRow
		if err := rows.Scan(&r.ID, &r.Title, &r.Description, &r.Priority, &r.DueDate, &r.Status, &r.CreatorName, &r.TotalAssigned, &r.Completed, &r.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan task row: %w", err)
		}
		result = append(result, r)
	}

	return result, total, nil
}

// Unarchive restores an archived task to active.
func (s *Store) Unarchive(ctx context.Context, tenantID, taskID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "task.Unarchive",
		`UPDATE tasks SET status = 'active', updated_at = now() WHERE id = $1 AND tenant_id = $2 AND status = 'archived'`,
		taskID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("unarchive task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Archive marks a task as archived.
func (s *Store) Archive(ctx context.Context, tenantID, taskID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "task.Archive",
		`UPDATE tasks SET status = 'archived', updated_at = now() WHERE id = $1 AND tenant_id = $2`,
		taskID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("archive task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SummaryStats returns aggregate task counts for the admin dashboard.
// Optional from/to filter by created_at; optional subOrgScope for sub-org filtering.
// Archived tasks are excluded so the donut total matches the sum of slices.
func (s *Store) SummaryStats(ctx context.Context, tenantID uuid.UUID, from, to *time.Time, subOrgScope *uuid.UUID) (*SummaryStats, error) {
	where := "tenant_id = $1 AND status <> 'archived'"
	args := []any{tenantID}
	n := 2
	if from != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", n)
		args = append(args, *from)
		n++
	}
	if to != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", n)
		args = append(args, *to)
		n++
	}
	if subOrgScope != nil {
		where += fmt.Sprintf(" AND (sub_org_id IS NULL OR sub_org_id = $%d)", n)
		args = append(args, subOrgScope)
	}

	var stats SummaryStats
	err := s.db.QueryRow(ctx, "task.SummaryStats",
		fmt.Sprintf(`SELECT
		    COUNT(*),
		    COUNT(*) FILTER (WHERE status = 'active'),
		    COUNT(*) FILTER (WHERE status = 'draft'),
		    (SELECT COUNT(*) FROM task_user_status tus
		     JOIN tasks t2 ON t2.id = tus.task_id
		     WHERE t2.tenant_id = $1 AND t2.status <> 'archived' AND tus.status = 'submitted')
		 FROM tasks WHERE %s`, where),
		args...,
	).Scan(&stats.Total, &stats.Active, &stats.Draft, &stats.NeedsReview)
	if err != nil {
		return nil, fmt.Errorf("summary stats: %w", err)
	}
	return &stats, nil
}

// ListMyTasks returns tasks assigned to the given user. A limit > 0 bounds the
// result (used by the AI chat context to keep the prompt small); limit <= 0
// returns all of the user's active tasks (the IC task list).
func (s *Store) ListMyTasks(ctx context.Context, tenantID, userID uuid.UUID, search string, limit int) ([]MyTaskRow, error) {
	where := "t.tenant_id = $1 AND tus.user_id = $2 AND t.status = 'active'"
	args := []any{tenantID, userID}
	n := 3

	if search != "" {
		where += fmt.Sprintf(" AND (t.title ILIKE '%%' || $%d || '%%')", n)
		args = append(args, search)
		n++
	}

	limitClause := ""
	if limit > 0 {
		limitClause = fmt.Sprintf(" LIMIT $%d", n)
		args = append(args, limit)
	}

	query := fmt.Sprintf(`
		SELECT t.id, t.title, t.description, t.priority, t.due_date, tus.status,
		       COALESCE(i.name, '') AS creator_name, t.created_at
		FROM task_user_status tus
		JOIN tasks t ON t.id = tus.task_id
		LEFT JOIN users u ON u.id = t.created_by_user_id
		LEFT JOIN identities i ON i.id = u.identity_id
		WHERE %s
		ORDER BY t.due_date ASC NULLS LAST, t.created_at DESC%s
	`, where, limitClause)

	rows, err := s.db.Query(ctx, "task.ListMyTasks", query, args...)
	if err != nil {
		return nil, fmt.Errorf("list my tasks: %w", err)
	}
	defer rows.Close()

	var result []MyTaskRow
	for rows.Next() {
		var r MyTaskRow
		if err := rows.Scan(&r.ID, &r.Title, &r.Description, &r.Priority, &r.DueDate, &r.Status, &r.CreatorName, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan my task row: %w", err)
		}
		result = append(result, r)
	}
	return result, nil
}

// GetMyTask returns the IC detail view for a task, ensuring a completion row exists and marking it viewed.
func (s *Store) GetMyTask(ctx context.Context, tenantID, taskID, userID uuid.UUID) (*MyTaskDetail, error) {
	var exists bool
	err := s.db.QueryRow(ctx, "task.GetMyTask.check",
		`SELECT EXISTS(SELECT 1 FROM task_user_status WHERE task_id = $1 AND user_id = $2)`,
		taskID, userID,
	).Scan(&exists)
	if err != nil || !exists {
		return nil, ErrNotFound
	}

	var t Task
	var creatorName string
	err = s.db.QueryRow(ctx, "task.GetMyTask",
		`SELECT t.id, t.tenant_id, t.created_by_user_id, t.title, t.description, t.priority, t.due_date, t.status, t.created_at, t.updated_at,
		        COALESCE(i.name, '')
		 FROM tasks t
		 LEFT JOIN users u ON u.id = t.created_by_user_id
		 LEFT JOIN identities i ON i.id = u.identity_id
		 WHERE t.id = $1 AND t.tenant_id = $2`,
		taskID, tenantID,
	).Scan(&t.ID, &t.TenantID, &t.CreatedByUserID, &t.Title, &t.Description, &t.Priority, &t.DueDate, &t.Status, &t.CreatedAt, &t.UpdatedAt, &creatorName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get my task: %w", err)
	}

	comp, err := s.EnsureCompletion(ctx, tenantID, taskID, userID)
	if err != nil {
		return nil, err
	}

	if comp.ViewedAt == nil {
		_ = s.MarkViewed(ctx, comp.ID)
	}

	reqs, err := s.listRequirements(ctx, taskID)
	if err != nil {
		return nil, err
	}

	responses, err := s.listResponses(ctx, comp.ID)
	if err != nil {
		return nil, err
	}

	uploads, err := s.ListUploads(ctx, comp.ID)
	if err != nil {
		return nil, err
	}

	return &MyTaskDetail{
		Task:         t,
		Requirements: reqs,
		Completion:   *comp,
		Responses:    responses,
		Uploads:      uploads,
		CreatorName:  creatorName,
	}, nil
}

// MySummaryStats returns per-user task status counts.
// Optional from/to filter by task created_at.
func (s *Store) MySummaryStats(ctx context.Context, tenantID, userID uuid.UUID, from, to *time.Time) (*MySummaryStats, error) {
	where := "t.tenant_id = $1 AND tus.user_id = $2 AND t.status = 'active'"
	args := []any{tenantID, userID}
	n := 3
	if from != nil {
		where += fmt.Sprintf(" AND t.created_at >= $%d", n)
		args = append(args, *from)
		n++
	}
	if to != nil {
		where += fmt.Sprintf(" AND t.created_at <= $%d", n)
		args = append(args, *to)
	}

	var stats MySummaryStats
	err := s.db.QueryRow(ctx, "task.MySummaryStats",
		fmt.Sprintf(`SELECT
		    COUNT(*),
		    COUNT(*) FILTER (WHERE tus.status = 'to_do'),
		    COUNT(*) FILTER (WHERE tus.status = 'in_progress'),
		    COUNT(*) FILTER (WHERE tus.status = 'submitted'),
		    COUNT(*) FILTER (WHERE tus.status = 'approved')
		 FROM task_user_status tus
		 JOIN tasks t ON t.id = tus.task_id
		 WHERE %s`, where),
		args...,
	).Scan(&stats.Total, &stats.ToDo, &stats.InProgress, &stats.Submitted, &stats.Approved)
	if err != nil {
		return nil, fmt.Errorf("my summary stats: %w", err)
	}
	return &stats, nil
}

// EnsureCompletion upserts a completion row for the given task and user, verifying
// the task belongs to tenantID to prevent cross-tenant IDOR.
func (s *Store) EnsureCompletion(ctx context.Context, tenantID, taskID, userID uuid.UUID) (*Completion, error) {
	var c Completion
	err := s.db.QueryRow(ctx, "task.EnsureCompletion",
		`INSERT INTO task_completions (task_id, user_id)
		 SELECT $1, $2 WHERE EXISTS (SELECT 1 FROM tasks WHERE id = $1 AND tenant_id = $3)
		 ON CONFLICT (task_id, user_id) DO UPDATE SET updated_at = now()
		 RETURNING id, task_id, user_id, status, viewed_at, submitted_at, reviewed_at, reviewed_by, created_at, updated_at`,
		taskID, userID, tenantID,
	).Scan(&c.ID, &c.TaskID, &c.UserID, &c.Status, &c.ViewedAt, &c.SubmittedAt, &c.ReviewedAt, &c.ReviewedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("ensure completion: %w", err)
	}
	return &c, nil
}

// MarkViewed sets viewed_at on a completion if not already set.
func (s *Store) MarkViewed(ctx context.Context, completionID uuid.UUID) error {
	_, err := s.db.Exec(ctx, "task.MarkViewed",
		`UPDATE task_completions SET viewed_at = now(), updated_at = now() WHERE id = $1 AND viewed_at IS NULL`,
		completionID,
	)
	if err != nil {
		return fmt.Errorf("mark viewed: %w", err)
	}
	return nil
}

// SaveResponses upserts a batch of responses for a completion and updates status to in_progress.
func (s *Store) SaveResponses(ctx context.Context, completionID uuid.UUID, responses []SaveResponseParams) error {
	for _, r := range responses {
		_, err := s.db.Exec(ctx, "task.SaveResponses",
			`INSERT INTO task_responses (completion_id, requirement_id, text_value, bool_value)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (completion_id, requirement_id) DO UPDATE SET
			     text_value = EXCLUDED.text_value, bool_value = EXCLUDED.bool_value, updated_at = now()`,
			completionID, r.RequirementID, r.TextValue, r.BoolValue,
		)
		if err != nil {
			return fmt.Errorf("save response: %w", err)
		}
	}
	_, err := s.db.Exec(ctx, "task.SaveResponses.status",
		`UPDATE task_completions SET status = 'in_progress', updated_at = now()
		 WHERE id = $1 AND status IN ('to_do', 'rejected')`,
		completionID,
	)
	if err != nil {
		return fmt.Errorf("update completion status: %w", err)
	}
	return nil
}

// Submit marks a completion as submitted.
func (s *Store) Submit(ctx context.Context, completionID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "task.Submit",
		`UPDATE task_completions SET status = 'submitted', submitted_at = now(), updated_at = now()
		 WHERE id = $1 AND status IN ('to_do', 'in_progress', 'rejected')`,
		completionID,
	)
	if err != nil {
		return fmt.Errorf("submit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("cannot submit: invalid status")
	}
	return nil
}

// Approve marks a submitted completion as approved.
func (s *Store) Approve(ctx context.Context, completionID, reviewerID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "task.Approve",
		`UPDATE task_completions SET status = 'approved', reviewed_at = now(), reviewed_by = $2, updated_at = now()
		 WHERE id = $1 AND status = 'submitted'`,
		completionID, reviewerID,
	)
	if err != nil {
		return fmt.Errorf("approve: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("cannot approve: invalid status")
	}
	return nil
}

// Reject marks a submitted completion as rejected.
func (s *Store) Reject(ctx context.Context, completionID, reviewerID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "task.Reject",
		`UPDATE task_completions SET status = 'rejected', reviewed_at = now(), reviewed_by = $2, updated_at = now()
		 WHERE id = $1 AND status = 'submitted'`,
		completionID, reviewerID,
	)
	if err != nil {
		return fmt.Errorf("reject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("cannot reject: invalid status")
	}
	return nil
}

// CreateUpload inserts an upload record and populates ID and CreatedAt on the passed struct.
func (s *Store) CreateUpload(ctx context.Context, u *Upload) error {
	err := s.db.QueryRow(ctx, "task.CreateUpload",
		`INSERT INTO task_uploads (completion_id, requirement_id, file_name, file_size, content_type, storage_key)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		u.CompletionID, u.RequirementID, u.FileName, u.FileSize, u.ContentType, u.StorageKey,
	).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		return fmt.Errorf("create upload: %w", err)
	}
	return nil
}

// ListUploads returns all uploads for a completion.
func (s *Store) ListUploads(ctx context.Context, completionID uuid.UUID) ([]Upload, error) {
	rows, err := s.db.Query(ctx, "task.ListUploads",
		`SELECT id, completion_id, requirement_id, file_name, file_size, content_type, storage_key, created_at
		 FROM task_uploads WHERE completion_id = $1 ORDER BY created_at`,
		completionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list uploads: %w", err)
	}
	defer rows.Close()

	result := make([]Upload, 0)
	for rows.Next() {
		var u Upload
		if err := rows.Scan(&u.ID, &u.CompletionID, &u.RequirementID, &u.FileName, &u.FileSize, &u.ContentType, &u.StorageKey, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan upload: %w", err)
		}
		result = append(result, u)
	}
	return result, nil
}

// GetUploadForDownload returns the storage key, file name, and content type for an upload,
// scoped to the given tenant (via task chain) for access control.
func (s *Store) GetUploadForDownload(ctx context.Context, tenantID, uploadID uuid.UUID) (key, fileName, contentType string, err error) {
	err = s.db.QueryRow(ctx, "task.GetUploadForDownload",
		`SELECT u.storage_key, u.file_name, u.content_type
		 FROM task_uploads u
		 JOIN task_completions c ON c.id = u.completion_id
		 JOIN tasks t ON t.id = c.task_id
		 WHERE u.id = $1 AND t.tenant_id = $2`,
		uploadID, tenantID,
	).Scan(&key, &fileName, &contentType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = ErrNotFound
		}
	}
	return
}

// DeleteUpload removes an upload and returns its storage key for cleanup.
// tenantID is verified via the completion→task join to prevent cross-tenant deletion.
func (s *Store) DeleteUpload(ctx context.Context, tenantID, uploadID uuid.UUID) (string, error) {
	var key string
	err := s.db.QueryRow(ctx, "task.DeleteUpload",
		`DELETE FROM task_uploads tu
		 USING task_completions tc
		 JOIN tasks t ON t.id = tc.task_id
		 WHERE tu.id = $1 AND tu.completion_id = tc.id AND t.tenant_id = $2
		 RETURNING tu.storage_key`,
		uploadID, tenantID,
	).Scan(&key)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("delete upload: %w", err)
	}
	return key, nil
}

// ListTenantUsers returns individual_contributor users in a tenant, optionally
// filtered by search. When subOrgScope is non-nil, results are limited to ICs
// in that sub-org — used to scope FSOs to their own sub-org.
func (s *Store) ListTenantUsers(ctx context.Context, tenantID uuid.UUID, search string, subOrgScope *uuid.UUID) ([]TenantUser, error) {
	where := "u.tenant_id = $1 AND u.role = 'individual_contributor'"
	args := []any{tenantID}
	n := 2
	if search != "" {
		where += fmt.Sprintf(" AND (i.name ILIKE '%%' || $%d || '%%' OR i.email ILIKE '%%' || $%d || '%%')", n, n)
		args = append(args, search)
		n++
	}
	if subOrgScope != nil {
		where += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM user_suborganizations us WHERE us.user_id = u.id AND us.suborganization_id = $%d)", n)
		args = append(args, *subOrgScope)
	}

	rows, err := s.db.Query(ctx, "task.ListTenantUsers",
		fmt.Sprintf(`SELECT u.id, i.name, i.email, u.role,
		        (SELECT us.suborganization_id FROM user_suborganizations us WHERE us.user_id = u.id LIMIT 1)
		 FROM users u JOIN identities i ON i.id = u.identity_id
		 WHERE %s ORDER BY i.name LIMIT 50`, where),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list tenant users: %w", err)
	}
	defer rows.Close()

	var result []TenantUser
	for rows.Next() {
		var tu TenantUser
		if err := rows.Scan(&tu.UserID, &tu.Name, &tu.Email, &tu.Role, &tu.SubOrgID); err != nil {
			return nil, fmt.Errorf("scan tenant user: %w", err)
		}
		result = append(result, tu)
	}
	return result, nil
}

// UsersInSubOrg returns the IDs of users (from the given set) that are members
// of the given sub-org. Used to validate FSO-submitted task assignment rules.
func (s *Store) UsersInSubOrg(ctx context.Context, userIDs []uuid.UUID, subOrgID uuid.UUID) (map[uuid.UUID]bool, error) {
	if len(userIDs) == 0 {
		return map[uuid.UUID]bool{}, nil
	}
	rows, err := s.db.Query(ctx, "task.UsersInSubOrg",
		`SELECT us.user_id
		 FROM user_suborganizations us
		 WHERE us.suborganization_id = $1 AND us.user_id = ANY($2)`,
		subOrgID, userIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("users in sub-org: %w", err)
	}
	defer rows.Close()
	out := make(map[uuid.UUID]bool, len(userIDs))
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan users in sub-org: %w", err)
		}
		out[id] = true
	}
	return out, nil
}

// ListSubOrgs returns suborganizations for a tenant.
func (s *Store) ListSubOrgs(ctx context.Context, tenantID uuid.UUID) ([]SubOrgOption, error) {
	rows, err := s.db.Query(ctx, "task.ListSubOrgs",
		`SELECT id, name FROM suborganizations WHERE tenant_id = $1 ORDER BY name`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list sub orgs: %w", err)
	}
	defer rows.Close()

	var result []SubOrgOption
	for rows.Next() {
		var o SubOrgOption
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, fmt.Errorf("scan sub org: %w", err)
		}
		result = append(result, o)
	}
	return result, nil
}

// AdminGetCompletion returns the full submission detail for a completion, scoped to a tenant.
func (s *Store) AdminGetCompletion(ctx context.Context, tenantID, completionID uuid.UUID) (*AdminSubmission, error) {
	var sub AdminSubmission
	err := s.db.QueryRow(ctx, "task.AdminGetCompletion",
		`SELECT t.title, i.name, i.email, tc.status, tc.submitted_at
		 FROM task_completions tc
		 JOIN tasks t ON t.id = tc.task_id
		 JOIN users u ON u.id = tc.user_id
		 JOIN identities i ON i.id = u.identity_id
		 WHERE tc.id = $1 AND t.tenant_id = $2`,
		completionID, tenantID,
	).Scan(&sub.TaskTitle, &sub.UserName, &sub.UserEmail, &sub.Status, &sub.SubmittedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get completion: %w", err)
	}

	taskID := uuid.Nil
	err = s.db.QueryRow(ctx, "task.AdminGetCompletion.taskID",
		`SELECT task_id FROM task_completions WHERE id = $1`, completionID,
	).Scan(&taskID)
	if err != nil {
		return nil, fmt.Errorf("get task id: %w", err)
	}

	reqs, err := s.listRequirements(ctx, taskID)
	if err != nil {
		return nil, err
	}
	sub.Requirements = reqs

	responses, err := s.listResponses(ctx, completionID)
	if err != nil {
		return nil, err
	}
	sub.Responses = responses

	uploads, err := s.ListUploads(ctx, completionID)
	if err != nil {
		return nil, err
	}
	sub.Uploads = uploads

	return &sub, nil
}

// GetSubmissionContext returns the task title and submitter's display name,
// used to build a meaningful action item title on submission.
func (s *Store) GetSubmissionContext(ctx context.Context, taskID, userID, tenantID uuid.UUID) (taskTitle, userName string, err error) {
	err = s.db.QueryRow(ctx, "task.GetSubmissionContext",
		`SELECT t.title, i.name
		 FROM tasks t, identities i
		 JOIN users u ON u.identity_id = i.id
		 WHERE t.id = $1 AND u.id = $2 AND u.tenant_id = $3`,
		taskID, userID, tenantID,
	).Scan(&taskTitle, &userName)
	return
}

// AssigneeContact is the minimal contact info needed to notify an IC about a task.
type AssigneeContact struct {
	UserID uuid.UUID
	Email  string
	Name   string
}

// ListAssigneeContacts returns the user_id, email, and name for every IC assigned
// to a task via the task_assigned_users view (direct, sub-org, or org-wide rules).
func (s *Store) ListAssigneeContacts(ctx context.Context, taskID uuid.UUID) ([]AssigneeContact, error) {
	rows, err := s.db.Query(ctx, "task.ListAssigneeContacts",
		`SELECT DISTINCT u.id, i.email, i.name
		 FROM task_assigned_users tau
		 JOIN users u ON u.id = tau.user_id
		 JOIN identities i ON i.id = u.identity_id
		 WHERE tau.task_id = $1`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("list assignee contacts: %w", err)
	}
	defer rows.Close()

	var out []AssigneeContact
	for rows.Next() {
		var a AssigneeContact
		if err := rows.Scan(&a.UserID, &a.Email, &a.Name); err != nil {
			return nil, fmt.Errorf("scan assignee contact: %w", err)
		}
		out = append(out, a)
	}
	return out, nil
}

// GetUserContact returns the email and display name for a tenant user.
// Returns empty strings (no error) if the user is not found.
func (s *Store) GetUserContact(ctx context.Context, tenantID, userID uuid.UUID) (email, name string, err error) {
	err = s.db.QueryRow(ctx, "task.GetUserContact",
		`SELECT i.email, i.name
		 FROM users u
		 JOIN identities i ON i.id = u.identity_id
		 WHERE u.id = $1 AND u.tenant_id = $2`,
		userID, tenantID,
	).Scan(&email, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	return
}

func (s *Store) listRequirements(ctx context.Context, taskID uuid.UUID) ([]Requirement, error) {
	rows, err := s.db.Query(ctx, "task.listRequirements",
		`SELECT id, task_id, kind, label, description, required, sort_order, ai_verification_criteria
		 FROM task_requirements WHERE task_id = $1 ORDER BY sort_order`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("list requirements: %w", err)
	}
	defer rows.Close()

	result := make([]Requirement, 0)
	for rows.Next() {
		var r Requirement
		if err := rows.Scan(&r.ID, &r.TaskID, &r.Kind, &r.Label, &r.Description, &r.Required, &r.SortOrder, &r.AIVerificationCriteria); err != nil {
			return nil, fmt.Errorf("scan requirement: %w", err)
		}
		result = append(result, r)
	}
	return result, nil
}

// GetRequirement returns a single requirement by ID, including its AI
// verification criteria. Used by the upload handler to decide whether to
// validate content type and enqueue a verification job.
func (s *Store) GetRequirement(ctx context.Context, requirementID uuid.UUID) (Requirement, error) {
	var r Requirement
	err := s.db.QueryRow(ctx, "task.GetRequirement",
		`SELECT id, task_id, kind, label, description, required, sort_order, ai_verification_criteria
		 FROM task_requirements WHERE id = $1`,
		requirementID,
	).Scan(&r.ID, &r.TaskID, &r.Kind, &r.Label, &r.Description, &r.Required, &r.SortOrder, &r.AIVerificationCriteria)
	if err != nil {
		return Requirement{}, fmt.Errorf("get requirement: %w", err)
	}
	return r, nil
}

func (s *Store) listRules(ctx context.Context, taskID uuid.UUID) ([]AssignmentRule, error) {
	rows, err := s.db.Query(ctx, "task.listRules",
		`SELECT id, task_id, rule_type, target_id FROM task_assignment_rules WHERE task_id = $1`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	defer rows.Close()

	result := make([]AssignmentRule, 0)
	for rows.Next() {
		var r AssignmentRule
		if err := rows.Scan(&r.ID, &r.TaskID, &r.RuleType, &r.TargetID); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}
		result = append(result, r)
	}
	return result, nil
}

func (s *Store) listResponses(ctx context.Context, completionID uuid.UUID) ([]Response, error) {
	rows, err := s.db.Query(ctx, "task.listResponses",
		`SELECT id, completion_id, requirement_id, text_value, bool_value
		 FROM task_responses WHERE completion_id = $1`,
		completionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list responses: %w", err)
	}
	defer rows.Close()

	result := make([]Response, 0)
	for rows.Next() {
		var r Response
		if err := rows.Scan(&r.ID, &r.CompletionID, &r.RequirementID, &r.TextValue, &r.BoolValue); err != nil {
			return nil, fmt.Errorf("scan response: %w", err)
		}
		result = append(result, r)
	}
	return result, nil
}

// ExportByDateRange returns all task completion rows for a tenant where the
// task due_date falls within [from, to] (inclusive). Used for CSV/PDF export.
func (s *Store) ExportByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]ExportRow, error) {
	rows, err := s.db.Query(ctx, "task.ExportByDateRange",
		`SELECT
		     t.title,
		     COALESCE(so.name, '') AS sub_org_name,
		     i.name AS assignee_name,
		     i.email AS assignee_email,
		     tc.status,
		     t.due_date,
		     tc.submitted_at
		 FROM task_completions tc
		 JOIN tasks t ON t.id = tc.task_id
		 JOIN users u ON u.id = tc.user_id
		 JOIN identities i ON i.id = u.identity_id
		 LEFT JOIN suborganizations so ON so.id = t.sub_org_id
		 WHERE t.tenant_id = $1
		   AND t.created_at >= $2
		   AND t.created_at <= $3
		 ORDER BY t.created_at, t.title, i.name`,
		tenantID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("export tasks by date range: %w", err)
	}
	defer rows.Close()

	var result []ExportRow
	for rows.Next() {
		var r ExportRow
		if err := rows.Scan(
			&r.TaskTitle, &r.SubOrgName, &r.AssigneeName, &r.AssigneeEmail,
			&r.Status, &r.DueDate, &r.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan export row: %w", err)
		}
		result = append(result, r)
	}
	return result, nil
}
