package task

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/fastfso/fastfso/backend/internal/verification"
)

var ErrNotFound = errors.New("task not found")

// Task is the core task entity. CreatedByUserID is nullable because removing
// a tenant member sets the column to NULL — the task survives the creator.
type Task struct {
	ID              uuid.UUID  `json:"id"`
	TenantID        uuid.UUID  `json:"tenant_id"`
	CreatedByUserID *uuid.UUID `json:"created_by_user_id"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Priority        string     `json:"priority"`
	DueDate         *time.Time `json:"due_date"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Requirement defines what the assignee must fill out.
type Requirement struct {
	ID                     uuid.UUID `json:"id"`
	TaskID                 uuid.UUID `json:"task_id"`
	Kind                   string    `json:"kind"`
	Label                  string    `json:"label"`
	Description            string    `json:"description"`
	Required               bool      `json:"required"`
	SortOrder              int       `json:"sort_order"`
	AIVerificationCriteria string    `json:"ai_verification_criteria"`
}

// AssignmentRule stores the intent-based assignment.
type AssignmentRule struct {
	ID       uuid.UUID  `json:"id"`
	TaskID   uuid.UUID  `json:"task_id"`
	RuleType string     `json:"rule_type"`
	TargetID *uuid.UUID `json:"target_id"`
}

// Completion tracks a user's progress on a task.
type Completion struct {
	ID          uuid.UUID  `json:"id"`
	TaskID      uuid.UUID  `json:"task_id"`
	UserID      uuid.UUID  `json:"user_id"`
	Status      string     `json:"status"`
	ViewedAt    *time.Time `json:"viewed_at"`
	SubmittedAt *time.Time `json:"submitted_at"`
	ReviewedAt  *time.Time `json:"reviewed_at"`
	ReviewedBy  *uuid.UUID `json:"reviewed_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Response stores a user's answer to a requirement.
type Response struct {
	ID            uuid.UUID `json:"id"`
	CompletionID  uuid.UUID `json:"completion_id"`
	RequirementID uuid.UUID `json:"requirement_id"`
	TextValue     *string   `json:"text_value"`
	BoolValue     *bool     `json:"bool_value"`
}

// Upload stores file upload metadata.
type Upload struct {
	ID            uuid.UUID `json:"id"`
	CompletionID  uuid.UUID `json:"completion_id"`
	RequirementID uuid.UUID `json:"requirement_id"`
	FileName      string    `json:"file_name"`
	FileSize      int64     `json:"file_size"`
	ContentType   string    `json:"content_type"`
	StorageKey    string    `json:"storage_key"`
	CreatedAt     time.Time `json:"created_at"`
}

// --- Query types ---

// TaskRow is returned by admin List queries (includes creator name and assignee counts).
type TaskRow struct {
	ID            uuid.UUID  `json:"id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Priority      string     `json:"priority"`
	DueDate       *time.Time `json:"due_date"`
	Status        string     `json:"status"`
	CreatorName   string     `json:"creator_name"`
	TotalAssigned int        `json:"total_assigned"`
	Completed     int        `json:"completed"`
	CreatedAt     time.Time  `json:"created_at"`
}

// AssigneeStatus is used in TaskDetail to show each assignee's progress.
type AssigneeStatus struct {
	CompletionID uuid.UUID  `json:"completion_id"`
	UserID       uuid.UUID  `json:"user_id"`
	UserName     string     `json:"user_name"`
	UserEmail    string     `json:"user_email"`
	Status       string     `json:"status"`
	ViewedAt     *time.Time `json:"viewed_at"`
	SubmittedAt  *time.Time `json:"submitted_at"`
}

// AdminSubmission is the admin view of a single user's task completion.
// Verifications maps upload_id -> verification record for any uploads whose
// requirement had AI criteria set. Empty for uploads without AI criteria.
type AdminSubmission struct {
	TaskTitle     string                          `json:"task_title"`
	UserName      string                          `json:"user_name"`
	UserEmail     string                          `json:"user_email"`
	Status        string                          `json:"status"`
	SubmittedAt   *time.Time                      `json:"submitted_at"`
	Requirements  []Requirement                   `json:"requirements"`
	Responses     []Response                      `json:"responses"`
	Uploads       []Upload                        `json:"uploads"`
	Verifications map[string]*verification.Record `json:"verifications"`
}

// TaskDetail is the admin view of a single task.
type TaskDetail struct {
	Task         Task             `json:"task"`
	Requirements []Requirement    `json:"requirements"`
	Rules        []AssignmentRule `json:"rules"`
	Assignees    []AssigneeStatus `json:"assignees"`
}

// MyTaskRow is returned by IC ListMyTasks.
type MyTaskRow struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    string     `json:"priority"`
	DueDate     *time.Time `json:"due_date"`
	Status      string     `json:"status"`
	CreatorName string     `json:"creator_name"`
	CreatedAt   time.Time  `json:"created_at"`
}

