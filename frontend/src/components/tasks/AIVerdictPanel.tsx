import { useState } from "react";
import type { Verification } from "../../api/tasks";
import { submitVerificationFeedback } from "../../api/tasks";

// AIVerdictPanel renders the full AI verification result for one upload in an
// admin review surface: status, confidence, extracted fields, discrepancies,
// reasoning, and "AI was correct/incorrect" feedback buttons. Used by both
// ViewActionItemModal (action-item-driven review) and TaskDetailModal
// (per-assignee admin task view).
export function AIVerdictPanel({ uploadId, verification }: { uploadId: string; verification: Verification }) {
  const [feedback, setFeedback] = useState<Verification["admin_feedback"]>(verification.admin_feedback);
  const [busy, setBusy] = useState(false);

  async function sendFeedback(value: "correct" | "incorrect") {
    if (busy) return;
    setBusy(true);
    try {
      await submitVerificationFeedback(uploadId, value);
      setFeedback(value);
    } catch {
      // best-effort; admins can retry
    } finally {
      setBusy(false);
    }
  }

  if (verification.status === "pending") {
    return (
      <div style={{ marginTop: 6, fontSize: 12, color: "var(--color-text-muted)" }}>
        AI verification in progress…
      </div>
    );
  }
  if (verification.status === "failed") {
    return (
      <div style={{
        marginTop: 6,
        padding: "6px 10px",
        fontSize: 12,
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-sm)",
        background: "var(--color-bg-elevated)",
        color: "var(--color-text-muted)",
      }}>
        <div>AI verification failed — manual review required.</div>
        {verification.error_message && (
          <div style={{ marginTop: 4, fontFamily: "var(--font-mono, monospace)", fontSize: 11, wordBreak: "break-word" }}>
            {verification.error_message}
          </div>
        )}
      </div>
    );
  }

  const flaggedStyle = verification.flagged
    ? { borderColor: "var(--color-warning, #E89B16)", background: "var(--color-warning-bg, #FFF7E6)" }
    : { borderColor: "var(--color-border)", background: "var(--color-bg-elevated)" };

  return (
    <div style={{
      marginTop: 6,
      padding: "8px 10px",
      border: "1px solid",
      borderRadius: "var(--radius-sm)",
      fontSize: 12,
      ...flaggedStyle,
    }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6 }}>
        <strong>
          {verification.flagged ? "AI flagged for review" : "AI verification passed"}
        </strong>
        <span style={{ color: "var(--color-text-muted)" }}>
          confidence {verification.confidence != null ? verification.confidence.toFixed(2) : "—"}
          {verification.model && <> · {verification.model}</>}
        </span>
      </div>

      {verification.reasoning && (
        <div style={{ marginBottom: 6, fontStyle: "italic" }}>{verification.reasoning}</div>
      )}

      {Object.keys(verification.extracted_fields).length > 0 && (
        <div style={{ marginBottom: 6 }}>
          <div style={{ color: "var(--color-text-muted)", marginBottom: 2 }}>Extracted:</div>
          <ul style={{ margin: 0, paddingLeft: 18 }}>
            {Object.entries(verification.extracted_fields).map(([k, v]) => (
              <li key={k}><strong>{k}:</strong> {v}</li>
            ))}
          </ul>
        </div>
      )}

      {verification.discrepancies.length > 0 && (
        <div style={{ marginBottom: 6 }}>
          <div style={{ color: "var(--color-text-muted)", marginBottom: 2 }}>Discrepancies:</div>
          <ul style={{ margin: 0, paddingLeft: 18 }}>
            {verification.discrepancies.map((d, i) => <li key={i}>{d}</li>)}
          </ul>
        </div>
      )}

      <div style={{ display: "flex", gap: 8, marginTop: 8, alignItems: "center" }}>
        <span style={{ color: "var(--color-text-muted)" }}>Was the AI correct?</span>
        <button
          type="button"
          onClick={() => void sendFeedback("correct")}
          disabled={busy}
          style={{
            padding: "2px 8px",
            fontSize: 12,
            border: "1px solid var(--color-border)",
            borderRadius: 4,
            background: feedback === "correct" ? "var(--color-primary)" : "transparent",
            color: feedback === "correct" ? "#fff" : "var(--color-text)",
            cursor: busy ? "not-allowed" : "pointer",
          }}
        >
          Correct
        </button>
        <button
          type="button"
          onClick={() => void sendFeedback("incorrect")}
          disabled={busy}
          style={{
            padding: "2px 8px",
            fontSize: 12,
            border: "1px solid var(--color-border)",
            borderRadius: 4,
            background: feedback === "incorrect" ? "var(--color-danger, #C01720)" : "transparent",
            color: feedback === "incorrect" ? "#fff" : "var(--color-text)",
            cursor: busy ? "not-allowed" : "pointer",
          }}
        >
          Incorrect
        </button>
      </div>
    </div>
  );
}
