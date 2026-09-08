//go:build integration

package task_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastfso/fastfso/backend/internal/task"
	"github.com/fastfso/fastfso/backend/internal/testutil"
)

// helpers

func setup(t *testing.T) (*task.Store, context.Context) {
	t.Helper()
	db := testutil.DB(t)
	return task.NewStore(db), context.Background()
}

type env struct {
	tenantID    uuid.UUID
	adminID     uuid.UUID // identity
	icID        uuid.UUID // identity
	adminUserID uuid.UUID
	icUserID    uuid.UUID
}

func setupEnv(t *testing.T) (*task.Store, context.Context, env) {
	t.Helper()
	db := testutil.DB(t)
	store := task.NewStore(db)
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

func createActiveTask(t *testing.T, store *task.Store, ctx context.Context, e env) *task.Task {
	t.Helper()
	created, err := store.Create(ctx, task.CreateParams{
		TenantID:        e.tenantID,
		CreatedByUserID: e.adminUserID,
		Title:           "Complete SF-86",
		Description:     "Fill out the security clearance form",
		Priority:        "high",
		Status:          "active",
		Requirements: []task.CreateRequirementParams{
			{Kind: "text", Label: "Full Name", Description: "Your legal name", Required: true, SortOrder: 0},
			{Kind: "checkbox", Label: "I agree", Description: "Accept terms", Required: true, SortOrder: 1},
		},
		Rules: []task.CreateRuleParams{
			{RuleType: "user", TargetID: &e.icUserID},
		},
	})
	require.NoError(t, err)
	return created
}

// tests

func TestStore_Create(t *testing.T) {
	store, ctx, e := setupEnv(t)

	created := createActiveTask(t, store, ctx, e)

	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "Complete SF-86", created.Title)
	assert.Equal(t, "active", created.Status)
	assert.False(t, created.CreatedAt.IsZero())

	// Verify via Get
	detail, err := store.Get(ctx, e.tenantID, created.ID)
	require.NoError(t, err)

	assert.Equal(t, created.ID, detail.Task.ID)
	assert.Len(t, detail.Requirements, 2)
	assert.Equal(t, "Full Name", detail.Requirements[0].Label)
	assert.Equal(t, "I agree", detail.Requirements[1].Label)

	assert.Len(t, detail.Rules, 1)
	assert.Equal(t, "user", detail.Rules[0].RuleType)

	// The user rule should resolve to 1 assignee
	assert.Len(t, detail.Assignees, 1)
	assert.Equal(t, e.icUserID, detail.Assignees[0].UserID)
	assert.Equal(t, "to_do", detail.Assignees[0].Status)
}

func TestStore_Create_OrgWide(t *testing.T) {
	db := testutil.DB(t)
	store := task.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Org Corp")
	adminIdentity := testutil.CreateIdentity(t, db, "boss@org.com")
	adminUserID := testutil.CreateUser(t, db, tenantID, adminIdentity, "administrator")

	// Create 3 users
	id1 := testutil.CreateIdentity(t, db, "u1@org.com")
	id2 := testutil.CreateIdentity(t, db, "u2@org.com")
	id3 := testutil.CreateIdentity(t, db, "u3@org.com")
	testutil.CreateUser(t, db, tenantID, id1, "individual_contributor")
	testutil.CreateUser(t, db, tenantID, id2, "individual_contributor")
	testutil.CreateUser(t, db, tenantID, id3, "individual_contributor")

	created, err := store.Create(ctx, task.CreateParams{
		TenantID:        tenantID,
		CreatedByUserID: adminUserID,
		Title:           "Org-wide task",
		Description:     "Everyone must do this",
		Priority:        "medium",
		Status:          "active",
		Rules: []task.CreateRuleParams{
			{RuleType: "org", TargetID: nil},
		},
	})
	require.NoError(t, err)

	detail, err := store.Get(ctx, tenantID, created.ID)
	require.NoError(t, err)

	// Only the 3 IC members should be assigned (admin excluded — org rule is IC-only)
	assert.Len(t, detail.Assignees, 3)
}

