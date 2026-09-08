// Package verification handles AI-assisted review of task upload documents.
// When an FSO sets ai_verification_criteria on a file_upload requirement, each
// upload to that requirement triggers an asynchronous Cloud Task that invokes
// Gemini to check the document against the criteria. Results are advisory:
// they surface to admins in the review queue, but never gate user submission.
package verification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/SSpencer740/fastfso/backend/internal/database"
)

// Status is the lifecycle state of a verification record.
const (
	StatusPending   = "pending"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// AdminFeedback values capture whether a human admin agreed with the AI verdict.
const (
	FeedbackCorrect   = "correct"
	FeedbackIncorrect = "incorrect"
)

// Verdict is the structured output the AI returns for one verification.
type Verdict struct {
	TypeMatch       bool              `json:"type_match"`
	ExtractedFields map[string]string `json:"extracted_fields"`
	Discrepancies   []string          `json:"discrepancies"`
	Flagged         bool              `json:"flagged"`
	Confidence      float64           `json:"confidence"`
	Reasoning       string            `json:"reasoning"`
}

// Record represents a row from upload_verifications.
type Record struct {
	ID               uuid.UUID         `json:"id"`
	UploadID         uuid.UUID         `json:"upload_id"`
	TenantID         uuid.UUID         `json:"tenant_id"`
	Status           string            `json:"status"`
	Flagged          bool              `json:"flagged"`
	Confidence       *float64          `json:"confidence,omitempty"`
	TypeMatch        *bool             `json:"type_match,omitempty"`
	ExtractedFields  map[string]string `json:"extracted_fields"`
	Discrepancies    []string          `json:"discrepancies"`
	Reasoning        string            `json:"reasoning"`
	ErrorMessage     string            `json:"error_message,omitempty"`
	CriteriaSnapshot string            `json:"criteria_snapshot"`
	Model            string            `json:"model"`
	AdminFeedback    *string           `json:"admin_feedback,omitempty"`
	AdminFeedbackAt  *time.Time        `json:"admin_feedback_at,omitempty"`
	AdminFeedbackBy  *uuid.UUID        `json:"admin_feedback_by,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// CreatePending inserts a verification row in the pending state. Called by the
// upload handler at the moment of upload so the UI can immediately show
// "verifying..." while the Cloud Task is in flight.
func (s *Store) CreatePending(ctx context.Context, uploadID, tenantID uuid.UUID, criteria string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRow(ctx, "verification.CreatePending",
		`INSERT INTO upload_verifications (upload_id, tenant_id, criteria_snapshot)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		uploadID, tenantID, criteria,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("verification create: %w", err)
	}
	return id, nil
}

// MarkSucceeded transitions a pending verification to 'succeeded' with the
// AI's verdict. Called by the verify worker after Gemini returns.
func (s *Store) MarkSucceeded(ctx context.Context, uploadID uuid.UUID, model string, v Verdict) error {
	fields, _ := json.Marshal(v.ExtractedFields)
	discs, _ := json.Marshal(v.Discrepancies)
	_, err := s.db.Exec(ctx, "verification.MarkSucceeded",
		`UPDATE upload_verifications
		 SET status = $1, flagged = $2, confidence = $3, type_match = $4,
		     extracted_fields = $5, discrepancies = $6, reasoning = $7,
		     model = $8, updated_at = now()
		 WHERE upload_id = $9`,
		StatusSucceeded, v.Flagged, v.Confidence, v.TypeMatch,
		fields, discs, v.Reasoning,
		model, uploadID,
	)
	if err != nil {
		return fmt.Errorf("verification mark succeeded: %w", err)
	}
	return nil
}

// MarkFailed transitions a pending verification to 'failed' with an error
// message. Failures are not flagged — they mean the AI couldn't reach a
// verdict, not that the document is suspect.
func (s *Store) MarkFailed(ctx context.Context, uploadID uuid.UUID, errorMessage string) error {
	_, err := s.db.Exec(ctx, "verification.MarkFailed",
		`UPDATE upload_verifications
		 SET status = $1, error_message = $2, updated_at = now()
		 WHERE upload_id = $3`,
		StatusFailed, errorMessage, uploadID,
	)
	if err != nil {
		return fmt.Errorf("verification mark failed: %w", err)
	}
	return nil
}

