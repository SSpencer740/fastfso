package actionitem

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/auth"
	"github.com/SSpencer740/fastfso/backend/internal/httputil"
)

type auditLogger interface {
	Log(ctx context.Context, p audit.LogParams)
}

type notifier interface {
	NotifyActionItemAssigned(ctx context.Context, userID uuid.UUID, to, name, sourceType string)
}

// SourceUpdater propagates a status change from an action item to the linked source entity.
// status is the mapped entity status (e.g. "approved", "rejected", "under_review").
type SourceUpdater func(ctx context.Context, tenantID, sourceID, reviewerID uuid.UUID, status string) error

// Handler exposes HTTP endpoints for action items.
type Handler struct {
	store          *Store
	audit          auditLogger
	notify         notifier
	logger         *slog.Logger
	sourceUpdaters map[string]SourceUpdater
}

// NewHandler creates a new action item Handler.
func NewHandler(store *Store, auditLog auditLogger, n notifier, logger *slog.Logger, sourceUpdaters map[string]SourceUpdater) *Handler {
	return &Handler{store: store, audit: auditLog, notify: n, logger: logger, sourceUpdaters: sourceUpdaters}
}

// List handles GET / for the action items list.
func (h *Handler) List(c *gin.Context) {
	sess, exists := auth.GetSession(c)
	if !exists || sess.TenantID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}
	tenantID := *sess.TenantID

	limit, offset := httputil.ParsePagination(c, 20)

	items, total, err := h.store.List(c.Request.Context(), ListFilters{
		TenantID:    tenantID,
		SourceType:  c.Query("source_type"),
		Status:      c.Query("status"),
		Search:      c.Query("search"),
		SubOrgScope: auth.SubOrgScope(c),
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		h.logger.Error("failed to list action items", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

// GetStats handles GET /stats for the action items dashboard.
func (h *Handler) GetStats(c *gin.Context) {
	sess, exists := auth.GetSession(c)
	if !exists || sess.TenantID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	stats, err := h.store.SummaryStats(c.Request.Context(), *sess.TenantID, auth.SubOrgScope(c))
	if err != nil {
		h.logger.Error("failed to get action item stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// Get handles GET /:id for a single action item.
func (h *Handler) Get(c *gin.Context) {
	sess, exists := auth.GetSession(c)
	if !exists || sess.TenantID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}
	tenantID := *sess.TenantID

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action item ID"})
		return
	}

	item, err := h.store.Get(c.Request.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "action item not found"})
			return
		}
		h.logger.Error("failed to get action item", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// UpdateStatus handles POST /:id/status for changing an action item's status.
func (h *Handler) UpdateStatus(c *gin.Context) {
	sess, exists := auth.GetSession(c)
	if !exists || sess.TenantID == nil || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}
	tenantID := *sess.TenantID
	userID := *sess.UserID

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action item ID"})
		return
	}

	var req struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()

	// Fetch the item before updating so we have source_type and source_id for cascade.
	item, err := h.store.Get(ctx, tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "action item not found"})
			return
		}
		h.logger.Error("failed to get action item", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.store.UpdateStatus(ctx, tenantID, id, req.Status, req.Notes); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "action item not found"})
			return
		}
		h.logger.Error("failed to update action item status", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Cascade the status change to the linked source entity.
	sourceStatus := ""
	switch req.Status {
	case "under_review":
		sourceStatus = "under_review"
	case "processed":
		sourceStatus = "approved"
	case "rejected":
		sourceStatus = "rejected"
	}
	if sourceStatus != "" && item.SourceID != nil {
		if updater, ok := h.sourceUpdaters[item.SourceType]; ok {
			if err := updater(ctx, tenantID, *item.SourceID, userID, sourceStatus); err != nil {
				h.logger.Error("failed to cascade status to source entity",
					"error", err, "source_type", item.SourceType, "source_id", item.SourceID)
			}
		}
	}

	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionActionItemUpdated,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"action_item_id": id, "status": req.Status},
	})

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// UpdateNotes handles POST /:id/notes for saving notes without changing status.
func (h *Handler) UpdateNotes(c *gin.Context) {
	sess, exists := auth.GetSession(c)
	if !exists || sess.TenantID == nil || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}
	tenantID := *sess.TenantID

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action item ID"})
		return
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := h.store.UpdateNotes(c.Request.Context(), tenantID, id, req.Notes); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "action item not found"})
			return
		}
		h.logger.Error("failed to update action item notes", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionActionItemNotesUpdated,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"action_item_id": id},
	})

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// Reassign handles PATCH /:id/assignee for changing the assigned FSO.
func (h *Handler) Reassign(c *gin.Context) {
	sess, exists := auth.GetSession(c)
	if !exists || sess.TenantID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}
	tenantID := *sess.TenantID

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action item ID"})
		return
	}

	var req struct {
		UserID *string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var userID *uuid.UUID
	if req.UserID != nil && *req.UserID != "" {
		parsed, err := uuid.Parse(*req.UserID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
			return
		}
		userID = &parsed
	}

	ctx := c.Request.Context()
	ai, err := h.store.UpdateAssignedTo(ctx, tenantID, id, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "action item not found"})
			return
		}
		if errors.Is(err, ErrAssigneeWrongTenant) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid assignee"})
			return
		}
		h.logger.Error("failed to reassign action item", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if ai.AssignedTo != nil && ai.AssigneeEmail != nil && ai.AssigneeName != nil {
		h.notify.NotifyActionItemAssigned(ctx, *ai.AssignedTo, *ai.AssigneeEmail, *ai.AssigneeName, ai.SourceType)
	}

	meta := map[string]any{"action_item_id": id}
	if userID != nil {
		meta["assigned_to"] = *userID
	} else {
		meta["assigned_to"] = nil
	}
	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionActionItemReassigned,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   meta,
	})

	c.JSON(http.StatusOK, ai)
}

// ListFSOUsers handles GET /fso-users for the assignee picker.
func (h *Handler) ListFSOUsers(c *gin.Context) {
	sess, exists := auth.GetSession(c)
	if !exists || sess.TenantID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	users, err := h.store.ListFSOUsers(c.Request.Context(), *sess.TenantID)
	if err != nil {
		h.logger.Error("failed to list fso users", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if users == nil {
		users = []FSOUser{}
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}
