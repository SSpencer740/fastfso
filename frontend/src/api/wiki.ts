import { api } from "./client";

export interface WikiPost {
  id: string;
  tenant_id: string;
  created_by_user_id: string;
  sub_org_id: string | null;
  creator_name: string;
  title: string;
  content: string;
  published: boolean;
  files: PostFile[];
  created_at: string;
  updated_at: string;
}

export interface PostFile {
  id: string;
  post_id: string;
  file_name: string;
  file_size: number;
  created_at: string;
}

export interface FSOInfo {
  name: string;
  email: string;
}

export async function listPublishedPosts(): Promise<WikiPost[]> {
  const res = await api<{ posts: WikiPost[] }>("/api/v1/wiki/");
  return res.posts;
}

export async function getFSOInfo(): Promise<FSOInfo | null> {
  const res = await api<{ fso: FSOInfo | null }>("/api/v1/wiki/fso");
  return res.fso;
}

export async function adminListPosts(subOrgId?: string): Promise<WikiPost[]> {
  const q = subOrgId ? `?sub_org_id=${encodeURIComponent(subOrgId)}` : "";
  const res = await api<{ posts: WikiPost[] }>(`/api/v1/admin/wiki/${q}`);
  return res.posts;
}

export async function adminGetPost(id: string): Promise<WikiPost> {
  return api<WikiPost>(`/api/v1/admin/wiki/${id}`);
}

export interface PostPayload {
  title: string;
  content: string;
  published: boolean;
  // Honored only for administrators by the backend. null = tenant-wide.
  sub_org_id?: string | null;
}

export async function adminCreatePost(payload: PostPayload): Promise<WikiPost> {
  return api<WikiPost>("/api/v1/admin/wiki/", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function adminUpdatePost(id: string, payload: PostPayload): Promise<WikiPost> {
  return api<WikiPost>(`/api/v1/admin/wiki/${id}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

export async function adminDeletePost(id: string): Promise<void> {
  await api(`/api/v1/admin/wiki/${id}`, { method: "DELETE" });
}

export async function adminUploadFile(postId: string, file: File): Promise<PostFile> {
  const csrf = document.cookie.match(/fastfso_csrf=([^;]*)/)?.[1];
  const headers: Record<string, string> = {};
  if (csrf) headers["X-CSRF-Token"] = decodeURIComponent(csrf);

  const body = new FormData();
  body.append("file", file);

  const res = await fetch(`/api/v1/admin/wiki/${postId}/files`, {
    method: "POST",
    headers,
    body,
    credentials: "include",
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "upload failed" }));
    throw new Error((err as { error: string }).error || "upload failed");
  }
  return res.json() as Promise<PostFile>;
}

export async function adminDeleteFile(fileId: string): Promise<void> {
  await api(`/api/v1/admin/wiki/files/${fileId}`, { method: "DELETE" });
}

// Returns the URL that serves/redirects to the PDF (works for both IC and admin).
export function wikiFileViewUrl(fileId: string): string {
  return `/api/v1/wiki/files/${fileId}`;
}
