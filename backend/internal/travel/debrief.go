package travel

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrDebriefNotFound = errors.New("travel debrief not found")

// Debrief is the post-travel debrief entity.
type Debrief struct {
	ID                uuid.UUID  `json:"id"`
	ReportID          uuid.UUID  `json:"report_id"`
	TenantID          uuid.UUID  `json:"tenant_id"`
	UserID            uuid.UUID  `json:"user_id"`
	Status            string     `json:"status"`
	DueDate           *time.Time `json:"due_date"`
	Q1ForeignContact  *bool      `json:"q1_foreign_contact"`
	Q1Details         string     `json:"q1_details"`
	Q2Surveillance    *bool      `json:"q2_surveillance"`
	Q2Details         string     `json:"q2_details"`
	Q3EquipmentLoss   *bool      `json:"q3_equipment_loss"`
	Q3Details         string     `json:"q3_details"`
	Q4UnusualRequests *bool      `json:"q4_unusual_requests"`
	Q4Details         string     `json:"q4_details"`
	SubmittedAt       *time.Time `json:"submitted_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	TripName          string     `json:"trip_name"`
}

// SubmitDebriefParams holds the IC's answers.
type SubmitDebriefParams struct {
	Q1ForeignContact  bool   `json:"q1_foreign_contact"`
	Q1Details         string `json:"q1_details"`
	Q2Surveillance    bool   `json:"q2_surveillance"`
	Q2Details         string `json:"q2_details"`
	Q3EquipmentLoss   bool   `json:"q3_equipment_loss"`
	Q3Details         string `json:"q3_details"`
	Q4UnusualRequests bool   `json:"q4_unusual_requests"`
	Q4Details         string `json:"q4_details"`
}

// PendingDebriefReport is used by the cron job.
type PendingDebriefReport struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	UserID   uuid.UUID
	TripName string
	SubOrgID *uuid.UUID
}

const debriefScanFields = `d.id, d.report_id, d.tenant_id, d.user_id, d.status, d.due_date,
	d.q1_foreign_contact, d.q1_details, d.q2_surveillance, d.q2_details,
	d.q3_equipment_loss, d.q3_details, d.q4_unusual_requests, d.q4_details,
	d.submitted_at, d.created_at, d.updated_at, tr.trip_name`

func scanDebrief(row interface {
	Scan(...any) error
}, d *Debrief) error {
	return row.Scan(
		&d.ID, &d.ReportID, &d.TenantID, &d.UserID, &d.Status, &d.DueDate,
		&d.Q1ForeignContact, &d.Q1Details, &d.Q2Surveillance, &d.Q2Details,
		&d.Q3EquipmentLoss, &d.Q3Details, &d.Q4UnusualRequests, &d.Q4Details,
		&d.SubmittedAt, &d.CreatedAt, &d.UpdatedAt, &d.TripName,
	)
}

// CreateDebrief inserts a new pending debrief for an IC.
func (s *Store) CreateDebrief(ctx context.Context, reportID, tenantID, userID uuid.UUID, dueDate *time.Time) (*Debrief, error) {
	var d Debrief
	err := s.db.QueryRow(ctx, "travel.CreateDebrief",
		`INSERT INTO travel_debriefs (report_id, tenant_id, user_id, due_date)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, report_id, tenant_id, user_id, status, due_date,
		           q1_foreign_contact, q1_details, q2_surveillance, q2_details,
		           q3_equipment_loss, q3_details, q4_unusual_requests, q4_details,
		           submitted_at, created_at, updated_at`,
		reportID, tenantID, userID, dueDate,
	).Scan(
		&d.ID, &d.ReportID, &d.TenantID, &d.UserID, &d.Status, &d.DueDate,
		&d.Q1ForeignContact, &d.Q1Details, &d.Q2Surveillance, &d.Q2Details,
		&d.Q3EquipmentLoss, &d.Q3Details, &d.Q4UnusualRequests, &d.Q4Details,
		&d.SubmittedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create travel debrief: %w", err)
	}
	return &d, nil
}

// MarkDebriefCreated sets debrief_created_at on the travel report.
func (s *Store) MarkDebriefCreated(ctx context.Context, reportID uuid.UUID) error {
	_, err := s.db.Exec(ctx, "travel.MarkDebriefCreated",
		`UPDATE travel_reports SET debrief_created_at = now() WHERE id = $1`,
		reportID,
	)
	if err != nil {
		return fmt.Errorf("mark debrief created: %w", err)
	}
	return nil
}

// ListPendingDebriefReports returns approved reports whose trip has fully ended
// and no debrief has been created yet.
func (s *Store) ListPendingDebriefReports(ctx context.Context) ([]PendingDebriefReport, error) {
	rows, err := s.db.Query(ctx, "travel.ListPendingDebriefReports",
		`SELECT tr.id, tr.tenant_id, tr.user_id, tr.trip_name, tr.sub_org_id
		 FROM travel_reports tr
		 WHERE tr.status = 'approved'
		   AND tr.debrief_created_at IS NULL
		   AND EXISTS (
		       SELECT 1 FROM travel_report_countries c
		       WHERE c.report_id = tr.id AND c.end_date IS NOT NULL
		   )
		   AND NOT EXISTS (
		       SELECT 1 FROM travel_report_countries c
		       WHERE c.report_id = tr.id
		         AND (c.end_date IS NULL OR c.end_date >= CURRENT_DATE)
		   )`,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending debrief reports: %w", err)
	}
	defer rows.Close()

	var result []PendingDebriefReport
	for rows.Next() {
		var r PendingDebriefReport
		if err := rows.Scan(&r.ID, &r.TenantID, &r.UserID, &r.TripName, &r.SubOrgID); err != nil {
			return nil, fmt.Errorf("scan pending debrief report: %w", err)
		}
		result = append(result, r)
	}
	return result, nil
}

// ListMyDebriefs returns all debriefs for a specific user within a tenant.
func (s *Store) ListMyDebriefs(ctx context.Context, tenantID, userID uuid.UUID) ([]Debrief, error) {
	rows, err := s.db.Query(ctx, "travel.ListMyDebriefs",
		fmt.Sprintf(`SELECT %s
		 FROM travel_debriefs d
		 JOIN travel_reports tr ON tr.id = d.report_id
		 WHERE d.tenant_id = $1 AND d.user_id = $2
		 ORDER BY d.created_at DESC`, debriefScanFields),
		tenantID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list my debriefs: %w", err)
	}
	defer rows.Close()

	var result []Debrief
	for rows.Next() {
		var d Debrief
		if err := scanDebrief(rows, &d); err != nil {
			return nil, fmt.Errorf("scan debrief: %w", err)
		}
		result = append(result, d)
	}
	return result, nil
}

// GetDebrief retrieves a single debrief by ID, scoped to the user.
func (s *Store) GetDebrief(ctx context.Context, tenantID, userID, id uuid.UUID) (*Debrief, error) {
	var d Debrief
	err := s.db.QueryRow(ctx, "travel.GetDebrief",
		fmt.Sprintf(`SELECT %s
		 FROM travel_debriefs d
		 JOIN travel_reports tr ON tr.id = d.report_id
		 WHERE d.id = $1 AND d.tenant_id = $2 AND d.user_id = $3`, debriefScanFields),
		id, tenantID, userID,
	).Scan(
		&d.ID, &d.ReportID, &d.TenantID, &d.UserID, &d.Status, &d.DueDate,
		&d.Q1ForeignContact, &d.Q1Details, &d.Q2Surveillance, &d.Q2Details,
		&d.Q3EquipmentLoss, &d.Q3Details, &d.Q4UnusualRequests, &d.Q4Details,
		&d.SubmittedAt, &d.CreatedAt, &d.UpdatedAt, &d.TripName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDebriefNotFound
		}
		return nil, fmt.Errorf("get debrief: %w", err)
	}
	return &d, nil
}

// GetDebriefAdmin retrieves a debrief by ID scoped to tenant only (for FSO use).
func (s *Store) GetDebriefAdmin(ctx context.Context, tenantID, id uuid.UUID) (*Debrief, error) {
	var d Debrief
	err := s.db.QueryRow(ctx, "travel.GetDebriefAdmin",
		fmt.Sprintf(`SELECT %s
		 FROM travel_debriefs d
		 JOIN travel_reports tr ON tr.id = d.report_id
		 WHERE d.id = $1 AND d.tenant_id = $2`, debriefScanFields),
		id, tenantID,
	).Scan(
		&d.ID, &d.ReportID, &d.TenantID, &d.UserID, &d.Status, &d.DueDate,
		&d.Q1ForeignContact, &d.Q1Details, &d.Q2Surveillance, &d.Q2Details,
		&d.Q3EquipmentLoss, &d.Q3Details, &d.Q4UnusualRequests, &d.Q4Details,
		&d.SubmittedAt, &d.CreatedAt, &d.UpdatedAt, &d.TripName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDebriefNotFound
		}
		return nil, fmt.Errorf("get debrief admin: %w", err)
	}
	return &d, nil
}

// SubmitDebrief saves the IC's answers and marks the debrief submitted.
func (s *Store) SubmitDebrief(ctx context.Context, id uuid.UUID, p SubmitDebriefParams) error {
	tag, err := s.db.Exec(ctx, "travel.SubmitDebrief",
		`UPDATE travel_debriefs
		 SET status = 'submitted',
		     q1_foreign_contact = $2, q1_details = $3,
		     q2_surveillance = $4, q2_details = $5,
		     q3_equipment_loss = $6, q3_details = $7,
		     q4_unusual_requests = $8, q4_details = $9,
		     submitted_at = now(),
		     updated_at = now()
		 WHERE id = $1 AND status = 'pending'`,
		id,
		p.Q1ForeignContact, p.Q1Details,
		p.Q2Surveillance, p.Q2Details,
		p.Q3EquipmentLoss, p.Q3Details,
		p.Q4UnusualRequests, p.Q4Details,
	)
	if err != nil {
		return fmt.Errorf("submit debrief: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDebriefNotFound
	}
	return nil
}
