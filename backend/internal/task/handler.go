package task

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/actionitem"
	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/auth"
	"github.com/SSpencer740/fastfso/backend/internal/httputil"
	"github.com/SSpencer740/fastfso/backend/internal/scan"
	"github.com/SSpencer740/fastfso/backend/internal/storage"
	"github.com/SSpencer740/fastfso/backend/internal/verification"
)

// verificationStore is the subset of verification.Store needed by the task
// handler. Import flow task → verification is safe (verification doesn't
// import task).
type verificationStore interface {
	CreatePending(ctx context.Context, uploadID, tenantID uuid.UUID, criteria string) (uuid.UUID, error)
	ListByCompletion(ctx context.Context, completionID uuid.UUID) (map[uuid.UUID]*verification.Record, error)
}

// verificationEnqueuer schedules verification Cloud Tasks.
type verificationEnqueuer interface {
	Enqueue(ctx context.Context, uploadID uuid.UUID) error
}

type auditLogger interface {
	Log(ctx context.Context, p audit.LogParams)
}

type notifier interface {
	NotifyActionItemAssigned(ctx context.Context, userID uuid.UUID, to, name, sourceType string)
	NotifyTaskAssigned(ctx context.Context, userID uuid.UUID, to, name string)
	NotifyTaskReviewed(ctx context.Context, userID uuid.UUID, to, name, decision string)
}

// Handler exposes HTTP endpoints for the task system.
type Handler struct {
	store        *Store
	actionItems  *actionitem.Store
	audit        auditLogger
	notify       notifier
	storage      storage.StorageBackend
	scanner      scan.Scanner
	verification verificationStore
	verifyQueue  verificationEnqueuer
	logger       *slog.Logger
}

// NewHandler creates a new task Handler. verification and verifyQueue may be
// nil when AI verification is not configured for the environment (e.g. local
// dev without Cloud Tasks) — UploadFile will skip verification gracefully.
func NewHandler(store *Store, actionItems *actionitem.Store, auditLog auditLogger, n notifier, sb storage.StorageBackend, scanner scan.Scanner, vs verificationStore, vq verificationEnqueuer, logger *slog.Logger) *Handler {
	return &Handler{
		store:        store,
		actionItems:  actionItems,
		audit:        auditLog,
		notify:       n,
		storage:      sb,
		scanner:      scanner,
		verification: vs,
		verifyQueue:  vq,
		logger:       logger,
	}
}

// sessionInfo extracts identity, tenant, and user IDs from the Gin context.
func sessionInfo(c *gin.Context) (identityID, tenantID, userID uuid.UUID, ok bool) {
	sess, exists := auth.GetSession(c)
	if !exists || sess.TenantID == nil || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return sess.IdentityID, *sess.TenantID, *sess.UserID, true
}

// --- Admin handlers ---

// validateRulesForFSO ensures every assignment rule submitted by a non-admin
// stays within the requester's sub-org. Returns a sanitized error message
// safe for the response body.
func (h *Handler) validateRulesForFSO(ctx context.Context, rules []CreateRuleParams, sessSubOrg *uuid.UUID) error {
	if sessSubOrg == nil {
		// An FSO with no sub-org assignment has no one to assign to.
		if len(rules) > 0 {
			return fmt.Errorf("you must belong to a sub-organization to assign tasks")
		}
		return nil
	}
	var userTargets []uuid.UUID
	for _, r := range rules {
		switch r.RuleType {
		case "org":
			return fmt.Errorf("only administrators may assign tasks tenant-wide")
		case "sub_org":
			if r.TargetID == nil || *r.TargetID != *sessSubOrg {
				return fmt.Errorf("you may only assign tasks within your own sub-organization")
			}
		case "user":
			if r.TargetID == nil {
				return fmt.Errorf("user assignment rule missing target_id")
			}
			userTargets = append(userTargets, *r.TargetID)
		default:
			return fmt.Errorf("invalid rule_type: %q", r.RuleType)
		}
	}
	if len(userTargets) > 0 {
		inSubOrg, err := h.store.UsersInSubOrg(ctx, userTargets, *sessSubOrg)
		if err != nil {
			h.logger.Error("validate rules: users in sub-org lookup", "error", err)
			return fmt.Errorf("internal error validating assignment")
		}
		for _, id := range userTargets {
			if !inSubOrg[id] {
				return fmt.Errorf("you may only assign tasks to users in your own sub-organization")
			}
		}
	}
	return nil
}

