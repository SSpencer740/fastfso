import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronDown, ChevronUp } from "lucide-react";
import { Modal } from "../ui/Modal";
import { StatusBadge } from "../ui/StatusBadge";
import { getTask, approveSubmission, rejectSubmission, archiveTask, unarchiveTask, getAdminSubmission } from "../../api/tasks";
import type { AdminSubmission, AssigneeStatus } from "../../api/tasks";
import { AIVerdictPanel } from "./AIVerdictPanel";
import { UploadPreview } from "./UploadPreview";
import { uuid_nil } from "../../utils/uuid";
import { useAuthStore } from "../../stores/authStore";
import { isReadOnlyFSO } from "../../utils/roles";

interface TaskDetailModalProps {
  open: boolean;
  onClose: () => void;
  taskId: string | null;
  onUpdated: () => void;
}

function SubmissionPanel({ completionId }: { completionId: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ["submission", completionId],
    queryFn: () => getAdminSubmission(completionId),
  });

  if (isLoading) return <div style={{ padding: "12px 0", fontSize: 13, color: "var(--color-text-muted)" }}>Loading...</div>;
  if (!data) return null;

  return <SubmissionDetail sub={data} />;
}

function SubmissionDetail({ sub }: { sub: AdminSubmission }) {
  const reqMap = Object.fromEntries(sub.requirements.map(r => [r.id, r]));
  const responseMap = Object.fromEntries(sub.responses.map(r => [r.requirement_id, r]));
  const uploadMap: Record<string, typeof sub.uploads> = {};
  for (const u of sub.uploads) {
    uploadMap[u.requirement_id] = [...(uploadMap[u.requirement_id] ?? []), u];
  }

  return (
    <div style={{ marginTop: 10, padding: "12px 14px", background: "var(--color-bg-elevated)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", fontSize: 13 }}>
      {sub.requirements.map(req => {
        const response = responseMap[req.id];
        const uploads = uploadMap[req.id] ?? [];
        void reqMap;

        return (
          <div key={req.id} style={{ marginBottom: 12 }}>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>{req.label}</div>
            {req.kind === "checkbox" && (
              <span style={{ color: response?.bool_value ? "var(--color-success, #22c55e)" : "var(--color-text-muted)" }}>
                {response?.bool_value ? "✓ Checked" : "Not checked"}
              </span>
            )}
            {(req.kind === "text" || req.kind === "textarea") && (
              <p style={{ margin: 0, whiteSpace: "pre-wrap", color: response?.text_value ? "var(--color-text)" : "var(--color-text-muted)" }}>
                {response?.text_value ?? "No response"}
              </p>
            )}
            {req.kind === "file_upload" && (
              uploads.length > 0
                ? uploads.map(u => (
                    <div key={u.id} style={{ marginBottom: 8 }}>
                      <UploadPreview upload={u} />
                      {sub.verifications?.[u.id] && (
                        <AIVerdictPanel uploadId={u.id} verification={sub.verifications[u.id]} />
                      )}
                    </div>
                  ))
                : <span style={{ color: "var(--color-text-muted)" }}>No file uploaded</span>
            )}
          </div>
        );
      })}
      {sub.requirements.length === 0 && (
        <span style={{ color: "var(--color-text-muted)" }}>No requirements for this task.</span>
      )}
    </div>
  );
}

