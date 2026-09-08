package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fastfso/fastfso/backend/internal/audit"
	"github.com/fastfso/fastfso/backend/internal/env"
	"github.com/fastfso/fastfso/backend/internal/httputil"
	"github.com/fastfso/fastfso/backend/internal/identity"
)

// AdminListSessions lists all active sessions (super admin).
func (h *Handler) AdminListSessions(c *gin.Context) {
	limit, offset := httputil.ParsePagination(c, 50)

	sessions, err := h.store.ListAllActiveSessions(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error("failed to list admin sessions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if sessions == nil {
		sessions = []AdminSessionRow{}
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// AdminRevokeSession revokes any session (super admin).
func (h *Handler) AdminRevokeSession(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	targetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
		return
	}

	if err := h.sessions.Revoke(c.Request.Context(), targetID, &sess.IdentityID, "admin_revoked"); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "session revoked"})
}

// AdminListAudit queries the audit log (super admin).
func (h *Handler) AdminListAudit(c *gin.Context) {
	limit, offset := httputil.ParsePagination(c, 50)

	var identityID *uuid.UUID
	if idStr := c.Query("identity_id"); idStr != "" {
		id, err := uuid.Parse(idStr)
		if err == nil {
			identityID = &id
		}
	}

	entries, err := h.audit.Query(c.Request.Context(), identityID, limit, offset)
	if err != nil {
		h.logger.Error("failed to query audit log", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

// TenantAuditLog queries the audit log scoped to the current tenant (administrator + fso).
func (h *Handler) TenantAuditLog(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok || sess.TenantID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	limit, offset := httputil.ParsePagination(c, 50)
	action := c.Query("action")
	search := c.Query("search")

	entries, total, err := h.audit.QueryByTenant(c.Request.Context(), *sess.TenantID, action, search, limit, offset)
	if err != nil {
		h.logger.Error("failed to query tenant audit log", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"entries": entries, "total": total})
}

// AdminListTenants lists all tenants (super admin).
func (h *Handler) AdminListTenants(c *gin.Context) {
	tenants, err := h.store.ListAllTenants(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to list tenants", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if tenants == nil {
		tenants = []AdminTenantRow{}
	}

	c.JSON(http.StatusOK, gin.H{"tenants": tenants})
}

// AdminDeleteTenant soft-deletes a tenant (super admin).
func (h *Handler) AdminDeleteTenant(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	if err := h.store.DeleteTenant(c.Request.Context(), tenantID); err != nil {
		h.logger.Error("failed to delete tenant", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTenantDeleted,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, gin.H{"message": "tenant deleted"})
}

type createTenantRequest struct {
	Name string `json:"name" binding:"required"`
}

// AdminCreateTenant creates a new tenant (super admin).
func (h *Handler) AdminCreateTenant(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	var req createTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	tenant, err := h.store.CreateTenant(c.Request.Context(), req.Name)
	if err != nil {
		h.logger.Error("failed to create tenant", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionTenantCreated,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"tenant_id": tenant.ID, "tenant_name": tenant.Name},
	})

	c.JSON(http.StatusCreated, gin.H{"tenant": tenant})
}

// AdminGetTenant returns tenant details with user list (super admin).
func (h *Handler) AdminGetTenant(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	detail, err := h.store.GetTenantDetail(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("failed to get tenant detail", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tenant": detail})
}

type updateTenantRequest struct {
	Name string `json:"name" binding:"required"`
}

// AdminUpdateTenant renames a tenant (super admin).
func (h *Handler) AdminUpdateTenant(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	var req updateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	if err := h.store.UpdateTenant(c.Request.Context(), tenantID, req.Name); err != nil {
		h.logger.Error("failed to update tenant", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTenantUpdated,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"new_name": req.Name},
	})

	c.JSON(http.StatusOK, gin.H{"message": "tenant updated"})
}

// AdminListIdentities lists all identities (super admin).
func (h *Handler) AdminListIdentities(c *gin.Context) {
	limit, offset := httputil.ParsePagination(c, 50)

	identities, err := h.store.ListAllIdentities(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error("failed to list identities", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if identities == nil {
		identities = []AdminIdentityRow{}
	}

	c.JSON(http.StatusOK, gin.H{"identities": identities})
}

type createIdentityRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// AdminCreateIdentity creates a new identity (super admin).
func (h *Handler) AdminCreateIdentity(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	var req createIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email, name, and password (min 8 chars) are required"})
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	ident, err := h.identities.Create(c.Request.Context(), req.Email, req.Name, &hash)
	if err != nil {
		h.logger.Error("failed to create identity", "error", err)
		c.JSON(http.StatusConflict, gin.H{"error": "identity already exists or invalid data"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionIdentityCreated,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"created_identity_id": ident.ID, "email": ident.Email},
	})

	c.JSON(http.StatusCreated, gin.H{"identity": gin.H{
		"id":             ident.ID,
		"email":          ident.Email,
		"name":           ident.Name,
		"is_super_admin": ident.IsSuperAdmin,
		"activated":      ident.Activated,
		"created_at":     ident.CreatedAt,
	}})
}

// AdminGetIdentity returns identity details with tenant memberships (super admin).
func (h *Handler) AdminGetIdentity(c *gin.Context) {
	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid identity ID"})
		return
	}

	detail, err := h.store.GetIdentityDetail(c.Request.Context(), identityID)
	if err != nil {
		h.logger.Error("failed to get identity detail", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "identity not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"identity": detail})
}

type resetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=8"`
}

// AdminResetPassword sets a new password for an identity (super admin).
func (h *Handler) AdminResetPassword(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid identity ID"})
		return
	}

	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password (min 8 chars) is required"})
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.identities.UpdatePassword(c.Request.Context(), identityID, hash); err != nil {
		h.logger.Error("failed to reset password", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "identity not found"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionPasswordReset,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"target_identity_id": identityID},
	})

	c.JSON(http.StatusOK, gin.H{"message": "password reset"})
}

type addIdentityToTenantRequest struct {
	TenantID uuid.UUID `json:"tenant_id" binding:"required"`
	Role     string    `json:"role" binding:"required"`
}

var validRoles = map[string]bool{
	"administrator":          true,
	"fso":                    true,
	"read_only_fso":          true,
	"individual_contributor": true,
}

// AdminAddIdentityToTenant adds an identity to a tenant with a role (super admin).
func (h *Handler) AdminAddIdentityToTenant(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid identity ID"})
		return
	}

	var req addIdentityToTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id and role are required"})
		return
	}

	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}

	if _, err := h.store.AddIdentityToTenant(c.Request.Context(), identityID, req.TenantID, req.Role); err != nil {
		h.logger.Error("failed to add identity to tenant", "error", err)
		c.JSON(http.StatusConflict, gin.H{"error": "identity already in this tenant or invalid tenant"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   &req.TenantID,
		Action:     audit.ActionIdentityAddedToTenant,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"target_identity_id": identityID, "role": req.Role},
	})

	c.JSON(http.StatusCreated, gin.H{"message": "identity added to tenant"})
}

type setSuperAdminRequest struct {
	IsSuperAdmin bool `json:"is_super_admin"`
}

// AdminSetSuperAdmin grants or revokes super admin status for an identity.
func (h *Handler) AdminSetSuperAdmin(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid identity ID"})
		return
	}

	var req setSuperAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_super_admin is required"})
		return
	}

	// Prevent self-demotion
	if identityID == sess.IdentityID && !req.IsSuperAdmin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot remove your own super admin status"})
		return
	}

	if err := h.identities.SetSuperAdmin(c.Request.Context(), identityID, req.IsSuperAdmin); err != nil {
		h.logger.Error("failed to set super admin", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "identity not found"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionSuperAdminSet,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"target_identity_id": identityID, "is_super_admin": req.IsSuperAdmin},
	})

	c.JSON(http.StatusOK, gin.H{"message": "super admin updated"})
}