// CreateTask handles POST / for task creation.
func (h *Handler) CreateTask(c *gin.Context) {
	identityID, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	var req CreateParams
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	sess, _ := auth.GetSession(c)
	req.TenantID = tenantID
	req.CreatedByUserID = userID
	// Admins may specify a sub_org_id in the body to scope the task to a sub-org.
	// FSO/IC always inherit their session sub_org_id (prevents spoofing).
	isAdmin := sess.UserRole != nil && *sess.UserRole == "administrator"
	if !isAdmin {
		req.SubOrgID = sess.SubOrgID
		// Validate assignment rules. Without this, an FSO could send rules
		// that target outside their sub-org even though the task itself is
		// stamped with their sub-org.
		if err := h.validateRulesForFSO(c.Request.Context(), req.Rules, sess.SubOrgID); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
	}
	if req.Status == "" {
		req.Status = "active"
	}

	t, err := h.store.Create(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("failed to create task", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	if t.Status == "active" {
		assignees, listErr := h.store.ListAssigneeContacts(c.Request.Context(), t.ID)
		if listErr != nil {
			h.logger.Warn("list assignee contacts for new task", "error", listErr)
		}
		for _, a := range assignees {
			h.notify.NotifyTaskAssigned(c.Request.Context(), a.UserID, a.Email, a.Name)
		}
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTaskCreated,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"task_id": t.ID},
	})

	c.JSON(http.StatusCreated, t)
}

// ListTasks handles GET / for the admin task list.
func (h *Handler) ListTasks(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	limit, offset := httputil.ParsePagination(c, 20)

	tasks, total, err := h.store.List(c.Request.Context(), ListFilters{
		TenantID:    tenantID,
		Status:      c.Query("status"),
		Priority:    c.Query("priority"),
		Search:      c.Query("search"),
		SubOrgScope: auth.SubOrgScope(c),
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		h.logger.Error("failed to list tasks", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tasks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tasks": tasks, "total": total})
}

// parseDateRange extracts optional ?from= and ?to= query params (YYYY-MM-DD).
func parseDateRange(c *gin.Context) (from, to *time.Time) {
	if s := c.Query("from"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			from = &t
		}
	}
	if s := c.Query("to"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			end := t.Add(24*time.Hour - time.Nanosecond) // inclusive end of day
			to = &end
		}
	}
	return
}

// GetTaskStats handles GET /stats for the admin dashboard.
func (h *Handler) GetTaskStats(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	from, to := parseDateRange(c)
	stats, err := h.store.SummaryStats(c.Request.Context(), tenantID, from, to, auth.SubOrgScope(c))
	if err != nil {
		h.logger.Error("failed to get task stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetTask handles GET /:id for the admin detail view.
func (h *Handler) GetTask(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
		return
	}

	detail, err := h.store.Get(c.Request.Context(), tenantID, taskID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		h.logger.Error("failed to get task", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, detail)
}

// ArchiveTask handles POST /:id/archive.
func (h *Handler) ArchiveTask(c *gin.Context) {
	identityID, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
		return
	}

	if err := h.store.Archive(c.Request.Context(), tenantID, taskID); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		h.logger.Error("failed to archive task", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	sess, _ := auth.GetSession(c)
	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTaskArchived,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"task_id": taskID},
	})

	c.JSON(http.StatusOK, gin.H{"status": "archived"})
}

// UnarchiveTask handles POST /:id/unarchive.
func (h *Handler) UnarchiveTask(c *gin.Context) {
	identityID, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
		return
	}

	if err := h.store.Unarchive(c.Request.Context(), tenantID, taskID); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		h.logger.Error("failed to unarchive task", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	sess, _ := auth.GetSession(c)
	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTaskArchived,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"task_id": taskID, "action": "unarchive"},
	})

	c.JSON(http.StatusOK, gin.H{"status": "active"})
}

