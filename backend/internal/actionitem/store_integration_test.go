//go:build integration

package actionitem_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SSpencer740/fastfso/backend/internal/actionitem"
	"github.com/SSpencer740/fastfso/backend/internal/database"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
)

func TestStore_Create(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")

	ai, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:    tenantID,
		SourceType:  "task_submission",
		Title:       "Review SF-86 for Alice",
		Description: "Annual review required",
		Priority:    "high",
	})
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, ai.ID)
	assert.Equal(t, tenantID, ai.TenantID)
	assert.Equal(t, "task_submission", ai.SourceType)
	assert.Nil(t, ai.SourceID)
	assert.Equal(t, "Review SF-86 for Alice", ai.Title)
	assert.Equal(t, "Annual review required", ai.Description)
	assert.Equal(t, "high", ai.Priority)
	assert.Equal(t, "pending", ai.Status)
	assert.Nil(t, ai.AssignedTo)
	assert.Equal(t, "", ai.Notes)
	assert.Nil(t, ai.DueDate)
	assert.False(t, ai.CreatedAt.IsZero())
	assert.False(t, ai.UpdatedAt.IsZero())
}

func TestStore_Create_WithAssignee(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "administrator")

	ai, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantID,
		SourceType: "visit_request",
		Title:      "Process visit request",
		Priority:   "medium",
		AssignedTo: &userID,
	})
	require.NoError(t, err)

	assert.NotNil(t, ai.AssignedTo)
	assert.Equal(t, userID, *ai.AssignedTo)
}

func TestStore_Get(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")

	created, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:    tenantID,
		SourceType:  "task_submission",
		Title:       "Review SF-86",
		Description: "Needs review",
		Priority:    "high",
	})
	require.NoError(t, err)

	got, err := store.Get(ctx, tenantID, created.ID)
	require.NoError(t, err)

	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, tenantID, got.TenantID)
	assert.Equal(t, "task_submission", got.SourceType)
	assert.Equal(t, "Review SF-86", got.Title)
	assert.Equal(t, "Needs review", got.Description)
	assert.Equal(t, "high", got.Priority)
	assert.Equal(t, "pending", got.Status)
}

func TestStore_Get_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")

	_, err := store.Get(ctx, tenantID, uuid.New())
	assert.ErrorIs(t, err, actionitem.ErrNotFound)
}

func TestStore_Get_WrongTenant(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantA := testutil.CreateTenant(t, db, "Tenant A")
	tenantB := testutil.CreateTenant(t, db, "Tenant B")

	created, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantA,
		SourceType: "task_submission",
		Title:      "Tenant A item",
		Priority:   "medium",
	})
	require.NoError(t, err)

	// Attempting to get with wrong tenant should return not found.
	_, err = store.Get(ctx, tenantB, created.ID)
	assert.ErrorIs(t, err, actionitem.ErrNotFound)
}

