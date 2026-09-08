package team

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/auth"
)

type auditLogger interface {
	Log(ctx context.Context, p audit.LogParams)
}

type Handler struct {
	store  *Store
	audit  auditLogger
	logger *slog.Logger
}

func NewHandler(store *Store, auditLog auditLogger, logger *slog.Logger) *Handler {
	return &Handler{store: store, audit: auditLog, logger: logger}
}

func sessionInfo(c *gin.Context) (identityID, tenantID, userID uuid.UUID, ok bool) {
	sess, exists := auth.GetSession(c)
	if !exists || sess.TenantID == nil || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return sess.IdentityID, *sess.TenantID, *sess.UserID, true
}

// List handles GET /api/v1/team
func (h *Handler) List(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	filters := ListFilters{
		TenantID:    tenantID,
		SubOrgScope: auth.SubOrgScope(c),
		Clearance:   c.Query("clearance"),
		Search:      c.Query("search"),
	}

	// sub_org_id is consumed by SubOrgScope for FSOs (it ignores the param). For
	// admins, SubOrgScope returns nil and we apply the sub-org dropdown via SubOrgID.
	if s := c.Query("sub_org_id"); s != "" && filters.SubOrgScope == nil {
		if id, err := uuid.Parse(s); err == nil {
			filters.SubOrgID = &id
		}
	}
	if s := c.Query("due_within_days"); s != "" {
		if days, err := strconv.Atoi(s); err == nil && days >= 0 {
			filters.DueWithinDays = &days
		}
	}

	// Pagination: default 50/page, hard cap 200 to bound the per-row sub-org
	// aggregation cost for large tenants.
	filters.Limit = 50
	if s := c.Query("limit"); s != "" {
		if l, err := strconv.Atoi(s); err == nil && l > 0 {
			filters.Limit = l
		}
	}
	if filters.Limit > 200 {
		filters.Limit = 200
	}
	if s := c.Query("offset"); s != "" {
		if o, err := strconv.Atoi(s); err == nil && o > 0 {
			filters.Offset = o
		}
	}

	members, total, err := h.store.List(c.Request.Context(), filters)
	if err != nil {
		h.logger.Error("failed to list team", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": members, "total": total})
}

// Stats handles GET /api/v1/team/stats
func (h *Handler) Stats(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}
	scope := auth.SubOrgScope(c)
	var subOrgID *uuid.UUID
	if s := c.Query("sub_org_id"); s != "" && scope == nil {
		if id, err := uuid.Parse(s); err == nil {
			subOrgID = &id
		}
	}
	stats, err := h.store.Stats(c.Request.Context(), tenantID, scope, subOrgID)
	if err != nil {
		h.logger.Error("failed to load team stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// Get handles GET /api/v1/team/:user_id
func (h *Handler) Get(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	detail, err := h.store.Get(c.Request.Context(), tenantID, userID, auth.SubOrgScope(c))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
			return
		}
		h.logger.Error("failed to load member", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, detail)
}

// SetClearance handles PUT /api/v1/team/:user_id/clearance
func (h *Handler) SetClearance(c *gin.Context) {
	identityID, tenantID, recorderID, ok := sessionInfo(c)
	if !ok {
		return
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	// Enforce sub-org scope at the write site too — FSOs cannot edit members outside their sub-org.
	if scope := auth.SubOrgScope(c); scope != nil {
		if _, err := h.store.Get(c.Request.Context(), tenantID, userID, scope); err != nil {
			if errors.Is(err, ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
				return
			}
			h.logger.Error("scope check failed", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
	}

	var p SetClearanceParams
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()
	rec, err := h.store.SetClearance(ctx, tenantID, userID, recorderID, p)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidClearance):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid clearance level"})
		case errors.Is(err, ErrInvalidInvestType):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid investigation type"})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		default:
			h.logger.Error("failed to set clearance", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	sess, _ := auth.GetSession(c)
	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &identityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionClearanceUpdated,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata: map[string]any{
			"user_id":   userID,
			"clearance": rec.Clearance,
			"record_id": rec.ID,
		},
	})

	c.JSON(http.StatusOK, rec)
}
