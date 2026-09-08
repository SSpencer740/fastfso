import { api } from "./client";

// --- Types ---

export interface Task {
  id: string;
  tenant_id: string;
  created_by_user_id: string;
  title: string;
  description: string;
  priority: string;
  due_date: string | null;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface Requirement {
  id: string;
  task_id: string;
  kind: string;
  label: string;
  description: string;
  required: boolean;
  sort_order: number;
  ai_verification_criteria: string;
}

// Verification mirrors backend verification.Record (the JSON fields). Status
// is "pending" while the async worker runs, "succeeded" once a verdict is
// in, "failed" if the AI call errored.
export interface Verification {
  id: string;
  upload_id: string;
  tenant_id: string;
  status: "pending" | "succeeded" | "failed";
  flagged: boolean;
  confidence?: number;
  type_match?: boolean;
  extracted_fields: Record<string, string>;
  discrepancies: string[];
  reasoning: string;
  error_message?: string;
  criteria_snapshot: string;
  model: string;
  admin_feedback?: "correct" | "incorrect";
  admin_feedback_at?: string;
  admin_feedback_by?: string;
  created_at: string;
  updated_at: string;
}

export interface AssignmentRule {
  id: string;
  task_id: string;
  rule_type: string;
  target_id: string | null;
}

export interface AssigneeStatus {
  completion_id: string;
  user_id: string;
  user_name: string;
  user_email: string;
  status: string;
  viewed_at: string | null;
  submitted_at: string | null;
}

export interface TaskDetail {
  task: Task;
  requirements: Requirement[];
  rules: AssignmentRule[];
  assignees: AssigneeStatus[];
}

export interface TaskRow {
  id: string;
  title: string;
  description: string;
  priority: string;
  due_date: string | null;
  status: string;
  creator_name: string;
  total_assigned: number;
  completed: number;
  created_at: string;
}

export interface MyTaskRow {
  id: string;
  title: string;
  description: string;
  priority: string;
  due_date: string | null;
  status: string;
  creator_name: string;
  created_at: string;
}

export interface Completion {
  id: string;
  task_id: string;
  user_id: string;
  status: string;
  viewed_at: string | null;
  submitted_at: string | null;
  reviewed_at: string | null;
  reviewed_by: string | null;
  created_at: string;
  updated_at: string;
}

export interface TaskResponse {
  id: string;
  completion_id: string;
  requirement_id: string;
  text_value: string | null;
  bool_value: boolean | null;
}

export interface Upload {
  id: string;
  completion_id: string;
  requirement_id: string;
  file_name: string;
  file_size: number;
  content_type: string;
  storage_key: string;
  created_at: string;
}

export interface MyTaskDetail {
  task: Task;
  requirements: Requirement[];
  completion: Completion;
  responses: TaskResponse[];
  uploads: Upload[];
  verifications: Record<string, Verification>;
  creator_name: string;
}

export interface SummaryStats {
  total: number;
  active: number;
  draft: number;
  needs_review: number;
}

export interface MySummaryStats {
  total: number;
  to_do: number;
  in_progress: number;
  submitted: number;
  approved: number;
}

export interface TenantUser {
  user_id: string;
  name: string;
  email: string;
  role: string;
  sub_org_id?: string;
}

export interface SubOrgOption {
  id: string;
  name: string;
}

export interface CreateTaskRequest {
  title: string;
  description: string;
  priority: string;
  due_date?: string;
  status?: string;
  sub_org_id?: string;
  requirements: {
    kind: string;
    label: string;
    description: string;
    required: boolean;
    sort_order: number;
    ai_verification_criteria?: string;
  }[];
  rules: {
    rule_type: string;
    target_id?: string;
  }[];
}

export interface AdminSubmission {
  task_title: string;
  user_name: string;
  user_email: string;
  status: string;
  submitted_at: string | null;
  requirements: Requirement[];
  responses: TaskResponse[];
  uploads: Upload[];
  verifications: Record<string, Verification>;
}

export async function submitVerificationFeedback(
  uploadId: string,
  feedback: "correct" | "incorrect",
): Promise<void> {
  const csrf = document.cookie.match(/(?:^|; )fastfso_csrf=([^;]*)/)?.[1];
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (csrf) headers["X-CSRF-Token"] = decodeURIComponent(csrf);
  const res = await fetch(`/api/v1/admin/tasks/uploads/${uploadId}/verification/feedback`, {
    method: "POST",
    body: JSON.stringify({ feedback }),
    headers,
    credentials: "include",
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: "feedback failed" }));
    throw new Error((body as { error?: string }).error || "feedback failed");
  }
}

export interface SaveResponseParams {
  requirement_id: string;
  text_value?: string | null;
  bool_value?: boolean | null;
}

// --- Admin functions ---

export async function createTask(req: CreateTaskRequest): Promise<Task> {
  return api<Task>("/api/v1/admin/tasks/", { method: "POST", body: JSON.stringify(req) });
}