func TestStore_List(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")

	_, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:    tenantID,
		SourceType:  "task_submission",
		Title:       "Task one",
		Description: "First task",
		Priority:    "low",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, actionitem.CreateParams{
		TenantID:    tenantID,
		SourceType:  "visit_request",
		Title:       "Visit request two",
		Description: "Second item",
		Priority:    "medium",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, actionitem.CreateParams{
		TenantID:    tenantID,
		SourceType:  "task_submission",
		Title:       "Task three",
		Description: "Third task",
		Priority:    "high",
	})
	require.NoError(t, err)

	// Unfiltered — returns all 3
	rows, total, err := store.List(ctx, actionitem.ListFilters{TenantID: tenantID})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, rows, 3)

	// Filter by source_type
	rows, total, err = store.List(ctx, actionitem.ListFilters{
		TenantID:   tenantID,
		SourceType: "task_submission",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, rows, 2)
	for _, r := range rows {
		assert.Equal(t, "task_submission", r.SourceType)
	}

	// Filter by status (all are pending by default)
	rows, total, err = store.List(ctx, actionitem.ListFilters{
		TenantID: tenantID,
		Status:   "pending",
	})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, rows, 3)

	rows, total, err = store.List(ctx, actionitem.ListFilters{
		TenantID: tenantID,
		Status:   "processed",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Len(t, rows, 0)

	// Search filter
	rows, total, err = store.List(ctx, actionitem.ListFilters{
		TenantID: tenantID,
		Search:   "Visit",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, rows, 1)
	assert.Equal(t, "Visit request two", rows[0].Title)

	// Pagination: limit=2, offset=0
	rows, total, err = store.List(ctx, actionitem.ListFilters{
		TenantID: tenantID,
		Limit:    2,
		Offset:   0,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, rows, 2)

	// Pagination: limit=2, offset=2
	rows, total, err = store.List(ctx, actionitem.ListFilters{
		TenantID: tenantID,
		Limit:    2,
		Offset:   2,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, rows, 1)

	// AssignedTo filter — only returns items assigned to a specific user
	identityID := testutil.CreateIdentity(t, db, "ic@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "individual_contributor")

	assigned, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantID,
		SourceType: "task_submission",
		Title:      "Assigned to IC",
		Priority:   "high",
		AssignedTo: &userID,
	})
	require.NoError(t, err)

	rows, total, err = store.List(ctx, actionitem.ListFilters{
		TenantID:   tenantID,
		AssignedTo: &userID,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, rows, 1)
	assert.Equal(t, assigned.ID, rows[0].ID)
}

func TestStore_List_WithAssignee(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "alice@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "administrator")

	_, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantID,
		SourceType: "task_submission",
		Title:      "Assigned task",
		Priority:   "medium",
		AssignedTo: &userID,
	})
	require.NoError(t, err)

	rows, total, err := store.List(ctx, actionitem.ListFilters{TenantID: tenantID})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, rows, 1)

	assert.NotNil(t, rows[0].AssigneeName)
	assert.Equal(t, "Alice", *rows[0].AssigneeName)
	assert.NotNil(t, rows[0].AssigneeEmail)
	assert.Equal(t, "alice@example.com", *rows[0].AssigneeEmail)
}

func TestStore_UpdateStatus(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")

	created, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantID,
		SourceType: "task_submission",
		Title:      "Review task",
		Priority:   "medium",
	})
	require.NoError(t, err)
	assert.Equal(t, "pending", created.Status)

	err = store.UpdateStatus(ctx, tenantID, created.ID, "under_review", "Assigned to reviewer")
	require.NoError(t, err)

	got, err := store.Get(ctx, tenantID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "under_review", got.Status)
	assert.Equal(t, "Assigned to reviewer", got.Notes)
	assert.True(t, got.UpdatedAt.After(created.UpdatedAt) || got.UpdatedAt.Equal(created.UpdatedAt))
}

func TestStore_UpdateStatus_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")

	err := store.UpdateStatus(ctx, tenantID, uuid.New(), "processed", "done")
	assert.ErrorIs(t, err, actionitem.ErrNotFound)
}

func TestStore_SummaryStats(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")

	// Empty tenant — all zeros
	stats, err := store.SummaryStats(ctx, tenantID, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.Total)
	assert.Equal(t, 0, stats.Pending)
	assert.Equal(t, 0, stats.UnderReview)
	assert.Equal(t, 0, stats.Processed)

	// Create items in various statuses
	item1, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantID,
		SourceType: "task_submission",
		Title:      "Pending item 1",
		Priority:   "low",
	})
	require.NoError(t, err)

	item2, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantID,
		SourceType: "visit_request",
		Title:      "Pending item 2",
		Priority:   "medium",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantID,
		SourceType: "travel_report",
		Title:      "Pending item 3",
		Priority:   "high",
	})
	require.NoError(t, err)

	// Move item1 to under_review
	err = store.UpdateStatus(ctx, tenantID, item1.ID, "under_review", "reviewing")
	require.NoError(t, err)

	// Move item2 to processed
	err = store.UpdateStatus(ctx, tenantID, item2.ID, "processed", "done")
	require.NoError(t, err)

	stats, err = store.SummaryStats(ctx, tenantID, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.Total)
	assert.Equal(t, 1, stats.Pending)
	assert.Equal(t, 1, stats.UnderReview)
	assert.Equal(t, 1, stats.Processed)
}

func TestStore_SummaryStats_TenantIsolation(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantA := testutil.CreateTenant(t, db, "Tenant A")
	tenantB := testutil.CreateTenant(t, db, "Tenant B")

	_, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantA,
		SourceType: "task_submission",
		Title:      "Tenant A item",
		Priority:   "medium",
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantB,
		SourceType: "task_submission",
		Title:      "Tenant B item",
		Priority:   "medium",
	})
	require.NoError(t, err)

	statsA, err := store.SummaryStats(ctx, tenantA, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, statsA.Total)

	statsB, err := store.SummaryStats(ctx, tenantB, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, statsB.Total)
}

// setPrimaryFSO sets primary_fso_user_id on a sub-org directly.
func setPrimaryFSO(t *testing.T, db database.DB, subOrgID, userID uuid.UUID) {
	t.Helper()
	_, err := db.Exec(context.Background(), "test.setPrimaryFSO",
		`UPDATE suborganizations SET primary_fso_user_id = $2 WHERE id = $1`,
		subOrgID, userID,
	)
	require.NoError(t, err)
}

