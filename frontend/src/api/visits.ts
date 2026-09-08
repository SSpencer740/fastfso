import { api } from "./client";

// --- Types ---

export interface VisitRequest {
  id: string;
  tenant_id: string;
  created_by_user_id: string;
  destination_name: string;
  diss_smo_code: string;
  visit_address: string;
  visit_start_date: string;
  visit_end_date: string;
  access_level: string;
  visit_description: string;
  poc_name: string;
  poc_email: string;
  poc_phone: string;
  security_poc_name: string;
  security_poc_email: string;
  security_poc_phone: string;
  status: string;
  cloned_from_id: string | null;
  dd_254_id?: string | null;
  reviewed_by: string | null;
  reviewed_at: string | null;
  reviewer_notes: string;
  created_at: string;
  updated_at: string;
}

export interface VisitRequestRow {
  id: string;
  destination_name: string;
  visit_start_date: string;
  visit_end_date: string;
  access_level: string;
  status: string;
  submitter_name: string;
  created_at: string;
}

export interface VisitRequestStats {
  total: number;
  submitted: number;
  under_review: number;
  approved: number;
  rejected: number;
  cancelled: number;
}

export interface CreateVisitRequestParams {
  destination_name: string;
  diss_smo_code: string;
  visit_address: string;
  visit_start_date: string;
  visit_end_date: string;
  access_level: string;
  visit_description: string;
  poc_name: string;
  poc_email: string;
  poc_phone: string;
  security_poc_name: string;
  security_poc_email: string;
  security_poc_phone: string;
  cloned_from_id?: string | null;
  dd_254_id?: string | null;
}

export interface MyDd254Authorization {
  id: string;
  contract_number: string;
  prime_contractor: string;
  classification_max: string;
  period_start: string | null;
  period_end: string | null;
}

export async function getMyDd254Authorizations(): Promise<MyDd254Authorization[]> {
  const res = await api<{ authorizations: MyDd254Authorization[] }>("/api/v1/me/dd254-authorizations");
  return res.authorizations ?? [];
}

export interface ExportVisitsParams {
  from: string;        // YYYY-MM-DD
  to: string;          // YYYY-MM-DD
  status?: string[];   // empty = all
  sub_org_id?: string;
  detail?: "full" | "summary";
}

// Builds the export URL with the given filters; browser handles the actual
// download via the response's Content-Disposition header. We don't use the
// shared `api()` helper here because that one parses JSON — we want the
// browser to receive a CSV blob and save-as.
export function exportVisitsUrl(p: ExportVisitsParams): string {
  const qs = new URLSearchParams();
  qs.set("from", p.from);
  qs.set("to", p.to);
  if (p.status && p.status.length > 0) qs.set("status", p.status.join(","));
  if (p.sub_org_id) qs.set("sub_org_id", p.sub_org_id);
  if (p.detail) qs.set("detail", p.detail);
  return `/api/v1/admin/visits/export?${qs.toString()}`;
}

export function formatAccessLevel(level: string): string {
  switch (level) {
    case "confidential": return "Confidential";
    case "secret": return "Secret";
    case "top_secret": return "Top Secret";
    case "top_secret_sci": return "Top Secret/SCI";
    default: return level;
  }
}

// --- IC functions ---

export async function createVisitRequest(req: CreateVisitRequestParams): Promise<VisitRequest> {
  return api<VisitRequest>("/api/v1/visits/", { method: "POST", body: JSON.stringify(req) });
}

export async function listMyVisits(params: {
  status?: string; search?: string; limit?: number; offset?: number;
} = {}): Promise<{ requests: VisitRequestRow[]; total: number }> {
  const qs = new URLSearchParams();
  if (params.status) qs.set("status", params.status);
  if (params.search) qs.set("search", params.search);
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.offset) qs.set("offset", String(params.offset));
  const query = qs.toString();
  return api(`/api/v1/visits/${query ? `?${query}` : ""}`);
}

export async function getMyVisit(id: string): Promise<VisitRequest> {
  return api(`/api/v1/visits/${id}`);
}

export async function getMyVisitStats(): Promise<VisitRequestStats> {
  return api("/api/v1/visits/stats");
}

export async function listCloneable(): Promise<{ requests: VisitRequestRow[] }> {
  return api("/api/v1/visits/cloneable");
}

export async function cancelVisitRequest(id: string): Promise<void> {
  await api(`/api/v1/visits/${id}/cancel`, { method: "POST" });
}

// --- Admin functions ---

export async function listAdminVisits(params: {
  status?: string; search?: string; sub_org_id?: string; limit?: number; offset?: number;
} = {}): Promise<{ requests: VisitRequestRow[]; total: number }> {
  const qs = new URLSearchParams();
  if (params.status) qs.set("status", params.status);
  if (params.search) qs.set("search", params.search);
  if (params.sub_org_id) qs.set("sub_org_id", params.sub_org_id);
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.offset) qs.set("offset", String(params.offset));
  const query = qs.toString();
  return api(`/api/v1/admin/visits/${query ? `?${query}` : ""}`);
}

export async function getAdminVisit(id: string): Promise<VisitRequest> {
  return api(`/api/v1/admin/visits/${id}`);
}

export async function getAdminVisitStats(subOrgId?: string): Promise<VisitRequestStats> {
  const q = subOrgId ? `?sub_org_id=${encodeURIComponent(subOrgId)}` : "";
  return api(`/api/v1/admin/visits/stats${q}`);
}

export async function updateVisitStatus(id: string, status: string, notes: string): Promise<void> {
  await api(`/api/v1/admin/visits/${id}/status`, {
    method: "POST",
    body: JSON.stringify({ status, notes }),
  });
}
