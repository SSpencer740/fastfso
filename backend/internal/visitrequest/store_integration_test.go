//go:build integration

package visitrequest_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SSpencer740/fastfso/backend/internal/testutil"
	"github.com/SSpencer740/fastfso/backend/internal/visitrequest"
)

// helpers

func setup(t *testing.T) (*visitrequest.Store, context.Context) {
	t.Helper()
	db := testutil.DB(t)
	return visitrequest.NewStore(db), context.Background()
}

type env struct {
	tenantID    uuid.UUID
	adminID     uuid.UUID // identity
	icID        uuid.UUID // identity
	adminUserID uuid.UUID
	icUserID    uuid.UUID
}

func setupEnv(t *testing.T) (*visitrequest.Store, context.Context, env) {
	t.Helper()
	db := testutil.DB(t)
	store := visitrequest.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	adminIdentity := testutil.CreateIdentity(t, db, "admin@acme.com")
	icIdentity := testutil.CreateIdentity(t, db, "alice@acme.com")
	adminUserID := testutil.CreateUser(t, db, tenantID, adminIdentity, "administrator")
	icUserID := testutil.CreateUser(t, db, tenantID, icIdentity, "individual_contributor")

	return store, ctx, env{
		tenantID:    tenantID,
		adminID:     adminIdentity,
		icID:        icIdentity,
		adminUserID: adminUserID,
		icUserID:    icUserID,
	}
}

func createVisitRequest(t *testing.T, store *visitrequest.Store, ctx context.Context, e env) *visitrequest.VisitRequest {
	t.Helper()
	created, err := store.Create(ctx, visitrequest.CreateParams{
		TenantID:         e.tenantID,
		CreatedByUserID:  e.icUserID,
		DestinationName:  "Acme Defense Systems",
		DissSmoCode:      "LM001",
		VisitAddress:     "100 Defense Way, Bethesda, MD 20817",
		VisitStartDate:   "2025-06-15",
		VisitEndDate:     "2025-06-20",
		AccessLevel:      "secret",
		VisitDescription: "Review classified project documents",
		PocName:          "Jane Smith",
		PocEmail:         "jane.smith@example.com",
		PocPhone:         "555-0100",
		SecurityPocName:  "Bob Jones",
		SecurityPocEmail: "bob.jones@example.com",
		SecurityPocPhone: "555-0101",
	})
	require.NoError(t, err)
	return created
}

// tests

func TestStore_Create(t *testing.T) {
	store, ctx, e := setupEnv(t)

	vr := createVisitRequest(t, store, ctx, e)

	assert.NotEqual(t, uuid.Nil, vr.ID)
	assert.Equal(t, e.tenantID, vr.TenantID)
	assert.Equal(t, e.icUserID, vr.CreatedByUserID)
	assert.Equal(t, "Acme Defense Systems", vr.DestinationName)
	assert.Equal(t, "LM001", vr.DissSmoCode)
	assert.Equal(t, "100 Defense Way, Bethesda, MD 20817", vr.VisitAddress)
	assert.Equal(t, "secret", vr.AccessLevel)
	assert.Equal(t, "Review classified project documents", vr.VisitDescription)
	assert.Equal(t, "Jane Smith", vr.PocName)
	assert.Equal(t, "jane.smith@example.com", vr.PocEmail)
	assert.Equal(t, "555-0100", vr.PocPhone)
	assert.Equal(t, "Bob Jones", vr.SecurityPocName)
	assert.Equal(t, "bob.jones@example.com", vr.SecurityPocEmail)
	assert.Equal(t, "555-0101", vr.SecurityPocPhone)
	assert.Equal(t, "submitted", vr.Status)
	assert.Nil(t, vr.ClonedFromID)
	assert.Nil(t, vr.ReviewedBy)
	assert.Nil(t, vr.ReviewedAt)
	assert.Equal(t, "", vr.ReviewerNotes)
	assert.False(t, vr.CreatedAt.IsZero())
	assert.False(t, vr.UpdatedAt.IsZero())
}