// ApproveSubmission handles POST /:id/assignees/:user_id/approve.
func (h *Handler) ApproveSubmission(c *gin.Context) {
	identityID, tenantID, reviewerUserID, ok := sessionInfo(c)
	if !ok {
		return
	}

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
		return
	}
	assigneeUserID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	comp, err := h.store.EnsureCompletion(c.Request.Context(), tenantID, taskID, assigneeUserID)
	if err != nil {
		h.logger.Error("failed to ensure completion", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.store.Approve(c.Request.Context(), comp.ID, reviewerUserID); err != nil {
		h.logger.Error("failed to approve submission", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.actionItems.ResolveBySource(c.Request.Context(), tenantID, "task_submission", comp.ID); err != nil {
		h.logger.Error("failed to resolve action item for task approval", "error", err)
	}

	if email, name, err := h.store.GetUserContact(c.Request.Context(), tenantID, assigneeUserID); err == nil && email != "" {
		h.notify.NotifyTaskReviewed(c.Request.Context(), assigneeUserID, email, name, "approved")
	}

	sess, _ := auth.GetSession(c)
	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTaskApproved,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"task_id": taskID, "assignee_user_id": assigneeUserID},
	})

	c.JSON(http.StatusOK, gin.H{"status": "approved"})
}

// RejectSubmission handles POST /:id/assignees/:user_id/reject.
func (h *Handler) RejectSubmission(c *gin.Context) {
	identityID, tenantID, reviewerUserID, ok := sessionInfo(c)
	if !ok {
		return
	}

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
		return
	}
	assigneeUserID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	comp, err := h.store.EnsureCompletion(c.Request.Context(), tenantID, taskID, assigneeUserID)
	if err != nil {
		h.logger.Error("failed to ensure completion", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.store.Reject(c.Request.Context(), comp.ID, reviewerUserID); err != nil {
		h.logger.Error("failed to reject submission", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.actionItems.ResolveBySource(c.Request.Context(), tenantID, "task_submission", comp.ID); err != nil {
		h.logger.Error("failed to resolve action item for task rejection", "error", err)
	}

	if email, name, err := h.store.GetUserContact(c.Request.Context(), tenantID, assigneeUserID); err == nil && email != "" {
		h.notify.NotifyTaskReviewed(c.Request.Context(), assigneeUserID, email, name, "rejected")
	}

	sess, _ := auth.GetSession(c)
	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTaskRejected,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"task_id": taskID, "assignee_user_id": assigneeUserID},
	})

	c.JSON(http.StatusOK, gin.H{"status": "rejected"})
}

// GetSubmission handles GET /completions/:completion_id for admin submission review.
func (h *Handler) GetSubmission(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	completionID, err := uuid.Parse(c.Param("completion_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid completion ID"})
		return
	}

	sub, err := h.store.AdminGetCompletion(c.Request.Context(), tenantID, completionID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "submission not found"})
			return
		}
		h.logger.Error("failed to get submission", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	sub.Verifications = h.fetchVerifications(c.Request.Context(), completionID)
	c.JSON(http.StatusOK, sub)
}

// fetchVerifications returns a string-keyed map (upload_id -> record) ready
// for JSON serialization. Always returns a non-nil map so the JSON shape is
// stable. Failures are logged but do not block the response — verification
// status is advisory.
func (h *Handler) fetchVerifications(ctx context.Context, completionID uuid.UUID) map[string]*verification.Record {
	out := map[string]*verification.Record{}
	if h.verification == nil {
		return out
	}
	verifications, err := h.verification.ListByCompletion(ctx, completionID)
	if err != nil {
		h.logger.Warn("failed to list verifications", "completion_id", completionID, "error", err)
		return out
	}
	for uploadID, rec := range verifications {
		out[uploadID.String()] = rec
	}
	return out
}