// AdminSuspendIdentity suspends an identity and revokes all its active sessions.
func (h *Handler) AdminSuspendIdentity(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid identity ID"})
		return
	}

	// Prevent self-suspension
	if identityID == sess.IdentityID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot suspend yourself"})
		return
	}

	ctx := c.Request.Context()

	// Check if this would leave no active super admins
	ident, err := h.identities.GetByID(ctx, identityID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "identity not found"})
		return
	}

	if ident.IsSuperAdmin {
		count, err := h.identities.CountActiveSuperAdmins(ctx)
		if err != nil {
			h.logger.Error("failed to count active super admins", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if count <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot suspend the last active super admin"})
			return
		}
	}

	if err := h.identities.Suspend(ctx, identityID); err != nil {
		h.logger.Error("failed to suspend identity", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "identity not found or already suspended"})
		return
	}

	// Revoke all sessions for this identity
	revoked, err := h.sessions.RevokeByIdentity(ctx, identityID, "identity_suspended")
	if err != nil {
		h.logger.Error("failed to revoke sessions for suspended identity", "error", err)
	}

	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionIdentitySuspended,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"target_identity_id": identityID, "sessions_revoked": revoked},
	})

	c.JSON(http.StatusOK, gin.H{"message": "identity suspended", "sessions_revoked": revoked})
}

