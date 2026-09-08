import { api } from "./client";

export interface AuditEntry {
  id: string;
  identity_id: string | null;
  action: string;
  ip_address: string;
  user_agent: string;
  identity_name: string;
  identity_email: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

// Kept in sync with the action constants in backend/internal/audit/audit.go.
// Super-admin-only actions (tenant_created, identity_*, super_admin_*) are
// intentionally omitted — tenant administrators don't see them.
export const AUDIT_ACTIONS = [
  // Auth
  "login_attempt",
  "login_success",
  "login_failure",
  "2fa_success",
  "2fa_failure",
  "logout",
  "session_revoked",
  "context_switch",
  "password_changed",
  "password_reset",
  "password_reset_requested",
  "email_changed",
  "passkey_registered",
  "passkey_removed",
  "passkey_removed_by_admin",
  "totp_enrolled",
  "totp_removed",
  "sso_configured",
  "invite_accepted",
  "invite_resent",
  // Tasks
  "task_created",
  "task_archived",
  "task_submitted",
  "task_approved",
  "task_rejected",
  // Action items
  "action_item_updated",
  "action_item_notes_updated",
  "action_item_reassigned",
  // Travel
  "travel_report_created",
  "travel_report_submitted",
  "travel_report_approved",
  "travel_report_rejected",
  // Visits
  "visit_request_submitted",
  "visit_request_reviewed",
  // Reports
  "report_submitted",
  "report_reviewed",
  // Members
  "member_invited",
  "member_role_updated",
  "member_removed",
  // Wiki
  "wiki_post_created",
  "wiki_post_updated",
  "wiki_post_deleted",
  "wiki_file_uploaded",
  "wiki_file_deleted",
  // Sub-orgs
  "sub_org_created",
  "sub_org_updated",
  "sub_org_deleted",
  "sub_org_primary_fso_set",
  "sub_org_user_assigned",
] as const;

export async function listTenantAuditLog(params: {
  action?: string;
  search?: string;
  limit?: number;
  offset?: number;
} = {}): Promise<{ entries: AuditEntry[]; total: number }> {
  const qs = new URLSearchParams();
  if (params.action) qs.set("action", params.action);
  if (params.search) qs.set("search", params.search);
  if (params.limit != null) qs.set("limit", String(params.limit));
  if (params.offset != null) qs.set("offset", String(params.offset));
  const query = qs.toString();
  return api(`/api/v1/admin/audit/${query ? `?${query}` : ""}`);
}
