import { useState, useEffect, useCallback, useRef } from "react";
import { AlertTriangle, Loader2, CheckCircle2 } from "lucide-react";
import { Modal } from "../ui/Modal";
import { StatusBadge } from "../ui/StatusBadge";
import { FileUploadZone } from "./FileUploadZone";
import type { MyTaskDetail, Upload, SaveResponseParams, Verification } from "../../api/tasks";
import { getMyTask, saveResponses, submitTask } from "../../api/tasks";

interface CompleteTaskModalProps {
  open: boolean;
  onClose: () => void;
  taskId: string | null;
  onSubmitted: () => void;
}

export function CompleteTaskModal({ open, onClose, taskId, onSubmitted }: CompleteTaskModalProps) {
  const [detail, setDetail] = useState<MyTaskDetail | null>(null);
  const [formValues, setFormValues] = useState<Record<string, string | boolean>>({});
  const [uploads, setUploads] = useState<Upload[]>([]);
  const [verifications, setVerifications] = useState<Record<string, Verification>>({});
  const [certified, setCertified] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const pollRef = useRef<number | null>(null);

  const load = useCallback(async () => {
    if (!taskId) return;
    try {
      const d = await getMyTask(taskId);
      setDetail(d);
      setUploads(d.uploads || []);
      setVerifications(d.verifications || {});
      // Pre-fill from existing responses
      const vals: Record<string, string | boolean> = {};
      for (const resp of (d.responses || [])) {
        if (resp.text_value != null) vals[resp.requirement_id] = resp.text_value;
        if (resp.bool_value != null) vals[resp.requirement_id] = resp.bool_value;
      }
      setFormValues(vals);
    } catch {
      setError("Failed to load task");
    }
  }, [taskId]);

  useEffect(() => {
    if (open && taskId) {
      setError("");
      setCertified(false);
      load();
    }
  }, [open, taskId, load]);

  // Poll while any verification is pending, so the banner updates without
  // requiring the user to refresh. Stops as soon as all verifications have
  // landed in a terminal state.
  useEffect(() => {
    const pending = Object.values(verifications).some(v => v.status === "pending");
    if (!open || !pending) {
      if (pollRef.current) {
        window.clearTimeout(pollRef.current);
        pollRef.current = null;
      }
      return;
    }
    pollRef.current = window.setTimeout(() => { void load(); }, 3000);
    return () => {
      if (pollRef.current) window.clearTimeout(pollRef.current);
    };
  }, [verifications, open, load]);

  async function handleSave() {
    if (!detail) return;
    const responses: SaveResponseParams[] = detail.requirements.map(req => ({
      requirement_id: req.id,
      text_value: typeof formValues[req.id] === "string" ? formValues[req.id] as string : null,
      bool_value: typeof formValues[req.id] === "boolean" ? formValues[req.id] as boolean : null,
    }));
    await saveResponses(detail.task.id, responses);
  }

  async function handleSubmit() {
    if (!detail || !certified) return;
    setSubmitting(true);
    setError("");
    try {
      await handleSave();
      await submitTask(detail.task.id);
      onSubmitted();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to submit");
    } finally {
      setSubmitting(false);
    }
  }

  if (!detail) return null;

  const isReadOnly = detail.completion.status === "submitted" || detail.completion.status === "approved";

  return (
    <Modal open={open} onClose={onClose} title={detail.task.title}>
      {error && <div className="alert alert-error">{error}</div>}

      <div style={{ display: "flex", gap: 16, marginBottom: 16, fontSize: 13, color: "#6B8294" }}>
        {detail.task.due_date && <span>Due {new Date(detail.task.due_date).toLocaleDateString()}</span>}
        <span>Assigned by {detail.creator_name}</span>
        <StatusBadge status={detail.task.priority} />
      </div>

      {detail.task.description && (
        <p style={{ fontSize: 14, color: "#4A5E6D", marginBottom: 16 }}>{detail.task.description}</p>
      )}

      {detail.requirements.map(req => (
        <div key={req.id} className="form-field">
          <label>
            {req.label}
            {req.required && <span style={{ color: "#C01720" }}> *</span>}
          </label>
          {req.kind === "text" && (
            <input
              placeholder={req.description || `Enter ${req.label.toLowerCase()}...`}
              value={(formValues[req.id] as string) || ""}
              onChange={e => setFormValues({ ...formValues, [req.id]: e.target.value })}
              disabled={isReadOnly}
            />
          )}
          {req.kind === "textarea" && (
            <textarea
              style={{ width: "100%", minHeight: 80, padding: "0.6rem 0.75rem", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", fontFamily: "inherit", fontSize: "0.9rem", resize: "vertical", background: "var(--color-surface)", color: "var(--color-text)" }}
              placeholder={req.description || `Enter ${req.label.toLowerCase()}...`}
              value={(formValues[req.id] as string) || ""}
              onChange={e => setFormValues({ ...formValues, [req.id]: e.target.value })}
              disabled={isReadOnly}
            />
          )}
          {req.kind === "file_upload" && (
            <>
              <FileUploadZone
                taskId={detail.task.id}
                requirementId={req.id}
                uploads={uploads}
                readOnly={isReadOnly}
                onUploadsChange={u => {
                  setUploads(u);
                  // Refresh task detail so a newly-created pending verification
                  // shows up immediately — the polling effect needs at least
                  // one pending entry in state to start its loop.
                  if (req.ai_verification_criteria) void load();
                }}
              />
              {req.ai_verification_criteria && (
                <div style={{ marginTop: 6, fontSize: 12, color: "var(--color-text-muted)" }}>
                  AI verification will check this upload against the task criteria.
                </div>
              )}
              {uploads
                .filter(u => u.requirement_id === req.id)
                .map(u => {
                  const v = verifications[u.id];
                  if (!v) return null;
                  return <VerificationBanner key={u.id} verification={v} fileName={u.file_name} />;
                })}
            </>
          )}
          {req.kind === "checkbox" && (
            <label style={{ display: "flex", alignItems: "center", gap: 8, cursor: "pointer" }}>
              <input
                type="checkbox"
                checked={!!formValues[req.id]}
                onChange={e => setFormValues({ ...formValues, [req.id]: e.target.checked })}
                disabled={isReadOnly}
              />
              {req.description || req.label}
            </label>
          )}
        </div>
      ))}

      {!isReadOnly && Object.values(verifications).some(v => v.flagged) && (
        <div style={{
          marginTop: 12,
          padding: "10px 12px",
          background: "var(--color-warning-bg, #FFF7E6)",
          border: "1px solid var(--color-warning, #E89B16)",
          borderRadius: "var(--radius-md)",
          fontSize: 13,
          color: "var(--color-text)",
        }}>
          <strong>AI flagged one or more of your uploads.</strong> Please double-check the file{Object.values(verifications).filter(v => v.flagged).length === 1 ? "" : "s"} before submitting. You can still submit if you believe the upload is correct.
        </div>
      )}

      {!isReadOnly && (
        <>
          <label style={{ display: "flex", alignItems: "center", gap: 8, margin: "16px 0", fontSize: 14, cursor: "pointer" }}>
            <input type="checkbox" checked={certified} onChange={e => setCertified(e.target.checked)} />
            I certify that the information provided is accurate and complete
          </label>

          <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, paddingTop: 16, borderTop: "1px solid var(--color-border)" }}>
            <button type="button" className="btn btn-secondary" onClick={onClose}>Cancel</button>
            <button
              type="button"
              className="btn btn-primary"
              style={{ width: "auto" }}
              disabled={!certified || submitting}
              onClick={handleSubmit}
            >
              {submitting
                ? "Submitting..."
                : Object.values(verifications).some(v => v.flagged)
                  ? "Submit anyway"
                  : "Submit Task"}
            </button>
          </div>
        </>
      )}
    </Modal>
  );
}