// AdminUnsuspendIdentity removes the suspension from an identity.
func (h *Handler) AdminUnsuspendIdentity(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid identity ID"})
		return
	}

	if err := h.identities.Unsuspend(c.Request.Context(), identityID); err != nil {
		h.logger.Error("failed to unsuspend identity", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "identity not found or not suspended"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionIdentityUnsuspended,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"target_identity_id": identityID},
	})

	c.JSON(http.StatusOK, gin.H{"message": "identity unsuspended"})
}

// AdminDeleteIdentity hard-deletes an identity (super admin).
func (h *Handler) AdminDeleteIdentity(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid identity ID"})
		return
	}

	// Prevent self-deletion
	if identityID == sess.IdentityID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete yourself"})
		return
	}

	ctx := c.Request.Context()

	// Check if this would leave no active super admins
	ident, err := h.identities.GetByID(ctx, identityID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "identity not found"})
		return
	}

	if ident.IsSuperAdmin {
		count, err := h.identities.CountActiveSuperAdmins(ctx)
		if err != nil {
			h.logger.Error("failed to count active super admins", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if count <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete the last active super admin"})
			return
		}
	}

	if err := h.identities.Delete(ctx, identityID); err != nil {
		h.logger.Error("failed to delete identity", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete identity"})
		return
	}

	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionIdentityDeleted,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"deleted_identity_id": identityID, "email": ident.Email},
	})

	c.JSON(http.StatusOK, gin.H{"message": "identity deleted"})
}

// adminTenantID resolves the tenant ID from either a URL param (super-admin)
// or the current session (tenant-admin).
func adminTenantID(c *gin.Context) (uuid.UUID, bool) {
	// Super-admin: tenant ID from URL
	if idStr := c.Param("id"); idStr != "" {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
			return uuid.Nil, false
		}
		return id, true
	}
	// Tenant-admin: tenant ID from session
	sess, ok := GetSession(c)
	if !ok || sess.TenantID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no tenant"})
		return uuid.Nil, false
	}
	return *sess.TenantID, true
}

// AdminSuspendTenant suspends a tenant and revokes all its active sessions.
func (h *Handler) AdminSuspendTenant(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	tenantID, ok := adminTenantID(c)
	if !ok {
		return
	}

	if err := h.store.SuspendTenant(c.Request.Context(), tenantID); err != nil {
		h.logger.Error("failed to suspend tenant", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found or already suspended"})
		return
	}

	// Bulk revoke all sessions for this tenant
	revoked, err := h.sessions.RevokeByTenant(c.Request.Context(), tenantID, "tenant_suspended")
	if err != nil {
		h.logger.Error("failed to revoke sessions for suspended tenant", "error", err)
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTenantSuspended,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"sessions_revoked": revoked},
	})

	c.JSON(http.StatusOK, gin.H{"message": "tenant suspended", "sessions_revoked": revoked})
}

// AdminUnsuspendTenant removes the suspension from a tenant.
func (h *Handler) AdminUnsuspendTenant(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	tenantID, ok := adminTenantID(c)
	if !ok {
		return
	}

	if err := h.store.UnsuspendTenant(c.Request.Context(), tenantID); err != nil {
		h.logger.Error("failed to unsuspend tenant", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found or not suspended"})
		return
	}

	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionTenantUnsuspended,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, gin.H{"message": "tenant unsuspended"})
}

// TenantListMembers returns all members of the caller's tenant (tenant admin).
func (h *Handler) TenantListMembers(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok || sess.TenantID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no tenant"})
		return
	}
	members, err := h.store.ListTenantMembers(c.Request.Context(), *sess.TenantID, c.Query("search"))
	if err != nil {
		h.logger.Error("failed to list tenant members", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if members == nil {
		members = []TenantMemberRow{}
	}
	c.JSON(http.StatusOK, gin.H{"members": members})
}

type updateMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// TenantUpdateMemberRole updates a member's role within the caller's tenant (tenant admin).
func (h *Handler) TenantUpdateMemberRole(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok || sess.TenantID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no tenant"})
		return
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}
	var req updateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role is required"})
		return
	}
	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}
	if err := h.store.UpdateMemberRole(c.Request.Context(), userID, *sess.TenantID, req.Role); err != nil {
		h.logger.Error("failed to update member role", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}
	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   sess.TenantID,
		Action:     audit.ActionMemberRoleUpdated,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"user_id": userID, "new_role": req.Role},
	})
	c.JSON(http.StatusOK, gin.H{"message": "role updated"})
}