func TestStore_Create_SubOrg(t *testing.T) {
	db := testutil.DB(t)
	store := task.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "SubOrg Corp")
	adminIdentity := testutil.CreateIdentity(t, db, "boss@suborg.com")
	adminUserID := testutil.CreateUser(t, db, tenantID, adminIdentity, "administrator")

	// Create sub-org and assign 2 of 3 users
	subOrgID := testutil.CreateSubOrg(t, db, tenantID, "Engineering")

	id1 := testutil.CreateIdentity(t, db, "eng1@suborg.com")
	id2 := testutil.CreateIdentity(t, db, "eng2@suborg.com")
	id3 := testutil.CreateIdentity(t, db, "sales1@suborg.com")
	u1 := testutil.CreateUser(t, db, tenantID, id1, "individual_contributor")
	u2 := testutil.CreateUser(t, db, tenantID, id2, "individual_contributor")
	testutil.CreateUser(t, db, tenantID, id3, "individual_contributor") // not in sub-org

	testutil.AssignUserToSubOrg(t, db, u1, subOrgID)
	testutil.AssignUserToSubOrg(t, db, u2, subOrgID)

	created, err := store.Create(ctx, task.CreateParams{
		TenantID:        tenantID,
		CreatedByUserID: adminUserID,
		Title:           "Sub-org task",
		Description:     "Engineering only",
		Priority:        "low",
		Status:          "active",
		Rules: []task.CreateRuleParams{
			{RuleType: "sub_org", TargetID: &subOrgID},
		},
	})
	require.NoError(t, err)

	detail, err := store.Get(ctx, tenantID, created.ID)
	require.NoError(t, err)

	// Only the 2 sub-org members should be assigned
	assert.Len(t, detail.Assignees, 2)
}

func TestStore_NewUserGetsExistingOrgTask(t *testing.T) {
	db := testutil.DB(t)
	store := task.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Late Joiner Corp")
	adminIdentity := testutil.CreateIdentity(t, db, "admin@late.com")
	adminUserID := testutil.CreateUser(t, db, tenantID, adminIdentity, "administrator")

	// Create org-wide task BEFORE the new user exists
	created, err := store.Create(ctx, task.CreateParams{
		TenantID:        tenantID,
		CreatedByUserID: adminUserID,
		Title:           "Org task",
		Description:     "For everyone",
		Priority:        "medium",
		Status:          "active",
		Rules: []task.CreateRuleParams{
			{RuleType: "org", TargetID: nil},
		},
	})
	require.NoError(t, err)

	// Add new user AFTER task creation
	newIdentity := testutil.CreateIdentity(t, db, "newbie@late.com")
	newUserID := testutil.CreateUser(t, db, tenantID, newIdentity, "individual_contributor")

	// The view should include the new user
	myTasks, err := store.ListMyTasks(ctx, tenantID, newUserID, "", 0)
	require.NoError(t, err)
	require.Len(t, myTasks, 1)
	assert.Equal(t, created.ID, myTasks[0].ID)
}

func TestStore_ListMyTasks(t *testing.T) {
	store, ctx, e := setupEnv(t)

	// Create task assigned to IC
	assigned := createActiveTask(t, store, ctx, e)

	// Create task NOT assigned to IC (assigned to admin only)
	_, err := store.Create(ctx, task.CreateParams{
		TenantID:        e.tenantID,
		CreatedByUserID: e.adminUserID,
		Title:           "Admin-only task",
		Description:     "Not for IC",
		Priority:        "low",
		Status:          "active",
		Rules: []task.CreateRuleParams{
			{RuleType: "user", TargetID: &e.adminUserID},
		},
	})
	require.NoError(t, err)

	tasks, err := store.ListMyTasks(ctx, e.tenantID, e.icUserID, "", 0)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, assigned.ID, tasks[0].ID)
	assert.Equal(t, "to_do", tasks[0].Status)
}