func TestStore_Create_WithClone(t *testing.T) {
	store, ctx, e := setupEnv(t)

	original := createVisitRequest(t, store, ctx, e)

	cloned, err := store.Create(ctx, visitrequest.CreateParams{
		TenantID:         e.tenantID,
		CreatedByUserID:  e.icUserID,
		DestinationName:  "Acme Defense Systems",
		DissSmoCode:      "LM001",
		VisitAddress:     "100 Defense Way, Bethesda, MD 20817",
		VisitStartDate:   "2025-07-01",
		VisitEndDate:     "2025-07-05",
		AccessLevel:      "secret",
		VisitDescription: "Follow-up visit",
		PocName:          "Jane Smith",
		PocEmail:         "jane.smith@example.com",
		PocPhone:         "555-0100",
		SecurityPocName:  "Bob Jones",
		SecurityPocEmail: "bob.jones@example.com",
		SecurityPocPhone: "555-0101",
		ClonedFromID:     &original.ID,
	})
	require.NoError(t, err)

	assert.NotEqual(t, original.ID, cloned.ID)
	assert.NotNil(t, cloned.ClonedFromID)
	assert.Equal(t, original.ID, *cloned.ClonedFromID)
}

func TestStore_Get(t *testing.T) {
	store, ctx, e := setupEnv(t)

	created := createVisitRequest(t, store, ctx, e)

	got, err := store.Get(ctx, e.tenantID, created.ID)
	require.NoError(t, err)

	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, e.tenantID, got.TenantID)
	assert.Equal(t, e.icUserID, got.CreatedByUserID)
	assert.Equal(t, "Acme Defense Systems", got.DestinationName)
	assert.Equal(t, "LM001", got.DissSmoCode)
	assert.Equal(t, "100 Defense Way, Bethesda, MD 20817", got.VisitAddress)
	assert.Equal(t, "secret", got.AccessLevel)
	assert.Equal(t, "Review classified project documents", got.VisitDescription)
	assert.Equal(t, "Jane Smith", got.PocName)
	assert.Equal(t, "jane.smith@example.com", got.PocEmail)
	assert.Equal(t, "555-0100", got.PocPhone)
	assert.Equal(t, "Bob Jones", got.SecurityPocName)
	assert.Equal(t, "bob.jones@example.com", got.SecurityPocEmail)
	assert.Equal(t, "555-0101", got.SecurityPocPhone)
	assert.Equal(t, "submitted", got.Status)
	assert.Nil(t, got.ClonedFromID)
	assert.Nil(t, got.ReviewedBy)
	assert.Nil(t, got.ReviewedAt)
	assert.Equal(t, "", got.ReviewerNotes)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestStore_Get_NotFound(t *testing.T) {
	store, ctx := setup(t)
	tenantID := uuid.New()

	_, err := store.Get(ctx, tenantID, uuid.New())
	assert.ErrorIs(t, err, visitrequest.ErrNotFound)
}

func TestStore_Get_WrongTenant(t *testing.T) {
	db := testutil.DB(t)
	store := visitrequest.NewStore(db)
	ctx := context.Background()

	tenantA := testutil.CreateTenant(t, db, "Acme Corp")
	tenantB := testutil.CreateTenant(t, db, "Other Corp")
	identity := testutil.CreateIdentity(t, db, "alice@acme.com")
	userID := testutil.CreateUser(t, db, tenantA, identity, "individual_contributor")

	created, err := store.Create(ctx, visitrequest.CreateParams{
		TenantID:         tenantA,
		CreatedByUserID:  userID,
		DestinationName:  "Acme Defense Systems",
		DissSmoCode:      "LM001",
		VisitAddress:     "100 Defense Way",
		VisitStartDate:   "2025-06-15",
		VisitEndDate:     "2025-06-20",
		AccessLevel:      "secret",
		VisitDescription: "Test visit",
		PocName:          "POC",
		PocEmail:         "poc@example.com",
		PocPhone:         "555-0000",
		SecurityPocName:  "Sec POC",
		SecurityPocEmail: "sec@example.com",
		SecurityPocPhone: "555-0001",
	})
	require.NoError(t, err)

	// Attempting to get with wrong tenant should return not found.
	_, err = store.Get(ctx, tenantB, created.ID)
	assert.ErrorIs(t, err, visitrequest.ErrNotFound)
}

