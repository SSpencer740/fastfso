//go:build integration

package visitrequest_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SSpencer740/fastfso/backend/internal/database"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
	"github.com/SSpencer740/fastfso/backend/internal/visitrequest"
)

// TestStore_Export_SnapshotsClearanceAtReviewTime is the load-bearing audit
// invariant: after a visit is approved, the visitor's clearance can change,
// but the export must still show what authorized the decision.
func TestStore_Export_SnapshotsClearanceAtReviewTime(t *testing.T) {
	db := testutil.DB(t)
	store := visitrequest.NewStore(db)
	ctx := context.Background()
	tenantID := testutil.CreateTenant(t, db, "Acme")
	icIdentity := testutil.CreateIdentity(t, db, "ic@acme.com")
	icUserID := testutil.CreateUser(t, db, tenantID, icIdentity, "individual_contributor")
	adminUserID := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "admin@acme.com"), "administrator")

	// IC had Secret at the time of approval.
	insertClearance(t, db, icUserID, "secret", "2025-01-01")

	vr, err := store.Create(ctx, visitrequest.CreateParams{
		TenantID:         tenantID,
		CreatedByUserID:  icUserID,
		DestinationName:  "Dahlgren",
		DissSmoCode:      "ABCDEFGH",
		VisitAddress:     "VA",
		VisitStartDate:   "2025-06-15",
		VisitEndDate:     "2025-06-20",
		AccessLevel:      "secret",
		VisitDescription: "Review",
		PocName:          "Jane", PocEmail: "j@x.com", PocPhone: "1",
		SecurityPocName: "Bob", SecurityPocEmail: "b@x.com", SecurityPocPhone: "1",
	})
	require.NoError(t, err)
	require.NoError(t, store.UpdateStatus(ctx, tenantID, vr.ID, adminUserID, "approved", "ok"))

	// AFTER approval, the IC's clearance gets revoked. The current-state
	// clearance is now 'none'; the snapshot must still show 'secret'.
	supersedeAndInsertClearance(t, db, icUserID, "none")

	rows, err := store.Export(ctx, visitrequest.ExportFilters{
		TenantID: tenantID,
		From:     time.Now().AddDate(-1, 0, 0),
		To:       time.Now().AddDate(1, 0, 0),
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)

	require.NotNil(t, rows[0].ClearanceAtReview, "snapshot must be populated for reviewed visits")
	assert.Equal(t, "secret", *rows[0].ClearanceAtReview,
		"audit-defining invariant: export shows clearance as-of approval, not current")
}

// TestStore_Export_AuthorizationConfirmed verifies the boolean is true only
// when every gate (read-on temporal, classification cap, period of perf)
// holds at reviewed_at.
func TestStore_Export_AuthorizationConfirmed(t *testing.T) {
	db := testutil.DB(t)
	store := visitrequest.NewStore(db)
	ctx := context.Background()
	tenantID := testutil.CreateTenant(t, db, "Acme")
	icUserID := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "ic@acme.com"), "individual_contributor")
	adminUserID := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "admin@acme.com"), "administrator")

	dd254ID := insertDd254(t, db, tenantID, adminUserID, "C-1", "top_secret", "2024-01-01", "2027-01-01")
	grantDd254Access(t, db, dd254ID, icUserID)

	// Visit covered by the DD254 at the right level, within POP.
	vr, err := store.Create(ctx, visitrequest.CreateParams{
		TenantID: tenantID, CreatedByUserID: icUserID,
		DestinationName: "X", DissSmoCode: "ABCDEFGH", VisitAddress: "Y",
		VisitStartDate: "2025-06-15", VisitEndDate: "2025-06-20",
		AccessLevel: "secret", VisitDescription: "ok",
		PocName: "a", PocEmail: "a@x.com", PocPhone: "1",
		SecurityPocName: "b", SecurityPocEmail: "b@x.com", SecurityPocPhone: "1",
		DD254ID: &dd254ID,
	})
	require.NoError(t, err)
	require.NoError(t, store.UpdateStatus(ctx, tenantID, vr.ID, adminUserID, "approved", ""))

	rows, err := store.Export(ctx, visitrequest.ExportFilters{
		TenantID: tenantID,
		From:     time.Now().AddDate(-1, 0, 0),
		To:       time.Now().AddDate(1, 0, 0),
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].AuthorizationConfirmedAtReview)
	require.NotNil(t, rows[0].DD254ContractNumber)
	assert.Equal(t, "C-1", *rows[0].DD254ContractNumber)
}

