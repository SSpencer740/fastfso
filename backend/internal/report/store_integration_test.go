//go:build integration

package report_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SSpencer740/fastfso/backend/internal/database"
	"github.com/SSpencer740/fastfso/backend/internal/report"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
)

func setup(t *testing.T) (context.Context, database.DB, *report.Store, uuid.UUID, uuid.UUID) {
	t.Helper()
	db := testutil.DB(t)
	store := report.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "individual_contributor")

	return ctx, db, store, tenantID, userID
}

func TestStore_Create(t *testing.T) {
	ctx, _, store, tenantID, userID := setup(t)

	r, err := store.Create(ctx, report.CreateParams{
		TenantID:        tenantID,
		CreatedByUserID: userID,
		ReportingFor:    "self",
		SubjectName:     "",
		ReportType:      "financial",
		Details:         "I inherited $15,000 from my grandmother.",
	})
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, r.ID)
	assert.Equal(t, tenantID, r.TenantID)
	assert.Equal(t, userID, r.CreatedByUserID)
	assert.Equal(t, "self", r.ReportingFor)
	assert.Equal(t, "", r.SubjectName)
	assert.Equal(t, "financial", r.ReportType)
	assert.Equal(t, "I inherited $15,000 from my grandmother.", r.Details)
	assert.Equal(t, "unreviewed", r.Status)
	assert.Nil(t, r.ReviewedBy)
	assert.Nil(t, r.ReviewedAt)
	assert.False(t, r.CreatedAt.IsZero())
	assert.False(t, r.UpdatedAt.IsZero())
}

func TestStore_Create_Other(t *testing.T) {
	ctx, _, store, tenantID, userID := setup(t)

	r, err := store.Create(ctx, report.CreateParams{
		TenantID:        tenantID,
		CreatedByUserID: userID,
		ReportingFor:    "other",
		SubjectName:     "Bob Smith",
		ReportType:      "cohabitant",
		Details:         "Bob recently moved in with a foreign national.",
	})
	require.NoError(t, err)

	assert.Equal(t, "other", r.ReportingFor)
	assert.Equal(t, "Bob Smith", r.SubjectName)
}

func TestStore_Get(t *testing.T) {
	ctx, _, store, tenantID, userID := setup(t)

	created, err := store.Create(ctx, report.CreateParams{
		TenantID:        tenantID,
		CreatedByUserID: userID,
		ReportingFor:    "self",
		ReportType:      "arrests",
		Details:         "I was arrested on 2025-01-01.",
	})
	require.NoError(t, err)

	got, err := store.Get(ctx, tenantID, created.ID)
	require.NoError(t, err)

	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, tenantID, got.TenantID)
	assert.Equal(t, userID, got.CreatedByUserID)
	assert.Equal(t, "Alice", got.CreatorName)
	assert.Equal(t, "alice@example.com", got.CreatorEmail)
	assert.Equal(t, "self", got.ReportingFor)
	assert.Equal(t, "arrests", got.ReportType)
	assert.Equal(t, "unreviewed", got.Status)
}

func TestStore_Get_NotFound(t *testing.T) {
	ctx, _, store, tenantID, _ := setup(t)

	_, err := store.Get(ctx, tenantID, uuid.New())
	assert.ErrorIs(t, err, report.ErrNotFound)
}

func TestStore_Get_WrongTenant(t *testing.T) {
	ctx, db, store, tenantID, userID := setup(t)

	otherTenant := testutil.CreateTenant(t, db, "Other Corp")

	created, err := store.Create(ctx, report.CreateParams{
		TenantID:        tenantID,
		CreatedByUserID: userID,
		ReportingFor:    "self",
		ReportType:      "financial",
		Details:         "Details.",
	})
	require.NoError(t, err)

	_, err = store.Get(ctx, otherTenant, created.ID)
	assert.ErrorIs(t, err, report.ErrNotFound)
}

func TestStore_GetCreatorName(t *testing.T) {
	db := testutil.DB(t)
	store := report.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "bob@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "individual_contributor")

	name, err := store.GetCreatorName(ctx, tenantID, userID)
	require.NoError(t, err)
	assert.Equal(t, "Bob", name)
}

