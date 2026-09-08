//go:build integration

package travel_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SSpencer740/fastfso/backend/internal/database"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
	"github.com/SSpencer740/fastfso/backend/internal/travel"
)

type env struct {
	db          database.DB
	tenantID    uuid.UUID
	adminID     uuid.UUID // identity ID
	icID        uuid.UUID // identity ID
	adminUserID uuid.UUID
	icUserID    uuid.UUID
}

func setupEnv(t *testing.T) env {
	t.Helper()
	db := testutil.DB(t)

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	adminID := testutil.CreateIdentity(t, db, "admin@acme.com")
	icID := testutil.CreateIdentity(t, db, "ic@acme.com")
	adminUserID := testutil.CreateUser(t, db, tenantID, adminID, "administrator")
	icUserID := testutil.CreateUser(t, db, tenantID, icID, "individual_contributor")

	return env{
		db:          db,
		tenantID:    tenantID,
		adminID:     adminID,
		icID:        icID,
		adminUserID: adminUserID,
		icUserID:    icUserID,
	}
}

func createDraftReport(t *testing.T, store *travel.Store, ctx context.Context, e env) *travel.Report {
	t.Helper()
	startDate := "2026-06-01"
	endDate := "2026-06-10"
	startDate2 := "2026-06-11"
	endDate2 := "2026-06-20"
	report, err := store.Create(ctx, travel.CreateParams{
		TenantID:           e.tenantID,
		UserID:             e.icUserID,
		TripName:           "Europe Trip",
		MultiCountry:       true,
		PassportNumber:     "AB1234567",
		EmergencyFirstName: "Jane",
		EmergencyLastName:  "Doe",
		EmergencyPhone:     "555-0100",
		AdditionalComments: "Business travel",
		Countries: []travel.CreateCountryParams{
			{
				CountryName:      "Germany",
				SortOrder:        0,
				StartDate:        &startDate,
				EndDate:          &endDate,
				Reason:           "official_non_dod",
				Transportation:   []string{"flight", "train"},
				HasCompanions:    true,
				CompanionsDetail: "spouse",
			},
			{
				CountryName:        "France",
				SortOrder:          1,
				StartDate:          &startDate2,
				EndDate:            &endDate2,
				Reason:             "other",
				Transportation:     []string{"train"},
				HasForeignContacts: true,
				ContactsDetail:     "local partner",
			},
		},
	})
	require.NoError(t, err)
	return report
}

