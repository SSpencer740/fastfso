import { api } from "./client";
import type { ClearanceLevel } from "./team";

export type Dd254Status = "active" | "expired" | "superseded";

export interface Dd254Form {
  id: string;
  tenant_id: string;
  sub_org_id?: string | null;
  sub_org_name?: string | null;
  contract_number: string;
  prime_contractor: string;
  classification_max: ClearanceLevel;
  period_start?: string | null;
  period_end?: string | null;
  filename: string;
  content_type: string;
  size_bytes: number;
  markings: string;
  status: Dd254Status;
  supersedes_id?: string | null;
  uploaded_by: string;
  uploaded_by_name: string;
  read_on_count: number;
  created_at: string;
  updated_at: string;
}

export interface Dd254AccessGrant {
  user_id: string;
  name: string;
  email: string;
  briefed_at?: string | null;
  debriefed_at?: string | null;
  added_by: string;
  added_at: string;
}

export interface Dd254Detail extends Dd254Form {
  access_grants: Dd254AccessGrant[];
}

export interface Dd254Stats {
  active: number;
  expiring_soon: number;
  expired: number;
  no_read_on: number;
}

export interface Dd254Suggestion extends Dd254Form {
  match_reason: "exact_class" | "above_class";
}

export interface UploadDd254Params {
  file: File;
  contract_number: string;
  prime_contractor: string;
  classification_max: ClearanceLevel;
  period_start?: string;
  period_end?: string;
  sub_org_id?: string;
}

export async function listDd254(params: {
  sub_org_id?: string;
  status?: string;
  class?: string;
  search?: string;
} = {}): Promise<{ forms: Dd254Form[] }> {
  const qs = new URLSearchParams();
  if (params.sub_org_id) qs.set("sub_org_id", params.sub_org_id);
  if (params.status) qs.set("status", params.status);
  if (params.class) qs.set("class", params.class);
  if (params.search) qs.set("search", params.search);
  const query = qs.toString();
  return api(`/api/v1/dd254/${query ? `?${query}` : ""}`);
}

export async function getDd254Stats(subOrgId?: string): Promise<Dd254Stats> {
  const q = subOrgId ? `?sub_org_id=${encodeURIComponent(subOrgId)}` : "";
  return api(`/api/v1/dd254/stats${q}`);
}

export async function getDd254(id: string): Promise<Dd254Detail> {
  return api(`/api/v1/dd254/${id}`);
}

export interface UploadDd254Result {
  form: Dd254Form;
  scanned: boolean; // false in local-dev Noop mode, true wherever ClamAV is wired
}

export async function uploadDd254(p: UploadDd254Params): Promise<UploadDd254Result> {
  const csrf = document.cookie.match(/fastfso_csrf=([^;]*)/)?.[1];
  const headers: Record<string, string> = {};
  if (csrf) headers["X-CSRF-Token"] = decodeURIComponent(csrf);

  const body = new FormData();
  body.append("file", p.file);
  body.append("contract_number", p.contract_number);
  body.append("prime_contractor", p.prime_contractor);
  body.append("classification_max", p.classification_max);
  if (p.period_start) body.append("period_start", p.period_start);
  if (p.period_end) body.append("period_end", p.period_end);
  if (p.sub_org_id) body.append("sub_org_id", p.sub_org_id);
  body.append("markings", "unclassified");
  body.append("cui_attestation", "true");

  const res = await fetch("/api/v1/dd254/", {
    method: "POST",
    headers,
    body,
    credentials: "include",
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "upload failed" }));
    throw new Error((err as { error: string }).error || "upload failed");
  }
  return res.json() as Promise<UploadDd254Result>;
}

export async function deleteDd254(id: string): Promise<void> {
  await api(`/api/v1/dd254/${id}`, { method: "DELETE" });
}

export async function grantDd254Access(
  id: string,
  userId: string,
  briefedAt?: string | null,
  debriefedAt?: string | null,
): Promise<void> {
  await api(`/api/v1/dd254/${id}/access`, {
    method: "POST",
    body: JSON.stringify({ user_id: userId, briefed_at: briefedAt, debriefed_at: debriefedAt }),
  });
}

export async function revokeDd254Access(id: string, userId: string): Promise<void> {
  await api(`/api/v1/dd254/${id}/access/${userId}`, { method: "DELETE" });
}

export function dd254FileUrl(id: string): string {
  return `/api/v1/dd254/${id}/file`;
}

// Visit-review integration
export async function getDd254Suggestions(
  visitRequestId: string,
): Promise<{ suggestions: Dd254Suggestion[] }> {
  return api(`/api/v1/admin/visits/${visitRequestId}/dd254-suggestions`);
}

export async function linkDd254ToVisit(
  visitRequestId: string,
  dd254Id: string | null,
): Promise<void> {
  await api(`/api/v1/admin/visits/${visitRequestId}/dd254`, {
    method: "PUT",
    body: JSON.stringify({ dd_254_id: dd254Id }),
  });
}
