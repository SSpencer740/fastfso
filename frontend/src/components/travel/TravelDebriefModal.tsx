import { useState } from "react";
import { AlertTriangle } from "lucide-react";
import { Button } from "../ui/Button";
import { Alert } from "../ui/Alert";
import type { TravelDebrief, SubmitDebriefRequest } from "../../api/travel";
import { submitDebrief } from "../../api/travel";

interface Props {
  debrief: TravelDebrief;
  onClose: () => void;
  onSubmitted: () => void;
}

const QUESTIONS: {
  key: keyof Pick<SubmitDebriefRequest, "q1_foreign_contact" | "q2_surveillance" | "q3_equipment_loss" | "q4_unusual_requests">;
  detailKey: keyof Pick<SubmitDebriefRequest, "q1_details" | "q2_details" | "q3_details" | "q4_details">;
  text: string;
}[] = [
  {
    key: "q1_foreign_contact",
    detailKey: "q1_details",
    text: "Were you approached or contacted by any foreign national seeking information about your work, affiliation, or access?",
  },
  {
    key: "q2_surveillance",
    detailKey: "q2_details",
    text: "Did you observe any suspicious surveillance, photography, or monitoring of yourself or your colleagues?",
  },
  {
    key: "q3_equipment_loss",
    detailKey: "q3_details",
    text: "Was any equipment, device, or sensitive material lost, stolen, or potentially compromised during the trip?",
  },
  {
    key: "q4_unusual_requests",
    detailKey: "q4_details",
    text: "Did you receive any unusual or unexpected requests for information, access, or assistance?",
  },
];

export function TravelDebriefModal({ debrief, onClose, onSubmitted }: Props) {
  const [answers, setAnswers] = useState<Record<string, boolean | null>>({
    q1_foreign_contact: null,
    q2_surveillance: null,
    q3_equipment_loss: null,
    q4_unusual_requests: null,
  });
  const [details, setDetails] = useState<Record<string, string>>({
    q1_details: "",
    q2_details: "",
    q3_details: "",
    q4_details: "",
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  const allAnswered = QUESTIONS.every((q) => answers[q.key] !== null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!allAnswered) return;
    setError("");
    setSubmitting(true);
    try {
      const req: SubmitDebriefRequest = {
        q1_foreign_contact: answers.q1_foreign_contact as boolean,
        q1_details: details.q1_details,
        q2_surveillance: answers.q2_surveillance as boolean,
        q2_details: details.q2_details,
        q3_equipment_loss: answers.q3_equipment_loss as boolean,
        q3_details: details.q3_details,
        q4_unusual_requests: answers.q4_unusual_requests as boolean,
        q4_details: details.q4_details,
      };
      await submitDebrief(debrief.id, req);
      onSubmitted();
    } catch {
      setError("Failed to submit debrief. Please try again.");
    } finally {
      setSubmitting(false);
    }
  }

  const isOverdue = debrief.due_date && new Date(debrief.due_date) < new Date();

  return (
    <div
      className="modal-overlay"
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div className="modal" style={{ maxWidth: 600 }}>
        <div className="modal-header">
          <h2>Post-Travel Debrief</h2>
          <button className="modal-close" onClick={onClose}>×</button>
        </div>

        <div className="modal-body">
          <div style={{ marginBottom: 20 }}>
            <div style={{ fontWeight: 600, fontSize: 15 }}>{debrief.trip_name}</div>
            {debrief.due_date && (
              <div style={{ fontSize: 13, marginTop: 4, color: isOverdue ? "var(--color-danger)" : "var(--color-text-muted)", display: "flex", alignItems: "center", gap: 4 }}>
                {isOverdue && <AlertTriangle size={13} />}
                Due {new Date(debrief.due_date).toLocaleDateString()}
              </div>
            )}
          </div>

          <p style={{ fontSize: 13, color: "var(--color-text-muted)", marginBottom: 20 }}>
            As part of your security obligations, please answer the following post-travel debrief questions honestly and completely.
          </p>

          {error && <Alert variant="error">{error}</Alert>}

          <form onSubmit={handleSubmit}>
            <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
              {QUESTIONS.map((q, i) => {
                const answer = answers[q.key];
                return (
                  <div key={q.key}>
                    <div style={{ fontSize: 14, fontWeight: 500, marginBottom: 10, lineHeight: 1.4 }}>
                      {i + 1}. {q.text}
                    </div>
                    <div style={{ display: "flex", gap: 10, marginBottom: answer === true ? 10 : 0 }}>
                      {([true, false] as const).map((val) => (
                        <button
                          key={String(val)}
                          type="button"
                          onClick={() => setAnswers(prev => ({ ...prev, [q.key]: val }))}
                          style={{
                            padding: "6px 20px",
                            fontSize: 13,
                            fontWeight: 600,
                            border: `2px solid ${answer === val ? (val ? "var(--color-danger)" : "var(--color-success)") : "var(--color-border)"}`,
                            borderRadius: "var(--radius-md)",
                            background: answer === val
                              ? val ? "var(--color-danger-muted, rgba(220,53,69,0.1))" : "var(--color-success-muted, rgba(40,167,69,0.1))"
                              : "var(--color-bg-elevated)",
                            color: answer === val
                              ? val ? "var(--color-danger)" : "var(--color-success)"
                              : "var(--color-text)",
                            cursor: "pointer",
                          }}
                        >
                          {val ? "Yes" : "No"}
                        </button>
                      ))}
                    </div>
                    {answer === true && (
                      <textarea
                        value={details[q.detailKey]}
                        onChange={(e) => setDetails(prev => ({ ...prev, [q.detailKey]: e.target.value }))}
                        placeholder="Please provide details..."
                        rows={3}
                        style={{
                          width: "100%",
                          boxSizing: "border-box",
                          padding: "8px 10px",
                          fontSize: 13,
                          border: "1px solid var(--color-border)",
                          borderRadius: "var(--radius-md)",
                          background: "var(--color-bg-elevated)",
                          color: "var(--color-text)",
                          resize: "vertical",
                        }}
                      />
                    )}
                  </div>
                );
              })}
            </div>

            <div style={{ marginTop: 24, display: "flex", gap: 10, justifyContent: "flex-end" }}>
              <Button type="button" size="sm" onClick={onClose}>Cancel</Button>
              <Button
                type="submit"
                size="sm"
                disabled={!allAnswered}
                loading={submitting}
                loadingText="Submitting..."
              >
                Submit Debrief
              </Button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