// VerificationBanner renders the per-upload AI verification status under the
// upload zone. Only the user-visible bits are shown — the admin gets the full
// reasoning in the admin review modal.
function VerificationBanner({ verification, fileName }: { verification: Verification; fileName: string }) {
  if (verification.status === "pending") {
    return (
      <div style={{
        marginTop: 6,
        padding: "6px 10px",
        background: "var(--color-bg-elevated)",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-sm)",
        fontSize: 12,
        color: "var(--color-text-muted)",
        display: "flex",
        alignItems: "center",
        gap: 6,
      }}>
        <Loader2 size={12} className="spin" />
        Verifying {fileName}…
      </div>
    );
  }
  if (verification.status === "failed") {
    return (
      <div style={{
        marginTop: 6,
        padding: "6px 10px",
        background: "var(--color-bg-elevated)",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-sm)",
        fontSize: 12,
        color: "var(--color-text-muted)",
      }}>
        AI verification unavailable for {fileName}. An admin will review manually.
      </div>
    );
  }
  if (verification.flagged) {
    return (
      <div style={{
        marginTop: 6,
        padding: "8px 10px",
        background: "var(--color-warning-bg, #FFF7E6)",
        border: "1px solid var(--color-warning, #E89B16)",
        borderRadius: "var(--radius-sm)",
        fontSize: 13,
        color: "var(--color-text)",
        display: "flex",
        alignItems: "flex-start",
        gap: 8,
      }}>
        <AlertTriangle size={14} style={{ marginTop: 2, color: "var(--color-warning, #E89B16)", flexShrink: 0 }} />
        <div>
          <div><strong>AI flagged {fileName}.</strong> Make sure this is the right file before submitting.</div>
          {verification.discrepancies.length > 0 && (
            <ul style={{ marginTop: 4, marginBottom: 0, paddingLeft: 18, fontSize: 12 }}>
              {verification.discrepancies.map((d, i) => <li key={i}>{d}</li>)}
            </ul>
          )}
        </div>
      </div>
    );
  }
  return (
    <div style={{
      marginTop: 6,
      padding: "6px 10px",
      background: "var(--color-bg-elevated)",
      border: "1px solid var(--color-border)",
      borderRadius: "var(--radius-sm)",
      fontSize: 12,
      color: "var(--color-text-muted)",
      display: "flex",
      alignItems: "center",
      gap: 6,
    }}>
      <CheckCircle2 size={12} style={{ color: "var(--color-success, #28A745)" }} />
      AI verification passed for {fileName}.
    </div>
  );
}
