//go:build integration

package verification_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SSpencer740/fastfso/backend/internal/database"
	"github.com/SSpencer740/fastfso/backend/internal/testutil"
	"github.com/SSpencer740/fastfso/backend/internal/verification"
)

// createUploadFixture builds the FK chain needed to insert into
// upload_verifications: tenant -> identity -> user -> task -> requirement ->
// completion -> upload. Returns (uploadID, tenantID).
func createUploadFixture(t *testing.T, db database.DB) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "ic@example.com")
	userID := testutil.CreateUser(t, db, tenantID, identityID, "individual_contributor")

	var taskID uuid.UUID
	require.NoError(t, db.QueryRow(ctx, "test.task",
		`INSERT INTO tasks (tenant_id, created_by_user_id, title, status)
		 VALUES ($1, $2, $3, 'active') RETURNING id`,
		tenantID, userID, "Cyber Awareness 2026").Scan(&taskID))

	var reqID uuid.UUID
	require.NoError(t, db.QueryRow(ctx, "test.requirement",
		`INSERT INTO task_requirements (task_id, kind, label, ai_verification_criteria)
		 VALUES ($1, 'file_upload', 'Cert', 'verify cyber awareness cert with assignee name') RETURNING id`,
		taskID).Scan(&reqID))

	var completionID uuid.UUID
	require.NoError(t, db.QueryRow(ctx, "test.completion",
		`INSERT INTO task_completions (task_id, user_id, status)
		 VALUES ($1, $2, 'in_progress') RETURNING id`,
		taskID, userID).Scan(&completionID))

	var uploadID uuid.UUID
	require.NoError(t, db.QueryRow(ctx, "test.upload",
		`INSERT INTO task_uploads (completion_id, requirement_id, file_name, file_size, content_type, storage_key)
		 VALUES ($1, $2, 'cert.pdf', 1024, 'application/pdf', 'tenants/x/tasks/y/cert.pdf') RETURNING id`,
		completionID, reqID).Scan(&uploadID))

	return uploadID, tenantID
}

func TestStore_CreatePending(t *testing.T) {
	db := testutil.DB(t)
	store := verification.NewStore(db)
	ctx := context.Background()

	uploadID, tenantID := createUploadFixture(t, db)
	id, err := store.CreatePending(ctx, uploadID, tenantID, "verify the cert")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, id)

	rec, err := store.GetByUpload(ctx, uploadID)
	require.NoError(t, err)
	assert.Equal(t, verification.StatusPending, rec.Status)
	assert.False(t, rec.Flagged)
	assert.Equal(t, "verify the cert", rec.CriteriaSnapshot)
}

func TestStore_MarkSucceeded(t *testing.T) {
	db := testutil.DB(t)
	store := verification.NewStore(db)
	ctx := context.Background()

	uploadID, tenantID := createUploadFixture(t, db)
	_, err := store.CreatePending(ctx, uploadID, tenantID, "verify")
	require.NoError(t, err)

	v := verification.Verdict{
		TypeMatch:       true,
		ExtractedFields: map[string]string{"name": "Alice Doe", "course": "Cyber Awareness 2026"},
		Discrepancies:   []string{},
		Flagged:         false,
		Confidence:      0.92,
		Reasoning:       "Cert matches expected format and name.",
	}
	require.NoError(t, store.MarkSucceeded(ctx, uploadID, "gemini-2.5-flash", v))

	rec, err := store.GetByUpload(ctx, uploadID)
	require.NoError(t, err)
	assert.Equal(t, verification.StatusSucceeded, rec.Status)
	assert.False(t, rec.Flagged)
	require.NotNil(t, rec.Confidence)
	assert.InDelta(t, 0.92, *rec.Confidence, 0.0001)
	require.NotNil(t, rec.TypeMatch)
	assert.True(t, *rec.TypeMatch)
	assert.Equal(t, "Alice Doe", rec.ExtractedFields["name"])
	assert.Equal(t, "gemini-2.5-flash", rec.Model)
}