// MyTaskDetail is the IC view of a single task (with their responses).
// Verifications follows the same shape as AdminSubmission.
type MyTaskDetail struct {
	Task          Task                            `json:"task"`
	Requirements  []Requirement                   `json:"requirements"`
	Completion    Completion                      `json:"completion"`
	Responses     []Response                      `json:"responses"`
	Uploads       []Upload                        `json:"uploads"`
	Verifications map[string]*verification.Record `json:"verifications"`
	CreatorName   string                          `json:"creator_name"`
}

// --- Params ---

type CreateRequirementParams struct {
	Kind                   string `json:"kind"`
	Label                  string `json:"label"`
	Description            string `json:"description"`
	Required               bool   `json:"required"`
	SortOrder              int    `json:"sort_order"`
	AIVerificationCriteria string `json:"ai_verification_criteria"`
}

type CreateRuleParams struct {
	RuleType string     `json:"rule_type"`
	TargetID *uuid.UUID `json:"target_id"`
}

type CreateParams struct {
	TenantID        uuid.UUID
	CreatedByUserID uuid.UUID
	SubOrgID        *uuid.UUID                `json:"sub_org_id"`
	Title           string                    `json:"title"`
	Description     string                    `json:"description"`
	Priority        string                    `json:"priority"`
	DueDate         *string                   `json:"due_date"`
	Status          string                    `json:"status"`
	Requirements    []CreateRequirementParams `json:"requirements"`
	Rules           []CreateRuleParams        `json:"rules"`
}

type ListFilters struct {
	TenantID    uuid.UUID
	Status      string
	Priority    string
	Search      string
	SubOrgScope *uuid.UUID // nil = no filter; non-nil = this sub-org + tenant-wide
	Limit       int
	Offset      int
}

// SaveResponseParams is one response in a batch save.
type SaveResponseParams struct {
	RequirementID uuid.UUID `json:"requirement_id"`
	TextValue     *string   `json:"text_value"`
	BoolValue     *bool     `json:"bool_value"`
}

// --- Stats ---

type SummaryStats struct {
	Total       int `json:"total"`
	Active      int `json:"active"`
	Draft       int `json:"draft"`
	NeedsReview int `json:"needs_review"`
}

type MySummaryStats struct {
	Total      int `json:"total"`
	ToDo       int `json:"to_do"`
	InProgress int `json:"in_progress"`
	Submitted  int `json:"submitted"`
	Approved   int `json:"approved"`
}

// --- Lookup types ---

type TenantUser struct {
	UserID   uuid.UUID  `json:"user_id"`
	Name     string     `json:"name"`
	Email    string     `json:"email"`
	Role     string     `json:"role"`
	SubOrgID *uuid.UUID `json:"sub_org_id"`
}

type SubOrgOption struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// ExportRow is one row in a task completion export report.
type ExportRow struct {
	TaskTitle     string
	SubOrgName    string // empty string means org-wide
	AssigneeName  string
	AssigneeEmail string
	Status        string
	DueDate       *time.Time
	CompletedAt   *time.Time
}