func TestStore_List(t *testing.T) {
	store, ctx, e := setupEnv(t)

	// Create 3 visit requests
	for i, dest := range []string{"Acme Defense Systems", "Beacon Aerospace", "Cardinal Aviation"} {
		_, err := store.Create(ctx, visitrequest.CreateParams{
			TenantID:         e.tenantID,
			CreatedByUserID:  e.icUserID,
			DestinationName:  dest,
			DissSmoCode:      "CODE",
			VisitAddress:     "123 Street",
			VisitStartDate:   "2025-06-15",
			VisitEndDate:     "2025-06-20",
			AccessLevel:      "secret",
			VisitDescription: "Visit " + string(rune('A'+i)),
			PocName:          "POC",
			PocEmail:         "poc@example.com",
			PocPhone:         "555-0000",
			SecurityPocName:  "Sec POC",
			SecurityPocEmail: "sec@example.com",
			SecurityPocPhone: "555-0001",
		})
		require.NoError(t, err)
	}

	// Unfiltered — returns all 3
	rows, total, err := store.List(ctx, visitrequest.ListFilters{TenantID: e.tenantID})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, rows, 3)

	// Pagination: limit=2, offset=0
	rows, total, err = store.List(ctx, visitrequest.ListFilters{
		TenantID: e.tenantID,
		Limit:    2,
		Offset:   0,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, rows, 2)

	// Pagination: limit=2, offset=2
	rows, total, err = store.List(ctx, visitrequest.ListFilters{
		TenantID: e.tenantID,
		Limit:    2,
		Offset:   2,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, rows, 1)
}

func TestStore_List_StatusFilter(t *testing.T) {
	store, ctx, e := setupEnv(t)

	vr1 := createVisitRequest(t, store, ctx, e)

	_, err := store.Create(ctx, visitrequest.CreateParams{
		TenantID:         e.tenantID,
		CreatedByUserID:  e.icUserID,
		DestinationName:  "Beacon Aerospace",
		DissSmoCode:      "RT001",
		VisitAddress:     "456 Defense Blvd",
		VisitStartDate:   "2025-07-01",
		VisitEndDate:     "2025-07-05",
		AccessLevel:      "top_secret",
		VisitDescription: "Second visit",
		PocName:          "POC",
		PocEmail:         "poc@example.com",
		PocPhone:         "555-0000",
		SecurityPocName:  "Sec POC",
		SecurityPocEmail: "sec@example.com",
		SecurityPocPhone: "555-0001",
	})
	require.NoError(t, err)

	// Approve the first request
	err = store.UpdateStatus(ctx, e.tenantID, vr1.ID, e.adminUserID, "approved", "Looks good")
	require.NoError(t, err)

	// Filter by submitted
	rows, total, err := store.List(ctx, visitrequest.ListFilters{
		TenantID: e.tenantID,
		Status:   "submitted",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, rows, 1)
	assert.Equal(t, "Beacon Aerospace", rows[0].DestinationName)

	// Filter by approved
	rows, total, err = store.List(ctx, visitrequest.ListFilters{
		TenantID: e.tenantID,
		Status:   "approved",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, rows, 1)
	assert.Equal(t, "Acme Defense Systems", rows[0].DestinationName)
}

func TestStore_List_UserScope(t *testing.T) {
	db := testutil.DB(t)
	store := visitrequest.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	icIdentity := testutil.CreateIdentity(t, db, "alice@acme.com")
	icUserID := testutil.CreateUser(t, db, tenantID, icIdentity, "individual_contributor")
	otherIdentity := testutil.CreateIdentity(t, db, "bob@acme.com")
	otherUserID := testutil.CreateUser(t, db, tenantID, otherIdentity, "individual_contributor")

	e := env{tenantID: tenantID, icID: icIdentity, icUserID: icUserID}

	// IC creates a visit request
	createVisitRequest(t, store, ctx, e)

	// Other user creates a visit request
	_, err := store.Create(ctx, visitrequest.CreateParams{
		TenantID:         e.tenantID,
		CreatedByUserID:  otherUserID,
		DestinationName:  "Delta Aeronautics",
		DissSmoCode:      "DA001",
		VisitAddress:     "789 Defense Ln",
		VisitStartDate:   "2025-08-01",
		VisitEndDate:     "2025-08-05",
		AccessLevel:      "secret",
		VisitDescription: "Bob's visit",
		PocName:          "POC",
		PocEmail:         "poc@example.com",
		PocPhone:         "555-0000",
		SecurityPocName:  "Sec POC",
		SecurityPocEmail: "sec@example.com",
		SecurityPocPhone: "555-0001",
	})
	require.NoError(t, err)

	// Unscoped (admin view) — returns all 2
	rows, total, err := store.List(ctx, visitrequest.ListFilters{TenantID: e.tenantID})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, rows, 2)

	// Scoped to IC user — returns only 1
	rows, total, err = store.List(ctx, visitrequest.ListFilters{
		TenantID: e.tenantID,
		UserID:   &e.icUserID,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, rows, 1)
	assert.Equal(t, "Acme Defense Systems", rows[0].DestinationName)
}

func TestStore_UpdateStatus(t *testing.T) {
	store, ctx, e := setupEnv(t)

	created := createVisitRequest(t, store, ctx, e)
	assert.Equal(t, "submitted", created.Status)

	err := store.UpdateStatus(ctx, e.tenantID, created.ID, e.adminUserID, "approved", "Approved for travel")
	require.NoError(t, err)

	got, err := store.Get(ctx, e.tenantID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "approved", got.Status)
	assert.Equal(t, "Approved for travel", got.ReviewerNotes)
	assert.NotNil(t, got.ReviewedBy)
	assert.Equal(t, e.adminUserID, *got.ReviewedBy)
	assert.NotNil(t, got.ReviewedAt)
	assert.True(t, got.UpdatedAt.After(created.UpdatedAt) || got.UpdatedAt.Equal(created.UpdatedAt))
}

func TestStore_SummaryStats(t *testing.T) {
	store, ctx, e := setupEnv(t)

	// Empty tenant — all zeros
	stats, err := store.SummaryStats(ctx, e.tenantID, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.Total)
	assert.Equal(t, 0, stats.Submitted)
	assert.Equal(t, 0, stats.UnderReview)
	assert.Equal(t, 0, stats.Approved)
	assert.Equal(t, 0, stats.Rejected)

	// Create 3 visit requests
	vr1 := createVisitRequest(t, store, ctx, e)

	vr2, err := store.Create(ctx, visitrequest.CreateParams{
		TenantID:         e.tenantID,
		CreatedByUserID:  e.icUserID,
		DestinationName:  "Beacon Aerospace",
		DissSmoCode:      "RT001",
		VisitAddress:     "456 Defense Blvd",
		VisitStartDate:   "2025-07-01",
		VisitEndDate:     "2025-07-05",
		AccessLevel:      "top_secret",
		VisitDescription: "Second visit",
		PocName:          "POC",
		PocEmail:         "poc@example.com",
		PocPhone:         "555-0000",
		SecurityPocName:  "Sec POC",
		SecurityPocEmail: "sec@example.com",
		SecurityPocPhone: "555-0001",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, visitrequest.CreateParams{
		TenantID:         e.tenantID,
		CreatedByUserID:  e.icUserID,
		DestinationName:  "Cardinal Aviation",
		DissSmoCode:      "BA001",
		VisitAddress:     "789 Aero Way",
		VisitStartDate:   "2025-08-01",
		VisitEndDate:     "2025-08-10",
		AccessLevel:      "secret",
		VisitDescription: "Third visit",
		PocName:          "POC",
		PocEmail:         "poc@example.com",
		PocPhone:         "555-0000",
		SecurityPocName:  "Sec POC",
		SecurityPocEmail: "sec@example.com",
		SecurityPocPhone: "555-0001",
	})
	require.NoError(t, err)

	// Approve vr1
	err = store.UpdateStatus(ctx, e.tenantID, vr1.ID, e.adminUserID, "approved", "Approved")
	require.NoError(t, err)

	// Reject vr2
	err = store.UpdateStatus(ctx, e.tenantID, vr2.ID, e.adminUserID, "rejected", "Denied")
	require.NoError(t, err)

	stats, err = store.SummaryStats(ctx, e.tenantID, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.Total)
	assert.Equal(t, 1, stats.Submitted)
	assert.Equal(t, 0, stats.UnderReview)
	assert.Equal(t, 1, stats.Approved)
	assert.Equal(t, 1, stats.Rejected)
}

func TestStore_SummaryStatsByUser(t *testing.T) {
	store, ctx, e := setupEnv(t)

	// IC creates 2 requests
	createVisitRequest(t, store, ctx, e)

	_, err := store.Create(ctx, visitrequest.CreateParams{
		TenantID:         e.tenantID,
		CreatedByUserID:  e.icUserID,
		DestinationName:  "Beacon Aerospace",
		DissSmoCode:      "RT001",
		VisitAddress:     "456 Defense Blvd",
		VisitStartDate:   "2025-07-01",
		VisitEndDate:     "2025-07-05",
		AccessLevel:      "top_secret",
		VisitDescription: "Second visit",
		PocName:          "POC",
		PocEmail:         "poc@example.com",
		PocPhone:         "555-0000",
		SecurityPocName:  "Sec POC",
		SecurityPocEmail: "sec@example.com",
		SecurityPocPhone: "555-0001",
	})
	require.NoError(t, err)

	// Admin creates 1 request (should not show in IC stats)
	_, err = store.Create(ctx, visitrequest.CreateParams{
		TenantID:         e.tenantID,
		CreatedByUserID:  e.adminUserID,
		DestinationName:  "Cardinal Aviation",
		DissSmoCode:      "BA001",
		VisitAddress:     "789 Aero Way",
		VisitStartDate:   "2025-08-01",
		VisitEndDate:     "2025-08-10",
		AccessLevel:      "secret",
		VisitDescription: "Admin visit",
		PocName:          "POC",
		PocEmail:         "poc@example.com",
		PocPhone:         "555-0000",
		SecurityPocName:  "Sec POC",
		SecurityPocEmail: "sec@example.com",
		SecurityPocPhone: "555-0001",
	})
	require.NoError(t, err)

	// IC stats — should only see 2
	stats, err := store.SummaryStats(ctx, e.tenantID, &e.icUserID, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.Total)
	assert.Equal(t, 2, stats.Submitted)

	// Tenant-wide stats — should see all 3
	allStats, err := store.SummaryStats(ctx, e.tenantID, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, allStats.Total)
}

func TestStore_ListCloneable(t *testing.T) {
	store, ctx, e := setupEnv(t)

	// Create 3 requests, approve one, reject one, leave one as submitted
	vr1 := createVisitRequest(t, store, ctx, e)

	vr2, err := store.Create(ctx, visitrequest.CreateParams{
		TenantID:         e.tenantID,
		CreatedByUserID:  e.icUserID,
		DestinationName:  "Beacon Aerospace",
		DissSmoCode:      "RT001",
		VisitAddress:     "456 Defense Blvd",
		VisitStartDate:   "2025-07-01",
		VisitEndDate:     "2025-07-05",
		AccessLevel:      "top_secret",
		VisitDescription: "Second visit",
		PocName:          "POC",
		PocEmail:         "poc@example.com",
		PocPhone:         "555-0000",
		SecurityPocName:  "Sec POC",
		SecurityPocEmail: "sec@example.com",
		SecurityPocPhone: "555-0001",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, visitrequest.CreateParams{
		TenantID:         e.tenantID,
		CreatedByUserID:  e.icUserID,
		DestinationName:  "Cardinal Aviation",
		DissSmoCode:      "BA001",
		VisitAddress:     "789 Aero Way",
		VisitStartDate:   "2025-08-01",
		VisitEndDate:     "2025-08-10",
		AccessLevel:      "secret",
		VisitDescription: "Third visit",
		PocName:          "POC",
		PocEmail:         "poc@example.com",
		PocPhone:         "555-0000",
		SecurityPocName:  "Sec POC",
		SecurityPocEmail: "sec@example.com",
		SecurityPocPhone: "555-0001",
	})
	require.NoError(t, err)

	// Approve vr1
	err = store.UpdateStatus(ctx, e.tenantID, vr1.ID, e.adminUserID, "approved", "Approved")
	require.NoError(t, err)

	// Reject vr2
	err = store.UpdateStatus(ctx, e.tenantID, vr2.ID, e.adminUserID, "rejected", "Denied")
	require.NoError(t, err)

	// ListCloneable should return submitted + approved (2 results, not the rejected one)
	rows, err := store.ListCloneable(ctx, e.tenantID, e.icUserID)
	require.NoError(t, err)
	assert.Len(t, rows, 2)

	// Verify only submitted and approved are returned
	statuses := map[string]bool{}
	for _, r := range rows {
		statuses[r.Status] = true
	}
	assert.True(t, statuses["submitted"])
	assert.True(t, statuses["approved"])
	assert.False(t, statuses["rejected"])
}
