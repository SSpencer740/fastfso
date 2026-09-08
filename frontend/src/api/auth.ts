import { api } from "./client";

export interface LoginResponse {
  state: "pre_auth" | "pre_tenant";
  available_2fa?: string[];
  setup_2fa_required?: boolean;
}

export interface Tenant {
  id: string;
  name: string;
  role: string;
  suspended: boolean;
}

export interface TenantsResponse {
  tenants: Tenant[];
  can_access_admin_panel: boolean;
}

export interface User {
  id: string;
  name: string;
  role: string;
  tenant_name: string;
  tenant_id: string;
}

export interface SelectTenantResponse {
  state: "authenticated";
  user: User;
}

export interface SelectAdminResponse {
  state: "authenticated";
  is_super_admin: boolean;
}

export interface Identity {
  id: string;
  email: string;
  name: string;
  is_super_admin: boolean;
}

export interface MeResponse {
  state: string;
  identity: Identity;
  is_super_admin: boolean;
  user?: User;
  can_switch_context?: boolean;
}

export interface SessionInfo {
  id: string;
  state: string;
  ip_address: string;
  user_agent: string;
  auth_method: string;
  created_at: string;
  last_active_at: string;
  expires_at: string;
}

export interface SessionsResponse {
  sessions: SessionInfo[];
  current_session_id: string;
}

export interface TwoFAMethodsResponse {
  methods: string[];
}

export interface SSOOption {
  tenant_name: string;
  sso_url: string;
}

export interface IdentifyResponse {
  methods: string[];
  sso_options: SSOOption[];
}

export function identify(email: string) {
  return api<IdentifyResponse>("/api/auth/login/identify", {
    method: "POST",
    body: JSON.stringify({ email }),
  });
}

export function login(email: string, password: string) {
  return api<LoginResponse>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

export function getMe() {
  return api<MeResponse>("/api/auth/me");
}

export function getTenants() {
  return api<TenantsResponse>("/api/auth/tenants");
}

export function selectTenant(tenantId: string) {
  return api<SelectTenantResponse>("/api/auth/select-tenant", {
    method: "POST",
    body: JSON.stringify({ tenant_id: tenantId }),
  });
}

export function selectAdmin() {
  return api<SelectAdminResponse>("/api/auth/select-admin", {
    method: "POST",
  });
}

export function logout() {
  return api<{ message: string }>("/api/auth/logout", {
    method: "POST",
  });
}

export function getSessions() {
  return api<SessionsResponse>("/api/auth/sessions");
}

export function revokeSession(sessionId: string) {
  return api<{ message: string }>(`/api/auth/sessions/${sessionId}`, {
    method: "DELETE",
  });
}

export function revokeAllOtherSessions() {
  return api<{ message: string; revoked: number }>("/api/auth/sessions", {
    method: "DELETE",
  });
}

export function get2FAMethods() {
  return api<TwoFAMethodsResponse>("/api/auth/2fa/methods");
}

export function switchContext() {
  return api<{ state: string }>("/api/auth/switch-context", {
    method: "POST",
  });
}

export interface InviteInfoResponse {
  email: string;
  name: string;
  tenant_name: string;
}

export function getInviteInfo(token: string) {
  return api<InviteInfoResponse>("/api/auth/invite/info", {
    method: "POST",
    body: JSON.stringify({ token }),
  });
}

export function acceptInvite(
  token: string,
  password: string,
  name?: string,
) {
  return api<{ state: string }>("/api/auth/invite/accept", {
    method: "POST",
    body: JSON.stringify({ token, password, name }),
  });
}

export function requestPasswordReset(email: string) {
  return api<{ message: string }>("/api/auth/password-reset/request", {
    method: "POST",
    body: JSON.stringify({ email }),
  });
}

export function confirmPasswordReset(
  token: string,
  password: string,
  totpCode?: string,
) {
  return api<{ message: string }>("/api/auth/password-reset/confirm", {
    method: "POST",
    body: JSON.stringify({
      token,
      password,
      ...(totpCode ? { totp_code: totpCode } : {}),
    }),
  });
}