function AssigneeRow({
  assignee,
  onApprove,
  onReject,
  readOnly,
}: {
  assignee: AssigneeStatus;
  onApprove: () => void;
  onReject: () => void;
  readOnly?: boolean;
}) {
  const [expanded, setExpanded] = useState(false);
  const canView = ["submitted", "approved", "rejected"].includes(assignee.status) && assignee.completion_id !== uuid_nil;

  return (
    <div style={{ border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", background: "var(--color-bg-elevated)", overflow: "hidden" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 12, padding: "10px 12px" }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontWeight: 500, fontSize: 14 }}>{assignee.user_name}</div>
          <div style={{ fontSize: 12, color: "var(--color-text-muted)" }}>{assignee.user_email}</div>
        </div>
        <StatusBadge status={assignee.status} />
        {!readOnly && assignee.status === "submitted" && (
          <div style={{ display: "flex", gap: 4 }}>
            <button className="btn btn-sm" style={{ color: "#1A7A5A", borderColor: "#1A7A5A" }} onClick={onApprove}>Approve</button>
            <button className="btn btn-sm btn-danger" onClick={onReject}>Reject</button>
          </div>
        )}
        {canView && (
          <button
            type="button"
            className="btn btn-sm"
            onClick={() => setExpanded(e => !e)}
            style={{ display: "flex", alignItems: "center", gap: 4 }}
          >
            {expanded ? <><ChevronUp size={14} /> Hide</> : <><ChevronDown size={14} /> View</>}
          </button>
        )}
      </div>
      {expanded && <div style={{ borderTop: "1px solid var(--color-border)", padding: "0 12px 12px" }}>
        <SubmissionPanel completionId={assignee.completion_id} />
      </div>}
    </div>
  );
}

export function TaskDetailModal({ open, onClose, taskId, onUpdated }: TaskDetailModalProps) {
  const queryClient = useQueryClient();
  const role = useAuthStore((s) => s.user?.role);
  const readOnly = isReadOnlyFSO(role);
  const [error, setError] = useState("");

  const { data: detail } = useQuery({
    queryKey: ["task", taskId],
    queryFn: () => getTask(taskId!),
    enabled: open && !!taskId,
  });

  async function handleApprove(userId: string) {
    if (!taskId) return;
    try {
      await approveSubmission(taskId, userId);
      void queryClient.invalidateQueries({ queryKey: ["task", taskId] });
      onUpdated();
    } catch { setError("Failed to approve"); }
  }

  async function handleReject(userId: string) {
    if (!taskId) return;
    try {
      await rejectSubmission(taskId, userId);
      void queryClient.invalidateQueries({ queryKey: ["task", taskId] });
      onUpdated();
    } catch { setError("Failed to reject"); }
  }

  async function handleArchive() {
    if (!taskId) return;
    if (!confirm("Archive this task? It will be hidden from all assignees immediately.")) return;
    try {
      await archiveTask(taskId);
      onUpdated();
      onClose();
    } catch { setError("Failed to archive task"); }
  }

  async function handleUnarchive() {
    if (!taskId) return;
    try {
      await unarchiveTask(taskId);
      void queryClient.invalidateQueries({ queryKey: ["task", taskId] });
      onUpdated();
    } catch { setError("Failed to unarchive task"); }
  }

  if (!detail) return null;

  return (
    <Modal open={open} onClose={onClose} title={detail.task.title} size="wide">
      {error && <div className="alert alert-error">{error}</div>}

      <div style={{ display: "flex", gap: 12, marginBottom: 16, fontSize: 13, color: "var(--color-text-muted)" }}>
        <StatusBadge status={detail.task.status} />
        <StatusBadge status={detail.task.priority} />
        {detail.task.due_date && <span>Due {new Date(detail.task.due_date).toLocaleDateString()}</span>}
      </div>

      {detail.task.description && (
        <p style={{ fontSize: 14, color: "var(--color-text-muted)", marginBottom: 16 }}>{detail.task.description}</p>
      )}

      <div className="section-header">REQUIREMENTS ({(detail.requirements ?? []).length})</div>
      {(detail.requirements ?? []).map(req => (
        <div key={req.id} style={{ fontSize: 13, padding: "4px 0", color: "var(--color-text-muted)" }}>
          {req.label} — <span style={{ color: "var(--color-text-muted)" }}>{req.kind.replace(/_/g, " ")}</span>
          {req.required && <span style={{ color: "#C01720" }}> (required)</span>}
        </div>
      ))}

      {!readOnly && (
        <div style={{ display: "flex", justifyContent: "flex-end", marginBottom: 20 }}>
          {detail.task.status === "active" && (
            <button className="btn btn-sm btn-danger" onClick={handleArchive}>
              Archive Task
            </button>
          )}
          {detail.task.status === "archived" && (
            <button className="btn btn-sm" onClick={handleUnarchive}>
              Unarchive Task
            </button>
          )}
        </div>
      )}

      <div className="section-header" style={{ marginTop: 0 }}>ASSIGNEES ({detail.assignees?.length || 0})</div>
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {(detail.assignees || []).map(a => (
          <AssigneeRow
            key={a.user_id}
            assignee={a}
            onApprove={() => handleApprove(a.user_id)}
            onReject={() => handleReject(a.user_id)}
            readOnly={readOnly}
          />
        ))}
      </div>
    </Modal>
  );
}
