package dd254

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

	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/auth"
	"github.com/SSpencer740/fastfso/backend/internal/httputil"
	"github.com/SSpencer740/fastfso/backend/internal/scan"
	"github.com/SSpencer740/fastfso/backend/internal/storage"
)

const maxFileSize = 25 << 20 // 25 MB — DD254s are short PDFs

type auditLogger interface {
	Log(ctx context.Context, p audit.LogParams)
}

type Handler struct {
	store   *Store
	storage storage.StorageBackend
	audit   auditLogger
	scanner scan.Scanner
	logger  *slog.Logger
}

func NewHandler(store *Store, sb storage.StorageBackend, auditLog auditLogger, scanner scan.Scanner, logger *slog.Logger) *Handler {
	return &Handler{store: store, storage: sb, audit: auditLog, scanner: scanner, logger: logger}
}

func sessionInfo(c *gin.Context) (identityID, tenantID, userID uuid.UUID, ok bool) {
	sess, exists := auth.GetSession(c)
	if !exists || sess.TenantID == nil || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return sess.IdentityID, *sess.TenantID, *sess.UserID, true
}

func (h *Handler) logAudit(c *gin.Context, tenantID uuid.UUID, action string, metadata map[string]any) {
	sess, ok := auth.GetSession(c)
	if !ok {
		return
	}
	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     action,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   metadata,
	})
}

func parseOptDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func parseOptUUID(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// List handles GET /api/v1/dd254
func (h *Handler) List(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}
	filters := ListFilters{
		TenantID:    tenantID,
		SubOrgScope: auth.SubOrgScope(c),
		Status:      c.Query("status"),
		Class:       c.Query("class"),
		Search:      c.Query("search"),
	}
	if filters.SubOrgScope == nil {
		if id, err := parseOptUUID(c.Query("sub_org_id")); err == nil && id != nil {
			filters.SubOrgID = id
		}
	}
	forms, err := h.store.List(c.Request.Context(), filters)
	if err != nil {
		h.logger.Error("dd254 list", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if forms == nil {
		forms = []Form{}
	}
	c.JSON(http.StatusOK, gin.H{"forms": forms})
}

// Stats handles GET /api/v1/dd254/stats
func (h *Handler) Stats(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}
	scope := auth.SubOrgScope(c)
	var subOrgID *uuid.UUID
	if scope == nil {
		if id, err := parseOptUUID(c.Query("sub_org_id")); err == nil {
			subOrgID = id
		}
	}
	stats, err := h.store.Stats(c.Request.Context(), tenantID, scope, subOrgID)
	if err != nil {
		h.logger.Error("dd254 stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// Get handles GET /api/v1/dd254/:id
func (h *Handler) Get(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	detail, err := h.store.Get(c.Request.Context(), tenantID, id, auth.SubOrgScope(c))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		h.logger.Error("dd254 get", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, detail)
}

// Upload handles POST /api/v1/dd254 — multipart form upload with metadata + attestation.
func (h *Handler) Upload(c *gin.Context) {
	identityID, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFileSize)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer func() { _ = file.Close() }()

	if !strings.EqualFold(filepath.Ext(header.Filename), ".pdf") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only PDF files are allowed"})
		return
	}

	// CUI gate — refuse anything not explicitly attested as unclassified.
	markings := c.PostForm("markings")
	if markings != RequiredMarkings {
		c.JSON(http.StatusBadRequest, gin.H{"error": "DD254 must be marked UNCLASSIFIED — fastFSO is CMMC L1 and does not store CUI"})
		return
	}
	if c.PostForm("cui_attestation") != "true" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CUI attestation is required"})
		return
	}

	contractNumber := strings.TrimSpace(c.PostForm("contract_number"))
	if contractNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contract_number is required"})
		return
	}
	classMax := c.PostForm("classification_max")
	if !ValidClassification(classMax) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid classification_max"})
		return
	}

	periodStart, err := parseOptDate(c.PostForm("period_start"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period_start"})
		return
	}
	periodEnd, err := parseOptDate(c.PostForm("period_end"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period_end"})
		return
	}
	subOrgID, err := parseOptUUID(c.PostForm("sub_org_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sub_org_id"})
		return
	}
	// FSOs cannot override their sub-org scope when uploading.
	if scope := auth.SubOrgScope(c); scope != nil {
		subOrgID = scope
	}

	if err := httputil.ScanAndRewind(c.Request.Context(), h.scanner, file); err != nil {
		var infected *scan.InfectedError
		if errors.As(err, &infected) {
			h.logger.Warn("rejected infected dd254 upload", "signature", infected.Signature, "filename", header.Filename)
			c.JSON(http.StatusBadRequest, gin.H{"error": "file rejected by malware scanner"})
			return
		}
		h.logger.Error("malware scan failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "malware scanner unavailable, please try again"})
		return
	}

	// Generate the DD254 id up front so the storage key embeds it before insert.
	dd254ID := uuid.New()
	key := fmt.Sprintf("%s/%s", storage.DD254KeyPrefix(tenantID, dd254ID), header.Filename)
	if err := h.storage.Upload(c.Request.Context(), key, file, "application/pdf"); err != nil {
		h.logger.Error("dd254 storage upload", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
		return
	}

	form, err := h.store.Create(c.Request.Context(), CreateParams{
		ID:                dd254ID,
		TenantID:          tenantID,
		SubOrgID:          subOrgID,
		ContractNumber:    contractNumber,
		PrimeContractor:   strings.TrimSpace(c.PostForm("prime_contractor")),
		ClassificationMax: classMax,
		PeriodStart:       periodStart,
		PeriodEnd:         periodEnd,
		StorageKey:        key,
		Filename:          header.Filename,
		ContentType:       "application/pdf",
		SizeBytes:         header.Size,
		Markings:          markings,
		UploadedBy:        userID,
	})
	if err != nil {
		// best-effort storage cleanup so we don't leave orphan files
		_ = h.storage.Delete(c.Request.Context(), key)
		h.logger.Error("dd254 create", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.logAudit(c, tenantID, audit.ActionDD254Uploaded, map[string]any{
		"dd254_id":        form.ID,
		"contract_number": contractNumber,
		"classification":  classMax,
		"size":            header.Size,
		"identity_id":     identityID,
	})
	// Honest reporting for the frontend toast: false in local dev with the
	// Noop scanner, true wherever a real malware scanner is wired (cloud
	// requires CLAMAV_ADDR per router init).
	_, isNoop := h.scanner.(scan.Noop)
	c.JSON(http.StatusCreated, gin.H{
		"form":    form,
		"scanned": !isNoop,
	})
}

// ViewFile streams the DD254 PDF for inline viewing. Audit-logged on every fetch.
func (h *Handler) ViewFile(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	key, filename, contentType, err := h.store.GetStorageKey(c.Request.Context(), tenantID, id, auth.SubOrgScope(c))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		h.logger.Error("dd254 get key", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	rc, err := h.storage.Download(c.Request.Context(), key)
	if err != nil {
		h.logger.Error("dd254 download", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer func() { _ = rc.Close() }()

	h.logAudit(c, tenantID, audit.ActionDD254Viewed, map[string]any{"dd254_id": id})

	if contentType == "" {
		contentType = "application/pdf"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
	c.Header("Cache-Control", "private, max-age=300")
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}

// Delete soft-deletes the DD254 (status=superseded).
func (h *Handler) Delete(c *gin.Context) {
	identityID, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	_, err = h.store.Delete(c.Request.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		h.logger.Error("dd254 delete", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	h.logAudit(c, tenantID, audit.ActionDD254Deleted, map[string]any{
		"dd254_id":    id,
		"identity_id": identityID,
	})
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// GrantAccess handles POST /api/v1/dd254/:id/access
func (h *Handler) GrantAccess(c *gin.Context) {
	identityID, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}
	dd254ID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var p GrantAccessParams
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	briefed, err := parseOptDateStr(p.BriefedAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid briefed_at"})
		return
	}
	debriefed, err := parseOptDateStr(p.DebriefedAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid debriefed_at"})
		return
	}
	if err := h.store.GrantAccess(c.Request.Context(), tenantID, dd254ID, p.UserID, userID, briefed, debriefed); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		case errors.Is(err, ErrUserNotInTenant):
			c.JSON(http.StatusBadRequest, gin.H{"error": "user does not belong to this tenant"})
		default:
			h.logger.Error("dd254 grant access", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	h.logAudit(c, tenantID, audit.ActionDD254AccessGranted, map[string]any{
		"dd254_id":    dd254ID,
		"user_id":     p.UserID,
		"identity_id": identityID,
	})
	c.JSON(http.StatusOK, gin.H{"status": "granted"})
}

// RevokeAccess handles DELETE /api/v1/dd254/:id/access/:user_id
func (h *Handler) RevokeAccess(c *gin.Context) {
	identityID, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}
	dd254ID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	targetUserID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if err := h.store.RevokeAccess(c.Request.Context(), tenantID, dd254ID, targetUserID); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		h.logger.Error("dd254 revoke access", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	h.logAudit(c, tenantID, audit.ActionDD254AccessRevoked, map[string]any{
		"dd254_id":    dd254ID,
		"user_id":     targetUserID,
		"identity_id": identityID,
	})
	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

// MyAuthorizations handles GET /api/v1/me/dd254-authorizations — the
// current user's active read-on entries, used by the visit-submit wizard's
// contract picker.
func (h *Handler) MyAuthorizations(c *gin.Context) {
	_, tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}
	auths, err := h.store.MyAuthorizations(c.Request.Context(), tenantID, userID)
	if err != nil {
		h.logger.Error("dd254 my authorizations", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"authorizations": auths})
}

// SuggestForVisit handles GET /api/v1/admin/visits/:id/dd254-suggestions
func (h *Handler) SuggestForVisit(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}
	visitID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	suggestions, err := h.store.SuggestForVisit(c.Request.Context(), tenantID, visitID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "visit not found"})
			return
		}
		h.logger.Error("dd254 suggest", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if suggestions == nil {
		suggestions = []Suggestion{}
	}
	c.JSON(http.StatusOK, gin.H{"suggestions": suggestions})
}

// LinkVisitRequest handles PUT /api/v1/admin/visits/:id/dd254
func (h *Handler) LinkVisitRequest(c *gin.Context) {
	identityID, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}
	visitID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		DD254ID *string `json:"dd_254_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	dd254ID, err := parseOptUUID(stringOrEmpty(body.DD254ID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dd_254_id"})
		return
	}
	if err := h.store.SetVisitRequestDD254(c.Request.Context(), tenantID, visitID, dd254ID); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		h.logger.Error("dd254 link visit", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	h.logAudit(c, tenantID, audit.ActionVisitRequestReviewed, map[string]any{
		"visit_request_id": visitID,
		"dd254_id":         dd254ID,
		"action":           "dd254_linked",
		"identity_id":      identityID,
	})
	c.JSON(http.StatusOK, gin.H{"status": "linked"})
}

func parseOptDateStr(s *string) (*time.Time, error) {
	if s == nil {
		return nil, nil
	}
	return parseOptDate(*s)
}

func stringOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
