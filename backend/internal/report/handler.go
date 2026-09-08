package report

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fastfso/fastfso/backend/internal/actionitem"
	"github.com/fastfso/fastfso/backend/internal/audit"
	"github.com/fastfso/fastfso/backend/internal/auth"
	"github.com/fastfso/fastfso/backend/internal/httputil"
)

type auditLogger interface {
	Log(ctx context.Context, p audit.LogParams)
}

type notifier interface {
	NotifyActionItemAssigned(ctx context.Context, userID uuid.UUID, to, name, sourceType string)
}

// Handler exposes HTTP endpoints for reports.
type Handler struct {
	store       *Store
	actionItems *actionitem.Store
	audit       auditLogger
	notify      notifier
	logger      *slog.Logger
}

// NewHandler creates a new report Handler.
func NewHandler(store *Store, actionItems *actionitem.Store, auditLog auditLogger, n notifier, logger *slog.Logger) *Handler {
	return &Handler{store: store, actionItems: actionItems, audit: auditLog, notify: n, logger: logger}
}

func sessionInfo(c *gin.Context) (identityID, tenantID, userID uuid.UUID, ok bool) {
	sess, exists := auth.GetSession(c)
	if !exists || sess.TenantID == nil || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return sess.IdentityID, *sess.TenantID, *sess.UserID, true
}

// --- IC handlers ---

// Create handles POST / for submitting a new report.
func (h *Handler) Create(c *gin.Context) {
	identityID, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	var req CreateParams
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.TenantID = tenantID
	req.CreatedByUserID = userID
	sess, _ := auth.GetSession(c)
	req.SubOrgID = sess.SubOrgID

	r, err := h.store.Create(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("failed to create report", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Create action item for FSO review.
	creatorName, err := h.store.GetCreatorName(c.Request.Context(), tenantID, userID)
	if err != nil {
		h.logger.Error("failed to get creator name for action item title", "error", err)
		creatorName = "Unknown"
	}
	title := "Process " + ReportTypeLabel(r.ReportType) + " for " + creatorName
	ai, aiErr := h.actionItems.Create(c.Request.Context(), actionitem.CreateParams{
		TenantID:    tenantID,
		SourceType:  "report",
		SourceID:    &r.ID,
		Title:       title,
		Description: "A life event report requires FSO review",
		Priority:    "medium",
		SubOrgID:    r.SubOrgID,
	})
	if aiErr != nil {
		h.logger.Error("failed to create action item for report", "error", aiErr)
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
		Action:     audit.ActionReportSubmitted,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"report_id": r.ID, "report_type": r.ReportType},
	})

	c.JSON(http.StatusCreated, r)
}

// ListMy handles GET / for the IC's own reports.
func (h *Handler) ListMy(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	limit, offset := httputil.ParsePagination(c, 20)

	reports, total, err := h.store.List(c.Request.Context(), ListFilters{
		TenantID: tenantID,
		UserID:   &userID,
		Status:   c.Query("status"),
		Search:   c.Query("search"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		h.logger.Error("failed to list reports", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if reports == nil {
		reports = []ReportRow{}
	}
	c.JSON(http.StatusOK, gin.H{"reports": reports, "total": total})
}

// MyStats handles GET /stats for the IC dashboard.
func (h *Handler) MyStats(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	stats, err := h.store.SummaryStats(c.Request.Context(), tenantID, &userID, nil)
	if err != nil {
		h.logger.Error("failed to get report stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetMy handles GET /:id for an IC viewing their own report.
func (h *Handler) GetMy(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report ID"})
		return
	}

	r, err := h.store.Get(c.Request.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		h.logger.Error("failed to get report", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if r.CreatedByUserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		return
	}

	c.JSON(http.StatusOK, r)
}

// --- Admin handlers ---

// AdminList handles GET / for the admin report list.
func (h *Handler) AdminList(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	limit, offset := httputil.ParsePagination(c, 20)

	reports, total, err := h.store.List(c.Request.Context(), ListFilters{
		TenantID:    tenantID,
		Status:      c.Query("status"),
		Search:      c.Query("search"),
		SubOrgScope: auth.SubOrgScope(c),
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		h.logger.Error("failed to list reports", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if reports == nil {
		reports = []ReportRow{}
	}
	c.JSON(http.StatusOK, gin.H{"reports": reports, "total": total})
}

// AdminStats handles GET /stats for the admin dashboard.
func (h *Handler) AdminStats(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	stats, err := h.store.SummaryStats(c.Request.Context(), tenantID, nil, auth.SubOrgScope(c))
	if err != nil {
		h.logger.Error("failed to get report stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// AdminGet handles GET /:id for the admin detail view.
func (h *Handler) AdminGet(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report ID"})
		return
	}

	r, err := h.store.Get(c.Request.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		h.logger.Error("failed to get report", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, r)
}

// AdminUpdateStatus handles POST /:id/status for updating a report's status.
func (h *Handler) AdminUpdateStatus(c *gin.Context) {
	identityID, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report ID"})
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()
	if err := h.store.UpdateStatus(ctx, tenantID, id, userID, req.Status); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		h.logger.Error("failed to update report status", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if req.Status == "processed" {
		if err := h.actionItems.ResolveBySource(ctx, tenantID, "report", id); err != nil {
			h.logger.Error("failed to resolve action items for report", "error", err)
		}
	}

	sess, _ := auth.GetSession(c)
	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionReportReviewed,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"report_id": id, "status": req.Status, "reviewer_id": identityID},
	})

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