func TestStore_Create(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	startDate := "2026-07-01"
	endDate := "2026-07-15"

	report, err := store.Create(ctx, travel.CreateParams{
		TenantID:           e.tenantID,
		UserID:             e.icUserID,
		TripName:           "Japan Trip",
		MultiCountry:       false,
		PassportNumber:     "CD9876543",
		EmergencyFirstName: "John",
		EmergencyLastName:  "Smith",
		EmergencyPhone:     "555-0200",
		AdditionalComments: "Vacation",
		Countries: []travel.CreateCountryParams{
			{
				CountryName:    "Japan",
				SortOrder:      0,
				StartDate:      &startDate,
				EndDate:        &endDate,
				Reason:         "vacation_personal",
				Transportation: []string{"flight"},
			},
		},
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, report.ID)
	assert.Equal(t, "Japan Trip", report.TripName)
	assert.Equal(t, "draft", report.Status)
	assert.False(t, report.MultiCountry)

	// Verify via IC Get (owner-scoped)
	detail, err := store.Get(ctx, e.tenantID, e.icUserID, report.ID)
	require.NoError(t, err)
	assert.Equal(t, "Japan Trip", detail.Report.TripName)
	assert.Len(t, detail.Countries, 1)
	assert.Equal(t, "Japan", detail.Countries[0].CountryName)
	assert.NotNil(t, detail.Countries[0].StartDate)
	assert.Equal(t, "Ic", detail.CreatorName)
}

func TestStore_Create_MultiCountry(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	report := createDraftReport(t, store, ctx, e)

	detail, err := store.Get(ctx, e.tenantID, e.icUserID, report.ID)
	require.NoError(t, err)
	assert.True(t, detail.Report.MultiCountry)
	assert.Len(t, detail.Countries, 2)
	assert.Equal(t, "Germany", detail.Countries[0].CountryName)
	assert.Equal(t, "France", detail.Countries[1].CountryName)
	assert.Equal(t, 0, detail.Countries[0].SortOrder)
	assert.Equal(t, 1, detail.Countries[1].SortOrder)
}

func TestStore_Get_NotFound(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	_, err := store.Get(ctx, e.tenantID, e.icUserID, uuid.New())
	assert.ErrorIs(t, err, travel.ErrNotFound)
}

func TestStore_Get_WrongTenant(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	report := createDraftReport(t, store, ctx, e)
	otherTenantID := testutil.CreateTenant(t, e.db, "Other Corp")

	_, err := store.Get(ctx, otherTenantID, e.icUserID, report.ID)
	assert.ErrorIs(t, err, travel.ErrNotFound)
}

func TestStore_Get_WrongUser(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	report := createDraftReport(t, store, ctx, e)

	// Admin trying to use the IC-scoped Get should be denied
	_, err := store.Get(ctx, e.tenantID, e.adminUserID, report.ID)
	assert.ErrorIs(t, err, travel.ErrNotFound)
}

func TestStore_AdminGet(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	report := createDraftReport(t, store, ctx, e)

	// AdminGet can fetch any report in the tenant regardless of owner
	detail, err := store.AdminGet(ctx, e.tenantID, report.ID)
	require.NoError(t, err)
	assert.Equal(t, "Europe Trip", detail.Report.TripName)
}

func TestStore_AdminGet_WrongTenant(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	report := createDraftReport(t, store, ctx, e)
	otherTenantID := testutil.CreateTenant(t, e.db, "Other Corp")

	_, err := store.AdminGet(ctx, otherTenantID, report.ID)
	assert.ErrorIs(t, err, travel.ErrNotFound)
}

func TestStore_MarkUnderReview(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	report := createDraftReport(t, store, ctx, e)

	// Submit first
	err := store.Submit(ctx, e.tenantID, e.icUserID, report.ID)
	require.NoError(t, err)

	// Mark under review
	err = store.MarkUnderReview(ctx, e.tenantID, report.ID)
	require.NoError(t, err)

	detail, err := store.AdminGet(ctx, e.tenantID, report.ID)
	require.NoError(t, err)
	assert.Equal(t, "under_review", detail.Report.Status)

	// Calling again on an already-under-review report should be a no-op
	err = store.MarkUnderReview(ctx, e.tenantID, report.ID)
	require.NoError(t, err)
	detail, err = store.AdminGet(ctx, e.tenantID, report.ID)
	require.NoError(t, err)
	assert.Equal(t, "under_review", detail.Report.Status)
}

func TestStore_List(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := store.Create(ctx, travel.CreateParams{
			TenantID: e.tenantID,
			UserID:   e.icUserID,
			TripName: "Trip " + string(rune('A'+i)),
		})
		require.NoError(t, err)
	}

	reports, total, err := store.List(ctx, travel.ListFilters{
		TenantID: e.tenantID,
		Limit:    2,
		Offset:   0,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, reports, 2)

	reports2, total2, err := store.List(ctx, travel.ListFilters{
		TenantID: e.tenantID,
		Limit:    2,
		Offset:   2,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, total2)
	assert.Len(t, reports2, 1)
}

func TestStore_List_StatusFilter(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	_, err := store.Create(ctx, travel.CreateParams{
		TenantID: e.tenantID,
		UserID:   e.icUserID,
		TripName: "Draft Trip",
	})
	require.NoError(t, err)

	submitted, err := store.Create(ctx, travel.CreateParams{
		TenantID: e.tenantID,
		UserID:   e.icUserID,
		TripName: "Submitted Trip",
	})
	require.NoError(t, err)
	err = store.Submit(ctx, e.tenantID, e.icUserID, submitted.ID)
	require.NoError(t, err)

	drafts, total, err := store.List(ctx, travel.ListFilters{
		TenantID: e.tenantID,
		Status:   "draft",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, drafts, 1)
	assert.Equal(t, "Draft Trip", drafts[0].TripName)

	subs, total, err := store.List(ctx, travel.ListFilters{
		TenantID: e.tenantID,
		Status:   "submitted",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, subs, 1)
	assert.Equal(t, "Submitted Trip", subs[0].TripName)
}

func TestStore_List_Search(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	_, err := store.Create(ctx, travel.CreateParams{
		TenantID: e.tenantID,
		UserID:   e.icUserID,
		TripName: "Tokyo Conference",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, travel.CreateParams{
		TenantID: e.tenantID,
		UserID:   e.icUserID,
		TripName: "Berlin Meeting",
	})
	require.NoError(t, err)

	reports, total, err := store.List(ctx, travel.ListFilters{
		TenantID: e.tenantID,
		Search:   "Tokyo",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, reports, 1)
	assert.Equal(t, "Tokyo Conference", reports[0].TripName)
}

func TestStore_Update_Draft(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	report := createDraftReport(t, store, ctx, e)

	newStart := "2026-08-01"
	newEnd := "2026-08-10"
	err := store.Update(ctx, e.tenantID, e.icUserID, report.ID, travel.UpdateParams{
		TripName:           "Updated Trip",
		MultiCountry:       false,
		PassportNumber:     "XY0000000",
		EmergencyFirstName: "Bob",
		EmergencyLastName:  "Jones",
		EmergencyPhone:     "555-9999",
		AdditionalComments: "Changed plans",
		Countries: []travel.CreateCountryParams{
			{
				CountryName:    "Italy",
				SortOrder:      0,
				StartDate:      &newStart,
				EndDate:        &newEnd,
				Reason:         "vacation_personal",
				Transportation: []string{"flight", "car"},
			},
		},
	})
	require.NoError(t, err)

	detail, err := store.Get(ctx, e.tenantID, e.icUserID, report.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Trip", detail.Report.TripName)
	assert.Equal(t, "XY0000000", detail.Report.PassportNumber)
	assert.Equal(t, "Bob", detail.Report.EmergencyFirstName)
	assert.Len(t, detail.Countries, 1)
	assert.Equal(t, "Italy", detail.Countries[0].CountryName)
}

func TestStore_Update_WrongUser(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	report := createDraftReport(t, store, ctx, e)

	err := store.Update(ctx, e.tenantID, e.adminUserID, report.ID, travel.UpdateParams{
		TripName: "Should Not Update",
	})
	assert.ErrorIs(t, err, travel.ErrNotFound)
}

func TestStore_Submit(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	report := createDraftReport(t, store, ctx, e)

	err := store.Submit(ctx, e.tenantID, e.icUserID, report.ID)
	require.NoError(t, err)

	detail, err := store.Get(ctx, e.tenantID, e.icUserID, report.ID)
	require.NoError(t, err)
	assert.Equal(t, "submitted", detail.Report.Status)
	assert.NotNil(t, detail.Report.SubmittedAt)

	// Submitting again should fail (not a draft anymore)
	err = store.Submit(ctx, e.tenantID, e.icUserID, report.ID)
	assert.ErrorIs(t, err, travel.ErrNotFound)
}

func TestStore_Submit_WrongUser(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	report := createDraftReport(t, store, ctx, e)

	err := store.Submit(ctx, e.tenantID, e.adminUserID, report.ID)
	assert.ErrorIs(t, err, travel.ErrNotFound)
}

func TestStore_UpdateStatus(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	report := createDraftReport(t, store, ctx, e)

	err := store.Submit(ctx, e.tenantID, e.icUserID, report.ID)
	require.NoError(t, err)

	err = store.UpdateStatus(ctx, e.tenantID, report.ID, "approved", e.adminUserID)
	require.NoError(t, err)

	// Use AdminGet to verify — admin reviewing doesn't change ownership
	detail, err := store.AdminGet(ctx, e.tenantID, report.ID)
	require.NoError(t, err)
	assert.Equal(t, "approved", detail.Report.Status)
	assert.NotNil(t, detail.Report.ReviewedAt)
	assert.NotNil(t, detail.Report.ReviewedBy)
	assert.Equal(t, e.adminUserID, *detail.Report.ReviewedBy)
}

func TestStore_SummaryStats(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	_, err := store.Create(ctx, travel.CreateParams{TenantID: e.tenantID, UserID: e.icUserID, TripName: "Draft 1"})
	require.NoError(t, err)
	_, err = store.Create(ctx, travel.CreateParams{TenantID: e.tenantID, UserID: e.icUserID, TripName: "Draft 2"})
	require.NoError(t, err)

	submitted, err := store.Create(ctx, travel.CreateParams{TenantID: e.tenantID, UserID: e.icUserID, TripName: "Submitted"})
	require.NoError(t, err)
	err = store.Submit(ctx, e.tenantID, e.icUserID, submitted.ID)
	require.NoError(t, err)

	approved, err := store.Create(ctx, travel.CreateParams{TenantID: e.tenantID, UserID: e.icUserID, TripName: "Approved"})
	require.NoError(t, err)
	err = store.Submit(ctx, e.tenantID, e.icUserID, approved.ID)
	require.NoError(t, err)
	err = store.UpdateStatus(ctx, e.tenantID, approved.ID, "approved", e.adminUserID)
	require.NoError(t, err)

	stats, err := store.SummaryStats(ctx, e.tenantID, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 4, stats.Total)
	assert.Equal(t, 2, stats.Draft)
	assert.Equal(t, 1, stats.Submitted)
	assert.Equal(t, 1, stats.Approved)
	assert.Equal(t, 0, stats.Rejected)

	userStats, err := store.SummaryStats(ctx, e.tenantID, &e.icUserID, nil)
	require.NoError(t, err)
	assert.Equal(t, 4, userStats.Total)
}

func TestStore_Upload_CRUD(t *testing.T) {
	e := setupEnv(t)
	store := travel.NewStore(e.db)
	ctx := context.Background()

	report := createDraftReport(t, store, ctx, e)

	upload, err := store.CreateUpload(ctx, report.ID, "passport.pdf", 1024, "application/pdf", "tenants/xxx/travel/yyy/passport.pdf")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, upload.ID)
	assert.Equal(t, "passport.pdf", upload.FileName)
	assert.Equal(t, int64(1024), upload.FileSize)
	assert.Equal(t, "application/pdf", upload.ContentType)

	uploads, err := store.ListUploads(ctx, report.ID)
	require.NoError(t, err)
	assert.Len(t, uploads, 1)
	assert.Equal(t, upload.ID, uploads[0].ID)

	// Verify uploads appear in AdminGet detail
	detail, err := store.AdminGet(ctx, e.tenantID, report.ID)
	require.NoError(t, err)
	assert.Len(t, detail.Uploads, 1)

	storageKey, err := store.DeleteUpload(ctx, e.tenantID, upload.ID)
	require.NoError(t, err)
	assert.Equal(t, "tenants/xxx/travel/yyy/passport.pdf", storageKey)

	uploads, err = store.ListUploads(ctx, report.ID)
	require.NoError(t, err)
	assert.Empty(t, uploads)

	_, err = store.DeleteUpload(ctx, e.tenantID, upload.ID)
	assert.ErrorIs(t, err, travel.ErrNotFound)
}
