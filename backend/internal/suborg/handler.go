package suborg

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fastfso/fastfso/backend/internal/audit"
	"github.com/fastfso/fastfso/backend/internal/auth"
)

type sessionStore interface {
	UpdateSubOrg(ctx context.Context, userID uuid.UUID, subOrgID *uuid.UUID) error
}

type auditLogger interface {
	Log(ctx context.Context, p audit.LogParams)
}

type Handler struct {
	store    *Store
	sessions sessionStore
	audit    auditLogger
	logger   *slog.Logger
}

func NewHandler(store *Store, sessions sessionStore, auditLog auditLogger, logger *slog.Logger) *Handler {
	return &Handler{store: store, sessions: sessions, audit: auditLog, logger: logger}
}

func tenantID(c *gin.Context) (uuid.UUID, bool) {
	sess, ok := auth.GetSession(c)
	if !ok || sess.TenantID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return uuid.Nil, false
	}
	return *sess.TenantID, true
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

// List handles GET /api/v1/admin/sub-orgs.
func (h *Handler) List(c *gin.Context) {
	tid, ok := tenantID(c)
	if !ok {
		return
	}
	orgs, err := h.store.List(c.Request.Context(), tid)
	if err != nil {
		h.logger.Error("suborg list", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if orgs == nil {
		orgs = []SubOrg{}
	}
	c.JSON(http.StatusOK, gin.H{"sub_orgs": orgs})
}

// Create handles POST /api/v1/admin/sub-orgs.
func (h *Handler) Create(c *gin.Context) {
	tid, ok := tenantID(c)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	org, err := h.store.Create(c.Request.Context(), tid, body.Name)
	if err != nil {
		if errors.Is(err, ErrNameTaken) {
			c.JSON(http.StatusConflict, gin.H{"error": "a sub-organization with that name already exists"})
			return
		}
		h.logger.Error("suborg create", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	h.logAudit(c, tid, audit.ActionSubOrgCreated, map[string]any{
		"sub_org_id": org.ID,
		"name":       org.Name,
	})
	c.JSON(http.StatusCreated, org)
}

// Update handles PUT /api/v1/admin/sub-orgs/:id.
func (h *Handler) Update(c *gin.Context) {
	tid, ok := tenantID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if err := h.store.Update(c.Request.Context(), tid, id, body.Name); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "sub-organization not found"})
		case errors.Is(err, ErrNameTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "a sub-organization with that name already exists"})
		default:
			h.logger.Error("suborg update", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	h.logAudit(c, tid, audit.ActionSubOrgUpdated, map[string]any{
		"sub_org_id": id,
		"name":       body.Name,
	})
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// Delete handles DELETE /api/v1/admin/sub-orgs/:id.
func (h *Handler) Delete(c *gin.Context) {
	tid, ok := tenantID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.store.Delete(c.Request.Context(), tid, id); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "sub-organization not found"})
		case errors.Is(err, ErrDefaultLocked):
			c.JSON(http.StatusForbidden, gin.H{"error": "the Default sub-organization cannot be deleted"})
		default:
			h.logger.Error("suborg delete", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	h.logAudit(c, tid, audit.ActionSubOrgDeleted, map[string]any{"sub_org_id": id})
	c.Status(http.StatusNoContent)
}

// SetPrimaryFSO handles PUT /api/v1/admin/sub-orgs/:id/primary-fso.
func (h *Handler) SetPrimaryFSO(c *gin.Context) {
	tid, ok := tenantID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		UserID *string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	var userID *uuid.UUID
	if body.UserID != nil && *body.UserID != "" {
		parsed, err := uuid.Parse(*body.UserID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
			return
		}
		userID = &parsed
	}
	if err := h.store.SetPrimaryFSO(c.Request.Context(), tid, id, userID); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "sub-organization not found"})
			return
		}
		h.logger.Error("suborg set primary fso", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	meta := map[string]any{"sub_org_id": id}
	if userID != nil {
		meta["user_id"] = *userID
	} else {
		meta["user_id"] = nil
	}
	h.logAudit(c, tid, audit.ActionSubOrgPrimaryFSOSet, meta)
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// GetMyFSO handles GET /api/v1/me/fso — returns the primary FSO contact for the caller's sub-org.
func (h *Handler) GetMyFSO(c *gin.Context) {
	sess, ok := auth.GetSession(c)
	if !ok || sess.TenantID == nil || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}
	contact, err := h.store.GetPrimaryFSO(c.Request.Context(), *sess.TenantID, *sess.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusOK, gin.H{"fso": nil})
			return
		}
		h.logger.Error("get my fso", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"fso": contact})
}

// SetUserSubOrg handles PUT /api/v1/admin/members/:user_id/sub-org.
func (h *Handler) SetUserSubOrg(c *gin.Context) {
	tid, ok := tenantID(c)
	if !ok {
		return
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	var body struct {
		SubOrgID string `json:"sub_org_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.SubOrgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sub_org_id is required"})
		return
	}
	subOrgID, err := uuid.Parse(body.SubOrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sub_org_id"})
		return
	}
	if err := h.store.SetUserSubOrg(c.Request.Context(), tid, userID, subOrgID); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "sub-organization not found"})
			return
		}
		h.logger.Error("suborg set user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if err := h.sessions.UpdateSubOrg(c.Request.Context(), userID, &subOrgID); err != nil {
		h.logger.Error("suborg update session sub_org", "error", err)
	}
	h.logAudit(c, tid, audit.ActionSubOrgUserAssigned, map[string]any{
		"sub_org_id": subOrgID,
		"user_id":    userID,
	})
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
