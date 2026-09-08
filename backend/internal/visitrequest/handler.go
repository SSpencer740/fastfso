package visitrequest

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

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

// dd254Access is the slice of the dd254 store we need to validate the IC's
// declared contract on submit. Defined as an interface so we keep the
// dependency direction one-way (visitrequest doesn't import dd254).
type dd254Access interface {
	UserHasAccess(ctx context.Context, tenantID, userID, dd254ID uuid.UUID) (bool, error)
}

// Handler exposes HTTP endpoints for visit requests.
type Handler struct {
	store       *Store
	actionItems *actionitem.Store
	audit       auditLogger
	notify      notifier
	dd254       dd254Access
	logger      *slog.Logger
}

// NewHandler creates a new visit request Handler.
func NewHandler(store *Store, actionItems *actionitem.Store, auditLog auditLogger, n notifier, dd dd254Access, logger *slog.Logger) *Handler {
	return &Handler{store: store, actionItems: actionItems, audit: auditLog, notify: n, dd254: dd, logger: logger}
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

// --- IC handlers ---

// Create handles POST / for submitting a new visit request.
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
	sess, _ := auth.GetSession(c)
	req.TenantID = tenantID
	req.CreatedByUserID = userID
	req.SubOrgID = sess.SubOrgID

	// If the IC declared a contract (DD254), confirm they're actually
	// read-on. Wizard pre-filters this list, so a miss here means either
	// API bypass or the access entry was revoked since the wizard loaded.
	if req.DD254ID != nil {
		ok, err := h.dd254.UserHasAccess(c.Request.Context(), tenantID, userID, *req.DD254ID)
		if err != nil {
			h.logger.Error("dd254 access check on visit create", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "you are not read-on to the selected DD254"})
			return
		}
	}

	vr, err := h.store.Create(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("failed to create visit request", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create visit request"})
		return
	}

	// Create action item for FSO review.
	ai, aiErr := h.actionItems.Create(c.Request.Context(), actionitem.CreateParams{
		TenantID:    tenantID,
		SourceType:  "visit_request",
		SourceID:    &vr.ID,
		Title:       "Visit Request Submitted",
		Description: "A visit request requires FSO review",
		Priority:    "medium",
		SubOrgID:    vr.SubOrgID,
	})
	if aiErr != nil {
		h.logger.Error("failed to create action item for visit request", "error", aiErr)
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
		Action:     audit.ActionVisitRequestSubmitted,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"visit_request_id": vr.ID},
	})

	c.JSON(http.StatusCreated, vr)
}

// ListMy handles GET / for the IC's own visit requests.
func (h *Handler) ListMy(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	limit, offset := httputil.ParsePagination(c, 20)

	requests, total, err := h.store.List(c.Request.Context(), ListFilters{
		TenantID: tenantID,
		UserID:   &userID,
		Status:   c.Query("status"),
		Search:   c.Query("search"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		h.logger.Error("failed to list my visit requests", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"requests": requests, "total": total})
}

// MyStats handles GET /stats for the IC dashboard.
func (h *Handler) MyStats(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	stats, err := h.store.SummaryStats(c.Request.Context(), tenantID, &userID, nil)
	if err != nil {
		h.logger.Error("failed to get my visit request stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetMy handles GET /:id for an IC viewing their own visit request.
func (h *Handler) GetMy(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid visit request ID"})
		return
	}

	vr, err := h.store.Get(c.Request.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "visit request not found"})
			return
		}
		h.logger.Error("failed to get visit request", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if vr.CreatedByUserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "visit request not found"})
		return
	}

	c.JSON(http.StatusOK, vr)
}

// ListCloneable handles GET /cloneable for the clone picker.
func (h *Handler) ListCloneable(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	requests, err := h.store.ListCloneable(c.Request.Context(), tenantID, userID)
	if err != nil {
		h.logger.Error("failed to list cloneable visit requests", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"requests": requests})
}

// Cancel handles POST /:id/cancel for an IC cancelling their own submitted visit request.
func (h *Handler) Cancel(c *gin.Context) {
	identityID, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid visit request ID"})
		return
	}

	ctx := c.Request.Context()
	if err := h.store.Cancel(ctx, tenantID, id, userID); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "visit request not found or cannot be cancelled"})
			return
		}
		h.logger.Error("failed to cancel visit request", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	sess, _ := auth.GetSession(c)
	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionVisitRequestSubmitted, // reuse closest existing action
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"visit_request_id": id, "action": "cancelled"},
	})

	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// --- Admin handlers ---

