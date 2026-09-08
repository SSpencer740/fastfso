import { api } from "./client";

export type ClearanceLevel = "none" | "confidential" | "secret" | "top_secret" | "ts_sci";
export type InvestigationType = "T3" | "T3R" | "T5" | "T5R";

export interface SubOrgRef {
  id: string;
  name: string;
}

export interface Member {
  user_id: string;
  identity_id: string;
  name: string;
  email: string;
  role: string;
  suspended: boolean;
  sub_orgs: SubOrgRef[];
  clearance: ClearanceLevel | "";
  investigation_type?: InvestigationType | null;
  eligibility_date?: string | null;
  last_investigation_date?: string | null;
  next_investigation_date?: string | null;
  clearance_recorded_at?: string | null;
}

export interface ClearanceRecord {
  id: string;
  user_id: string;
  clearance: ClearanceLevel;
  investigation_type?: InvestigationType | null;
  eligibility_date?: string | null;
  last_investigation_date?: string | null;
  next_investigation_date?: string | null;
  notes: string;
  recorded_by: string;
  recorded_by_name: string;
  recorded_at: string;
  superseded_at?: string | null;
}

export interface MemberDetail extends Member {
  history: ClearanceRecord[];
}

export interface TeamStats {
  members: number;
  due_within_90: number;
  overdue: number;
}

export interface SetClearanceParams {
  clearance: ClearanceLevel;
  investigation_type?: InvestigationType | null;
  eligibility_date?: string | null;
  last_investigation_date?: string | null;
  next_investigation_date?: string | null;
  notes?: string;
}

export const clearanceLabels: Record<ClearanceLevel | "", string> = {
  "": "—",
  none: "None",
  confidential: "Confidential",
  secret: "Secret",
  top_secret: "Top Secret",
  ts_sci: "TS/SCI",
};

export async function listTeam(params: {
  sub_org_id?: string;
  clearance?: string;
  search?: string;
  due_within_days?: number;
  limit?: number;
  offset?: number;
} = {}): Promise<{ members: Member[]; total: number }> {
  const qs = new URLSearchParams();
  if (params.sub_org_id) qs.set("sub_org_id", params.sub_org_id);
  if (params.clearance) qs.set("clearance", params.clearance);
  if (params.search) qs.set("search", params.search);
  if (params.due_within_days != null) qs.set("due_within_days", String(params.due_within_days));
  if (params.limit != null) qs.set("limit", String(params.limit));
  if (params.offset != null) qs.set("offset", String(params.offset));
  const query = qs.toString();
  return api(`/api/v1/team/${query ? `?${query}` : ""}`);
}

export async function getTeamStats(subOrgId?: string): Promise<TeamStats> {
  const q = subOrgId ? `?sub_org_id=${encodeURIComponent(subOrgId)}` : "";
  return api(`/api/v1/team/stats${q}`);
}

export async function getMember(userId: string): Promise<MemberDetail> {
  return api(`/api/v1/team/${userId}`);
}

export async function setClearance(userId: string, p: SetClearanceParams): Promise<ClearanceRecord> {
  return api(`/api/v1/team/${userId}/clearance`, {
    method: "PUT",
    body: JSON.stringify(p),
  });
}
