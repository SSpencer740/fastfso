import { api } from "./client";

export interface Report {
  id: string;
  tenant_id: string;
  created_by_user_id: string;
  creator_name: string;
  creator_email: string;
  reporting_for: string;
  subject_name: string;
  report_type: string;
  details: string;
  status: string;
  reviewed_by: string | null;
  reviewed_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface ReportRow {
  id: string;
  reporting_for: string;
  subject_name: string;
  report_type: string;
  status: string;
  creator_name: string;
  created_at: string;
}

export interface ReportStats {
  total: number;
  unreviewed: number;
  under_review: number;
  processed: number;
}

export interface CreateReportParams {
  reporting_for: string;
  subject_name: string;
  report_type: string;
  details: string;
}

export const SELF_REPORT_TYPES = [
  { value: "alcohol_drug_treatment", label: "Alcohol/Drug Related Treatment" },
  { value: "arrests", label: "Arrests" },
  { value: "cohabitant", label: "Cohabitant" },
  { value: "elicitation", label: "Elicitation, Exploitation, Blackmail, Coercion, or Enticement" },
  { value: "financial", label: "Financial" },
  { value: "foreign_activities", label: "Foreign Activities" },
  { value: "foreign_contacts", label: "Foreign Contacts" },
  { value: "marriage_divorce", label: "Marriage/Divorce" },
  { value: "media_contact", label: "Media Contact" },
  { value: "other", label: "Other" },
] as const;

export const FCL_REPORT_TYPES = [
  { value: "fcl_change_of_ownership", label: "Change of Ownership" },
  { value: "fcl_address_change", label: "Address Change of Company" },
  { value: "fcl_unable_to_safeguard", label: "Unable to Meet Safeguarding Requirements" },
] as const;

export const REPORT_TYPE_INSTRUCTIONS: Record<string, string> = {
  alcohol_drug_treatment: "Please provide the following details: Reason, treatment provider including contact information, dates of treatment.",
  arrests: "Please provide the following details: Date of the arrest, location of the arrest, charges and circumstances, disposition.",
  cohabitant: "Please provide the following details: Name, citizenship(s), date of birth, place of birth, duration of association with this individual.",
  elicitation: "Please provide the following details: Date of incident, name of individual(s) involved, nature of incident, method of contact, type of information being sought, background, circumstances, and current state of the matter.",
  financial: "Reportable activities include: Bankruptcy, over 120 days delinquent on any debt, garnishments, unusual infusion of assets of $10,000 or greater. Please provide: Type of issue or anomaly, dollar value, reason.",
  foreign_activities: "Please provide details on any: Involvement in foreign business (nature, countries, business name); foreign bank account (institution, country); ownership of foreign property; foreign citizenship; foreign passport or identity card use; voting in a foreign election; adoption of non-U.S. citizen children.",
  foreign_contacts: "Report any interactions with foreign intelligence entities or foreign nationals. Include: Service(s) involved, name(s) of individual(s), date(s) of contact, nature of contact, likelihood of future contacts, citizenship(s), occupation, nature of relationship, duration and frequency.",
  marriage_divorce: "Please provide the following details: Name of other party, citizenship(s) of other party, date of birth, place of birth, date of marriage/divorce.",
  media_contact: "Please provide: Date(s) of contact, name of media outlet, name of media representative, nature and purpose of contact, whether classified information was involved, current status of the contact.",
  other: "If your reportable event does not meet any of the above criteria, please describe the situation below.",
  fcl_change_of_ownership: "Please describe the change of ownership. Note: Any discussion of the sale of the firm to a foreign owner, even if no sale occurs, must be reported.",
  fcl_address_change: "Please provide the new address and effective date of the change.",
  fcl_unable_to_safeguard: "Please describe the circumstances under which the firm is unable to meet the requirements to safeguard classified information.",
};

export function formatReportType(type: string): string {
  const all = [...SELF_REPORT_TYPES, ...FCL_REPORT_TYPES];
  return all.find(t => t.value === type)?.label ?? type;
}

// --- IC functions ---

export async function createReport(params: CreateReportParams): Promise<Report> {
  return api("/api/v1/reports/", { method: "POST", body: JSON.stringify(params) });
}

export async function listMyReports(params: { status?: string; search?: string; limit?: number; offset?: number } = {}): Promise<{ reports: ReportRow[]; total: number }> {
  const qs = new URLSearchParams();
  if (params.status) qs.set("status", params.status);
  if (params.search) qs.set("search", params.search);
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.offset) qs.set("offset", String(params.offset));
  const q = qs.toString();
  return api(`/api/v1/reports/${q ? `?${q}` : ""}`);
}

export async function getMyReport(id: string): Promise<Report> {
  return api(`/api/v1/reports/${id}`);
}

export async function getMyReportStats(): Promise<ReportStats> {
  return api("/api/v1/reports/stats");
}

// --- Admin functions ---

export async function listAdminReports(params: { status?: string; search?: string; sub_org_id?: string; limit?: number; offset?: number } = {}): Promise<{ reports: ReportRow[]; total: number }> {
  const qs = new URLSearchParams();
  if (params.status) qs.set("status", params.status);
  if (params.search) qs.set("search", params.search);
  if (params.sub_org_id) qs.set("sub_org_id", params.sub_org_id);
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.offset) qs.set("offset", String(params.offset));
  const q = qs.toString();
  return api(`/api/v1/admin/reports/${q ? `?${q}` : ""}`);
}

export async function getAdminReport(id: string): Promise<Report> {
  return api(`/api/v1/admin/reports/${id}`);
}

export async function getAdminReportStats(subOrgId?: string): Promise<ReportStats> {
  const q = subOrgId ? `?sub_org_id=${encodeURIComponent(subOrgId)}` : "";
  return api(`/api/v1/admin/reports/stats${q}`);
}

export async function updateReportStatus(id: string, status: string): Promise<void> {
  await api(`/api/v1/admin/reports/${id}/status`, { method: "POST", body: JSON.stringify({ status }) });
}