// AdminList handles GET / for the admin visit request list.
func (h *Handler) AdminList(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	limit, offset := httputil.ParsePagination(c, 20)

	requests, total, err := h.store.List(c.Request.Context(), ListFilters{
		TenantID:    tenantID,
		Status:      c.Query("status"),
		Search:      c.Query("search"),
		SubOrgScope: auth.SubOrgScope(c),
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		h.logger.Error("failed to list visit requests", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"requests": requests, "total": total})
}

// AdminStats handles GET /stats for the admin dashboard.
func (h *Handler) AdminStats(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	stats, err := h.store.SummaryStats(c.Request.Context(), tenantID, nil, auth.SubOrgScope(c))
	if err != nil {
		h.logger.Error("failed to get visit request stats", "error", err)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid visit request ID"})
		return
	}

	vr, err := h.store.Get(c.Request.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "visit request not found"})
			return
		}
		h.logger.Error("failed to get visit request", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, vr)
}

// Export streams an audit CSV of visits matching the given filters. The
// CSV begins with a UTF-8 BOM and uses CRLF line endings so Excel opens it
// without mangling characters or row separation.
func (h *Handler) Export(c *gin.Context) {
	identityID, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	from, err := parseDateParam(c.Query("from"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'from' (expected YYYY-MM-DD)"})
		return
	}
	to, err := parseDateParam(c.Query("to"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'to' (expected YYYY-MM-DD)"})
		return
	}
	// Make `to` inclusive of the whole day.
	to = to.Add(24*time.Hour - time.Second)

	var statuses []string
	if s := c.Query("status"); s != "" {
		statuses = strings.Split(s, ",")
	}
	var subOrgID *uuid.UUID
	if s := c.Query("sub_org_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			subOrgID = &id
		}
	}
	detail := c.DefaultQuery("detail", "full")

	filters := ExportFilters{
		TenantID:    tenantID,
		From:        from,
		To:          to,
		Statuses:    statuses,
		SubOrgScope: auth.SubOrgScope(c),
		SubOrgID:    subOrgID,
		Detail:      detail,
	}

	filename := fmt.Sprintf("fastfso-visits-%s_to_%s.csv", from.Format("2006-01-02"), filters.To.Format("2006-01-02"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	w := c.Writer
	// UTF-8 BOM so Excel detects encoding correctly on Windows.
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	writeExportHeader(cw, detail)

	// Stream rows straight to the response so a wide date range on a large
	// tenant doesn't buffer the whole result set in memory. Headers are already
	// committed, so a mid-stream DB error can only truncate the file — log it.
	rowCount, err := h.store.ExportEach(c.Request.Context(), filters, func(r ExportRow) error {
		writeExportRow(cw, r, detail)
		return nil
	})
	cw.Flush()
	if err != nil {
		h.logger.Error("visit export", "error", err, "rows_written", rowCount)
	}

	sess, _ := auth.GetSession(c)
	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionVisitsExported,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata: map[string]any{
			"from":        from.Format("2006-01-02"),
			"to":          filters.To.Format("2006-01-02"),
			"statuses":    statuses,
			"sub_org_id":  subOrgID,
			"detail":      detail,
			"row_count":   rowCount,
		},
	})
}

func parseDateParam(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	return time.Parse("2006-01-02", s)
}

// AdminUpdateStatus handles POST /:id/status for changing a visit request's status.
func (h *Handler) AdminUpdateStatus(c *gin.Context) {
	identityID, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid visit request ID"})
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
	if err := h.store.UpdateStatus(ctx, tenantID, id, userID, req.Status, req.Notes); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "visit request not found"})
			return
		}
		h.logger.Error("failed to update visit request status", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Resolve linked action item when the visit request reaches a terminal status.
	if req.Status == "approved" || req.Status == "rejected" {
		if err := h.actionItems.ResolveBySource(ctx, tenantID, "visit_request", id); err != nil {
			h.logger.Error("failed to resolve action items for visit request", "error", err)
		}
	}

	sess, _ := auth.GetSession(c)
	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionVisitRequestReviewed,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"visit_request_id": id, "status": req.Status},
	})

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
