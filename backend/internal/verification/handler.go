package verification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fastfso/fastfso/backend/internal/aiusage"
	"github.com/fastfso/fastfso/backend/internal/auth"
	"github.com/fastfso/fastfso/backend/internal/database"
	"github.com/fastfso/fastfso/backend/internal/storage"
	"github.com/fastfso/fastfso/backend/internal/telemetry"
)

// AIFeature is the aiusage feature key for verification calls. Matches the
// chat handler's key style; rolls up into the same ai_usage table.
const AIFeature = "task_upload_verification"

// VerifyTaskBody is the JSON payload Cloud Tasks delivers to the verify
// endpoint. Holding just the upload_id (the rest is fetched from the DB) keeps
// the task body small and avoids serializing tenant context.
type VerifyTaskBody struct {
	UploadID uuid.UUID `json:"upload_id"`
}

// TaskCreator abstracts Cloud Tasks client for testing. Matches the shape used
// by internal/email/sender_queue.go.
type TaskCreator interface {
	CreateTask(ctx context.Context, req *taskspb.CreateTaskRequest, opts ...interface{}) (*taskspb.Task, error)
}

// Enqueuer schedules a Cloud Task that hits the verify endpoint.
type Enqueuer struct {
	client    *cloudtasks.Client
	queue     string
	targetURL string
	audience  string
	saEmail   string
	logger    *slog.Logger
}

// NewEnqueuer constructs an Enqueuer pointed at the backend's verify endpoint.
// Matches the wiring shape used by email.NewQueueSender.
func NewEnqueuer(client *cloudtasks.Client, queue, backendURL, saEmail string, logger *slog.Logger) *Enqueuer {
	return &Enqueuer{
		client:    client,
		queue:     queue,
		targetURL: backendURL + "/api/tasks/verify-upload",
		audience:  backendURL,
		saEmail:   saEmail,
		logger:    logger,
	}
}

// Enqueue schedules a verification job. Failures here are logged but should
// not roll back the upload — verification is advisory and can be retried.
func (e *Enqueuer) Enqueue(ctx context.Context, uploadID uuid.UUID) error {
	body, err := json.Marshal(VerifyTaskBody{UploadID: uploadID})
	if err != nil {
		return fmt.Errorf("marshal verify task: %w", err)
	}
	req := &taskspb.CreateTaskRequest{
		Parent: e.queue,
		Task: &taskspb.Task{
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        e.targetURL,
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       body,
					AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
						OidcToken: &taskspb.OidcToken{
							ServiceAccountEmail: e.saEmail,
							Audience:            e.audience,
						},
					},
				},
			},
		},
	}
	task, err := e.client.CreateTask(ctx, req)
	if err != nil {
		return fmt.Errorf("create verify task: %w", err)
	}
	e.logger.InfoContext(ctx, "verification task enqueued", "upload_id", uploadID, "task", task.Name)
	return nil
}

// uploadContext bundles everything the worker needs from the DB to perform a
// verification: storage location, content type, criteria, tenant/user scope,
// and assignee identity (name + email) so the AI can check criteria like
// "verify the name on the cert matches the assignee."
type uploadContext struct {
	UploadID      uuid.UUID
	TenantID      uuid.UUID
	UserID        uuid.UUID
	StorageKey    string
	ContentType   string
	Criteria      string
	AssigneeName  string
	AssigneeEmail string
}

// Worker carries the dependencies for processing one verification job.
type Worker struct {
	db       database.DB
	store    *Store
	verifier *Verifier
	storage  storage.StorageBackend
	usage    *aiusage.Store
	logger   *slog.Logger
}

func NewWorker(db database.DB, store *Store, verifier *Verifier, storageBackend storage.StorageBackend, usage *aiusage.Store, logger *slog.Logger) *Worker {
	return &Worker{
		db:       db,
		store:    store,
		verifier: verifier,
		storage:  storageBackend,
		usage:    usage,
		logger:   logger,
	}
}

// loadContext fetches the upload + requirement + completion in one query so
// the worker has everything it needs without round-tripping through three
// packages. Reads criteria from task_requirements (the canonical source) —
// the criteria_snapshot on the verification row is the snapshot at enqueue
// time, used for audit display, not for the AI call.
func (w *Worker) loadContext(ctx context.Context, uploadID uuid.UUID) (*uploadContext, error) {
	c := &uploadContext{UploadID: uploadID}
	err := w.db.QueryRow(ctx, "verification.loadContext",
		`SELECT u.storage_key, u.content_type,
		        comp.user_id, t.tenant_id,
		        r.ai_verification_criteria,
		        i.name, i.email
		 FROM task_uploads u
		 JOIN task_completions comp ON comp.id = u.completion_id
		 JOIN tasks t ON t.id = comp.task_id
		 JOIN task_requirements r ON r.id = u.requirement_id
		 JOIN users usr ON usr.id = comp.user_id
		 JOIN identities i ON i.id = usr.identity_id
		 WHERE u.id = $1`,
		uploadID,
	).Scan(&c.StorageKey, &c.ContentType, &c.UserID, &c.TenantID, &c.Criteria,
		&c.AssigneeName, &c.AssigneeEmail)
	if err != nil {
		return nil, fmt.Errorf("load upload context: %w", err)
	}
	if c.Criteria == "" {
		return nil, fmt.Errorf("upload %s has no verification criteria", uploadID)
	}
	return c, nil
}

