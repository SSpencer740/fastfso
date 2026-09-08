//go:build integration

package reminders_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SSpencer740/fastfso/backend/internal/database"
	"github.com/SSpencer740/fastfso/backend/internal/email"
	"github.com/SSpencer740/fastfso/backend/internal/reminders"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
)

// recordedReminder captures a NotifyTaskReminder call for assertion.
type recordedReminder struct {
	UserID uuid.UUID
	Counts email.ReminderCounts
}

type recordedClearance struct {
	FSOUserID uuid.UUID
	Counts    email.ClearanceReminderCounts
}

type recordedDd254 struct {
	FSOUserID uuid.UUID
	Counts    email.Dd254ReminderCounts
}

type captureNotifier struct {
	mu         sync.Mutex
	reminders  []recordedReminder
	clearances []recordedClearance
	dd254s     []recordedDd254
}

func (c *captureNotifier) NotifyTaskReminder(_ context.Context, userID uuid.UUID, _, _ string, counts email.ReminderCounts) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reminders = append(c.reminders, recordedReminder{UserID: userID, Counts: counts})
}

func (c *captureNotifier) NotifyClearanceDue(_ context.Context, fsoUserID uuid.UUID, _, _ string, counts email.ClearanceReminderCounts) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearances = append(c.clearances, recordedClearance{FSOUserID: fsoUserID, Counts: counts})
}

func (c *captureNotifier) NotifyDd254Expiring(_ context.Context, fsoUserID uuid.UUID, _, _ string, counts email.Dd254ReminderCounts) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dd254s = append(c.dd254s, recordedDd254{FSOUserID: fsoUserID, Counts: counts})
}

func (c *captureNotifier) seenFor(userID uuid.UUID) (recordedReminder, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range c.reminders {
		if r.UserID == userID {
			return r, true
		}
	}
	return recordedReminder{}, false
}

func setUserSettings(t *testing.T, db database.DB, userID uuid.UUID, freq string) {
	t.Helper()
	_, err := db.Exec(context.Background(), "test.setUserSettings",
		`INSERT INTO user_settings (user_id, notification_frequency)
		 VALUES ($1, $2)
		 ON CONFLICT (user_id) DO UPDATE SET notification_frequency = EXCLUDED.notification_frequency`,
		userID, freq,
	)
	require.NoError(t, err)
}

