import { api } from "./client";

export type MemberRole =
  | "administrator"
  | "fso"
  | "read_only_fso"
  | "individual_contributor";

export interface Member {
  user_id: string;
  name: string;
  email: string;
  role: MemberRole;
  sub_org_id: string | null;
  sub_org_name: string | null;
}

export interface SubOrg {
  id: string;
  tenant_id: string;
  name: string;
  member_count: number;
  primary_fso_user_id: string | null;
  primary_fso_name: string | null;
  primary_fso_email: string | null;
  created_at: string;
  updated_at: string;
}

export interface FSOContact {
  name: string;
  email: string;
}

export const ROLE_LABELS: Record<MemberRole, string> = {
  administrator: "Administrator",
  fso: "FSO",
  read_only_fso: "Read-Only FSO",
  individual_contributor: "Individual Contributor",
};

export function listMembers(search?: string) {
  const qs = search ? `?search=${encodeURIComponent(search)}` : "";
  return api<{ members: Member[] }>(`/api/v1/admin/members${qs}`);
}

export function inviteMember(email: string, name: string, role: MemberRole, subOrgId?: string) {
  return api<{ message: string; identity_id: string }>("/api/v1/admin/invite", {
    method: "POST",
    body: JSON.stringify({
      email,
      name,
      role,
      ...(subOrgId ? { sub_org_id: subOrgId } : {}),
    }),
  });
}

export interface BulkInviteRow {
  email: string;
  name?: string;
  role: MemberRole;
  sub_org_id?: string | null;
}

export interface BulkInviteFailure {
  email: string;
  reason: string;
}

export interface BulkInviteResult {
  succeeded: number;
  failed: BulkInviteFailure[];
}

export function bulkInviteMembers(invites: BulkInviteRow[], skipEmail: boolean) {
  return api<BulkInviteResult>("/api/v1/admin/invite/bulk", {
    method: "POST",
    body: JSON.stringify({ invites, skip_email: skipEmail }),
  });
}

export function updateMemberRole(userId: string, role: MemberRole) {
  return api<{ message: string }>(`/api/v1/admin/members/${userId}/role`, {
    method: "PATCH",
    body: JSON.stringify({ role }),
  });
}

export function setMemberSubOrg(userId: string, subOrgId: string) {
  return api<{ message: string }>(`/api/v1/admin/members/${userId}/sub-org`, {
    method: "PUT",
    body: JSON.stringify({ sub_org_id: subOrgId }),
  });
}

export function removeMember(userId: string) {
  return api<{ message: string }>(`/api/v1/admin/members/${userId}`, {
    method: "DELETE",
  });
}

// --- Sub-organization management ---

export function listSubOrgs() {
  return api<{ sub_orgs: SubOrg[] }>("/api/v1/admin/sub-orgs");
}

export function createSubOrg(name: string) {
  return api<SubOrg>("/api/v1/admin/sub-orgs", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

export function updateSubOrg(id: string, name: string) {
  return api<{ status: string }>(`/api/v1/admin/sub-orgs/${id}`, {
    method: "PUT",
    body: JSON.stringify({ name }),
  });
}

export function deleteSubOrg(id: string) {
  return api<void>(`/api/v1/admin/sub-orgs/${id}`, { method: "DELETE" });
}

export function setPrimaryFSO(subOrgId: string, userId: string | null) {
  return api<{ status: string }>(
    `/api/v1/admin/sub-orgs/${subOrgId}/primary-fso`,
    {
      method: "PUT",
      body: JSON.stringify({ user_id: userId }),
    },
  );
}

export function getMyFSO() {
  return api<{ fso: FSOContact | null }>("/api/v1/me/fso");
}

// --- SSO configuration (tenant admin) ---

export interface TenantSSOConfig {
  id: string;
  tenant_id: string;
  protocol: "oidc" | "saml";
  entity_id?: string;
  sso_url?: string;
  certificate?: string;
  client_id?: string;
  issuer_url?: string;
  enabled: boolean;
  auto_provision: boolean;
  default_role: MemberRole;
  created_at: string;
  updated_at: string;
}

export interface TenantSSOConfigUpdate {
  protocol: "oidc" | "saml";
  entity_id?: string;
  sso_url?: string;
  certificate?: string;
  client_id?: string;
  client_secret?: string;
  issuer_url?: string;
  enabled: boolean;
  auto_provision: boolean;
  default_role: MemberRole;
}

export function getTenantSSOConfig() {
  return api<TenantSSOConfig>("/api/v1/admin/sso");
}

export function updateTenantSSOConfig(config: TenantSSOConfigUpdate) {
  return api<TenantSSOConfig>("/api/v1/admin/sso", {
    method: "PUT",
    body: JSON.stringify(config),
  });
}

export interface SSOEmailDomain {
  id: string;
  domain: string;
  sso_configuration_id: string;
  created_at: string;
}

export function listSSOEmailDomains() {
  return api<{ domains: SSOEmailDomain[] }>("/api/v1/admin/sso/domains");
}

export function addSSOEmailDomain(domain: string) {
  return api<SSOEmailDomain>("/api/v1/admin/sso/domains", {
    method: "POST",
    body: JSON.stringify({ domain }),
  });
}

export function removeSSOEmailDomain(domain: string) {
  return api<void>(`/api/v1/admin/sso/domains/${encodeURIComponent(domain)}`, {
    method: "DELETE",
  });
}