// TenantRemoveMember removes a member from the caller's tenant (tenant admin).
func (h *Handler) TenantRemoveMember(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok || sess.TenantID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no tenant"})
		return
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}
	if err := h.store.RemoveMember(c.Request.Context(), userID, *sess.TenantID); err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			h.logger.Error("failed to remove member", "error", err, "user_id", userID, "tenant_id", *sess.TenantID)
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
			return
		}
		// Orphan-identity cleanup failures are warnings, not blockers — the
		// membership was already removed (the user's reason for clicking),
		// so we report success to the UI but log the leftover identity
		// row for follow-up.
		if IsOrphanCleanupError(err) {
			h.logger.Warn("orphan identity not cleaned up after member remove",
				"error", err, "user_id", userID, "tenant_id", *sess.TenantID)
		} else {
			h.logger.Error("failed to remove member", "error", err, "user_id", userID, "tenant_id", *sess.TenantID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove member"})
			return
		}
	}
	h.audit.Log(c.Request.Context(), audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   sess.TenantID,
		Action:     audit.ActionMemberRemoved,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"user_id": userID},
	})
	c.JSON(http.StatusOK, gin.H{"message": "member removed"})
}

type inviteAdminRequest struct {
	Email    string  `json:"email" binding:"required,email"`
	Name     string  `json:"name"`
	Role     string  `json:"role"`
	SubOrgID *string `json:"sub_org_id"`
}

type bulkInviteRequest struct {
	Invites   []inviteAdminRequest `json:"invites" binding:"required,min=1,max=500"`
	SkipEmail bool                 `json:"skip_email"`
}