func TestStore_Create_AutoAssignPrimaryFSO(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	fsoIdentity := testutil.CreateIdentity(t, db, "fso@acme.com")
	fsoUserID := testutil.CreateUser(t, db, tenantID, fsoIdentity, "fso")
	subOrgID := testutil.CreateSubOrg(t, db, tenantID, "Operations")
	setPrimaryFSO(t, db, subOrgID, fsoUserID)

	ai, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantID,
		SourceType: "visit_request",
		Title:      "Visit Request",
		Priority:   "medium",
		SubOrgID:   &subOrgID,
	})
	require.NoError(t, err)

	// Auto-assigned to the sub-org's primary FSO.
	require.NotNil(t, ai.AssignedTo)
	assert.Equal(t, fsoUserID, *ai.AssignedTo)
	require.NotNil(t, ai.AssigneeEmail)
	assert.Equal(t, "fso@acme.com", *ai.AssigneeEmail)
}

func TestStore_Create_ExplicitAssigneeTakesPrecedence(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	fsoIdentity := testutil.CreateIdentity(t, db, "fso@acme.com")
	fsoUserID := testutil.CreateUser(t, db, tenantID, fsoIdentity, "fso")
	subOrgID := testutil.CreateSubOrg(t, db, tenantID, "Operations")
	setPrimaryFSO(t, db, subOrgID, fsoUserID)

	otherIdentity := testutil.CreateIdentity(t, db, "other@acme.com")
	otherUserID := testutil.CreateUser(t, db, tenantID, otherIdentity, "fso")

	ai, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantID,
		SourceType: "report",
		Title:      "Report",
		Priority:   "high",
		AssignedTo: &otherUserID, // explicit overrides primary FSO
		SubOrgID:   &subOrgID,
	})
	require.NoError(t, err)
	require.NotNil(t, ai.AssignedTo)
	assert.Equal(t, otherUserID, *ai.AssignedTo)
}

func TestStore_Create_NoSubOrg_NoAutoAssign(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")

	ai, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantID,
		SourceType: "report",
		Title:      "Tenant-wide report",
		Priority:   "low",
	})
	require.NoError(t, err)
	assert.Nil(t, ai.AssignedTo)
	assert.Nil(t, ai.AssigneeEmail)
}

func TestStore_UpdateAssignedTo(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	fsoIdentity := testutil.CreateIdentity(t, db, "fso@acme.com")
	fsoUserID := testutil.CreateUser(t, db, tenantID, fsoIdentity, "fso")

	ai, err := store.Create(ctx, actionitem.CreateParams{
		TenantID:   tenantID,
		SourceType: "report",
		Title:      "Report",
		Priority:   "medium",
	})
	require.NoError(t, err)
	assert.Nil(t, ai.AssignedTo)

	// Assign to the FSO.
	updated, err := store.UpdateAssignedTo(ctx, tenantID, ai.ID, &fsoUserID)
	require.NoError(t, err)
	require.NotNil(t, updated.AssignedTo)
	assert.Equal(t, fsoUserID, *updated.AssignedTo)
	require.NotNil(t, updated.AssigneeEmail)
	assert.Equal(t, "fso@acme.com", *updated.AssigneeEmail)

	// Clear the assignment.
	cleared, err := store.UpdateAssignedTo(ctx, tenantID, ai.ID, nil)
	require.NoError(t, err)
	assert.Nil(t, cleared.AssignedTo)
	assert.Nil(t, cleared.AssigneeEmail)
}

func TestStore_UpdateAssignedTo_NotFound(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	fsoIdentity := testutil.CreateIdentity(t, db, "fso@acme.com")
	fsoUserID := testutil.CreateUser(t, db, tenantID, fsoIdentity, "fso")

	_, err := store.UpdateAssignedTo(ctx, tenantID, uuid.New(), &fsoUserID)
	assert.ErrorIs(t, err, actionitem.ErrNotFound)
}

func TestStore_ListFSOUsers(t *testing.T) {
	db := testutil.DB(t)
	store := actionitem.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")

	fsoIdentity := testutil.CreateIdentity(t, db, "fso@acme.com")
	testutil.CreateUser(t, db, tenantID, fsoIdentity, "fso")

	adminIdentity := testutil.CreateIdentity(t, db, "admin@acme.com")
	testutil.CreateUser(t, db, tenantID, adminIdentity, "administrator")

	icIdentity := testutil.CreateIdentity(t, db, "ic@acme.com")
	testutil.CreateUser(t, db, tenantID, icIdentity, "individual_contributor")

	users, err := store.ListFSOUsers(ctx, tenantID)
	require.NoError(t, err)

	emails := make([]string, len(users))
	for i, u := range users {
		emails[i] = u.Email
	}
	assert.Contains(t, emails, "fso@acme.com")
	assert.Contains(t, emails, "admin@acme.com")
	assert.NotContains(t, emails, "ic@acme.com", "ICs should not appear in FSO user list")
}