// TestStore_Export_FiltersAndDateRange checks status filter + review-date
// window. Visits outside the window must be excluded.
func TestStore_Export_FiltersAndDateRange(t *testing.T) {
	db := testutil.DB(t)
	store := visitrequest.NewStore(db)
	ctx := context.Background()
	tenantID := testutil.CreateTenant(t, db, "Acme")
	icUserID := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "ic@acme.com"), "individual_contributor")
	adminUserID := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "admin@acme.com"), "administrator")

	mkVisit := func(status string) uuid.UUID {
		t.Helper()
		vr, err := store.Create(ctx, visitrequest.CreateParams{
			TenantID: tenantID, CreatedByUserID: icUserID,
			DestinationName: "X", DissSmoCode: "ABCDEFGH", VisitAddress: "Y",
			VisitStartDate: "2025-06-15", VisitEndDate: "2025-06-20",
			AccessLevel: "secret", VisitDescription: "ok",
			PocName: "a", PocEmail: "a@x.com", PocPhone: "1",
			SecurityPocName: "b", SecurityPocEmail: "b@x.com", SecurityPocPhone: "1",
		})
		require.NoError(t, err)
		if status != "submitted" {
			require.NoError(t, store.UpdateStatus(ctx, tenantID, vr.ID, adminUserID, status, ""))
		}
		return vr.ID
	}
	mkVisit("approved")
	mkVisit("rejected")
	mkVisit("submitted")

	wideFrom := time.Now().AddDate(-1, 0, 0)
	wideTo := time.Now().AddDate(1, 0, 0)

	// No status filter — all three rows.
	all, err := store.Export(ctx, visitrequest.ExportFilters{
		TenantID: tenantID, From: wideFrom, To: wideTo,
	})
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// Status filter narrows to approved + rejected.
	some, err := store.Export(ctx, visitrequest.ExportFilters{
		TenantID: tenantID, From: wideFrom, To: wideTo,
		Statuses: []string{"approved", "rejected"},
	})
	require.NoError(t, err)
	assert.Len(t, some, 2)

	// Date range outside the visits — zero rows.
	farPast := time.Now().AddDate(-20, 0, 0)
	zero, err := store.Export(ctx, visitrequest.ExportFilters{
		TenantID: tenantID, From: farPast, To: farPast.AddDate(0, 0, 1),
	})
	require.NoError(t, err)
	assert.Len(t, zero, 0)
}

// --- helpers ---

func insertClearance(t *testing.T, db database.DB, userID uuid.UUID, level, recordedDate string) {
	t.Helper()
	_, err := db.Exec(context.Background(), "test.insertClearance",
		`WITH ins AS (
			INSERT INTO user_clearance_records (user_id, clearance, recorded_by, recorded_at)
			VALUES ($1, $2::clearance_level, $1, $3::timestamptz)
			RETURNING id
		)
		UPDATE users SET current_clearance_id = (SELECT id FROM ins) WHERE id = $1`,
		userID, level, recordedDate)
	require.NoError(t, err)
}

func supersedeAndInsertClearance(t *testing.T, db database.DB, userID uuid.UUID, newLevel string) {
	t.Helper()
	ctx := context.Background()
	_, err := db.Exec(ctx, "test.supersede",
		`UPDATE user_clearance_records SET superseded_at = now()
		 WHERE user_id = $1 AND superseded_at IS NULL`, userID)
	require.NoError(t, err)
	_, err = db.Exec(ctx, "test.insertNew",
		`WITH ins AS (
			INSERT INTO user_clearance_records (user_id, clearance, recorded_by)
			VALUES ($1, $2::clearance_level, $1)
			RETURNING id
		)
		UPDATE users SET current_clearance_id = (SELECT id FROM ins) WHERE id = $1`,
		userID, newLevel)
	require.NoError(t, err)
}

func insertDd254(t *testing.T, db database.DB, tenantID, uploaderID uuid.UUID, contract, classMax, periodStart, periodEnd string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRow(context.Background(), "test.insertDd254",
		`INSERT INTO dd254_forms
		 (tenant_id, contract_number, classification_max, period_start, period_end,
		  storage_key, filename, content_type, size_bytes, markings,
		  cui_attestation_by, uploaded_by)
		 VALUES ($1, $2, $3::clearance_level, $4::date, $5::date,
		         'k', 'f.pdf', 'application/pdf', 1, 'unclassified', $6, $6)
		 RETURNING id`,
		tenantID, contract, classMax, periodStart, periodEnd, uploaderID,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func grantDd254Access(t *testing.T, db database.DB, dd254ID, userID uuid.UUID) {
	t.Helper()
	_, err := db.Exec(context.Background(), "test.grantDd254Access",
		`INSERT INTO dd254_user_access (dd254_id, user_id, added_by)
		 VALUES ($1, $2, $2)`,
		dd254ID, userID)
	require.NoError(t, err)
}