type bulkInviteFailure struct {
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

type bulkInviteResponse struct {
	Succeeded int                 `json:"succeeded"`
	Failed    []bulkInviteFailure `json:"failed"`
}

// inviteOneParams is the internal shape both per-user and bulk invite paths
// pass to inviteOneMember.
type inviteOneParams struct {
	Email     string
	Name      string
	Role      string
	SubOrgID  *uuid.UUID
	SkipEmail bool
}

type inviteOneResult struct {
	IdentityID      uuid.UUID
	UserID          uuid.UUID
	IdentityCreated bool
}

// inviteOneMember finds or creates the identity, adds them to the tenant
// with the requested role, optionally moves them to a specific sub-org, and
// (unless SkipEmail) sends whichever invite email is appropriate for the
// invitee's auth situation. Each branch's error is returned as-is so the
// caller can decide whether to fail the whole request (per-user invite) or
// record a per-row failure (bulk invite).
func (h *Handler) inviteOneMember(ctx context.Context, tenantID uuid.UUID, p inviteOneParams) (*inviteOneResult, error) {
	role := p.Role
	if role == "" {
		role = "administrator"
	}
	if !validRoles[role] {
		return nil, fmt.Errorf("invalid role %q", role)
	}

	// Find or create identity.
	ident, err := h.identities.GetByEmail(ctx, p.Email)
	created := false
	if err != nil {
		name := p.Name
		if name == "" {
			if i := strings.Index(p.Email, "@"); i > 0 {
				name = strings.ToUpper(p.Email[:1]) + p.Email[1:i]
			} else {
				name = p.Email
			}
		}
		ident, err = h.identities.Create(ctx, p.Email, name, nil)
		if err != nil {
			return nil, fmt.Errorf("create identity: %w", err)
		}
		created = true
	}

	userID, err := h.store.AddIdentityToTenant(ctx, ident.ID, tenantID, role)
	if err != nil {
		return nil, fmt.Errorf("identity already in this tenant: %w", err)
	}

	// Sub-org assignment (optional). The default sub-org trigger has
	// already placed the new user in "Default"; SetUserSubOrg replaces
	// that with the requested sub-org and validates tenant ownership.
	if p.SubOrgID != nil && h.subOrgs != nil {
		if err := h.subOrgs.SetUserSubOrg(ctx, tenantID, userID, *p.SubOrgID); err != nil {
			// Roll back the membership row so the admin can retry with a
			// valid sub-org without hitting "already in tenant."
			_ = h.store.RemoveMember(ctx, userID, tenantID)
			return nil, fmt.Errorf("assign sub-org: %w", err)
		}
	}

	// Email selection (skipped for bulk silent provisioning).
	if !p.SkipEmail && h.email != nil {
		detail, derr := h.store.GetTenantDetail(ctx, tenantID)
		if derr != nil {
			h.logger.Error("failed to get tenant detail for invite email", "error", derr)
		} else {
			h.sendInviteEmail(ctx, tenantID, p.Email, detail.Name, ident, created)
		}
	}

	return &inviteOneResult{IdentityID: ident.ID, UserID: userID, IdentityCreated: created}, nil
}

// sendInviteEmail picks among SSO / setup-token / regular invite based on
// the invitee's email domain and identity state. Logs and swallows email
// failures — the membership has already been written, and an email retry
// is independent of that.
func (h *Handler) sendInviteEmail(ctx context.Context, tenantID uuid.UUID, email, tenantName string, ident *identity.Identity, created bool) {
	appURL := env.FrontendURL()
	ssoMatched := false
	if domain := emailDomain(email); domain != "" {
		matched, err := h.store.HasEnabledSSODomain(ctx, tenantID, domain)
		if err != nil {
			h.logger.Warn("sso domain lookup for invite", "error", err, "domain", domain)
		}
		ssoMatched = matched
	}
	switch {
	case ssoMatched:
		if err := h.email.SendInviteSSO(ctx, email, tenantName); err != nil {
			h.logger.Error("failed to send sso invite email", "error", err)
		}
	case created || !ident.HasPassword():
		if h.invites != nil {
			token, err := h.invites.Generate(ctx, ident.ID)
			if err != nil {
				h.logger.Error("failed to generate invite token", "error", err)
			} else if err := h.email.SendAdminInviteSetup(ctx, email, tenantName, appURL, token); err != nil {
				h.logger.Error("failed to send invite setup email", "error", err)
			}
		}
	default:
		if err := h.email.SendAdminInvite(ctx, email, tenantName, appURL); err != nil {
			h.logger.Error("failed to send admin invite email", "error", err)
		}
	}
}

// parseSubOrgID converts a string field into a *uuid.UUID, mapping nil and
// empty strings to nil.
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

// emailDomain returns the lowercased domain portion of an email address, or
// "" if the address is malformed. Used by the invite path to decide whether
// the recipient should get the SSO invite or the standard setup email.
func emailDomain(addr string) string {
	addr = strings.ToLower(strings.TrimSpace(addr))
	i := strings.LastIndex(addr, "@")
	if i < 0 || i == len(addr)-1 {
		return ""
	}
	return addr[i+1:]
}

// AdminInviteAdmin invites a single member. Thin wrapper over inviteOneMember.
func (h *Handler) AdminInviteAdmin(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	tenantID, ok := adminTenantID(c)
	if !ok {
		return
	}

	var req inviteAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
		return
	}
	subOrgID, err := parseSubOrgID(req.SubOrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sub_org_id"})
		return
	}

	ctx := c.Request.Context()
	result, err := h.inviteOneMember(ctx, tenantID, inviteOneParams{
		Email:    req.Email,
		Name:     req.Name,
		Role:     req.Role,
		SubOrgID: subOrgID,
	})
	if err != nil {
		h.logger.Error("invite member", "error", err, "email", req.Email)
		switch {
		case strings.Contains(err.Error(), "invalid role"):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		case strings.Contains(err.Error(), "already in this tenant"):
			c.JSON(http.StatusConflict, gin.H{"error": "identity already in this tenant"})
		case strings.Contains(err.Error(), "assign sub-org"):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sub-organization"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to invite member"})
		}
		return
	}

	role := req.Role
	if role == "" {
		role = "administrator"
	}
	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		TenantID:   &tenantID,
		Action:     audit.ActionAdminInvited,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata: map[string]any{
			"invited_email":    req.Email,
			"role":             role,
			"identity_created": result.IdentityCreated,
		},
	})

	c.JSON(http.StatusCreated, gin.H{
		"message":          "member invited",
		"identity_created": result.IdentityCreated,
		"identity_id":      result.IdentityID,
	})
}

