import { useState, useEffect, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Pencil, Trash2, Globe, User, Paperclip, X, FileText, Maximize2 } from "lucide-react";
import { useAuthStore } from "../../stores/authStore";
import { PageHeader } from "../../components/ui/PageHeader";
import { Modal } from "../../components/ui/Modal";
import { isAdmin, isAdministrator, isReadOnlyFSO } from "../../utils/roles";
import { SubOrgFilter } from "../../components/ui/SubOrgFilter";
import {
  listPublishedPosts,
  getFSOInfo,
  adminListPosts,
  adminCreatePost,
  adminUpdatePost,
  adminDeletePost,
  adminUploadFile,
  adminDeleteFile,
  wikiFileViewUrl,
} from "../../api/wiki";
import { listSubOrgs } from "../../api/customerAdmin";
import type { SubOrg } from "../../api/customerAdmin";
import type { WikiPost, PostFile, PostPayload } from "../../api/wiki";

// --- Post viewer modal ---

function PDFFullScreen({ file, onClose }: { file: PostFile; onClose: () => void }) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) { if (e.key === "Escape") onClose(); }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      style={{ position: "fixed", inset: 0, zIndex: 1000, background: "rgba(0,0,0,0.85)", display: "flex", flexDirection: "column" }}
      onClick={onClose}
    >
      <div
        style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "10px 16px", background: "rgba(0,0,0,0.6)", flexShrink: 0 }}
        onClick={e => e.stopPropagation()}
      >
        <span style={{ fontSize: 14, color: "#fff", fontWeight: 500 }}>{file.file_name}</span>
        <button
          type="button"
          onClick={onClose}
          style={{ background: "none", border: "none", cursor: "pointer", color: "#fff", display: "flex", alignItems: "center", gap: 6, fontSize: 13, padding: "4px 8px" }}
        >
          <X size={16} /> Close
        </button>
      </div>
      <div style={{ flex: 1, padding: "12px 16px 16px" }} onClick={e => e.stopPropagation()}>
        <iframe
          src={wikiFileViewUrl(file.id)}
          title={file.file_name}
          style={{ width: "100%", height: "100%", border: "none", borderRadius: "var(--radius-md)" }}
        />
      </div>
    </div>
  );
}

