import { api } from "./client";

// --- Tenants ---

export interface AdminTenant {
  id: string;
  name: string;
  created_at: string;
  updated_at: string;
  user_count: number;
  suspended_at?: string;
}

export interface AdminTenantUser {
  user_id: string;
  email: string;
  name: string;
  role: string;
  identity_id?: string;
}

export interface AdminTenantDetail {
  id: string;
  name: string;
  created_at: string;
  updated_at: string;
  suspended_at?: string;
  users: AdminTenantUser[];
}

export function getAdminTenants() {
  return api<{ tenants: AdminTenant[] }>("/api/admin/tenants");
}

export function createTenant(name: string) {
  return api<{ tenant: AdminTenant }>("/api/admin/tenants", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

export function getTenant(id: string) {
  return api<{ tenant: AdminTenantDetail }>(`/api/admin/tenants/${id}`);
}

export interface AIFeatureUsage {
  feature: string;
  calls: number;
  prompt_tokens: number;
  response_tokens: number;
}

export function getTenantAIUsage(id: string) {
  return api<{ since: string; features: AIFeatureUsage[] }>(
    `/api/admin/tenants/${id}/ai-usage`,
  );
}

export function updateTenant(id: string, name: string) {
  return api<{ message: string }>(`/api/admin/tenants/${id}`, {
    method: "PUT",
    body: JSON.stringify({ name }),
  });
}

export function deleteTenant(id: string) {
  return api<{ message: string }>(`/api/admin/tenants/${id}`, {
    method: "DELETE",
  });
}

export function suspendTenant(id: string) {
  return api<{ message: string; sessions_revoked: number }>(
    `/api/admin/tenants/${id}/suspend`,
    { method: "POST" },
  );
}

export function unsuspendTenant(id: string) {
  return api<{ message: string }>(`/api/admin/tenants/${id}/unsuspend`, {
    method: "POST",
  });
}

export function inviteAdmin(id: string, email: string, name?: string) {
  return api<{
    message: string;
    identity_created: boolean;
    identity_id: string;
  }>(`/api/admin/tenants/${id}/invite`, {
    method: "POST",
    body: JSON.stringify({ email, name }),
  });
}

// --- SSO ---

export interface SSOConfig {
  id: string;
  tenant_id: string;
  protocol: string;
  entity_id?: string;
  sso_url?: string;
  certificate?: string;
  client_id?: string;
  issuer_url?: string;
  enabled: boolean;
  auto_provision: boolean;
  default_role: string;
  created_at: string;
  updated_at: string;
}

export interface SSOConfigUpdate {
  protocol: string;
  entity_id?: string;
  sso_url?: string;
  certificate?: string;
  client_id?: string;
  client_secret?: string;
  issuer_url?: string;
  enabled: boolean;
  auto_provision: boolean;
  default_role: string;
}

export function getSSOConfig(tenantId: string) {
  return api<SSOConfig>(`/api/admin/tenants/${tenantId}/sso`);
}

export function updateSSOConfig(tenantId: string, config: SSOConfigUpdate) {
  return api<SSOConfig>(`/api/admin/tenants/${tenantId}/sso`, {
    method: "PUT",
    body: JSON.stringify(config),
  });
}

// --- Identities ---

export interface AdminIdentity {
  id: string;
  email: string;
  name: string;
  is_super_admin: boolean;
  activated: boolean;
  suspended_at?: string;
  created_at: string;
  tenant_count: number;
}

export interface AdminIdentityDetail {
  id: string;
  email: string;
  name: string;
  is_super_admin: boolean;
  activated: boolean;
  suspended_at?: string;
  created_at: string;
  tenants: { TenantID: string; Name: string; Role: string }[];
}

export function getAdminIdentities(limit = 50, offset = 0) {
  return api<{ identities: AdminIdentity[] }>(
    `/api/admin/identities?limit=${limit}&offset=${offset}`,
  );
}

export function createIdentity(email: string, name: string, password: string) {
  return api<{ identity: AdminIdentity }>("/api/admin/identities", {
    method: "POST",
    body: JSON.stringify({ email, name, password }),
  });
}

export function getIdentity(id: string) {
  return api<{ identity: AdminIdentityDetail }>(
    `/api/admin/identities/${id}`,
  );
}

export function resetPassword(id: string, password: string) {
  return api<{ message: string }>(
    `/api/admin/identities/${id}/reset-password`,
    {
      method: "POST",
      body: JSON.stringify({ password }),
    },
  );
}

export function setSuperAdmin(id: string, isSuperAdmin: boolean) {
  return api<{ message: string }>(`/api/admin/identities/${id}/super-admin`, {
    method: "POST",
    body: JSON.stringify({ is_super_admin: isSuperAdmin }),
  });
}

export function suspendIdentity(id: string) {
  return api<{ message: string; sessions_revoked: number }>(
    `/api/admin/identities/${id}/suspend`,
    { method: "POST" },
  );
}

export function unsuspendIdentity(id: string) {
  return api<{ message: string }>(`/api/admin/identities/${id}/unsuspend`, {
    method: "POST",
  });
}

export function deleteIdentity(id: string) {
  return api<{ message: string }>(`/api/admin/identities/${id}`, {
    method: "DELETE",
  });
}

export function resendInvite(id: string) {
  return api<{ message: string }>(
    `/api/admin/identities/${id}/resend-invite`,
    { method: "POST" },
  );
}

export function changeIdentityEmail(id: string, email: string) {
  return api<{ message: string }>(`/api/admin/identities/${id}/email`, {
    method: "PUT",
    body: JSON.stringify({ email }),
  });
}

export function addIdentityToTenant(
  identityId: string,
  tenantId: string,
  role: string,
) {
  return api<{ message: string }>(
    `/api/admin/identities/${identityId}/tenants`,
    {
      method: "POST",
      body: JSON.stringify({ tenant_id: tenantId, role }),
    },
  );
}

export interface AdminPasskey {
  id: string;
  friendly_name: string | null;
  last_used_at: string | null;
  created_at: string;
}

export function listIdentityPasskeys(identityId: string) {
  return api<{ passkeys: AdminPasskey[] }>(
    `/api/admin/identities/${identityId}/passkeys`,
  );
}

export function deleteIdentityPasskey(identityId: string, passkeyId: string) {
  return api<{ message: string }>(
    `/api/admin/identities/${identityId}/passkeys/${passkeyId}`,
    { method: "DELETE" },
  );
}