// GetByUpload returns the verification record for a single upload, or
// pgx.ErrNoRows if none exists (e.g., the requirement has no AI criteria set).
func (s *Store) GetByUpload(ctx context.Context, uploadID uuid.UUID) (*Record, error) {
	r := &Record{}
	var fields, discs []byte
	err := s.db.QueryRow(ctx, "verification.GetByUpload",
		`SELECT id, upload_id, tenant_id, status, flagged, confidence, type_match,
		        extracted_fields, discrepancies, reasoning, error_message,
		        criteria_snapshot, model, admin_feedback, admin_feedback_at,
		        admin_feedback_by, created_at, updated_at
		 FROM upload_verifications
		 WHERE upload_id = $1`,
		uploadID,
	).Scan(
		&r.ID, &r.UploadID, &r.TenantID, &r.Status, &r.Flagged, &r.Confidence,
		&r.TypeMatch, &fields, &discs, &r.Reasoning, &r.ErrorMessage,
		&r.CriteriaSnapshot, &r.Model, &r.AdminFeedback, &r.AdminFeedbackAt,
		&r.AdminFeedbackBy, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(fields, &r.ExtractedFields)
	_ = json.Unmarshal(discs, &r.Discrepancies)
	return r, nil
}

// ListByCompletion returns verifications for every upload tied to a given
// task_completion, keyed by upload_id. Used by the task detail API to
// surface verification state alongside each uploaded file.
func (s *Store) ListByCompletion(ctx context.Context, completionID uuid.UUID) (map[uuid.UUID]*Record, error) {
	rows, err := s.db.Query(ctx, "verification.ListByCompletion",
		`SELECT v.id, v.upload_id, v.tenant_id, v.status, v.flagged, v.confidence,
		        v.type_match, v.extracted_fields, v.discrepancies, v.reasoning,
		        v.error_message, v.criteria_snapshot, v.model, v.admin_feedback,
		        v.admin_feedback_at, v.admin_feedback_by, v.created_at, v.updated_at
		 FROM upload_verifications v
		 JOIN task_uploads u ON u.id = v.upload_id
		 WHERE u.completion_id = $1`,
		completionID,
	)
	if err != nil {
		return nil, fmt.Errorf("verification list by completion: %w", err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID]*Record)
	for rows.Next() {
		r := &Record{}
		var fields, discs []byte
		if err := rows.Scan(
			&r.ID, &r.UploadID, &r.TenantID, &r.Status, &r.Flagged, &r.Confidence,
			&r.TypeMatch, &fields, &discs, &r.Reasoning, &r.ErrorMessage,
			&r.CriteriaSnapshot, &r.Model, &r.AdminFeedback, &r.AdminFeedbackAt,
			&r.AdminFeedbackBy, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("verification scan: %w", err)
		}
		_ = json.Unmarshal(fields, &r.ExtractedFields)
		_ = json.Unmarshal(discs, &r.Discrepancies)
		out[r.UploadID] = r
	}
	return out, rows.Err()
}

// RecordFeedback persists an admin's correct/incorrect tag on a verification.
// Feedback overrides any previous feedback (admin can change their mind).
func (s *Store) RecordFeedback(ctx context.Context, uploadID, adminUserID uuid.UUID, feedback string) error {
	if feedback != FeedbackCorrect && feedback != FeedbackIncorrect {
		return fmt.Errorf("invalid feedback value: %q", feedback)
	}
	tag, err := s.db.Exec(ctx, "verification.RecordFeedback",
		`UPDATE upload_verifications
		 SET admin_feedback = $1, admin_feedback_at = now(), admin_feedback_by = $2,
		     updated_at = now()
		 WHERE upload_id = $3`,
		feedback, adminUserID, uploadID,
	)
	if err != nil {
		return fmt.Errorf("verification record feedback: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