function PostViewModal({ post, onClose }: { post: WikiPost | null; onClose: () => void }) {
  const [activeFileId, setActiveFileId] = useState<string | null>(null);
  const [fullScreen, setFullScreen] = useState(false);

  const files = post?.files ?? [];
  // Derive activeFile from the current post's file list so it auto-clears when the post changes.
  const activeFile = files.find(f => f.id === activeFileId) ?? null;

  return (
    <>
      {fullScreen && activeFile && (
        <PDFFullScreen file={activeFile} onClose={() => setFullScreen(false)} />
      )}

      <Modal open={!!post} onClose={onClose} title={post?.title ?? ""}>
        <p style={{ fontSize: 13, color: "var(--color-text-muted)", marginBottom: 16 }}>
          Posted by {post?.creator_name} &mdash; {post ? new Date(post.created_at).toLocaleDateString() : ""}
        </p>

        {post?.content && (
          <div style={{ fontSize: 14, lineHeight: 1.7, whiteSpace: "pre-wrap", color: "var(--color-text)", marginBottom: files.length > 0 ? 20 : 0 }}>
            {post.content}
          </div>
        )}

        {files.length > 0 && (
          <div>
            <div style={{ fontSize: 12, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--color-text-muted)", marginBottom: 8 }}>
              Attachments
            </div>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginBottom: activeFile ? 16 : 0 }}>
              {files.map(f => (
                <button
                  key={f.id}
                  type="button"
                  onClick={() => { setActiveFileId(activeFileId === f.id ? null : f.id); setFullScreen(false); }}
                  style={{
                    display: "flex", alignItems: "center", gap: 6,
                    padding: "6px 12px", fontSize: 13, borderRadius: "var(--radius-md)",
                    border: "1px solid",
                    cursor: "pointer",
                    borderColor: activeFileId === f.id ? "var(--color-primary)" : "var(--color-border)",
                    background: activeFileId === f.id ? "color-mix(in srgb, var(--color-primary) 10%, transparent)" : "var(--color-bg-elevated)",
                    color: "var(--color-text)",
                  }}
                >
                  <FileText size={14} style={{ flexShrink: 0 }} />
                  {f.file_name}
                </button>
              ))}
            </div>

            {activeFile && (
              <div style={{ border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", overflow: "hidden", marginTop: 8 }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "flex-end", padding: "6px 10px", borderBottom: "1px solid var(--color-border)", background: "var(--color-bg-elevated)" }}>
                  <button
                    type="button"
                    onClick={() => setFullScreen(true)}
                    style={{ background: "none", border: "none", cursor: "pointer", color: "var(--color-text-muted)", display: "flex", alignItems: "center", gap: 5, fontSize: 12, padding: "2px 4px" }}
                  >
                    <Maximize2 size={13} /> Full screen
                  </button>
                </div>
                <iframe
                  src={wikiFileViewUrl(activeFile.id)}
                  title={activeFile.file_name}
                  style={{ width: "100%", height: 500, border: "none", display: "block" }}
                />
              </div>
            )}
          </div>
        )}
      </Modal>
    </>
  );
}

// --- Post editor modal (admin) ---

interface PostEditorModalProps {
  open: boolean;
  onClose: () => void;
  post: WikiPost | null; // null = create, non-null = edit
  onSaved: (created?: WikiPost) => void;
}

function PostEditorModal({ open, onClose, post, onSaved }: PostEditorModalProps) {
  const role = useAuthStore(s => s.user?.role);
  // The backend only honors sub_org_id from the strict administrator role
  // (FSOs always inherit their session sub-org), so the editor's sub-org
  // selector should only show for administrators — not FSOs.
  const userIsAdmin = isAdministrator(role);
  const [title, setTitle] = useState(post?.title ?? "");
  const [content, setContent] = useState(post?.content ?? "");
  const [published, setPublished] = useState(post?.published ?? false);
  // "" means tenant-wide. Only honored by the backend for administrators.
  const [subOrgId, setSubOrgId] = useState<string>(post?.sub_org_id ?? "");
  const [subOrgs, setSubOrgs] = useState<SubOrg[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  // File management state (only relevant when editing an existing post)
  const [files, setFiles] = useState<PostFile[]>(post?.files ?? []);
  const [uploading, setUploading] = useState(false);
  const [uploadStatus, setUploadStatus] = useState<{ kind: "success" | "error"; message: string } | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Sync form state whenever the modal opens with a different post.
  useEffect(() => {
    if (!open) return;
    setTitle(post?.title ?? "");
    setContent(post?.content ?? "");
    setPublished(post?.published ?? false);
    setSubOrgId(post?.sub_org_id ?? "");
    setFiles(post?.files ?? []);
    setError("");
    setUploadStatus(null);
  }, [open, post]);

  // Fetch the tenant's sub-orgs once for the admin selector. FSOs never see
  // the dropdown, so don't waste the call.
  useEffect(() => {
    if (!open || !userIsAdmin) return;
    listSubOrgs()
      .then(r => setSubOrgs(r.sub_orgs ?? []))
      .catch(() => {/* dropdown stays empty; tenant-wide is still selectable */});
  }, [open, userIsAdmin]);

  // The "Default" sub-org is the implicit catch-all every tenant has. It is
  // hidden from the selector because "All sub-organizations (tenant-wide)"
  // already covers the same intent and surfacing both confuses admins. The
  // exception: if the post being edited is currently scoped to Default
  // (legacy data, or an FSO whose session sub-org is Default), keep it
  // visible so the admin can see where the post lives and change it
  // intentionally rather than silently promote it.
  const visibleSubOrgs = subOrgs.filter(
    o => o.name !== "Default" || o.id === post?.sub_org_id,
  );

  async function handleSave() {
    if (!title.trim()) { setError("Title is required"); return; }
    setSaving(true);
    setError("");
    const payload: PostPayload = { title: title.trim(), content, published };
    if (userIsAdmin) {
      payload.sub_org_id = subOrgId || null;
    }
    try {
      if (post?.id) {
        await adminUpdatePost(post.id, payload);
        onSaved();
        onClose();
      } else {
        const created = await adminCreatePost(payload);
        onSaved(created); // parent reopens modal in edit mode with the new post
      }
    } catch {
      setError("Failed to save post");
    } finally {
      setSaving(false);
    }
  }

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file || !post?.id) return;
    e.target.value = "";

    setUploading(true);
    setUploadStatus(null);
    try {
      const uploaded = await adminUploadFile(post.id, file);
      setFiles(prev => [...prev, uploaded]);
      onSaved(); // refresh tile grid
      setUploadStatus({ kind: "success", message: `${file.name} uploaded and scanned` });
    } catch (err) {
      setUploadStatus({ kind: "error", message: err instanceof Error ? err.message : "Upload failed" });
    } finally {
      setUploading(false);
    }
  }

  async function handleDeleteFile(fileId: string) {
    try {
      await adminDeleteFile(fileId);
      setFiles(prev => prev.filter(f => f.id !== fileId));
      onSaved();
    } catch {
      setUploadStatus({ kind: "error", message: "Failed to delete file" });
    }
  }

  const isEdit = !!post?.id;

  return (
    <Modal open={open} onClose={onClose} title={isEdit ? "Edit Post" : "New Post"}>
      {error && <div className="alert alert-error" style={{ marginBottom: 12 }}>{error}</div>}

      <div style={{ marginBottom: 12 }}>
        <label style={{ display: "block", fontSize: 12, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--color-text-muted)", marginBottom: 6 }}>
          Title
        </label>
        <input
          type="text"
          className="input"
          value={title}
          onChange={e => setTitle(e.target.value)}
          placeholder="Post title"
          autoFocus
        />
      </div>

      {userIsAdmin && (
        <div style={{ marginBottom: 12 }}>
          <label style={{ display: "block", fontSize: 12, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--color-text-muted)", marginBottom: 6 }}>
            Sub-organization
          </label>
          <select
            className="input"
            value={subOrgId}
            onChange={e => setSubOrgId(e.target.value)}
            style={{ width: "100%" }}
          >
            <option value="">All sub-organizations (tenant-wide)</option>
            {visibleSubOrgs.map(o => (
              <option key={o.id} value={o.id}>{o.name}</option>
            ))}
          </select>
          <p style={{ fontSize: 12, color: "var(--color-text-muted)", margin: "6px 0 0" }}>
            Tenant-wide posts are visible to everyone; sub-org posts only appear in that sub-org's wiki.
          </p>
        </div>
      )}

      <div style={{ marginBottom: 12 }}>
        <label style={{ display: "block", fontSize: 12, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--color-text-muted)", marginBottom: 6 }}>
          Content
        </label>
        <textarea
          style={{ width: "100%", minHeight: 160, padding: "0.6rem 0.75rem", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", fontFamily: "inherit", fontSize: "0.9rem", resize: "vertical", background: "var(--color-surface)", color: "var(--color-text)", boxSizing: "border-box", lineHeight: 1.6 }}
          value={content}
          onChange={e => setContent(e.target.value)}
          placeholder="Write your post here..."
        />
      </div>

      {/* File attachments — only available after the post is saved (has an ID) */}
      {isEdit && (
        <div style={{ marginBottom: 12 }}>
          <div style={{ fontSize: 12, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--color-text-muted)", marginBottom: 8 }}>
            PDF Attachments
          </div>

          {uploadStatus && (
            <div
              role={uploadStatus.kind === "error" ? "alert" : "status"}
              style={{
                fontSize: 13,
                marginBottom: 8,
                padding: "6px 10px",
                borderRadius: "var(--radius-md)",
                border: `1px solid ${uploadStatus.kind === "error" ? "var(--color-danger, #ef4444)" : "var(--color-success, #2A9E5C)"}`,
                color: uploadStatus.kind === "error" ? "var(--color-danger, #ef4444)" : "var(--color-success, #2A9E5C)",
              }}
            >
              {uploadStatus.kind === "success" ? "✓ " : "✗ "}
              {uploadStatus.message}
            </div>
          )}

          {files.length > 0 && (
            <div style={{ display: "flex", flexDirection: "column", gap: 6, marginBottom: 10 }}>
              {files.map(f => (
                <div key={f.id} style={{ display: "flex", alignItems: "center", gap: 8, padding: "6px 10px", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", background: "var(--color-bg-elevated)", fontSize: 13 }}>
                  <FileText size={14} style={{ color: "var(--color-text-muted)", flexShrink: 0 }} />
                  <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{f.file_name}</span>
                  <span style={{ fontSize: 12, color: "var(--color-text-muted)", flexShrink: 0 }}>{(f.file_size / 1024).toFixed(0)} KB</span>
                  <button
                    type="button"
                    onClick={() => handleDeleteFile(f.id)}
                    style={{ background: "none", border: "none", cursor: "pointer", color: "var(--color-text-muted)", padding: 0, display: "flex", alignItems: "center" }}
                    title="Remove"
                  >
                    <X size={14} />
                  </button>
                </div>
              ))}
            </div>
          )}

          <button
            type="button"
            className="btn btn-sm"
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
            style={{ display: "flex", alignItems: "center", gap: 6 }}
          >
            <Paperclip size={14} />
            {uploading ? "Uploading..." : "Attach PDF"}
          </button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".pdf,application/pdf"
            style={{ display: "none" }}
            onChange={handleFileChange}
          />
          <p style={{ fontSize: 12, color: "var(--color-text-muted)", margin: "6px 0 0" }}>PDF files only, max 20 MB</p>
        </div>
      )}

      {!isEdit && (
        <p style={{ fontSize: 12, color: "var(--color-text-muted)", marginBottom: 12 }}>
          Save the post first, then reopen it to attach PDF files.
        </p>
      )}

      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 16 }}>
        <input
          type="checkbox"
          id="wiki-published"
          checked={published}
          onChange={e => setPublished(e.target.checked)}
          style={{ cursor: "pointer" }}
        />
        <label htmlFor="wiki-published" style={{ fontSize: 13, cursor: "pointer" }}>
          Publish immediately (visible to all users)
        </label>
      </div>

      <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, borderTop: "1px solid var(--color-border)", paddingTop: 16 }}>
        <button className="btn btn-sm" onClick={onClose} disabled={saving}>Cancel</button>
        <button className="btn btn-primary" style={{ width: "auto" }} onClick={handleSave} disabled={saving}>
          {saving ? "Saving..." : isEdit ? "Save Changes" : "Save Post"}
        </button>
      </div>
    </Modal>
  );
}

// --- Wiki tile ---

interface WikiTileProps {
  title: string;
  subtitle?: string;
  body?: string;
  icon?: React.ReactNode;
  onClick?: () => void;
  actions?: React.ReactNode;
  draft?: boolean;
  fileCount?: number;
}

function WikiTile({ title, subtitle, body, icon, onClick, actions, draft, fileCount }: WikiTileProps) {
  return (
    <div
      style={{
        background: "var(--color-bg-elevated)",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-md)",
        padding: "1.25rem",
        display: "flex",
        flexDirection: "column",
        gap: 8,
        cursor: onClick ? "pointer" : "default",
        position: "relative",
        transition: "border-color 0.15s",
      }}
      onClick={onClick}
      onMouseEnter={e => onClick && ((e.currentTarget as HTMLDivElement).style.borderColor = "var(--color-primary)")}
      onMouseLeave={e => onClick && ((e.currentTarget as HTMLDivElement).style.borderColor = "var(--color-border)")}
    >
      {draft && (
        <span style={{ position: "absolute", top: 10, right: 10, fontSize: 11, padding: "2px 8px", borderRadius: "var(--radius-sm)", background: "var(--color-border)", color: "var(--color-text-muted)", fontWeight: 600 }}>
          Draft
        </span>
      )}
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        {icon && <div style={{ color: "var(--color-primary)", flexShrink: 0 }}>{icon}</div>}
        <div style={{ fontWeight: 600, fontSize: 15, flex: 1, paddingRight: draft ? 48 : 0 }}>{title}</div>
      </div>
      {subtitle && <div style={{ fontSize: 12, color: "var(--color-text-muted)" }}>{subtitle}</div>}
      {body && (
        <div style={{ fontSize: 13, color: "var(--color-text-muted)", overflow: "hidden", display: "-webkit-box", WebkitLineClamp: 3, WebkitBoxOrient: "vertical" }}>
          {body}
        </div>
      )}
      {fileCount !== undefined && fileCount > 0 && (
        <div style={{ display: "flex", alignItems: "center", gap: 4, fontSize: 12, color: "var(--color-text-muted)" }}>
          <Paperclip size={12} />
          {fileCount} attachment{fileCount !== 1 ? "s" : ""}
        </div>
      )}
      {actions && (
        <div style={{ display: "flex", gap: 6, marginTop: 4 }} onClick={e => e.stopPropagation()}>
          {actions}
        </div>
      )}
    </div>
  );
}

// --- IC view ---

function ICWikiView() {
  const [viewPost, setViewPost] = useState<WikiPost | null>(null);

  const { data: posts = [], isLoading, isError } = useQuery({
    queryKey: ["wiki-posts"],
    queryFn: listPublishedPosts,
  });
  const { data: fso } = useQuery({
    queryKey: ["wiki-fso"],
    queryFn: getFSOInfo,
  });

  return (
    <>
      <PageHeader title="Government Security Homepage" description="Security policies, procedures, and resources from your FSO" />

      {isError && (
        <div className="alert alert-error" style={{ marginBottom: 16 }}>Failed to load posts. Please refresh.</div>
      )}

      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))", gap: 16 }}>
        {/* Your FSO tile — always first */}
        <WikiTile
          title="Your FSO"
          icon={<User size={20} />}
          subtitle={fso ? fso.email : undefined}
          body={fso ? `Contact: ${fso.name}` : "No FSO has been assigned to your organisation yet."}
        />

        {posts.map(post => (
          <WikiTile
            key={post.id}
            title={post.title}
            icon={<Globe size={20} />}
            subtitle={`${post.creator_name} · ${new Date(post.created_at).toLocaleDateString()}`}
            body={post.content}
            fileCount={post.files.length}
            onClick={() => setViewPost(post)}
          />
        ))}

        {isLoading && (
          <div style={{ gridColumn: "1 / -1", textAlign: "center", color: "var(--color-text-muted)", padding: "2rem", fontSize: 14 }}>
            Loading...
          </div>
        )}

        {!isLoading && posts.length === 0 && (
          <div style={{ gridColumn: "1 / -1", textAlign: "center", color: "var(--color-text-muted)", padding: "2rem", fontSize: 14 }}>
            No posts have been published yet.
          </div>
        )}
      </div>

      <PostViewModal post={viewPost} onClose={() => setViewPost(null)} />
    </>
  );
}

// --- Admin view ---

function AdminWikiView() {
  const queryClient = useQueryClient();
  const role = useAuthStore(s => s.user?.role);
  const readOnly = isReadOnlyFSO(role);
  const [viewPost, setViewPost] = useState<WikiPost | null>(null);
  const [editPost, setEditPost] = useState<WikiPost | null | "new">(null);
  const [deleteError, setDeleteError] = useState("");
  const [subOrgId, setSubOrgId] = useState("");

  const { data: posts = [], isLoading, isError } = useQuery({
    queryKey: ["wiki-posts-admin", subOrgId],
    queryFn: () => adminListPosts(subOrgId || undefined),
  });
  const { data: fso } = useQuery({
    queryKey: ["wiki-fso"],
    queryFn: getFSOInfo,
  });

  function invalidate() {
    void queryClient.invalidateQueries({ queryKey: ["wiki-posts-admin"] });
    void queryClient.invalidateQueries({ queryKey: ["wiki-posts"] });
  }

  function handleSaved(created?: WikiPost) {
    invalidate();
    if (created) {
      // New post just created — reopen in edit mode so files can be attached
      setEditPost(created);
    }
  }

  async function handleDelete(post: WikiPost) {
    if (!confirm(`Delete "${post.title}"? This cannot be undone.`)) return;
    setDeleteError("");
    try {
      await adminDeletePost(post.id);
      invalidate();
    } catch {
      setDeleteError(`Failed to delete "${post.title}". Please try again.`);
    }
  }

  return (
    <>
      <PageHeader
        title="Government Security Homepage"
        description="Manage wiki posts for your organisation"
        actions={!readOnly && (
          <button className="btn btn-primary" style={{ width: "auto" }} onClick={() => setEditPost("new")}>
            <Plus size={16} style={{ marginRight: 6 }} /> New Post
          </button>
        )}
      />

      {deleteError && (
        <div className="alert alert-error" style={{ marginBottom: 16 }}>{deleteError}</div>
      )}

      {isError && (
        <div className="alert alert-error" style={{ marginBottom: 16 }}>Failed to load posts. Please refresh.</div>
      )}

      <div style={{ marginBottom: 16 }}>
        <SubOrgFilter value={subOrgId} onChange={setSubOrgId} />
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))", gap: 16 }}>
        {/* Your FSO tile */}
        <WikiTile
          title="Your FSO"
          icon={<User size={20} />}
          subtitle={fso ? fso.email : undefined}
          body={fso ? `Contact: ${fso.name}` : "No FSO assigned. Add an FSO-role user to display contact info here."}
        />

        {isLoading && (
          <div style={{ gridColumn: "1 / -1", textAlign: "center", color: "var(--color-text-muted)", padding: "2rem", fontSize: 14 }}>
            Loading...
          </div>
        )}

        {posts.map(post => (
          <WikiTile
            key={post.id}
            title={post.title}
            icon={<Globe size={20} />}
            subtitle={`${post.creator_name} · ${new Date(post.created_at).toLocaleDateString()}`}
            body={post.content}
            draft={!post.published}
            fileCount={post.files.length}
            onClick={() => setViewPost(post)}
            actions={!readOnly ? (
              <>
                <button
                  className="btn btn-sm"
                  onClick={() => setEditPost(post)}
                  style={{ display: "flex", alignItems: "center", gap: 4 }}
                >
                  <Pencil size={13} /> Edit
                </button>
                <button
                  className="btn btn-sm btn-danger"
                  onClick={() => handleDelete(post)}
                  style={{ display: "flex", alignItems: "center", gap: 4 }}
                >
                  <Trash2 size={13} /> Delete
                </button>
              </>
            ) : undefined}
          />
        ))}

        {posts.length === 0 && (
          <div style={{ gridColumn: "1 / -1", textAlign: "center", color: "var(--color-text-muted)", padding: "2rem", fontSize: 14 }}>
            No posts yet. Click "New Post" to create your first wiki entry.
          </div>
        )}
      </div>

      <PostViewModal post={viewPost} onClose={() => setViewPost(null)} />
      <PostEditorModal
        open={!!editPost}
        onClose={() => setEditPost(null)}
        post={editPost === "new" ? null : editPost}
        onSaved={handleSaved}
      />
    </>
  );
}

// --- Main export ---

export function WikiPage() {
  const role = useAuthStore(s => s.user?.role);
  return isAdmin(role) ? <AdminWikiView /> : <ICWikiView />;
}

export default WikiPage;
