import { useState } from "react";
import { Modal } from "../ui/Modal";
import { Alert } from "../ui/Alert";
import { Button } from "../ui/Button";
import { FormField } from "../ui/FormField";
import {
  SELF_REPORT_TYPES,
  FCL_REPORT_TYPES,
  REPORT_TYPE_INSTRUCTIONS,
  createReport,
} from "../../api/reports";
import { ApiError } from "../../api/client";

interface ReportWizardProps {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
  showFCL: boolean;
}

type WizardMode = "self" | "other" | "fcl";

const MODE_OPTIONS: { value: WizardMode; label: string; sub: string }[] = [
  { value: "self",  label: "Self",  sub: "Reporting for myself" },
  { value: "other", label: "Other", sub: "On behalf of someone else" },
];

const FCL_MODE: { value: WizardMode; label: string; sub: string } = {
  value: "fcl", label: "FCL", sub: "Facilities clearance reporting",
};

export function ReportWizard({ open, onClose, onCreated, showFCL }: ReportWizardProps) {
  const [mode, setMode] = useState<WizardMode | null>(null);
  const [subjectName, setSubjectName] = useState("");
  const [reportType, setReportType] = useState("");
  const [details, setDetails] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  function reset() {
    setMode(null);
    setSubjectName("");
    setReportType("");
    setDetails("");
    setError("");
  }

  function handleClose() {
    reset();
    onClose();
  }

  function handleModeChange(m: WizardMode) {
    setMode(m);
    setReportType("");
    setDetails("");
  }

  const modeOptions = showFCL ? [...MODE_OPTIONS, FCL_MODE] : MODE_OPTIONS;
  const availableTypes = mode === "fcl" ? FCL_REPORT_TYPES : SELF_REPORT_TYPES;
  const instructions = reportType ? REPORT_TYPE_INSTRUCTIONS[reportType] : null;

  async function handleSubmit() {
    setError("");
    if (!mode) { setError("Please select who you are reporting for."); return; }
    if (!reportType) { setError("Please select a report type."); return; }
    if (!details.trim()) { setError("Please provide details."); return; }
    if (mode === "other" && !subjectName.trim()) { setError("Please enter the name of the person you are reporting for."); return; }

    setSubmitting(true);
    try {
      await createReport({
        reporting_for: mode === "fcl" ? "fcl" : mode === "other" ? "other" : "self",
        subject_name: mode === "other" ? subjectName : "",
        report_type: reportType,
        details,
      });
      onCreated();
      handleClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to submit report");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal open={open} onClose={handleClose} title="Report Life Event">
      {error && <Alert variant="error" onDismiss={() => setError("")}>{error}</Alert>}

      {/* Mode selector */}
      <div style={{ marginBottom: 20 }}>
        <p style={{ margin: "0 0 10px", fontSize: 13, color: "var(--color-text-muted)" }}>
          Who are you reporting for?
        </p>
        <div style={{ display: "flex", gap: 8 }}>
          {modeOptions.map(opt => {
            const selected = mode === opt.value;
            return (
              <button
                key={opt.value}
                type="button"
                onClick={() => handleModeChange(opt.value)}
                style={{
                  flex: 1,
                  padding: "10px 12px",
                  borderRadius: "var(--radius-md)",
                  border: `2px solid ${selected ? "var(--color-primary)" : "var(--color-border)"}`,
                  background: selected ? "var(--color-primary-subtle, color-mix(in srgb, var(--color-primary) 10%, transparent))" : "var(--color-bg-elevated)",
                  color: selected ? "var(--color-primary)" : "var(--color-text)",
                  cursor: "pointer",
                  textAlign: "left",
                  transition: "border-color 0.15s, background 0.15s",
                }}
              >
                <div style={{ fontWeight: 600, fontSize: 13 }}>{opt.label}</div>
                <div style={{ fontSize: 12, marginTop: 2, opacity: 0.75 }}>{opt.sub}</div>
              </button>
            );
          })}
        </div>
      </div>

      {/* Fields that appear once a mode is chosen */}
      {mode && (
        <>
          {mode === "other" && (
            <FormField label="Name of person you are reporting for" htmlFor="subject-name">
              <input
                id="subject-name"
                type="text"
                value={subjectName}
                onChange={e => setSubjectName(e.target.value)}
                placeholder="Full name"
              />
            </FormField>
          )}

          <FormField label="Report Type">
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              {availableTypes.map(t => {
                const selected = reportType === t.value;
                return (
                  <button
                    key={t.value}
                    type="button"
                    onClick={() => { setReportType(t.value); setDetails(""); }}
                    style={{
                      padding: "8px 12px",
                      borderRadius: "var(--radius-md)",
                      border: `2px solid ${selected ? "var(--color-primary)" : "var(--color-border)"}`,
                      background: selected ? "var(--color-primary-subtle, color-mix(in srgb, var(--color-primary) 10%, transparent))" : "var(--color-bg-elevated)",
                      color: selected ? "var(--color-primary)" : "var(--color-text)",
                      cursor: "pointer",
                      textAlign: "left",
                      fontSize: 13,
                      fontWeight: selected ? 600 : 400,
                      transition: "border-color 0.15s, background 0.15s",
                    }}
                  >
                    {t.label}
                  </button>
                );
              })}
            </div>
          </FormField>

          {instructions && (
            <div style={{
              background: "var(--color-bg-elevated)",
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-md)",
              padding: "0.75rem 1rem",
              marginBottom: 16,
              fontSize: 13,
              color: "var(--color-text-muted)",
              lineHeight: 1.6,
            }}>
              {instructions}
            </div>
          )}

          {reportType && (
            <FormField label="Details" htmlFor="report-details">
              <textarea
                id="report-details"
                rows={6}
                value={details}
                onChange={e => setDetails(e.target.value)}
                placeholder="Provide the requested information here..."
                style={{
                  width: "100%",
                  padding: "0.6rem 0.75rem",
                  border: "1px solid var(--color-border-strong)",
                  borderRadius: "var(--radius-md)",
                  background: "var(--color-bg-elevated)",
                  color: "inherit",
                  fontSize: "1rem",
                  fontFamily: "inherit",
                  resize: "vertical",
                }}
              />
            </FormField>
          )}

          <div style={{ display: "flex", justifyContent: "flex-end", paddingTop: 16, borderTop: "1px solid var(--color-border)" }}>
            <Button
              variant="primary"
              onClick={handleSubmit}
              loading={submitting}
              loadingText="Submitting..."
              disabled={!reportType || !details.trim() || (mode === "other" && !subjectName.trim())}
            >
              Submit Report
            </Button>
          </div>
        </>
      )}
    </Modal>
  );
}