// ListUsers handles GET /users for the user picker. Non-admins are scoped to
// users in their session sub-org.
func (h *Handler) ListUsers(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	sess, _ := auth.GetSession(c)
	var subOrgScope *uuid.UUID
	if sess.UserRole == nil || *sess.UserRole != "administrator" {
		subOrgScope = sess.SubOrgID
		if subOrgScope == nil {
			// FSO with no sub-org gets an empty list rather than every user.
			c.JSON(http.StatusOK, gin.H{"users": []TenantUser{}})
			return
		}
	}

	users, err := h.store.ListTenantUsers(c.Request.Context(), tenantID, c.Query("search"), subOrgScope)
	if err != nil {
		h.logger.Error("failed to list users", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

// ListSubOrgs handles GET /sub-orgs for the sub-org picker. Non-admins only
// see their own sub-org (or nothing if they have no sub-org assignment).
func (h *Handler) ListSubOrgs(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	sess, _ := auth.GetSession(c)
	orgs, err := h.store.ListSubOrgs(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("failed to list sub-orgs", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if sess.UserRole == nil || *sess.UserRole != "administrator" {
		filtered := orgs[:0]
		for _, o := range orgs {
			if sess.SubOrgID != nil && o.ID == *sess.SubOrgID {
				filtered = append(filtered, o)
			}
		}
		orgs = filtered
	}
	c.JSON(http.StatusOK, gin.H{"sub_orgs": orgs})
}

// --- IC handlers ---

// ListMyTasks handles GET / for the IC task list.
func (h *Handler) ListMyTasks(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	tasks, err := h.store.ListMyTasks(c.Request.Context(), tenantID, userID, c.Query("search"), 0)
	if err != nil {
		h.logger.Error("failed to list my tasks", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

// GetMyTaskStats handles GET /stats for the IC dashboard.
func (h *Handler) GetMyTaskStats(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	from, to := parseDateRange(c)
	stats, err := h.store.MySummaryStats(c.Request.Context(), tenantID, userID, from, to)
	if err != nil {
		h.logger.Error("failed to get my task stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetMyTask handles GET /:id for the IC detail view.
func (h *Handler) GetMyTask(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
		return
	}

	detail, err := h.store.GetMyTask(c.Request.Context(), tenantID, taskID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		h.logger.Error("failed to get my task", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if detail.Completion.ID != uuid.Nil {
		detail.Verifications = h.fetchVerifications(c.Request.Context(), detail.Completion.ID)
	} else {
		detail.Verifications = map[string]*verification.Record{}
	}
	c.JSON(http.StatusOK, detail)
}

// SaveResponses handles POST /:id/responses for saving draft answers.
func (h *Handler) SaveResponses(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
		return
	}

	var req struct {
		Responses []SaveResponseParams `json:"responses"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	comp, err := h.store.EnsureCompletion(c.Request.Context(), tenantID, taskID, userID)
	if err != nil {
		h.logger.Error("failed to ensure completion", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.store.SaveResponses(c.Request.Context(), comp.ID, req.Responses); err != nil {
		h.logger.Error("failed to save responses", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "saved"})
}

// SubmitTask handles POST /:id/submit for submitting a completed task.
func (h *Handler) SubmitTask(c *gin.Context) {
	identityID, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
		return
	}

	comp, err := h.store.EnsureCompletion(c.Request.Context(), tenantID, taskID, userID)
	if err != nil {
		h.logger.Error("failed to ensure completion", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.store.Submit(c.Request.Context(), comp.ID); err != nil {
		h.logger.Error("failed to submit task", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create action item for FSO review
	taskTitle, userName, err := h.store.GetSubmissionContext(c.Request.Context(), taskID, userID, tenantID)
	if err != nil {
		h.logger.Error("failed to get submission context for action item", "error", err)
		taskTitle, userName = "Unknown Task", "Unknown User"
	}
	sess, _ := auth.GetSession(c)
	ai, aiErr := h.actionItems.Create(c.Request.Context(), actionitem.CreateParams{
		TenantID:    tenantID,
		SourceType:  "task_submission",
		SourceID:    &comp.ID,
		Title:       fmt.Sprintf("Review task submission: %s — %s", taskTitle, userName),
		Description: fmt.Sprintf("%s has submitted the task \"%s\" for review.", userName, taskTitle),
		Priority:    "medium",
		SubOrgID:    sess.SubOrgID,
	})
	if aiErr != nil {
		h.logger.Error("failed to create action item for task submission", "error", aiErr)
	} else if ai != nil {
		for _, t := range h.actionItems.ResolveNotifyTargets(c.Request.Context(), tenantID, ai) {
			tUserID, _ := uuid.Parse(t.UserID)
			h.notify.NotifyActionItemAssigned(c.Request.Context(), tUserID, t.Email, t.Name, ai.SourceType)
		}
	}
	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTaskSubmitted,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"task_id": taskID},
	})

	c.JSON(http.StatusOK, gin.H{"status": "submitted"})
}

// UploadFile handles POST /:id/upload/:requirement_id for file uploads.
func (h *Handler) UploadFile(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
		return
	}
	reqID, err := uuid.Parse(c.Param("requirement_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid requirement ID"})
		return
	}

	// Load the requirement so we know whether AI verification is enabled
	// and (if so) what content types we accept.
	requirement, err := h.store.GetRequirement(c.Request.Context(), reqID)
	if err != nil {
		h.logger.Error("failed to load requirement", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid requirement"})
		return
	}
	aiEnabled := requirement.AIVerificationCriteria != ""

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, httputil.MaxUploadSize)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required or exceeds size limit"})
		return
	}
	defer func() { _ = file.Close() }()

	safeName := filepath.Base(header.Filename)
	if safeName == "." || safeName == "/" || safeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
		return
	}
	contentType, err := httputil.SniffContentType(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}

	// When AI verification is enabled on this requirement, we restrict
	// uploads to types the verifier can actually process. This guarantees
	// "no AI flag" reliably means "AI checked it" — not "AI silently skipped
	// an unsupported file type." See verification.SupportedContentTypes.
	if aiEnabled && !isAISupportedContentType(contentType) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "this task requires PDF, JPG, PNG, or WEBP because AI verification is enabled",
		})
		return
	}
	// Same rationale for size: Vertex AI inline-data limit is 20 MB; cap
	// at 18 MB so the verification job doesn't hit the API limit. The
	// global MaxUploadSize (50 MB) still applies to non-AI uploads.
	if aiEnabled && header.Size > maxAIVerifyFileSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "AI-verified uploads must be 18 MB or smaller",
		})
		return
	}

	if err := httputil.ScanAndRewind(c.Request.Context(), h.scanner, file); err != nil {
		var infected *scan.InfectedError
		if errors.As(err, &infected) {
			h.logger.Warn("rejected infected upload", "signature", infected.Signature, "filename", safeName)
			c.JSON(http.StatusBadRequest, gin.H{"error": "file rejected by malware scanner"})
			return
		}
		h.logger.Error("malware scan failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "malware scanner unavailable, please try again"})
		return
	}

	comp, err := h.store.EnsureCompletion(c.Request.Context(), tenantID, taskID, userID)
	if err != nil {
		h.logger.Error("failed to ensure completion", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	key := fmt.Sprintf("%s/%s/%s", storage.KeyPrefix(tenantID, taskID), comp.ID, safeName)
	if err := h.storage.Upload(c.Request.Context(), key, file, contentType); err != nil {
		h.logger.Error("failed to upload file", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
		return
	}

	upload := &Upload{
		CompletionID:  comp.ID,
		RequirementID: reqID,
		FileName:      safeName,
		FileSize:      header.Size,
		ContentType:   contentType,
		StorageKey:    key,
	}
	if err := h.store.CreateUpload(c.Request.Context(), upload); err != nil {
		h.logger.Error("failed to record upload", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Kick off AI verification if the requirement asked for it. Failures
	// here are logged but do not roll back the upload — verification is
	// advisory, and an admin can manually re-run later if needed.
	if aiEnabled && h.verification != nil && h.verifyQueue != nil {
		if _, err := h.verification.CreatePending(c.Request.Context(), upload.ID, tenantID, requirement.AIVerificationCriteria); err != nil {
			h.logger.Error("failed to create pending verification", "upload_id", upload.ID, "error", err)
		} else if err := h.verifyQueue.Enqueue(c.Request.Context(), upload.ID); err != nil {
			h.logger.Error("failed to enqueue verification task", "upload_id", upload.ID, "error", err)
		}
	}

	c.JSON(http.StatusCreated, upload)
}

// isAISupportedContentType mirrors verification.SupportedContentTypes without
// importing the verification package directly — the task package only needs
// to gate at the boundary; the verifier owns the canonical list.
func isAISupportedContentType(t string) bool {
	switch t {
	case "application/pdf", "image/png", "image/jpeg", "image/webp":
		return true
	}
	return false
}

// maxAIVerifyFileSize mirrors verification.MaxAIVerifyFileSize. Kept as a
// local copy so the task handler doesn't import the verification package
// just to compare an int.
const maxAIVerifyFileSize int64 = 18 << 20 // 18 MB

// DownloadUpload handles GET /uploads/:upload_id/download — generates a signed URL
// and redirects the browser directly to the file (works for both GCS and local).
func (h *Handler) DownloadUpload(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	uploadID, err := uuid.Parse(c.Param("upload_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload ID"})
		return
	}

	key, fileName, _, err := h.store.GetUploadForDownload(c.Request.Context(), tenantID, uploadID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
			return
		}
		h.logger.Error("failed to get upload", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	url, err := h.storage.SignedURL(c.Request.Context(), key, 15*time.Minute)
	if err != nil {
		h.logger.Error("failed to sign URL", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	_ = fileName // available for Content-Disposition if needed in future
	c.Redirect(http.StatusFound, url)
}

// ServeLocalFile handles GET /files/*key — streams files from local storage in dev.
func (h *Handler) ServeLocalFile(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	key := c.Param("key")[1:] // strip leading slash

	// Storage keys are prefixed with "tenants/<tenant_id>/..." (see
	// storage.KeyPrefix). Reject any request whose key doesn't belong to
	// the caller's tenant — without this, a logged-in user from tenant A
	// could fetch tenant B's files by guessing the path. 404 (not 403) so
	// we don't confirm the file's existence to cross-tenant probes.
	keyTenant, ok := tenantFromKey(key)
	if !ok || keyTenant != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	rc, err := h.storage.Download(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	defer func() { _ = rc.Close() }()

	// Set Content-Type explicitly. Without it, c.Status() writes the response
	// headers before any auto-detection from the first bytes could happen,
	// and the browser doesn't know it's a PDF/image — so <embed> can't
	// render. Map common extensions; fall back to octet-stream.
	c.Header("Content-Type", contentTypeForKey(key))
	// `inline` lets browsers render PDFs/images directly in <iframe>/<embed>
	// tags and new tabs instead of forcing a save dialog. Users can still
	// save via right-click. The filename is preserved for save-as.
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filepath.Base(key)))
	// Override the global SecurityHeaders middleware which sets
	// X-Frame-Options: DENY. Admin preview iframes are same-origin and
	// need framing allowed; SAMEORIGIN keeps clickjacking protection.
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("Cache-Control", "private, max-age=300")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}

// tenantFromKey extracts the tenant UUID encoded as the second path segment
// of a storage key (the first segment is the literal "tenants"). Returns
// false if the key doesn't have the expected shape or the UUID is malformed.
func tenantFromKey(key string) (uuid.UUID, bool) {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) < 2 || parts[0] != "tenants" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// contentTypeForKey returns a content type based on a storage key's file
// extension. Only covers types the upload path accepts; unknown extensions
// get octet-stream so browsers offer to save.
func contentTypeForKey(key string) string {
	switch strings.ToLower(filepath.Ext(key)) {
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	}
	return "application/octet-stream"
}

// DeleteUpload handles DELETE /:id/upload/:upload_id for removing an uploaded file.
func (h *Handler) DeleteUpload(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	uploadID, err := uuid.Parse(c.Param("upload_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload ID"})
		return
	}

	key, err := h.store.DeleteUpload(c.Request.Context(), tenantID, uploadID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
			return
		}
		h.logger.Error("failed to delete upload", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.storage.Delete(c.Request.Context(), key); err != nil {
		h.logger.Error("failed to delete file from storage", "error", err)
		// Record already deleted, so still return success
	}

	c.Status(http.StatusNoContent)
}