func TestStore_List_All(t *testing.T) {
	db := testutil.DB(t)
	store := report.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	idA := testutil.CreateIdentity(t, db, "alice@example.com")
	idB := testutil.CreateIdentity(t, db, "bob@example.com")
	userA := testutil.CreateUser(t, db, tenantID, idA, "individual_contributor")
	userB := testutil.CreateUser(t, db, tenantID, idB, "individual_contributor")

	_, err := store.Create(ctx, report.CreateParams{
		TenantID: tenantID, CreatedByUserID: userA,
		ReportingFor: "self", ReportType: "financial", Details: "Details A1.",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, report.CreateParams{
		TenantID: tenantID, CreatedByUserID: userA,
		ReportingFor: "self", ReportType: "arrests", Details: "Details A2.",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, report.CreateParams{
		TenantID: tenantID, CreatedByUserID: userB,
		ReportingFor: "self", ReportType: "cohabitant", Details: "Details B1.",
	})
	require.NoError(t, err)

	// Admin list — all 3
	rows, total, err := store.List(ctx, report.ListFilters{TenantID: tenantID})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, rows, 3)

	// Filter by user — only userA's 2
	rows, total, err = store.List(ctx, report.ListFilters{TenantID: tenantID, UserID: &userA})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, rows, 2)

	// Search by report type
	rows, total, err = store.List(ctx, report.ListFilters{TenantID: tenantID, Search: "financial"})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, rows, 1)
	assert.Equal(t, "financial", rows[0].ReportType)

	// Search by creator name
	rows, total, err = store.List(ctx, report.ListFilters{TenantID: tenantID, Search: "Bob"})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "cohabitant", rows[0].ReportType)
}

func TestStore_List_StatusFilter(t *testing.T) {
	db := testutil.DB(t)
	store := report.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "individual_contributor")

	r1, err := store.Create(ctx, report.CreateParams{
		TenantID: tenantID, CreatedByUserID: userID,
		ReportingFor: "self", ReportType: "financial", Details: "d",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, report.CreateParams{
		TenantID: tenantID, CreatedByUserID: userID,
		ReportingFor: "self", ReportType: "arrests", Details: "d",
	})
	require.NoError(t, err)

	reviewerID := testutil.CreateIdentity(t, db, "fso@example.com")
	reviewerUserID := testutil.CreateUser(t, db, tenantID, reviewerID, "fso")
	err = store.UpdateStatus(ctx, tenantID, r1.ID, reviewerUserID, "under_review")
	require.NoError(t, err)

	rows, total, err := store.List(ctx, report.ListFilters{TenantID: tenantID, Status: "unreviewed"})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "arrests", rows[0].ReportType)

	rows, total, err = store.List(ctx, report.ListFilters{TenantID: tenantID, Status: "under_review"})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "financial", rows[0].ReportType)
}

func TestStore_List_Pagination(t *testing.T) {
	db := testutil.DB(t)
	store := report.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "individual_contributor")

	types := []string{"financial", "arrests", "cohabitant", "marriage_divorce", "other"}
	for _, rt := range types {
		_, err := store.Create(ctx, report.CreateParams{
			TenantID: tenantID, CreatedByUserID: userID,
			ReportingFor: "self", ReportType: rt, Details: "d",
		})
		require.NoError(t, err)
	}

	rows, total, err := store.List(ctx, report.ListFilters{TenantID: tenantID, Limit: 2, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, rows, 2)

	rows, total, err = store.List(ctx, report.ListFilters{TenantID: tenantID, Limit: 2, Offset: 4})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, rows, 1)
}

func TestStore_UpdateStatus(t *testing.T) {
	db := testutil.DB(t)
	store := report.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "individual_contributor")
	fsoID := testutil.CreateIdentity(t, db, "fso@example.com")
	fsoUserID := testutil.CreateUser(t, db, tenantID, fsoID, "fso")

	created, err := store.Create(ctx, report.CreateParams{
		TenantID: tenantID, CreatedByUserID: userID,
		ReportingFor: "self", ReportType: "financial", Details: "Details.",
	})
	require.NoError(t, err)
	assert.Equal(t, "unreviewed", created.Status)

	err = store.UpdateStatus(ctx, tenantID, created.ID, fsoUserID, "under_review")
	require.NoError(t, err)

	got, err := store.Get(ctx, tenantID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "under_review", got.Status)
	assert.Equal(t, fsoUserID, *got.ReviewedBy)
	assert.NotNil(t, got.ReviewedAt)

	err = store.UpdateStatus(ctx, tenantID, created.ID, fsoUserID, "processed")
	require.NoError(t, err)

	got, err = store.Get(ctx, tenantID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "processed", got.Status)
}