func TestStore_CompletionWorkflow(t *testing.T) {
	store, ctx, e := setupEnv(t)
	created := createActiveTask(t, store, ctx, e)

	// 1. EnsureCompletion
	comp, err := store.EnsureCompletion(ctx, e.tenantID, created.ID, e.icUserID)
	require.NoError(t, err)
	assert.Equal(t, "to_do", comp.Status)
	assert.Nil(t, comp.ViewedAt)

	// 2. MarkViewed
	err = store.MarkViewed(ctx, comp.ID)
	require.NoError(t, err)

	// Re-fetch to verify
	comp2, err := store.EnsureCompletion(ctx, e.tenantID, created.ID, e.icUserID)
	require.NoError(t, err)
	assert.NotNil(t, comp2.ViewedAt)

	// 3. Get requirements for responses
	detail, err := store.Get(ctx, e.tenantID, created.ID)
	require.NoError(t, err)
	require.Len(t, detail.Requirements, 2)

	textVal := "John Doe"
	boolVal := true
	err = store.SaveResponses(ctx, comp.ID, []task.SaveResponseParams{
		{RequirementID: detail.Requirements[0].ID, TextValue: &textVal},
		{RequirementID: detail.Requirements[1].ID, BoolValue: &boolVal},
	})
	require.NoError(t, err)

	// Verify status changed to in_progress
	comp3, err := store.EnsureCompletion(ctx, e.tenantID, created.ID, e.icUserID)
	require.NoError(t, err)
	assert.Equal(t, "in_progress", comp3.Status)

	// 4. Submit
	err = store.Submit(ctx, comp.ID)
	require.NoError(t, err)

	comp4, err := store.EnsureCompletion(ctx, e.tenantID, created.ID, e.icUserID)
	require.NoError(t, err)
	assert.Equal(t, "submitted", comp4.Status)
	assert.NotNil(t, comp4.SubmittedAt)

	// 5. Approve
	err = store.Approve(ctx, comp.ID, e.adminUserID)
	require.NoError(t, err)

	comp5, err := store.EnsureCompletion(ctx, e.tenantID, created.ID, e.icUserID)
	require.NoError(t, err)
	assert.Equal(t, "approved", comp5.Status)
	assert.NotNil(t, comp5.ReviewedAt)
	assert.Equal(t, &e.adminUserID, comp5.ReviewedBy)
}

