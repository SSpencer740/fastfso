package audit

import (
	"time"

	"github.com/google/uuid"
)

// Actions
const (
	ActionLoginAttempt           = "login_attempt"
	ActionLoginSuccess           = "login_success"
	ActionLoginFailure           = "login_failure"
	Action2FASuccess             = "2fa_success"
	Action2FAFailure             = "2fa_failure"
	ActionTenantSelected         = "tenant_selected"
	ActionSuperAdminAccess       = "super_admin_access"
	ActionSessionRevoked         = "session_revoked"
	ActionLogout                 = "logout"
	ActionPasswordChanged        = "password_changed"
	ActionPasskeyRegistered      = "passkey_registered"
	ActionPasskeyRemoved         = "passkey_removed"
	ActionPasskeyRemovedByAdmin  = "passkey_removed_by_admin"
	ActionTOTPEnrolled           = "totp_enrolled"
	ActionTOTPRemoved            = "totp_removed"
	ActionSSOConfigured          = "sso_configured"
	ActionEmailCodeSent          = "email_code_sent"
	ActionTenantCreated          = "tenant_created"
	ActionTenantUpdated          = "tenant_updated"
	ActionIdentityCreated        = "identity_created"
	ActionPasswordReset          = "password_reset"
	ActionPasswordResetRequested = "password_reset_requested"
	ActionIdentityAddedToTenant  = "identity_added_to_tenant"
	ActionTenantDeleted          = "tenant_deleted"
	ActionTenantSuspended        = "tenant_suspended"
	ActionTenantUnsuspended      = "tenant_unsuspended"
	ActionAdminInvited           = "admin_invited"
	ActionSuperAdminSet          = "super_admin_set"
	ActionIdentitySuspended      = "identity_suspended"
	ActionIdentityUnsuspended    = "identity_unsuspended"
	ActionIdentityDeleted        = "identity_deleted"
	ActionContextSwitch          = "context_switch"
	ActionInviteAccepted         = "invite_accepted"
	ActionInviteResent           = "invite_resent"
	ActionEmailChanged           = "email_changed"
	ActionTaskCreated            = "task_created"
	ActionTaskArchived           = "task_archived"
	ActionTaskSubmitted          = "task_submitted"
	ActionTaskApproved           = "task_approved"
	ActionTaskRejected           = "task_rejected"
	ActionActionItemUpdated      = "action_item_updated"
	ActionTravelReportCreated    = "travel_report_created"
	ActionTravelReportSubmitted  = "travel_report_submitted"
	ActionTravelReportApproved   = "travel_report_approved"
	ActionTravelReportRejected   = "travel_report_rejected"
	ActionVisitRequestSubmitted  = "visit_request_submitted"
	ActionVisitRequestReviewed   = "visit_request_reviewed"
	ActionReportSubmitted        = "report_submitted"
	ActionReportReviewed         = "report_reviewed"
	ActionMemberInvited          = "member_invited"
	ActionMemberRoleUpdated      = "member_role_updated"
	ActionMemberRemoved          = "member_removed"
	ActionActionItemNotesUpdated = "action_item_notes_updated"
	ActionActionItemReassigned   = "action_item_reassigned"
	ActionWikiPostCreated        = "wiki_post_created"
	ActionWikiPostUpdated        = "wiki_post_updated"
	ActionWikiPostDeleted        = "wiki_post_deleted"
	ActionWikiFileUploaded       = "wiki_file_uploaded"
	ActionWikiFileDeleted        = "wiki_file_deleted"
	ActionSubOrgCreated          = "sub_org_created"
	ActionSubOrgUpdated          = "sub_org_updated"
	ActionSubOrgDeleted          = "sub_org_deleted"
	ActionSubOrgPrimaryFSOSet    = "sub_org_primary_fso_set"
	ActionSubOrgUserAssigned     = "sub_org_user_assigned"
	ActionClearanceUpdated       = "clearance_updated"
	ActionDD254Uploaded          = "dd254_uploaded"
	ActionDD254Viewed            = "dd254_viewed"
	ActionDD254Deleted           = "dd254_deleted"
	ActionDD254AccessGranted     = "dd254_access_granted"
	ActionDD254AccessRevoked     = "dd254_access_revoked"
	ActionVisitsExported         = "visits_exported"
)

type Entry struct {
	ID         uuid.UUID      `json:"id"`
	IdentityID *uuid.UUID     `json:"identity_id,omitempty"`
	SessionID  *uuid.UUID     `json:"session_id,omitempty"`
	TenantID   *uuid.UUID     `json:"tenant_id,omitempty"`
	Action     string         `json:"action"`
	IPAddress  string         `json:"ip_address,omitempty"`
	UserAgent  string         `json:"user_agent,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}