func TestStore_UpdateStatus_NotFound(t *testing.T) {
	ctx, _, store, tenantID, userID := setup(t)

	err := store.UpdateStatus(ctx, tenantID, uuid.New(), userID, "processed")
	assert.ErrorIs(t, err, report.ErrNotFound)
}

func TestStore_SummaryStats(t *testing.T) {
	db := testutil.DB(t)
	store := report.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "individual_contributor")
	fsoID := testutil.CreateIdentity(t, db, "fso@example.com")
	fsoUserID := testutil.CreateUser(t, db, tenantID, fsoID, "fso")

	// Empty tenant
	stats, err := store.SummaryStats(ctx, tenantID, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.Total)
	assert.Equal(t, 0, stats.Unreviewed)
	assert.Equal(t, 0, stats.UnderReview)
	assert.Equal(t, 0, stats.Processed)

	r1, err := store.Create(ctx, report.CreateParams{
		TenantID: tenantID, CreatedByUserID: userID,
		ReportingFor: "self", ReportType: "financial", Details: "d",
	})
	require.NoError(t, err)

	r2, err := store.Create(ctx, report.CreateParams{
		TenantID: tenantID, CreatedByUserID: userID,
		ReportingFor: "self", ReportType: "arrests", Details: "d",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, report.CreateParams{
		TenantID: tenantID, CreatedByUserID: userID,
		ReportingFor: "self", ReportType: "cohabitant", Details: "d",
	})
	require.NoError(t, err)

	err = store.UpdateStatus(ctx, tenantID, r1.ID, fsoUserID, "under_review")
	require.NoError(t, err)
	err = store.UpdateStatus(ctx, tenantID, r2.ID, fsoUserID, "processed")
	require.NoError(t, err)

	stats, err = store.SummaryStats(ctx, tenantID, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.Total)
	assert.Equal(t, 1, stats.Unreviewed)
	assert.Equal(t, 1, stats.UnderReview)
	assert.Equal(t, 1, stats.Processed)
}

func TestStore_SummaryStats_FilterByUser(t *testing.T) {
	db := testutil.DB(t)
	store := report.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	idA := testutil.CreateIdentity(t, db, "alice@example.com")
	idB := testutil.CreateIdentity(t, db, "bob@example.com")
	userA := testutil.CreateUser(t, db, tenantID, idA, "individual_contributor")
	userB := testutil.CreateUser(t, db, tenantID, idB, "individual_contributor")

	_, err := store.Create(ctx, report.CreateParams{
		TenantID: tenantID, CreatedByUserID: userA,
		ReportingFor: "self", ReportType: "financial", Details: "d",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, report.CreateParams{
		TenantID: tenantID, CreatedByUserID: userA,
		ReportingFor: "self", ReportType: "arrests", Details: "d",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, report.CreateParams{
		TenantID: tenantID, CreatedByUserID: userB,
		ReportingFor: "self", ReportType: "cohabitant", Details: "d",
	})
	require.NoError(t, err)

	statsA, err := store.SummaryStats(ctx, tenantID, &userA, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, statsA.Total)

	statsB, err := store.SummaryStats(ctx, tenantID, &userB, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, statsB.Total)
}

func TestStore_SummaryStats_TenantIsolation(t *testing.T) {
	db := testutil.DB(t)
	store := report.NewStore(db)
	ctx := context.Background()

	tenantA := testutil.CreateTenant(t, db, "Tenant A")
	tenantB := testutil.CreateTenant(t, db, "Tenant B")
	idA := testutil.CreateIdentity(t, db, "alice@a.com")
	idB := testutil.CreateIdentity(t, db, "bob@b.com")
	userA := testutil.CreateUser(t, db, tenantA, idA, "individual_contributor")
	userB := testutil.CreateUser(t, db, tenantB, idB, "individual_contributor")

	_, err := store.Create(ctx, report.CreateParams{
		TenantID: tenantA, CreatedByUserID: userA,
		ReportingFor: "self", ReportType: "financial", Details: "d",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, report.CreateParams{
		TenantID: tenantA, CreatedByUserID: userA,
		ReportingFor: "self", ReportType: "arrests", Details: "d",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, report.CreateParams{
		TenantID: tenantB, CreatedByUserID: userB,
		ReportingFor: "self", ReportType: "cohabitant", Details: "d",
	})
	require.NoError(t, err)

	statsA, err := store.SummaryStats(ctx, tenantA, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, statsA.Total)

	statsB, err := store.SummaryStats(ctx, tenantB, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, statsB.Total)
}
