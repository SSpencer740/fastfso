import { useState, useEffect } from "react";
import { Modal } from "../ui/Modal";
import { Alert } from "../ui/Alert";
import { Button } from "../ui/Button";
import { Spinner } from "../ui/Spinner";
import { StatusBadge } from "../ui/StatusBadge";
import {
  type TravelReportDetail,
  getTravelReport,
  getMyTravelReport,
  approveTravelReport,
  rejectTravelReport,
} from "../../api/travel";
import { useAuthStore } from "../../stores/authStore";
import { isAdmin, isReadOnlyFSO } from "../../utils/roles";
import { formatDate } from "../../utils/dateFormat";

interface TravelReportDetailModalProps {
  open: boolean;
  onClose: () => void;
  reportId: string | null;
  onUpdated: () => void;
}

const sectionStyle: React.CSSProperties = {
  marginBottom: "1.25rem",
  padding: "0.75rem 1rem",
  border: "1px solid var(--color-border)",
  borderRadius: "var(--radius-lg)",
};

const labelStyle: React.CSSProperties = {
  fontSize: "0.75rem",
  color: "var(--color-text-muted)",
  textTransform: "uppercase",
  letterSpacing: "0.05em",
  marginBottom: "0.2rem",
};

const valueStyle: React.CSSProperties = {
  fontSize: "0.9rem",
  marginBottom: "0.6rem",
};