func TestStore_MarkFailed(t *testing.T) {
	db := testutil.DB(t)
	store := verification.NewStore(db)
	ctx := context.Background()

	uploadID, tenantID := createUploadFixture(t, db)
	_, err := store.CreatePending(ctx, uploadID, tenantID, "verify")
	require.NoError(t, err)

	require.NoError(t, store.MarkFailed(ctx, uploadID, "gemini timeout"))

	rec, err := store.GetByUpload(ctx, uploadID)
	require.NoError(t, err)
	assert.Equal(t, verification.StatusFailed, rec.Status)
	assert.Equal(t, "gemini timeout", rec.ErrorMessage)
	assert.False(t, rec.Flagged)
}

func TestStore_ListByCompletion(t *testing.T) {
	db := testutil.DB(t)
	store := verification.NewStore(db)
	ctx := context.Background()

	uploadID, tenantID := createUploadFixture(t, db)
	_, err := store.CreatePending(ctx, uploadID, tenantID, "verify")
	require.NoError(t, err)

	var completionID uuid.UUID
	require.NoError(t, db.QueryRow(ctx, "test.lookup_completion",
		`SELECT completion_id FROM task_uploads WHERE id = $1`, uploadID).Scan(&completionID))

	result, err := store.ListByCompletion(ctx, completionID)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, verification.StatusPending, result[uploadID].Status)
}

func TestStore_GetByUpload_NoRows(t *testing.T) {
	db := testutil.DB(t)
	store := verification.NewStore(db)
	ctx := context.Background()

	_, err := store.GetByUpload(ctx, uuid.New())
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestStore_RecordFeedback(t *testing.T) {
	db := testutil.DB(t)
	store := verification.NewStore(db)
	ctx := context.Background()

	uploadID, tenantID := createUploadFixture(t, db)
	_, err := store.CreatePending(ctx, uploadID, tenantID, "verify")
	require.NoError(t, err)

	adminIdentity := testutil.CreateIdentity(t, db, "admin@example.com")
	adminUser := testutil.CreateUser(t, db, tenantID, adminIdentity, "administrator")

	require.NoError(t, store.RecordFeedback(ctx, uploadID, adminUser, verification.FeedbackCorrect))
	rec, err := store.GetByUpload(ctx, uploadID)
	require.NoError(t, err)
	require.NotNil(t, rec.AdminFeedback)
	assert.Equal(t, verification.FeedbackCorrect, *rec.AdminFeedback)
	require.NotNil(t, rec.AdminFeedbackBy)
	assert.Equal(t, adminUser, *rec.AdminFeedbackBy)

	// Admins can change their mind.
	require.NoError(t, store.RecordFeedback(ctx, uploadID, adminUser, verification.FeedbackIncorrect))
	rec, err = store.GetByUpload(ctx, uploadID)
	require.NoError(t, err)
	require.NotNil(t, rec.AdminFeedback)
	assert.Equal(t, verification.FeedbackIncorrect, *rec.AdminFeedback)
}

func TestStore_RecordFeedback_RejectsInvalid(t *testing.T) {
	db := testutil.DB(t)
	store := verification.NewStore(db)
	ctx := context.Background()

	uploadID, tenantID := createUploadFixture(t, db)
	_, err := store.CreatePending(ctx, uploadID, tenantID, "verify")
	require.NoError(t, err)

	identityID := testutil.CreateIdentity(t, db, "admin@example.com")
	adminUser := testutil.CreateUser(t, db, tenantID, identityID, "administrator")

	err = store.RecordFeedback(context.Background(), uploadID, adminUser, "maybe")
	assert.Error(t, err)
}

func TestStore_RecordFeedback_NoUpload(t *testing.T) {
	db := testutil.DB(t)
	store := verification.NewStore(db)
	ctx := context.Background()

	tenantID := testutil.CreateTenant(t, db, "Acme Corp")
	identityID := testutil.CreateIdentity(t, db, "admin@example.com")
	adminUser := testutil.CreateUser(t, db, tenantID, identityID, "administrator")

	err := store.RecordFeedback(ctx, uuid.New(), adminUser, verification.FeedbackCorrect)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}
