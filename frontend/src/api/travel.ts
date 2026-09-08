import { api } from "./client";

// --- Types ---

export interface TravelReport {
  id: string;
  tenant_id: string;
  user_id: string;
  trip_name: string;
  multi_country: boolean;
  passport_number: string;
  status: string;
  emergency_first_name: string;
  emergency_last_name: string;
  emergency_phone: string;
  additional_comments: string;
  submitted_at: string | null;
  reviewed_at: string | null;
  reviewed_by: string | null;
  created_at: string;
  updated_at: string;
}

export interface TravelCountry {
  id: string;
  report_id: string;
  country_name: string;
  sort_order: number;
  start_date: string | null;
  end_date: string | null;
  reason: string;
  transportation: string[];
  has_companions: boolean;
  companions_detail: string;
  has_foreign_contacts: boolean;
  contacts_detail: string;
  created_at: string;
  updated_at: string;
}

export interface TravelUpload {
  id: string;
  report_id: string;
  file_name: string;
  file_size: number;
  content_type: string;
  storage_key: string;
  created_at: string;
}

export interface TravelReportRow {
  id: string;
  trip_name: string;
  status: string;
  countries: string;
  earliest_date: string | null;
  latest_date: string | null;
  creator_name: string;
  created_at: string;
}

export interface TravelReportDetail {
  report: TravelReport;
  countries: TravelCountry[];
  uploads: TravelUpload[];
  creator_name: string;
}

export interface CreateTravelCountryRequest {
  country_name: string;
  sort_order: number;
  start_date?: string | null;
  end_date?: string | null;
  reason: string;
  transportation: string[];
  has_companions: boolean;
  companions_detail: string;
  has_foreign_contacts: boolean;
  contacts_detail: string;
}

export interface CreateTravelReportRequest {
  trip_name: string;
  multi_country: boolean;
  passport_number: string;
  emergency_first_name: string;
  emergency_last_name: string;
  emergency_phone: string;
  additional_comments: string;
  countries: CreateTravelCountryRequest[];
}

export interface TravelStats {
  total: number;
  draft: number;
  submitted: number;
  under_review: number;
  approved: number;
  rejected: number;
}

// --- IC functions ---

export async function createTravelReport(req: CreateTravelReportRequest): Promise<TravelReport> {
  return api<TravelReport>("/api/v1/travel/", { method: "POST", body: JSON.stringify(req) });
}

export async function listMyTravelReports(params: {
  status?: string; search?: string; limit?: number; offset?: number;
} = {}): Promise<{ reports: TravelReportRow[]; total: number }> {
  const qs = new URLSearchParams();
  if (params.status) qs.set("status", params.status);
  if (params.search) qs.set("search", params.search);
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.offset) qs.set("offset", String(params.offset));
  const query = qs.toString();
  return api(`/api/v1/travel/${query ? `?${query}` : ""}`);
}

export async function getMyTravelReport(id: string): Promise<TravelReportDetail> {
  return api(`/api/v1/travel/${id}`);
}

export async function getMyTravelStats(): Promise<TravelStats> {
  return api("/api/v1/travel/stats");
}

export async function updateTravelReport(id: string, req: CreateTravelReportRequest): Promise<void> {
  await api(`/api/v1/travel/${id}`, { method: "PUT", body: JSON.stringify(req) });
}

export async function submitTravelReport(id: string): Promise<void> {
  await api(`/api/v1/travel/${id}/submit`, { method: "POST" });
}

export async function uploadTravelFile(reportId: string, file: File): Promise<TravelUpload> {
  const formData = new FormData();
  formData.append("file", file);
  const csrf = document.cookie.match(/(?:^|; )fastfso_csrf=([^;]*)/)?.[1];
  const headers: Record<string, string> = {};
  if (csrf) headers["X-CSRF-Token"] = decodeURIComponent(csrf);
  const res = await fetch(`/api/v1/travel/${reportId}/upload`, {
    method: "POST", body: formData, headers, credentials: "include",
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: "upload failed" }));
    throw new Error((body as { error?: string }).error || "upload failed");
  }
  return res.json();
}

export async function deleteTravelUpload(reportId: string, uploadId: string): Promise<void> {
  await api(`/api/v1/travel/${reportId}/upload/${uploadId}`, { method: "DELETE" });
}

export function getTravelUploadDownloadUrl(uploadId: string): string {
  return `/api/v1/admin/travel/uploads/${uploadId}/download`;
}

// --- Debrief types and functions ---

export interface TravelDebrief {
  id: string;
  report_id: string;
  tenant_id: string;
  user_id: string;
  status: "pending" | "submitted";
  due_date: string | null;
  trip_name: string;
  q1_foreign_contact: boolean | null;
  q1_details: string;
  q2_surveillance: boolean | null;
  q2_details: string;
  q3_equipment_loss: boolean | null;
  q3_details: string;
  q4_unusual_requests: boolean | null;
  q4_details: string;
  submitted_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface SubmitDebriefRequest {
  q1_foreign_contact: boolean;
  q1_details: string;
  q2_surveillance: boolean;
  q2_details: string;
  q3_equipment_loss: boolean;
  q3_details: string;
  q4_unusual_requests: boolean;
  q4_details: string;
}

export async function getMyDebriefs(): Promise<{ debriefs: TravelDebrief[] }> {
  return api("/api/v1/travel/debriefs");
}

export async function submitDebrief(id: string, req: SubmitDebriefRequest): Promise<void> {
  await api(`/api/v1/travel/debriefs/${id}/submit`, { method: "POST", body: JSON.stringify(req) });
}

// --- Admin functions ---

export async function listTravelReports(params: {
  status?: string; search?: string; sub_org_id?: string; limit?: number; offset?: number;
} = {}): Promise<{ reports: TravelReportRow[]; total: number }> {
  const qs = new URLSearchParams();
  if (params.status) qs.set("status", params.status);
  if (params.search) qs.set("search", params.search);
  if (params.sub_org_id) qs.set("sub_org_id", params.sub_org_id);
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.offset) qs.set("offset", String(params.offset));
  const query = qs.toString();
  return api(`/api/v1/admin/travel/${query ? `?${query}` : ""}`);
}

export async function getTravelReport(id: string): Promise<TravelReportDetail> {
  return api(`/api/v1/admin/travel/${id}`);
}

export async function getTravelStats(subOrgId?: string): Promise<TravelStats> {
  const q = subOrgId ? `?sub_org_id=${encodeURIComponent(subOrgId)}` : "";
  return api(`/api/v1/admin/travel/stats${q}`);
}

export async function getAdminDebrief(id: string): Promise<TravelDebrief> {
  return api(`/api/v1/admin/travel/debriefs/${id}`);
}

export async function approveTravelReport(id: string): Promise<void> {
  await api(`/api/v1/admin/travel/${id}/approve`, { method: "POST" });
}

export async function rejectTravelReport(id: string): Promise<void> {
  await api(`/api/v1/admin/travel/${id}/reject`, { method: "POST" });
}
