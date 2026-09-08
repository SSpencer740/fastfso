import { api } from "./client";

export interface ActionItem {
  id: string;
  tenant_id: string;
  source_type: string;
  source_id: string | null;
  title: string;
  description: string;
  priority: string;
  status: string;
  assigned_to: string | null;
  assignee_name: string | null;
  assignee_email: string | null;
  notes: string;
  due_date: string | null;
  created_at: string;
  updated_at: string;
}

export interface ActionItemRow {
  id: string;
  source_type: string;
  title: string;
  description: string;
  priority: string;
  status: string;
  assignee_name: string | null;
  assignee_email: string | null;
  due_date: string | null;
  created_at: string;
}

export interface ActionItemStats {
  total: number;
  pending: number;
  under_review: number;
  processed: number;
}

export async function listActionItems(params: {
  source_type?: string;
  status?: string;
  search?: string;
  sub_org_id?: string;
  limit?: number;
  offset?: number;
} = {}): Promise<{ items: ActionItemRow[]; total: number }> {
  const qs = new URLSearchParams();
  if (params.source_type) qs.set("source_type", params.source_type);
  if (params.status) qs.set("status", params.status);
  if (params.search) qs.set("search", params.search);
  if (params.sub_org_id) qs.set("sub_org_id", params.sub_org_id);
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.offset) qs.set("offset", String(params.offset));
  const query = qs.toString();
  return api(`/api/v1/admin/action-items/${query ? `?${query}` : ""}`);
}

export async function getActionItem(id: string): Promise<ActionItem> {
  return api(`/api/v1/admin/action-items/${id}`);
}

export async function updateActionItemStatus(id: string, status: string, notes = ""): Promise<void> {
  await api(`/api/v1/admin/action-items/${id}/status`, {
    method: "POST",
    body: JSON.stringify({ status, notes }),
  });
}

export async function updateActionItemNotes(id: string, notes: string): Promise<void> {
  await api(`/api/v1/admin/action-items/${id}/notes`, {
    method: "POST",
    body: JSON.stringify({ notes }),
  });
}

export async function getActionItemStats(subOrgId?: string): Promise<ActionItemStats> {
  const q = subOrgId ? `?sub_org_id=${encodeURIComponent(subOrgId)}` : "";
  return api(`/api/v1/admin/action-items/stats${q}`);
}

export interface FSOUser {
  user_id: string;
  name: string;
  email: string;
}

export async function listFSOUsers(): Promise<{ users: FSOUser[] }> {
  return api("/api/v1/admin/action-items/fso-users");
}

export async function reassignActionItem(id: string, userId: string | null): Promise<ActionItem> {
  return api(`/api/v1/admin/action-items/${id}/assignee`, {
    method: "PATCH",
    body: JSON.stringify({ user_id: userId }),
  });
}