func insertTaskWithDueDate(t *testing.T, db database.DB, tenantID, creatorID uuid.UUID, dueDate time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRow(context.Background(), "test.insertTask",
		`INSERT INTO tasks (tenant_id, created_by_user_id, title, description, priority, status, due_date)
		 VALUES ($1, $2, 'irrelevant', '', 'medium', 'active', $3)
		 RETURNING id`,
		tenantID, creatorID, dueDate,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func assignTask(t *testing.T, db database.DB, taskID, userID uuid.UUID) {
	t.Helper()
	_, err := db.Exec(context.Background(), "test.assignTask",
		`INSERT INTO task_assignment_rules (task_id, rule_type, target_id) VALUES ($1, 'user', $2)`,
		taskID, userID,
	)
	require.NoError(t, err)
}

func insertDebrief(t *testing.T, db database.DB, tenantID, userID uuid.UUID, dueDate *time.Time, status string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	// travel_debriefs requires a report_id; create a minimal report for it.
	var reportID uuid.UUID
	err := db.QueryRow(context.Background(), "test.insertReport",
		`INSERT INTO travel_reports (tenant_id, user_id, trip_name, multi_country, passport_number,
		    emergency_first_name, emergency_last_name, emergency_phone, additional_comments, status)
		 VALUES ($1, $2, 'trip', false, '', '', '', '', '', 'approved')
		 RETURNING id`,
		tenantID, userID,
	).Scan(&reportID)
	require.NoError(t, err)
	err = db.QueryRow(context.Background(), "test.insertDebrief",
		`INSERT INTO travel_debriefs (report_id, tenant_id, user_id, due_date, status)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		reportID, tenantID, userID, dueDate, status,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestReminders_OverdueAndDueSoon(t *testing.T) {
	db := testutil.DB(t)
	store := reminders.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	creator := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "fso@a.com"), "fso")
	ic := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "ic@a.com"), "individual_contributor")
	setUserSettings(t, db, ic, "every_task")

	// Use explicit UTC dates at midnight to avoid local-TZ casts shifting the
	// day boundary relative to CURRENT_DATE in postgres.
	today := time.Now().UTC()
	mid := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
	yesterday := mid(today).AddDate(0, 0, -1)
	tomorrow := mid(today).AddDate(0, 0, 1)
	farFuture := mid(today).AddDate(0, 0, 60)

	overdueTask := insertTaskWithDueDate(t, db, tenantID, creator, yesterday)
	assignTask(t, db, overdueTask, ic)
	dueSoonTask := insertTaskWithDueDate(t, db, tenantID, creator, tomorrow)
	assignTask(t, db, dueSoonTask, ic)
	farFutureTask := insertTaskWithDueDate(t, db, tenantID, creator, farFuture)
	assignTask(t, db, farFutureTask, ic)

	dueDate := yesterday
	insertDebrief(t, db, tenantID, ic, &dueDate, "pending") // overdue debrief
	dueDate2 := tomorrow
	insertDebrief(t, db, tenantID, ic, &dueDate2, "pending") // due-soon debrief

	counts, err := store.CountForUser(ctx, ic)
	require.NoError(t, err)
	assert.Equal(t, 1, counts.OverdueTasks, "yesterday's task is overdue")
	assert.Equal(t, 1, counts.DueSoonTasks, "tomorrow's task counts as due soon")
	assert.Equal(t, 1, counts.OverdueDebriefs)
	assert.Equal(t, 1, counts.DueSoonDebriefs)
}

func TestReminders_ListRecipients_ExcludesDailySummaryUsers(t *testing.T) {
	db := testutil.DB(t)
	store := reminders.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	icEvery := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "every@a.com"), "individual_contributor")
	icDaily := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "daily@a.com"), "individual_contributor")
	icDefault := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "default@a.com"), "individual_contributor")
	fso := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "fso@a.com"), "fso")

	setUserSettings(t, db, icEvery, "every_task")
	setUserSettings(t, db, icDaily, "daily_summary")
	// icDefault has no user_settings row → defaults to every_task (eligible).

	recipients, err := store.ListRecipients(ctx)
	require.NoError(t, err)

	got := map[uuid.UUID]bool{}
	for _, r := range recipients {
		got[r.UserID] = true
	}
	assert.True(t, got[icEvery], "every_task IC should be eligible")
	assert.True(t, got[icDefault], "IC with no settings row defaults to every_task and should be eligible")
	assert.False(t, got[icDaily], "daily_summary user should NOT be reminded — they get counts in their digest")
	assert.False(t, got[fso], "FSOs are not in the IC reminder population")
}

func TestReminders_RunSendsOnlyWhenContent(t *testing.T) {
	db := testutil.DB(t)
	store := reminders.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	creator := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "fso@a.com"), "fso")

	icBusy := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "busy@a.com"), "individual_contributor")
	icIdle := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "idle@a.com"), "individual_contributor")
	setUserSettings(t, db, icBusy, "every_task")
	setUserSettings(t, db, icIdle, "every_task")

	// Only icBusy has overdue work.
	overdue := insertTaskWithDueDate(t, db, tenantID, creator, time.Now().Add(-24*time.Hour))
	assignTask(t, db, overdue, icBusy)

	cap := &captureNotifier{}
	svc := reminders.New(store, cap, slogTestLogger())
	require.NoError(t, svc.Run(ctx))

	got, ok := cap.seenFor(icBusy)
	require.True(t, ok, "busy IC must have been notified")
	assert.Equal(t, 1, got.Counts.OverdueTasks)

	_, ok = cap.seenFor(icIdle)
	assert.False(t, ok, "idle IC has nothing pending; reminder must be skipped silently")
}

// slogTestLogger returns a logger that throws output away. Tests don't
// assert on log content; we just don't want noise during a green run.
func slogTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestService_Run_ClearanceReminders(t *testing.T) {
	db := testutil.DB(t)
	store := reminders.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	subOrgID := testutil.CreateSubOrg(t, db, tenantID, "Engineering")

	fsoUserID := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "fso@a.com"), "fso")
	icUserID := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "ic@a.com"), "individual_contributor")
	testutil.AssignUserToSubOrg(t, db, icUserID, subOrgID)

	// Make the FSO the primary FSO of the sub-org.
	_, err := db.Exec(ctx, "test.setPrimaryFSO",
		`UPDATE suborganizations SET primary_fso_user_id = $1 WHERE id = $2`,
		fsoUserID, subOrgID)
	require.NoError(t, err)

	// Create three clearance records in different buckets.
	insertClearance := func(userID uuid.UUID, daysOut int) {
		t.Helper()
		next := time.Now().AddDate(0, 0, daysOut).Format("2006-01-02")
		_, err := db.Exec(ctx, "test.insertClearance",
			`INSERT INTO user_clearance_records (user_id, clearance, next_investigation_date, recorded_by)
			 VALUES ($1, 'secret'::clearance_level, $2::date, $1)`,
			userID, next)
		require.NoError(t, err)
	}
	insertClearance(icUserID, -5) // overdue
	// Need separate users for separate active records (only one current per user).
	ic2 := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "ic2@a.com"), "individual_contributor")
	testutil.AssignUserToSubOrg(t, db, ic2, subOrgID)
	insertClearance(ic2, 20)
	ic3 := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "ic3@a.com"), "individual_contributor")
	testutil.AssignUserToSubOrg(t, db, ic3, subOrgID)
	insertClearance(ic3, 60)

	cap := &captureNotifier{}
	svc := reminders.New(store, cap, slogTestLogger())
	require.NoError(t, svc.Run(ctx))

	require.Len(t, cap.clearances, 1, "FSO should get exactly one aggregated email")
	got := cap.clearances[0]
	assert.Equal(t, fsoUserID, got.FSOUserID)
	assert.Equal(t, 1, got.Counts.Overdue)
	assert.Equal(t, 1, got.Counts.Due30Days)
	assert.Equal(t, 1, got.Counts.Due90Days)

	// Run again immediately — overdue still fires daily, but 30d and 90d are
	// gated. Reset overdue's last_due_reminder_at to yesterday so it qualifies.
	_, err = db.Exec(ctx, "test.backdate",
		`UPDATE user_clearance_records SET last_due_reminder_at = now() - INTERVAL '1 day'
		 WHERE next_investigation_date < CURRENT_DATE`)
	require.NoError(t, err)

	cap2 := &captureNotifier{}
	svc2 := reminders.New(store, cap2, slogTestLogger())
	require.NoError(t, svc2.Run(ctx))
	require.Len(t, cap2.clearances, 1)
	assert.Equal(t, 1, cap2.clearances[0].Counts.Overdue)
	assert.Equal(t, 0, cap2.clearances[0].Counts.Due30Days, "30d bucket gated by recent send")
	assert.Equal(t, 0, cap2.clearances[0].Counts.Due90Days, "90d bucket gated by recent send")
}

func TestService_Run_Dd254Reminders(t *testing.T) {
	db := testutil.DB(t)
	store := reminders.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme")
	subOrgID := testutil.CreateSubOrg(t, db, tenantID, "Engineering")
	fsoUserID := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "fso@a.com"), "fso")
	uploaderID := testutil.CreateUser(t, db, tenantID, testutil.CreateIdentity(t, db, "uploader@a.com"), "administrator")

	_, err := db.Exec(ctx, "test.setPrimaryFSO",
		`UPDATE suborganizations SET primary_fso_user_id = $1 WHERE id = $2`,
		fsoUserID, subOrgID)
	require.NoError(t, err)

	insertDd254 := func(contract string, daysOut int) {
		t.Helper()
		periodEnd := time.Now().AddDate(0, 0, daysOut).Format("2006-01-02")
		_, err := db.Exec(ctx, "test.insertDd254",
			`INSERT INTO dd254_forms
			 (tenant_id, sub_org_id, contract_number, classification_max,
			  period_start, period_end,
			  storage_key, filename, content_type, size_bytes, markings,
			  cui_attestation_by, uploaded_by)
			 VALUES ($1, $2, $3, 'secret'::clearance_level, CURRENT_DATE - INTERVAL '1 year', $4::date,
			         'key', 'f.pdf', 'application/pdf', 1, 'unclassified', $5, $5)`,
			tenantID, subOrgID, contract, periodEnd, uploaderID)
		require.NoError(t, err)
	}
	insertDd254("C-1", -10) // expired
	insertDd254("C-2", 20)  // 30d
	insertDd254("C-3", 60)  // 90d

	cap := &captureNotifier{}
	svc := reminders.New(store, cap, slogTestLogger())
	require.NoError(t, svc.Run(ctx))

	require.Len(t, cap.dd254s, 1)
	got := cap.dd254s[0]
	assert.Equal(t, fsoUserID, got.FSOUserID)
	assert.Equal(t, 1, got.Counts.Overdue)
	assert.Equal(t, 1, got.Counts.Due30Days)
	assert.Equal(t, 1, got.Counts.Due90Days)
}