// Process is the core verification routine: idempotent, safe to call from
// either the Cloud Tasks HTTP handler or the LocalEnqueuer goroutine. Returns
// nil on success or for skipped/already-processed cases; returns an error
// only on infrastructural failure (DB unreachable, etc.) — AI-call failures
// are persisted to the verification row and reported as nil so the caller
// returns 200 to Cloud Tasks (no retry).
func (w *Worker) Process(ctx context.Context, uploadID uuid.UUID) error {
	existing, err := w.store.GetByUpload(ctx, uploadID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			w.logger.WarnContext(ctx, "verify for missing record, skipping", "upload_id", uploadID)
			return nil
		}
		return fmt.Errorf("load record: %w", err)
	}
	if existing.Status != StatusPending {
		w.logger.InfoContext(ctx, "verify already finalized, skipping",
			"upload_id", uploadID, "status", existing.Status)
		return nil
	}

	uc, err := w.loadContext(ctx, uploadID)
	if err != nil {
		w.logger.ErrorContext(ctx, "verify load context", "upload_id", uploadID, "error", err)
		_ = w.store.MarkFailed(context.Background(), uploadID, "load_context: "+err.Error())
		return nil
	}

	reader, err := w.storage.Download(ctx, uc.StorageKey)
	if err != nil {
		w.logger.ErrorContext(ctx, "verify download", "upload_id", uploadID, "error", err)
		_ = w.store.MarkFailed(context.Background(), uploadID, "download: "+err.Error())
		return nil
	}
	defer func() { _ = reader.Close() }()

	aiCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	result, err := w.verifier.Verify(aiCtx, reader, uc.ContentType, uc.Criteria, Assignee{
		Name:  uc.AssigneeName,
		Email: uc.AssigneeEmail,
	})
	if err != nil {
		w.logger.ErrorContext(ctx, "verify AI call", "upload_id", uploadID, "error", err)
		_ = w.store.MarkFailed(context.Background(), uploadID, "ai_call: "+err.Error())
		if result.Model != "" {
			_ = w.usage.Record(context.Background(), uc.TenantID, uc.UserID, AIFeature, result.PromptTokens, result.ResponseTokens)
		}
		return nil
	}

	if err := w.store.MarkSucceeded(ctx, uploadID, result.Model, result.Verdict); err != nil {
		return fmt.Errorf("persist verdict: %w", err)
	}

	if err := w.usage.Record(ctx, uc.TenantID, uc.UserID, AIFeature, result.PromptTokens, result.ResponseTokens); err != nil {
		w.logger.WarnContext(ctx, "verify record ai usage", "upload_id", uploadID, "error", err)
	}
	telemetry.RecordAITokens(ctx, AIFeature, result.PromptTokens, result.ResponseTokens)
	return nil
}

// HandleVerifyTask is the Cloud Tasks callback. Auth is handled by the route
// group (auth.RequireCloudTasks) — we trust the upload_id in the body to
// come from a legitimate enqueue.
func (w *Worker) HandleVerifyTask(c *gin.Context) {
	var body VerifyTaskBody
	if err := c.ShouldBindJSON(&body); err != nil || body.UploadID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := w.Process(c.Request.Context(), body.UploadID); err != nil {
		w.logger.ErrorContext(c.Request.Context(), "verify process", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// HandleFeedback handles POST /api/v1/admin/tasks/uploads/:upload_id/verification/feedback.
// Admins use this to mark an AI verdict as correct or incorrect — the signal
// is persisted on the verification row. Worker hosts this because it already
// has store + logger; the route group enforces session + tenant + role.
func (w *Worker) HandleFeedback(c *gin.Context) {
	sess, ok := auth.GetSession(c)
	if !ok || sess.TenantID == nil || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	uploadID, err := uuid.Parse(c.Param("upload_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload id"})
		return
	}

	var body struct {
		Feedback string `json:"feedback"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if body.Feedback != FeedbackCorrect && body.Feedback != FeedbackIncorrect {
		c.JSON(http.StatusBadRequest, gin.H{"error": "feedback must be 'correct' or 'incorrect'"})
		return
	}

	// Cross-tenant scope check: the verification's tenant must match the
	// caller's session tenant. Without this, an admin in tenant A could
	// post feedback against an upload in tenant B by guessing UUIDs.
	rec, err := w.store.GetByUpload(c.Request.Context(), uploadID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "verification not found"})
			return
		}
		w.logger.ErrorContext(c.Request.Context(), "feedback get verification", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if rec.TenantID != *sess.TenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "verification not found"})
		return
	}

	if err := w.store.RecordFeedback(c.Request.Context(), uploadID, *sess.UserID, body.Feedback); err != nil {
		w.logger.ErrorContext(c.Request.Context(), "record feedback", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}
