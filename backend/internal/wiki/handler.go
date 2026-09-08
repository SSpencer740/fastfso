package wiki

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/auth"
	"github.com/SSpencer740/fastfso/backend/internal/httputil"
	"github.com/SSpencer740/fastfso/backend/internal/scan"
	"github.com/SSpencer740/fastfso/backend/internal/storage"
)

const maxFileSize = 20 << 20 // 20 MB

type auditLogger interface {
	Log(ctx context.Context, p audit.LogParams)
}

// Handler exposes HTTP endpoints for wiki posts.
type Handler struct {
	store   *Store
	storage storage.StorageBackend
	audit   auditLogger
	scanner scan.Scanner
	logger  *slog.Logger
}

// NewHandler creates a new wiki Handler.
func NewHandler(store *Store, sb storage.StorageBackend, auditLog auditLogger, scanner scan.Scanner, logger *slog.Logger) *Handler {
	return &Handler{store: store, storage: sb, audit: auditLog, scanner: scanner, logger: logger}
}

// parseSubOrgID accepts a JSON sub_org_id field (which may be missing, null,
// empty string, or a UUID) and returns the equivalent *uuid.UUID. Missing /
// null / empty all map to nil (tenant-wide).
func parseSubOrgID(raw *string) (*uuid.UUID, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func sessionInfo(c *gin.Context) (tenantID, userID uuid.UUID, ok bool) {
	sess, exists := auth.GetSession(c)
	if !exists || sess.TenantID == nil || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return uuid.Nil, uuid.Nil, false
	}
	return *sess.TenantID, *sess.UserID, true
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

// --- IC handlers ---

// ListPublished handles GET /wiki — returns published posts for any tenant user.
func (h *Handler) ListPublished(c *gin.Context) {
	tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	// ICs see published posts scoped to their sub-org + tenant-wide
	sess, _ := auth.GetSession(c)
	posts, err := h.store.List(c.Request.Context(), tenantID, true, sess.SubOrgID)
	if err != nil {
		h.logger.Error("wiki list published", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if posts == nil {
		posts = []*Post{}
	}
	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

// GetFSOInfo handles GET /wiki/fso — returns the FSO contact the viewer
// should see. Resolution is sub-org aware: the viewer's session sub-org's
// primary FSO wins, falling back to the tenant's oldest FSO when not set.
func (h *Handler) GetFSOInfo(c *gin.Context) {
	tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}
	sess, _ := auth.GetSession(c)

	fso, err := h.store.GetFSO(c.Request.Context(), tenantID, sess.SubOrgID)
	if err != nil {
		h.logger.Error("wiki get fso", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"fso": fso})
}

// ViewFile handles GET /wiki/files/:file_id — streams the PDF inline for any tenant user.
func (h *Handler) ViewFile(c *gin.Context) {
	tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	fileID, err := uuid.Parse(c.Param("file_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}

	key, f, err := h.store.GetFile(c.Request.Context(), tenantID, fileID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		h.logger.Error("wiki view file", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	rc, err := h.storage.Download(c.Request.Context(), key)
	if err != nil {
		h.logger.Error("wiki download file", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer func() { _ = rc.Close() }()

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, f.FileName))
	c.Header("Cache-Control", "private, max-age=300")
	c.Header("X-Frame-Options", "SAMEORIGIN") // allow embedding in our own app
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}

// ServeLocalFile handles GET /wiki/local-files/*key — streams PDFs in local dev.
func (h *Handler) ServeLocalFile(c *gin.Context) {
	_, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	key := c.Param("key")[1:] // strip leading slash
	rc, err := h.storage.Download(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	defer func() { _ = rc.Close() }()

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filepath.Base(key)))
	c.Header("Cache-Control", "private, max-age=300")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}

// --- Admin handlers ---

// AdminList handles GET /admin/wiki — returns all posts (published + drafts).
func (h *Handler) AdminList(c *gin.Context) {
	tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	posts, err := h.store.List(c.Request.Context(), tenantID, false, auth.SubOrgScope(c))
	if err != nil {
		h.logger.Error("wiki admin list", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if posts == nil {
		posts = []*Post{}
	}
	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

// AdminCreate handles POST /admin/wiki — creates a new wiki post.
func (h *Handler) AdminCreate(c *gin.Context) {
	tenantID, userID, ok := sessionInfo(c)
	if !ok {
		return
	}

	var body struct {
		Title     string `json:"title"`
		Content   string `json:"content"`
		Published bool   `json:"published"`
		// SubOrgID is honored only for administrators. FSOs always inherit
		// their session sub-org (prevents spoofing). Use \"\" or null for
		// tenant-wide (admin-only).
		SubOrgID *string `json:"sub_org_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	wikiSess, _ := auth.GetSession(c)
	subOrgID := wikiSess.SubOrgID
	if wikiSess.UserRole != nil && *wikiSess.UserRole == "administrator" {
		// Admins can place a post in any sub-org or make it tenant-wide.
		parsed, err := parseSubOrgID(body.SubOrgID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sub_org_id"})
			return
		}
		subOrgID = parsed
	}
	post, err := h.store.Create(c.Request.Context(), CreateParams{
		TenantID:        tenantID,
		CreatedByUserID: userID,
		SubOrgID:        subOrgID,
		Title:           body.Title,
		Content:         body.Content,
		Published:       body.Published,
	})
	if err != nil {
		h.logger.Error("wiki create", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	h.logAudit(c, tenantID, audit.ActionWikiPostCreated, map[string]any{
		"post_id":   post.ID,
		"published": body.Published,
	})
	c.JSON(http.StatusCreated, post)
}

// AdminGet handles GET /admin/wiki/:id.
func (h *Handler) AdminGet(c *gin.Context) {
	tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	post, err := h.store.Get(c.Request.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		h.logger.Error("wiki admin get", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, post)
}

// AdminUpdate handles PUT /admin/wiki/:id.
func (h *Handler) AdminUpdate(c *gin.Context) {
	tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var body struct {
		Title     string `json:"title"`
		Content   string `json:"content"`
		Published bool   `json:"published"`
		// SubOrgID is honored only for administrators. FSOs cannot move a
		// post between sub-orgs.
		SubOrgID *string `json:"sub_org_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	wikiSess, _ := auth.GetSession(c)
	params := UpdateParams{
		TenantID:  tenantID,
		ID:        id,
		Title:     body.Title,
		Content:   body.Content,
		Published: body.Published,
	}
	if wikiSess.UserRole != nil && *wikiSess.UserRole == "administrator" {
		parsed, err := parseSubOrgID(body.SubOrgID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sub_org_id"})
			return
		}
		params.SubOrgID = parsed
		params.UpdateSubOrgID = true
	}
	post, err := h.store.Update(c.Request.Context(), params)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		h.logger.Error("wiki update", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	h.logAudit(c, tenantID, audit.ActionWikiPostUpdated, map[string]any{
		"post_id":   id,
		"published": body.Published,
	})
	c.JSON(http.StatusOK, post)
}

// AdminDelete handles DELETE /admin/wiki/:id.
func (h *Handler) AdminDelete(c *gin.Context) {
	tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.store.Delete(c.Request.Context(), tenantID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		h.logger.Error("wiki delete", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	h.logAudit(c, tenantID, audit.ActionWikiPostDeleted, map[string]any{"post_id": id})
	c.JSON(http.StatusNoContent, nil)
}

// AdminUploadFile handles POST /admin/wiki/:id/files — attaches a PDF to a post.
func (h *Handler) AdminUploadFile(c *gin.Context) {
	tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	postID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return
	}

	// Verify the post belongs to this tenant.
	if _, err := h.store.Get(c.Request.Context(), tenantID, postID); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		h.logger.Error("wiki upload: get post", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
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

	if err := httputil.ScanAndRewind(c.Request.Context(), h.scanner, file); err != nil {
		var infected *scan.InfectedError
		if errors.As(err, &infected) {
			h.logger.Warn("rejected infected wiki upload", "signature", infected.Signature, "filename", header.Filename)
			c.JSON(http.StatusBadRequest, gin.H{"error": "file rejected by malware scanner"})
			return
		}
		h.logger.Error("malware scan failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "malware scanner unavailable, please try again"})
		return
	}

	key := fmt.Sprintf("%s/%s", storage.WikiKeyPrefix(tenantID, postID), header.Filename)
	if err := h.storage.Upload(c.Request.Context(), key, file, "application/pdf"); err != nil {
		h.logger.Error("wiki upload file", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
		return
	}

	f, err := h.store.AddFile(c.Request.Context(), postID, tenantID, header.Filename, key, header.Size)
	if err != nil {
		h.logger.Error("wiki add file record", "error", err)
		// best-effort cleanup
		_ = h.storage.Delete(c.Request.Context(), key)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	h.logAudit(c, tenantID, audit.ActionWikiFileUploaded, map[string]any{
		"post_id":   postID,
		"file_id":   f.ID,
		"file_name": header.Filename,
		"size":      header.Size,
	})
	c.JSON(http.StatusCreated, f)
}

// AdminDeleteFile handles DELETE /admin/wiki/files/:file_id — removes an attachment.
func (h *Handler) AdminDeleteFile(c *gin.Context) {
	tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	fileID, err := uuid.Parse(c.Param("file_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}

	key, err := h.store.DeleteFile(c.Request.Context(), tenantID, fileID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		h.logger.Error("wiki delete file", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if err := h.storage.Delete(c.Request.Context(), key); err != nil {
		h.logger.Warn("wiki delete file from storage", "error", err, "key", key)
	}
	h.logAudit(c, tenantID, audit.ActionWikiFileDeleted, map[string]any{"file_id": fileID})
	c.JSON(http.StatusNoContent, nil)
}