// AdminBulkInvite invites a list of members in one request. Failures are
// reported per-row so an admin pasting a CSV can fix the bad lines and
// re-submit just those. Optional skip_email suppresses every invite email
// — useful when the IT team wants to silently pre-provision the allowed
// list and let the IdP onboarding flow do the rest.
func (h *Handler) AdminBulkInvite(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	tenantID, ok := adminTenantID(c)
	if !ok {
		return
	}

	var req bulkInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()
	resp := bulkInviteResponse{Failed: []bulkInviteFailure{}}
	for _, inv := range req.Invites {
		subOrgID, err := parseSubOrgID(inv.SubOrgID)
		if err != nil {
			resp.Failed = append(resp.Failed, bulkInviteFailure{Email: inv.Email, Reason: "invalid sub_org_id"})
			continue
		}
		result, err := h.inviteOneMember(ctx, tenantID, inviteOneParams{
			Email:     inv.Email,
			Name:      inv.Name,
			Role:      inv.Role,
			SubOrgID:  subOrgID,
			SkipEmail: req.SkipEmail,
		})
		if err != nil {
			resp.Failed = append(resp.Failed, bulkInviteFailure{Email: inv.Email, Reason: bulkReason(err)})
			continue
		}
		resp.Succeeded++

		// Per-row audit log so each invite is individually traceable.
		role := inv.Role
		if role == "" {
			role = "administrator"
		}
		h.audit.Log(ctx, audit.LogParams{
			IdentityID: &sess.IdentityID,
			SessionID:  &sess.ID,
			TenantID:   &tenantID,
			Action:     audit.ActionAdminInvited,
			IPAddress:  c.ClientIP(),
			UserAgent:  c.GetHeader("User-Agent"),
			Metadata: map[string]any{
				"invited_email":    inv.Email,
				"role":             role,
				"identity_created": result.IdentityCreated,
				"bulk":             true,
				"skip_email":       req.SkipEmail,
			},
		})
	}

	c.JSON(http.StatusOK, resp)
}

// bulkReason maps an inviteOneMember error to the short reason string the
// frontend renders next to the offending row. Anything not classified falls
// through to a generic message — admins shouldn't see raw stack traces.
func bulkReason(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "invalid role"):
		return "invalid role"
	case strings.Contains(msg, "already in this tenant"):
		return "already a member"
	case strings.Contains(msg, "assign sub-org"):
		return "invalid sub-organization"
	case strings.Contains(msg, "create identity"):
		return "could not create identity"
	default:
		return "failed"
	}
}

// AdminResendInvite regenerates an invite token and resends the setup email.
func (h *Handler) AdminResendInvite(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid identity ID"})
		return
	}

	ctx := c.Request.Context()

	ident, err := h.identities.GetByID(ctx, identityID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "identity not found"})
		return
	}

	if ident.Activated {
		c.JSON(http.StatusBadRequest, gin.H{"error": "already activated, cannot resend"})
		return
	}

	if h.invites == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invite system not configured"})
		return
	}

	token, err := h.invites.Generate(ctx, identityID)
	if err != nil {
		h.logger.Error("failed to generate invite token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Get tenant name for the email
	memberships, err := h.store.ListTenantsByIdentity(ctx, identityID)
	tenantName := ""
	if err == nil && len(memberships) > 0 {
		tenantName = memberships[0].Name
	}

	if h.email != nil {
		appURL := env.FrontendURL()
		if err := h.email.SendAdminInviteSetup(ctx, ident.Email, tenantName, appURL, token); err != nil {
			h.logger.Error("failed to send invite setup email", "error", err)
		}
	}

	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionInviteResent,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"target_identity_id": identityID},
	})

	c.JSON(http.StatusOK, gin.H{"message": "invite resent"})
}

type changeEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// AdminChangeEmail updates the email for an unactivated identity.
func (h *Handler) AdminChangeEmail(c *gin.Context) {
	sess, ok := GetSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid identity ID"})
		return
	}

	var req changeEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid email is required"})
		return
	}

	ctx := c.Request.Context()

	ident, err := h.identities.GetByID(ctx, identityID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "identity not found"})
		return
	}

	if ident.Activated {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot change email after activation"})
		return
	}

	if err := h.identities.UpdateEmail(ctx, identityID, req.Email); err != nil {
		h.logger.Error("failed to update email", "error", err)
		c.JSON(http.StatusConflict, gin.H{"error": "email already in use or invalid"})
		return
	}

	h.audit.Log(ctx, audit.LogParams{
		IdentityID: &sess.IdentityID,
		SessionID:  &sess.ID,
		Action:     audit.ActionEmailChanged,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		Metadata:   map[string]any{"target_identity_id": identityID, "new_email": req.Email},
	})

	c.JSON(http.StatusOK, gin.H{"message": "email updated"})
}
