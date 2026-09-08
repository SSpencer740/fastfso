//go:build integration

package digest_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SSpencer740/fastfso/backend/internal/database"
	"github.com/SSpencer740/fastfso/backend/internal/digest"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
)

// setUserSettings inserts a user_settings row directly. The store package
// owns the upsert in production; we hit the table directly here to keep the
// test focused on digest behavior.
func setUserSettings(t *testing.T, db database.DB, userID uuid.UUID, freq string, lastSent *time.Time) {
	t.Helper()
	_, err := db.Exec(context.Background(), "test.setUserSettings",
		`INSERT INTO user_settings (user_id, notification_frequency, last_digest_sent_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE
		   SET notification_frequency = EXCLUDED.notification_frequency,
		       last_digest_sent_at    = EXCLUDED.last_digest_sent_at`,
		userID, freq, lastSent,
	)
	require.NoError(t, err)
}

func setIdentityActivated(t *testing.T, db database.DB, identityID uuid.UUID, activated bool) {
	t.Helper()
	_, err := db.Exec(context.Background(), "test.setIdentityActivated",
		`UPDATE identities SET activated = $2 WHERE id = $1`,
		identityID, activated,
	)
	require.NoError(t, err)
}

func insertActionItem(t *testing.T, db database.DB, tenantID uuid.UUID, assignedTo *uuid.UUID, status string, createdAt time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRow(context.Background(), "test.insertActionItem",
		`INSERT INTO action_items (tenant_id, source_type, title, description, priority, status, assigned_to, created_at, updated_at)
		 VALUES ($1, 'visit_request', 'irrelevant', '', 'medium', $2, $3, $4, $4)
		 RETURNING id`,
		tenantID, status, assignedTo, createdAt,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertTask(t *testing.T, db database.DB, tenantID, createdBy uuid.UUID, status string, createdAt time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRow(context.Background(), "test.insertTask",
		`INSERT INTO tasks (tenant_id, created_by_user_id, title, description, priority, status, created_at, updated_at)
		 VALUES ($1, $2, 'irrelevant', '', 'medium', $3, $4, $4)
		 RETURNING id`,
		tenantID, createdBy, status, createdAt,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func assignTaskToUser(t *testing.T, db database.DB, taskID, userID uuid.UUID) {
	t.Helper()
	_, err := db.Exec(context.Background(), "test.assignTaskToUser",
		`INSERT INTO task_assignment_rules (task_id, rule_type, target_id) VALUES ($1, 'user', $2)`,
		taskID, userID,
	)
	require.NoError(t, err)
}

func insertCompletion(t *testing.T, db database.DB, taskID, userID uuid.UUID, reviewedAt *time.Time) {
	t.Helper()
	_, err := db.Exec(context.Background(), "test.insertCompletion",
		`INSERT INTO task_completions (task_id, user_id, status, reviewed_at)
		 VALUES ($1, $2, $3, $4)`,
		taskID, userID,
		map[bool]string{true: "approved", false: "submitted"}[reviewedAt != nil],
		reviewedAt,
	)
	require.NoError(t, err)
}

func TestDigestStore_ListEligibleRecipients(t *testing.T) {
	db := testutil.DB(t)
	store := digest.NewStore(db)

	tenantID := testutil.CreateTenant(t, db, "T1")

	// Eligible: daily_summary, last_sent NULL.
	idA := testutil.CreateIdentity(t, db, "a@example.com")
	userA := testutil.CreateUser(t, db, tenantID, idA, "fso")
	setUserSettings(t, db, userA, "daily_summary", nil)

	// Eligible: daily_summary, last_sent old.
	idB := testutil.CreateIdentity(t, db, "b@example.com")
	userB := testutil.CreateUser(t, db, tenantID, idB, "administrator")
	old := time.Now().Add(-25 * time.Hour)
	setUserSettings(t, db, userB, "daily_summary", &old)

	// Not eligible: every_task setting.
	idC := testutil.CreateIdentity(t, db, "c@example.com")
	userC := testutil.CreateUser(t, db, tenantID, idC, "fso")
	setUserSettings(t, db, userC, "every_task", nil)

	// Not eligible: daily_summary but recently sent.
	idD := testutil.CreateIdentity(t, db, "d@example.com")
	userD := testutil.CreateUser(t, db, tenantID, idD, "fso")
	recent := time.Now().Add(-time.Hour)
	setUserSettings(t, db, userD, "daily_summary", &recent)

	// Not eligible: deactivated identity.
	idE := testutil.CreateIdentity(t, db, "e@example.com")
	userE := testutil.CreateUser(t, db, tenantID, idE, "fso")
	setUserSettings(t, db, userE, "daily_summary", nil)
	setIdentityActivated(t, db, idE, false)

	got, err := store.ListEligibleRecipients(context.Background())
	require.NoError(t, err)

	gotIDs := map[uuid.UUID]bool{}
	for _, r := range got {
		gotIDs[r.UserID] = true
	}
	assert.True(t, gotIDs[userA], "userA should be eligible (NULL last_sent)")
	assert.True(t, gotIDs[userB], "userB should be eligible (old last_sent)")
	assert.False(t, gotIDs[userC], "userC should be excluded (every_task)")
	assert.False(t, gotIDs[userD], "userD should be excluded (recent last_sent)")
	assert.False(t, gotIDs[userE], "userE should be excluded (deactivated identity)")
}

func TestDigestStore_CountForUser_FSO(t *testing.T) {
	db := testutil.DB(t)
	store := digest.NewStore(db)

	tenantID := testutil.CreateTenant(t, db, "T2")
	id := testutil.CreateIdentity(t, db, "fso@example.com")
	userID := testutil.CreateUser(t, db, tenantID, id, "fso")
	old := time.Now().Add(-25 * time.Hour)
	setUserSettings(t, db, userID, "daily_summary", &old)

	now := time.Now()

	// In-window pending assigned to user — counts.
	insertActionItem(t, db, tenantID, &userID, "pending", now.Add(-2*time.Hour))
	// Out-of-window — does not count.
	insertActionItem(t, db, tenantID, &userID, "pending", now.Add(-48*time.Hour))
	// In-window but processed — does not count.
	insertActionItem(t, db, tenantID, &userID, "processed", now.Add(-2*time.Hour))
	// Unassigned — does NOT count for non-admin FSO.
	insertActionItem(t, db, tenantID, nil, "pending", now.Add(-2*time.Hour))
	// Assigned to a different user — does not count.
	other := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "other@example.com"), "fso")
	insertActionItem(t, db, tenantID, &other, "pending", now.Add(-2*time.Hour))

	counts, err := store.CountForUser(context.Background(), digest.Recipient{UserID: userID, Role: "fso"})
	require.NoError(t, err)
	assert.Equal(t, 1, counts.NewActionItems)
	assert.Equal(t, 0, counts.NewTasksAssigned)
	assert.Equal(t, 0, counts.NewTaskReviews)
}

func TestDigestStore_CountForUser_Administrator_IncludesUnassigned(t *testing.T) {
	db := testutil.DB(t)
	store := digest.NewStore(db)

	tenantID := testutil.CreateTenant(t, db, "T3")
	otherTenantID := testutil.CreateTenant(t, db, "T3-other")

	id := testutil.CreateIdentity(t, db, "admin@example.com")
	adminID := testutil.CreateUser(t, db, tenantID, id, "administrator")
	old := time.Now().Add(-25 * time.Hour)
	setUserSettings(t, db, adminID, "daily_summary", &old)

	now := time.Now()
	// Assigned to admin — counts.
	insertActionItem(t, db, tenantID, &adminID, "pending", now.Add(-1*time.Hour))
	// Unassigned in admin's tenant — counts.
	insertActionItem(t, db, tenantID, nil, "pending", now.Add(-1*time.Hour))
	// Unassigned in a different tenant — does NOT count.
	insertActionItem(t, db, otherTenantID, nil, "pending", now.Add(-1*time.Hour))

	counts, err := store.CountForUser(context.Background(), digest.Recipient{UserID: adminID, Role: "administrator"})
	require.NoError(t, err)
	assert.Equal(t, 2, counts.NewActionItems)
}

func TestDigestStore_CountForUser_IC(t *testing.T) {
	db := testutil.DB(t)
	store := digest.NewStore(db)

	tenantID := testutil.CreateTenant(t, db, "T4")
	creator := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "creator@example.com"), "fso")
	ic := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "ic@example.com"), "individual_contributor")
	old := time.Now().Add(-25 * time.Hour)
	setUserSettings(t, db, ic, "daily_summary", &old)

	now := time.Now()

	// In-window task assigned to IC — counts.
	t1 := insertTask(t, db, tenantID, creator, "active", now.Add(-2*time.Hour))
	assignTaskToUser(t, db, t1, ic)
	// Out-of-window task assigned to IC — does not count.
	t2 := insertTask(t, db, tenantID, creator, "active", now.Add(-48*time.Hour))
	assignTaskToUser(t, db, t2, ic)
	// In-window task assigned to a different user — does not count.
	other := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "ic2@example.com"), "individual_contributor")
	t3 := insertTask(t, db, tenantID, creator, "active", now.Add(-2*time.Hour))
	assignTaskToUser(t, db, t3, other)
	// Inactive task — does not count.
	t4 := insertTask(t, db, tenantID, creator, "archived", now.Add(-2*time.Hour))
	assignTaskToUser(t, db, t4, ic)

	// In-window review of IC's submission — counts.
	reviewedAt := now.Add(-90 * time.Minute)
	insertCompletion(t, db, t2, ic, &reviewedAt)
	// Out-of-window review — does not count.
	oldReview := now.Add(-48 * time.Hour)
	insertCompletion(t, db, t1, ic, &oldReview)
	// Submission without a review — does not count.
	insertCompletion(t, db, t3, ic, nil)

	counts, err := store.CountForUser(context.Background(), digest.Recipient{UserID: ic, Role: "individual_contributor"})
	require.NoError(t, err)
	assert.Equal(t, 1, counts.NewTasksAssigned)
	assert.Equal(t, 1, counts.NewTaskReviews)
	assert.Equal(t, 0, counts.NewActionItems)
}

func TestDigestStore_MarkSent(t *testing.T) {
	db := testutil.DB(t)
	store := digest.NewStore(db)

	tenantID := testutil.CreateTenant(t, db, "T5")
	id := testutil.CreateIdentity(t, db, "user@example.com")
	userID := testutil.CreateUser(t, db, tenantID, id, "fso")
	setUserSettings(t, db, userID, "daily_summary", nil)

	require.NoError(t, store.MarkSent(context.Background(), userID))

	// User should no longer be eligible.
	got, err := store.ListEligibleRecipients(context.Background())
	require.NoError(t, err)
	for _, r := range got {
		assert.NotEqual(t, userID, r.UserID, "user should be excluded immediately after MarkSent")
	}
}
