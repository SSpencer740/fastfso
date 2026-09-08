package travel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/actionitem"
	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/auth"
	"github.com/SSpencer740/fastfso/backend/internal/httputil"
	"github.com/SSpencer740/fastfso/backend/internal/scan"
	"github.com/SSpencer740/fastfso/backend/internal/storage"
)

type auditLogger interface {
	Log(ctx context.Context, p audit.LogParams)
}

type notifier interface {
	NotifyActionItemAssigned(ctx context.Context, userID uuid.UUID, to, name, sourceType string)
	NotifyDebriefAssigned(ctx context.Context, userID uuid.UUID, to, name, debriefID string)
}

// Handler exposes HTTP endpoints for the travel report system.
type Handler struct {
	store       *Store
	actionItems *actionitem.Store
	audit       auditLogger
	notify      notifier
	storage     storage.StorageBackend
	scanner     scan.Scanner
	logger      *slog.Logger
}

// NewHandler creates a new travel Handler.
func NewHandler(store *Store, actionItems *actionitem.Store, auditLog auditLogger, n notifier, sb storage.StorageBackend, scanner scan.Scanner, logger *slog.Logger) *Handler {
	return &Handler{store: store, actionItems: actionItems, audit: auditLog, notify: n, storage: sb, scanner: scanner, logger: logger}
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

// CreateReport handles POST / for creating a new draft travel report.
func (h *Handler) CreateReport(c *gin.Context) {
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
	req.UserID = userID
	req.SubOrgID = sess.SubOrgID

	report, err := h.store.Create(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("failed to create travel report", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTravelReportCreated,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"report_id": report.ID},
	})

	c.JSON(http.StatusCreated, report)
}

// ListMyReports handles GET / for the IC's own travel reports.
func (h *Handler) ListMyReports(c *gin.Context) {
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
		h.logger.Error("failed to list travel reports", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if reports == nil {
		reports = []ReportRow{}
	}

	c.JSON(http.StatusOK, gin.H{"reports": reports, "total": total})
}

// GetMyStats handles GET /stats for the IC dashboard.
func (h *Handler) GetMyStats(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	stats, err := h.store.SummaryStats(c.Request.Context(), tenantID, &userID, nil)
	if err != nil {
		h.logger.Error("failed to get travel stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetReport handles GET /:id for the IC detail view.
func (h *Handler) GetReport(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	reportID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report ID"})
		return
	}

	detail, err := h.store.Get(c.Request.Context(), tenantID, userID, reportID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		h.logger.Error("failed to get travel report", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, detail)
}

// UpdateReport handles PUT /:id for editing a draft report.
func (h *Handler) UpdateReport(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	reportID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report ID"})
		return
	}

	var p UpdateParams
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := h.store.Update(c.Request.Context(), tenantID, userID, reportID, p); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found or not a draft"})
			return
		}
		h.logger.Error("failed to update travel report", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// SubmitReport handles POST /:id/submit for submitting a draft for review.
func (h *Handler) SubmitReport(c *gin.Context) {
	identityID, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	reportID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report ID"})
		return
	}

	ctx := c.Request.Context()
	if err := h.store.Submit(ctx, tenantID, userID, reportID); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found or not a draft"})
			return
		}
		h.logger.Error("failed to submit travel report", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Fetch the report to get its sub_org_id for the action item.
	sess, _ := auth.GetSession(c)
	ai, aiErr := h.actionItems.Create(ctx, actionitem.CreateParams{
		TenantID:    tenantID,
		SourceType:  "travel_report",
		SourceID:    &reportID,
		Title:       "Travel Report Submitted",
		Description: "A travel report requires FSO review",
		Priority:    "medium",
		SubOrgID:    sess.SubOrgID,
	})
	if aiErr != nil {
		h.logger.Error("failed to create action item for travel report", "error", aiErr)
	} else if ai != nil {
		for _, t := range h.actionItems.ResolveNotifyTargets(ctx, tenantID, ai) {
			tUserID, _ := uuid.Parse(t.UserID)
			h.notify.NotifyActionItemAssigned(ctx, tUserID, t.Email, t.Name, ai.SourceType)
		}
	}

	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTravelReportSubmitted,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"report_id": reportID},
	})

	c.JSON(http.StatusOK, gin.H{"status": "submitted"})
}

// UploadFile handles POST /:id/upload for file attachments.
func (h *Handler) UploadFile(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	reportID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report ID"})
		return
	}

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

	ctx := c.Request.Context()
	if err := httputil.ScanAndRewind(ctx, h.scanner, file); err != nil {
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

	key := fmt.Sprintf("%s/%s", storage.TravelKeyPrefix(tenantID, reportID), safeName)

	if err := h.storage.Upload(ctx, key, file, contentType); err != nil {
		h.logger.Error("failed to upload file", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
		return
	}

	upload, err := h.store.CreateUpload(ctx, reportID, safeName, header.Size, contentType, key)
	if err != nil {
		h.logger.Error("failed to record upload", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, upload)
}

// DeleteUpload handles DELETE /:id/upload/:upload_id for removing a file attachment.
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
	}

	c.Status(http.StatusNoContent)
}

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

	key, _, _, err := h.store.GetUploadForDownload(c.Request.Context(), tenantID, uploadID)
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

	c.Redirect(http.StatusFound, url)
}

// --- IC debrief handlers ---