export async function listTasks(params: {
  status?: string;
  priority?: string;
  search?: string;
  sub_org_id?: string;
  limit?: number;
  offset?: number;
} = {}): Promise<{ tasks: TaskRow[]; total: number }> {
  const qs = new URLSearchParams();
  if (params.status) qs.set("status", params.status);
  if (params.priority) qs.set("priority", params.priority);
  if (params.search) qs.set("search", params.search);
  if (params.sub_org_id) qs.set("sub_org_id", params.sub_org_id);
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.offset) qs.set("offset", String(params.offset));
  const query = qs.toString();
  return api(`/api/v1/admin/tasks/${query ? `?${query}` : ""}`);
}

export async function getTask(id: string): Promise<TaskDetail> {
  return api(`/api/v1/admin/tasks/${id}`);
}

export async function archiveTask(id: string): Promise<void> {
  await api(`/api/v1/admin/tasks/${id}/archive`, { method: "POST" });
}

export async function unarchiveTask(id: string): Promise<void> {
  await api(`/api/v1/admin/tasks/${id}/unarchive`, { method: "POST" });
}

export async function getTaskStats(from?: string, to?: string, subOrgId?: string): Promise<SummaryStats> {
  const qs = new URLSearchParams();
  if (from) qs.set("from", from);
  if (to) qs.set("to", to);
  if (subOrgId) qs.set("sub_org_id", subOrgId);
  const q = qs.toString();
  return api(`/api/v1/admin/tasks/stats${q ? `?${q}` : ""}`);
}

export async function approveSubmission(taskId: string, userId: string): Promise<void> {
  await api(`/api/v1/admin/tasks/${taskId}/assignees/${userId}/approve`, { method: "POST" });
}

export async function rejectSubmission(taskId: string, userId: string): Promise<void> {
  await api(`/api/v1/admin/tasks/${taskId}/assignees/${userId}/reject`, { method: "POST" });
}

export async function listTenantUsers(search?: string): Promise<TenantUser[]> {
  const qs = search ? `?search=${encodeURIComponent(search)}` : "";
  const res = await api<{ users: TenantUser[] }>(`/api/v1/admin/tasks/users${qs}`);
  return res.users ?? [];
}

export async function listSubOrgs(): Promise<SubOrgOption[]> {
  const res = await api<{ sub_orgs: SubOrgOption[] }>("/api/v1/admin/tasks/sub-orgs");
  return (res.sub_orgs ?? []).filter(o => o.name !== "Default");
}

// --- IC functions ---

export async function listMyTasks(search?: string): Promise<{ tasks: MyTaskRow[] }> {
  const qs = search ? `?search=${encodeURIComponent(search)}` : "";
  return api(`/api/v1/tasks/${qs}`);
}

export async function getMyTask(id: string): Promise<MyTaskDetail> {
  return api(`/api/v1/tasks/${id}`);
}

export async function getAdminSubmission(completionId: string): Promise<AdminSubmission> {
  return api(`/api/v1/admin/tasks/completions/${completionId}`);
}

export function getUploadDownloadUrl(uploadId: string): string {
  return `/api/v1/admin/tasks/uploads/${uploadId}/download`;
}

export async function getMyTaskStats(from?: string, to?: string): Promise<MySummaryStats> {
  const qs = new URLSearchParams();
  if (from) qs.set("from", from);
  if (to) qs.set("to", to);
  const q = qs.toString();
  return api(`/api/v1/tasks/stats${q ? `?${q}` : ""}`);
}

export async function saveResponses(taskId: string, responses: SaveResponseParams[]): Promise<void> {
  await api(`/api/v1/tasks/${taskId}/responses`, {
    method: "POST",
    body: JSON.stringify({ responses }),
  });
}

export async function submitTask(taskId: string): Promise<void> {
  await api(`/api/v1/tasks/${taskId}/submit`, { method: "POST" });
}

export async function uploadFile(taskId: string, requirementId: string, file: File): Promise<Upload> {
  const formData = new FormData();
  formData.append("file", file);

  // Need to get CSRF token manually for non-JSON requests
  const csrf = document.cookie.match(/(?:^|; )fastfso_csrf=([^;]*)/)?.[1];
  const headers: Record<string, string> = {};
  if (csrf) headers["X-CSRF-Token"] = decodeURIComponent(csrf);

  const res = await fetch(`/api/v1/tasks/${taskId}/upload/${requirementId}`, {
    method: "POST",
    body: formData,
    headers,
    credentials: "include",
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: "upload failed" }));
    throw new Error((body as { error?: string }).error || "upload failed");
  }

  return res.json();
}

export async function deleteUpload(taskId: string, uploadId: string): Promise<void> {
  await api(`/api/v1/tasks/${taskId}/upload/${uploadId}`, { method: "DELETE" });
}