func TestStore_SummaryStats(t *testing.T) {
	store, ctx, e := setupEnv(t)

	// Create tasks in various statuses
	_, err := store.Create(ctx, task.CreateParams{
		TenantID: e.tenantID, CreatedByUserID: e.adminUserID,
		Title: "Active 1", Priority: "high", Status: "active",
		Rules: []task.CreateRuleParams{{RuleType: "user", TargetID: &e.icUserID}},
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, task.CreateParams{
		TenantID: e.tenantID, CreatedByUserID: e.adminUserID,
		Title: "Active 2", Priority: "medium", Status: "active",
		Rules: []task.CreateRuleParams{{RuleType: "user", TargetID: &e.icUserID}},
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, task.CreateParams{
		TenantID: e.tenantID, CreatedByUserID: e.adminUserID,
		Title: "Draft 1", Priority: "low", Status: "draft",
	})
	require.NoError(t, err)

	stats, err := store.SummaryStats(ctx, e.tenantID, nil, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, 3, stats.Total)
	assert.Equal(t, 2, stats.Active)
	assert.Equal(t, 1, stats.Draft)
	assert.Equal(t, 0, stats.NeedsReview)
}

func TestStore_MySummaryStats(t *testing.T) {
	store, ctx, e := setupEnv(t)

	// Create 2 active tasks assigned to IC
	t1, err := store.Create(ctx, task.CreateParams{
		TenantID: e.tenantID, CreatedByUserID: e.adminUserID,
		Title: "Task 1", Priority: "high", Status: "active",
		Rules: []task.CreateRuleParams{{RuleType: "user", TargetID: &e.icUserID}},
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, task.CreateParams{
		TenantID: e.tenantID, CreatedByUserID: e.adminUserID,
		Title: "Task 2", Priority: "medium", Status: "active",
		Rules: []task.CreateRuleParams{{RuleType: "user", TargetID: &e.icUserID}},
	})
	require.NoError(t, err)

	// Submit one task
	comp, err := store.EnsureCompletion(ctx, e.tenantID, t1.ID, e.icUserID)
	require.NoError(t, err)
	err = store.Submit(ctx, comp.ID)
	require.NoError(t, err)

	stats, err := store.MySummaryStats(ctx, e.tenantID, e.icUserID, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, 2, stats.Total)
	assert.Equal(t, 1, stats.ToDo)
	assert.Equal(t, 0, stats.InProgress)
	assert.Equal(t, 1, stats.Submitted)
	assert.Equal(t, 0, stats.Approved)
}

func TestStore_Upload_CRUD(t *testing.T) {
	store, ctx, e := setupEnv(t)
	created := createActiveTask(t, store, ctx, e)

	// Get requirement ID for uploads
	detail, err := store.Get(ctx, e.tenantID, created.ID)
	require.NoError(t, err)
	require.NotEmpty(t, detail.Requirements)
	reqID := detail.Requirements[0].ID

	comp, err := store.EnsureCompletion(ctx, e.tenantID, created.ID, e.icUserID)
	require.NoError(t, err)

	// Create
	upload := &task.Upload{
		CompletionID:  comp.ID,
		RequirementID: reqID,
		FileName:      "sf86.pdf",
		FileSize:      1024,
		ContentType:   "application/pdf",
		StorageKey:    "uploads/sf86.pdf",
	}
	err = store.CreateUpload(ctx, upload)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, upload.ID)
	assert.False(t, upload.CreatedAt.IsZero())

	// List
	uploads, err := store.ListUploads(ctx, comp.ID)
	require.NoError(t, err)
	require.Len(t, uploads, 1)
	assert.Equal(t, "sf86.pdf", uploads[0].FileName)
	assert.Equal(t, int64(1024), uploads[0].FileSize)

	// Delete
	key, err := store.DeleteUpload(ctx, e.tenantID, upload.ID)
	require.NoError(t, err)
	assert.Equal(t, "uploads/sf86.pdf", key)

	// Verify deleted
	uploads, err = store.ListUploads(ctx, comp.ID)
	require.NoError(t, err)
	assert.Empty(t, uploads)

	// Delete again should return ErrNotFound
	_, err = store.DeleteUpload(ctx, e.tenantID, upload.ID)
	assert.ErrorIs(t, err, task.ErrNotFound)
}

func TestStore_GetUserContact(t *testing.T) {
	store, ctx, e := setupEnv(t)

	email, name, err := store.GetUserContact(ctx, e.tenantID, e.icUserID)
	require.NoError(t, err)
	assert.Equal(t, "alice@acme.com", email)
	assert.Equal(t, "Alice", name)
}

func TestStore_GetUserContact_WrongTenant(t *testing.T) {
	store, ctx, e := setupEnv(t)

	// User belongs to e.tenantID; a random UUID won't match — returns empty, no error.
	email, name, err := store.GetUserContact(ctx, uuid.New(), e.icUserID)
	require.NoError(t, err)
	assert.Empty(t, email)
	assert.Empty(t, name)
}

func TestStore_ExportByDateRange(t *testing.T) {
	store, ctx, e := setupEnv(t)

	// Create a task and assign it to the IC.
	created := createActiveTask(t, store, ctx, e)

	// Materialize a completion row for the IC.
	comp, err := store.EnsureCompletion(ctx, e.tenantID, created.ID, e.icUserID)
	require.NoError(t, err)

	// Submit the completion so status is "submitted".
	require.NoError(t, store.Submit(ctx, comp.ID))

	// Export covering today — should include the task.
	from := created.CreatedAt.Add(-time.Hour)
	to := created.CreatedAt.Add(time.Hour)
	rows, err := store.ExportByDateRange(ctx, e.tenantID, from, to)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	r := rows[0]
	assert.Equal(t, "Complete SF-86", r.TaskTitle)
	assert.Equal(t, "alice@acme.com", r.AssigneeEmail)
	assert.Equal(t, "submitted", r.Status)
	assert.NotNil(t, r.CompletedAt)

	// Export with a range that excludes the task.
	past := created.CreatedAt.Add(-48 * time.Hour)
	rows, err = store.ExportByDateRange(ctx, e.tenantID, past, past.Add(time.Hour))
	require.NoError(t, err)
	assert.Empty(t, rows)

	// Export for a different tenant should return nothing.
	rows, err = store.ExportByDateRange(ctx, uuid.New(), from, to)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// --- FSO scoping tests (added with fix/fso-task-scoping) ---

func TestStore_ListTenantUsers_SubOrgScope(t *testing.T) {
	store, ctx, e := setupEnv(t)
	db := testutil.DB(t)

	// Recreate the env after testutil.DB truncated everything.
	tenantID := testutil.CreateTenant(t, db, "Acme")
	subA := testutil.CreateSubOrg(t, db, tenantID, "SubA")
	subB := testutil.CreateSubOrg(t, db, tenantID, "SubB")

	idA := testutil.CreateIdentity(t, db, "ic-a@acme.com")
	idB := testutil.CreateIdentity(t, db, "ic-b@acme.com")
	uA := testutil.CreateUser(t, db, tenantID, idA, "individual_contributor")
	uB := testutil.CreateUser(t, db, tenantID, idB, "individual_contributor")
	testutil.AssignUserToSubOrg(t, db, uA, subA)
	testutil.AssignUserToSubOrg(t, db, uB, subB)

	// No scope → both users.
	all, err := store.ListTenantUsers(ctx, tenantID, "", nil)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	// Scope to SubA → only uA.
	scoped, err := store.ListTenantUsers(ctx, tenantID, "", &subA)
	require.NoError(t, err)
	require.Len(t, scoped, 1)
	assert.Equal(t, uA, scoped[0].UserID)

	// Scope to SubB → only uB.
	scoped, err = store.ListTenantUsers(ctx, tenantID, "", &subB)
	require.NoError(t, err)
	require.Len(t, scoped, 1)
	assert.Equal(t, uB, scoped[0].UserID)

	// Scope + search.
	scoped, err = store.ListTenantUsers(ctx, tenantID, "ic-a", &subA)
	require.NoError(t, err)
	assert.Len(t, scoped, 1)

	scoped, err = store.ListTenantUsers(ctx, tenantID, "ic-a", &subB)
	require.NoError(t, err)
	assert.Empty(t, scoped, "user in SubA should not appear when scoped to SubB")
	_ = e // referenced for symmetry with other tests' setup
}

func TestStore_UsersInSubOrg(t *testing.T) {
	store, ctx, _ := setupEnv(t)
	db := testutil.DB(t)

	tenantID := testutil.CreateTenant(t, db, "Acme")
	subA := testutil.CreateSubOrg(t, db, tenantID, "SubA")
	subB := testutil.CreateSubOrg(t, db, tenantID, "SubB")

	idA := testutil.CreateIdentity(t, db, "a@acme.com")
	idB := testutil.CreateIdentity(t, db, "b@acme.com")
	idC := testutil.CreateIdentity(t, db, "c@acme.com")
	uA := testutil.CreateUser(t, db, tenantID, idA, "individual_contributor")
	uB := testutil.CreateUser(t, db, tenantID, idB, "individual_contributor")
	uC := testutil.CreateUser(t, db, tenantID, idC, "individual_contributor")
	testutil.AssignUserToSubOrg(t, db, uA, subA)
	testutil.AssignUserToSubOrg(t, db, uB, subA)
	testutil.AssignUserToSubOrg(t, db, uC, subB)

	got, err := store.UsersInSubOrg(ctx, []uuid.UUID{uA, uB, uC}, subA)
	require.NoError(t, err)
	assert.True(t, got[uA])
	assert.True(t, got[uB])
	assert.False(t, got[uC], "uC is in SubB, not SubA")

	// Empty input is a no-op.
	got, err = store.UsersInSubOrg(ctx, nil, subA)
	require.NoError(t, err)
	assert.Empty(t, got)
}