// GetMyDebriefs handles GET /debriefs for the IC's pending debrief list.
func (h *Handler) GetMyDebriefs(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	debriefs, err := h.store.ListMyDebriefs(c.Request.Context(), tenantID, userID)
	if err != nil {
		h.logger.Error("failed to list debriefs", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if debriefs == nil {
		debriefs = []Debrief{}
	}
	c.JSON(http.StatusOK, gin.H{"debriefs": debriefs})
}

// SubmitDebrief handles POST /debriefs/:id/submit for the IC to complete a debrief.
func (h *Handler) SubmitDebrief(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	debriefID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid debrief ID"})
		return
	}

	var p SubmitDebriefParams
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()

	// Fetch first to get trip name and verify ownership.
	debrief, err := h.store.GetDebrief(ctx, tenantID, userID, debriefID)
	if err != nil {
		if errors.Is(err, ErrDebriefNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "debrief not found"})
			return
		}
		h.logger.Error("failed to get debrief", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.store.SubmitDebrief(ctx, debriefID, p); err != nil {
		if errors.Is(err, ErrDebriefNotFound) {
			c.JSON(http.StatusConflict, gin.H{"error": "debrief already submitted or not found"})
			return
		}
		h.logger.Error("failed to submit debrief", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Resolve the cron-created "pending" action item before creating the submission one.
	if err := h.actionItems.ResolveBySource(ctx, tenantID, "travel_debrief", debriefID); err != nil {
		h.logger.Error("failed to resolve pending debrief action item", "error", err)
	}

	// Look up the parent travel report's sub_org_id for scoping.
	var reportSubOrgID *uuid.UUID
	if parentReport, rErr := h.store.AdminGet(ctx, tenantID, debrief.ReportID); rErr == nil {
		reportSubOrgID = parentReport.Report.SubOrgID
	}

	// Create FSO action item with IC's answers.
	srcID := debriefID
	ai, aiErr := h.actionItems.Create(ctx, actionitem.CreateParams{
		TenantID:    tenantID,
		SourceType:  "travel_debrief",
		SourceID:    &srcID,
		Title:       fmt.Sprintf("Process Post-Travel Debrief: %s", debrief.TripName),
		Description: buildDebriefDescription(debrief.TripName, p),
		Priority:    "medium",
		SubOrgID:    reportSubOrgID,
	})
	if aiErr != nil {
		h.logger.Error("failed to create FSO action item for debrief", "error", aiErr)
	} else if ai != nil {
		for _, t := range h.actionItems.ResolveNotifyTargets(ctx, tenantID, ai) {
			tUserID, _ := uuid.Parse(t.UserID)
			h.notify.NotifyActionItemAssigned(ctx, tUserID, t.Email, t.Name, ai.SourceType)
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "submitted"})
}

// AdminGetDebrief handles GET /debriefs/:id for the FSO detail view.
func (h *Handler) AdminGetDebrief(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	debriefID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid debrief ID"})
		return
	}

	debrief, err := h.store.GetDebriefAdmin(c.Request.Context(), tenantID, debriefID)
	if err != nil {
		if errors.Is(err, ErrDebriefNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "debrief not found"})
			return
		}
		h.logger.Error("failed to get debrief", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, debrief)
}

// --- Admin handlers ---

// ListReports handles GET / for the admin travel report list.
func (h *Handler) ListReports(c *gin.Context) {
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
		h.logger.Error("failed to list travel reports", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if reports == nil {
		reports = []ReportRow{}
	}

	c.JSON(http.StatusOK, gin.H{"reports": reports, "total": total})
}

// GetStats handles GET /stats for the admin dashboard.
func (h *Handler) GetStats(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	stats, err := h.store.SummaryStats(c.Request.Context(), tenantID, nil, auth.SubOrgScope(c))
	if err != nil {
		h.logger.Error("failed to get travel stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// AdminGetReport handles GET /:id for the admin detail view.
func (h *Handler) AdminGetReport(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	reportID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report ID"})
		return
	}

	ctx := c.Request.Context()

	// Transition submitted → under_review when an admin opens the report.
	if err := h.store.MarkUnderReview(ctx, tenantID, reportID); err != nil {
		h.logger.Error("failed to mark travel report under review", "error", err)
	}

	detail, err := h.store.AdminGet(ctx, tenantID, reportID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		h.logger.Error("failed to get travel report", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, detail)
}

// ApproveReport handles POST /:id/approve for admin approval.
func (h *Handler) ApproveReport(c *gin.Context) {
	identityID, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	reportID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report ID"})
		return
	}

	ctx := c.Request.Context()
	if err := h.store.UpdateStatus(ctx, tenantID, reportID, "approved", userID); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		h.logger.Error("failed to approve travel report", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.actionItems.ResolveBySource(ctx, tenantID, "travel_report", reportID); err != nil {
		h.logger.Error("failed to resolve action items for travel report", "error", err)
	}

	sess, _ := auth.GetSession(c)
	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTravelReportApproved,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"report_id": reportID},
	})

	c.JSON(http.StatusOK, gin.H{"status": "approved"})
}

// RejectReport handles POST /:id/reject for admin rejection.
func (h *Handler) RejectReport(c *gin.Context) {
	identityID, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	reportID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report ID"})
		return
	}

	ctx := c.Request.Context()
	if err := h.store.UpdateStatus(ctx, tenantID, reportID, "rejected", userID); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		h.logger.Error("failed to reject travel report", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.actionItems.ResolveBySource(ctx, tenantID, "travel_report", reportID); err != nil {
		h.logger.Error("failed to resolve action items for travel report", "error", err)
	}

	sess, _ := auth.GetSession(c)
	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTravelReportRejected,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"report_id": reportID},
	})

	c.JSON(http.StatusOK, gin.H{"status": "rejected"})
}