export function TravelReportDetailModal({
  open,
  onClose,
  reportId,
  onUpdated,
}: TravelReportDetailModalProps) {
  const user = useAuthStore((s) => s.user);
  const role = user?.role ?? "";
  const admin = isAdmin(role);
  const readOnly = isReadOnlyFSO(role);

  const [detail, setDetail] = useState<TravelReportDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [actionLoading, setActionLoading] = useState(false);

  useEffect(() => {
    if (open && reportId) {
      loadDetail(reportId);
    } else {
      setDetail(null);
      setError("");
    }
  }, [open, reportId]);

  async function loadDetail(id: string) {
    setLoading(true);
    setError("");
    try {
      const data = await (admin ? getTravelReport(id) : getMyTravelReport(id));
      setDetail(data);
    } catch {
      setError("Failed to load travel report");
    } finally {
      setLoading(false);
    }
  }

  async function handleApprove() {
    if (!reportId) return;
    setActionLoading(true);
    setError("");
    try {
      await approveTravelReport(reportId);
      onUpdated();
      onClose();
    } catch {
      setError("Failed to approve report");
    } finally {
      setActionLoading(false);
    }
  }

  async function handleReject() {
    if (!reportId) return;
    setActionLoading(true);
    setError("");
    try {
      await rejectTravelReport(reportId);
      onUpdated();
      onClose();
    } catch {
      setError("Failed to reject report");
    } finally {
      setActionLoading(false);
    }
  }

  const report = detail?.report;
  const countries = detail?.countries ?? [];
  const uploads = detail?.uploads ?? [];
  const canReview = admin && !readOnly && report && (report.status === "submitted" || report.status === "under_review");

  return (
    <Modal open={open} onClose={onClose} title="Travel Report">
      {error && (
        <Alert variant="error" onDismiss={() => setError("")}>
          {error}
        </Alert>
      )}

      {loading && <Spinner text="Loading report..." />}

      {!loading && report && (
        <div>
          {/* Header */}
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              marginBottom: "1rem",
            }}
          >
            <div>
              <h4 style={{ margin: 0, fontSize: "1.1rem" }}>{report.trip_name}</h4>
              <span style={{ fontSize: "0.8rem", color: "var(--color-text-muted)" }}>
                by {detail.creator_name}
              </span>
            </div>
            <StatusBadge status={report.status} />
          </div>

          {/* Trip Info */}
          <div style={sectionStyle}>
            <h4 style={{ margin: "0 0 0.5rem", fontSize: "0.9rem" }}>Trip Information</h4>
            <div style={labelStyle}>Type</div>
            <div style={valueStyle}>{report.multi_country ? "Multi-country" : "Single country"}</div>
            <div style={labelStyle}>Passport</div>
            <div style={valueStyle}>
              {report.passport_number
                ? "*".repeat(Math.max(0, report.passport_number.length - 4)) +
                  report.passport_number.slice(-4)
                : "-"}
            </div>
          </div>

          {/* Countries */}
          {countries.map((country) => (
            <div key={country.id} style={sectionStyle}>
              <h4 style={{ margin: "0 0 0.5rem", fontSize: "0.9rem" }}>
                {country.country_name}
              </h4>
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 1rem" }}>
                <div>
                  <div style={labelStyle}>Start Date</div>
                  <div style={valueStyle}>{formatDate(country.start_date)}</div>
                </div>
                <div>
                  <div style={labelStyle}>End Date</div>
                  <div style={valueStyle}>{formatDate(country.end_date)}</div>
                </div>
              </div>
              <div style={labelStyle}>Reason</div>
              <div style={valueStyle}>{country.reason || "-"}</div>
              <div style={labelStyle}>Transportation</div>
              <div style={valueStyle}>
                {country.transportation.length > 0 ? country.transportation.join(", ") : "-"}
              </div>
              {country.has_companions && (
                <>
                  <div style={labelStyle}>Companions</div>
                  <div style={valueStyle}>{country.companions_detail || "-"}</div>
                </>
              )}
              {country.has_foreign_contacts && (
                <>
                  <div style={labelStyle}>Foreign Contacts</div>
                  <div style={valueStyle}>{country.contacts_detail || "-"}</div>
                </>
              )}
            </div>
          ))}

          {/* Emergency Contact */}
          <div style={sectionStyle}>
            <h4 style={{ margin: "0 0 0.5rem", fontSize: "0.9rem" }}>Emergency Contact</h4>
            <div style={labelStyle}>Name</div>
            <div style={valueStyle}>
              {report.emergency_first_name || report.emergency_last_name
                ? `${report.emergency_first_name} ${report.emergency_last_name}`.trim()
                : "-"}
            </div>
            <div style={labelStyle}>Phone</div>
            <div style={valueStyle}>{report.emergency_phone || "-"}</div>
          </div>

          {/* Additional Comments */}
          {report.additional_comments && (
            <div style={sectionStyle}>
              <h4 style={{ margin: "0 0 0.5rem", fontSize: "0.9rem" }}>Additional Comments</h4>
              <div style={{ fontSize: "0.9rem", whiteSpace: "pre-wrap" }}>
                {report.additional_comments}
              </div>
            </div>
          )}

          {/* Uploads */}
          {uploads.length > 0 && (
            <div style={sectionStyle}>
              <h4 style={{ margin: "0 0 0.5rem", fontSize: "0.9rem" }}>
                Attachments ({uploads.length})
              </h4>
              {uploads.map((u) => (
                <div
                  key={u.id}
                  style={{
                    fontSize: "0.85rem",
                    padding: "0.25rem 0",
                    color: "var(--color-text-secondary)",
                  }}
                >
                  {u.file_name}{" "}
                  <span style={{ color: "var(--color-text-muted)" }}>
                    ({(u.file_size / 1024).toFixed(1)} KB)
                  </span>
                </div>
              ))}
            </div>
          )}

          {/* Timestamps */}
          <div style={{ fontSize: "0.75rem", color: "var(--color-text-muted)", marginBottom: "1rem" }}>
            Created {formatDate(report.created_at)}
            {report.submitted_at && ` | Submitted ${formatDate(report.submitted_at)}`}
            {report.reviewed_at && ` | Reviewed ${formatDate(report.reviewed_at)}`}
          </div>

          {/* Admin actions */}
          {canReview && (
            <div style={{ display: "flex", gap: "0.5rem" }}>
              <Button
                variant="primary"
                onClick={handleApprove}
                loading={actionLoading}
                loadingText="Approving..."
                style={{ width: "auto" }}
              >
                Approve
              </Button>
              <Button
                variant="danger"
                onClick={handleReject}
                loading={actionLoading}
                loadingText="Rejecting..."
              >
                Reject
              </Button>
            </div>
          )}
        </div>
      )}
    </Modal>
  );
}
