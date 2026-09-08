import { useState, useEffect } from "react";
import { Modal } from "../ui/Modal";
import { StatusBadge } from "../ui/StatusBadge";
import { Spinner } from "../ui/Spinner";
import { Alert } from "../ui/Alert";
import { type Report, getMyReport, getAdminReport, updateReportStatus, formatReportType } from "../../api/reports";
import { ApiError } from "../../api/client";
import { useAuthStore } from "../../stores/authStore";
import { isReadOnlyFSO } from "../../utils/roles";

interface ViewReportModalProps {
  open: boolean;
  onClose: () => void;
  reportId: string | null;
  onUpdated: () => void;
  isAdminView?: boolean;
}

const reportingForLabels: Record<string, string> = {
  self: "Self",
  other: "On behalf of another",
  fcl: "FCL Reporting",
};

export function ViewReportModal({ open, onClose, reportId, onUpdated, isAdminView }: ViewReportModalProps) {
  const role = useAuthStore((s) => s.user?.role);
  const readOnly = isReadOnlyFSO(role);
  const [report, setReport] = useState<Report | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [updating, setUpdating] = useState(false);

  useEffect(() => {
    if (!open || !reportId) return;
    setLoading(true);
    setError("");
    const fetch = isAdminView ? getAdminReport : getMyReport;
    fetch(reportId)
      .then(setReport)
      .catch(err => setError(err instanceof ApiError ? err.message : "Failed to load report"))
      .finally(() => setLoading(false));
  }, [open, reportId, isAdminView]);

  function handleClose() {
    setReport(null);
    setError("");
    onClose();
  }

  async function handleStatusUpdate(status: string) {
    if (!reportId) return;
    setUpdating(true);
    try {
      await updateReportStatus(reportId, status);
      onUpdated();
      handleClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to update status");
    } finally {
      setUpdating(false);
    }
  }

  return (
    <Modal open={open} onClose={handleClose} title="Report Details">
      {loading && <Spinner text="Loading..." />}
      {error && <Alert variant="error">{error}</Alert>}

      {report && (
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", marginBottom: "1.25rem" }}>
            <h4 style={{ margin: 0 }}>{formatReportType(report.report_type)}</h4>
            <StatusBadge status={report.status} />
          </div>

          <div style={{ display: "grid", gap: 6, marginBottom: 16 }}>
            <Row label="Reporting For" value={reportingForLabels[report.reporting_for] ?? report.reporting_for} />
            <Row
              label={report.reporting_for === "self" ? "Reportee" : "Submitted By"}
              value={report.creator_email ? `${report.creator_name} (${report.creator_email})` : report.creator_name}
            />
            {report.subject_name && <Row label="Subject" value={report.subject_name} />}
            <Row label="Submitted" value={new Date(report.created_at).toLocaleDateString()} />
            {report.reviewed_at && <Row label="Reviewed" value={new Date(report.reviewed_at).toLocaleDateString()} />}
          </div>

          <h5 style={{ margin: "0 0 8px", fontSize: 12, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--color-text-muted)" }}>
            Details
          </h5>
          <p style={{ fontSize: 14, whiteSpace: "pre-wrap", background: "var(--color-bg-elevated)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", padding: "0.75rem 1rem", lineHeight: 1.6, margin: 0 }}>
            {report.details}
          </p>

          {isAdminView && !readOnly && report.status !== "processed" && (
            <div style={{ display: "flex", gap: 8, marginTop: 16, paddingTop: 16, borderTop: "1px solid var(--color-border)" }}>
              {report.status === "unreviewed" && (
                <button
                  className="btn btn-sm"
                  style={{ color: "#3A7CA5", borderColor: "#3A7CA5" }}
                  onClick={() => handleStatusUpdate("under_review")}
                  disabled={updating}
                >
                  Mark Under Review
                </button>
              )}
              <button
                className="btn btn-primary"
                style={{ width: "auto" }}
                onClick={() => handleStatusUpdate("processed")}
                disabled={updating}
              >
                Mark Processed
              </button>
            </div>
          )}
        </div>
      )}
    </Modal>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: "flex", gap: 8, fontSize: 13 }}>
      <span style={{ color: "var(--color-text-muted)", minWidth: 120, flexShrink: 0 }}>{label}</span>
      <span style={{ color: "var(--color-text)" }}>{value}</span>
    </div>
  );
}
