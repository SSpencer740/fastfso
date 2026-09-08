import { useState, useEffect } from "react";
import { Clipboard, Check } from "lucide-react";
import { Modal } from "../ui/Modal";
import { StatusBadge } from "../ui/StatusBadge";
import { useAuthStore } from "../../stores/authStore";
import { isReadOnlyFSO } from "../../utils/roles";
import { formatDate } from "../../utils/dateFormat";
import type { ActionItem, FSOUser } from "../../api/actionitems";
import { getActionItem, updateActionItemStatus, updateActionItemNotes, listFSOUsers, reassignActionItem } from "../../api/actionitems";
import { getAdminVisit } from "../../api/visits";
import type { VisitRequest } from "../../api/visits";
import { getTravelReport, getTravelUploadDownloadUrl } from "../../api/travel";
import type { TravelReportDetail } from "../../api/travel";
import { VisitRequestDetails } from "../visits/VisitRequestDetails";
import { getAdminReport, formatReportType } from "../../api/reports";
import type { Report } from "../../api/reports";
import { getAdminSubmission } from "../../api/tasks";
import type { AdminSubmission } from "../../api/tasks";
import { AIVerdictPanel } from "./AIVerdictPanel";
import { UploadPreview } from "./UploadPreview";
import { getAdminDebrief } from "../../api/travel";
import type { TravelDebrief } from "../../api/travel";

interface ViewActionItemModalProps {
  open: boolean;
  onClose: () => void;
  itemId: string | null;
  onUpdated: () => void;
}

function DetailRow({ label, value }: { label: string; value?: string | null }) {
  const [copied, setCopied] = useState(false);
  const [hovered, setHovered] = useState(false);
  if (!value) return null;

  function handleCopy() {
    void navigator.clipboard.writeText(value!).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  }

  return (
    <div style={{ display: "flex", gap: 8, marginBottom: 6, fontSize: 13 }}>
      <span style={{ color: "var(--color-text-muted)", minWidth: 140, flexShrink: 0 }}>{label}</span>
      <span
        onClick={handleCopy}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
        title="Click to copy"
        style={{ display: "inline-flex", alignItems: "center", gap: 4, color: "var(--color-text)", cursor: "pointer" }}
      >
        {copied
          ? <><Check size={12} style={{ color: "var(--color-success, #22c55e)" }} /><span style={{ color: "var(--color-success, #22c55e)", fontStyle: "italic" }}>Copied!</span></>
          : <>{value}{hovered && <Clipboard size={12} style={{ color: "var(--color-text-muted)", flexShrink: 0 }} />}</>
        }
      </span>
    </div>
  );
}

function SectionHeading({ children }: { children: React.ReactNode }) {
  return (
    <h5 style={{ margin: "16px 0 8px", fontSize: 12, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--color-text-muted)" }}>
      {children}
    </h5>
  );
}


function TravelReportDetails({ detail }: { detail: TravelReportDetail }) {
  const { report, countries, uploads, creator_name } = detail;
  return (
    <div style={{ borderTop: "1px solid var(--color-border)", paddingTop: 12, marginTop: 4 }}>
      <SectionHeading>Travel Report Details</SectionHeading>
      <DetailRow label="Trip Name" value={report.trip_name} />
      <DetailRow label="Submitted By" value={creator_name} />
      <DetailRow label="Passport" value={report.passport_number} />
      {report.additional_comments && (
        <DetailRow label="Comments" value={report.additional_comments} />
      )}

      {countries.length > 0 && (
        <>
          <SectionHeading>Countries / Destinations</SectionHeading>
          {countries.map((c, i) => (
            <div key={c.id} style={{ marginBottom: 10, paddingLeft: 8, borderLeft: "2px solid var(--color-border)" }}>
              <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 4 }}>
                {i + 1}. {c.country_name}
              </div>
              {(c.start_date || c.end_date) && (
                <DetailRow
                  label="Dates"
                  value={[c.start_date && new Date(c.start_date).toLocaleDateString(), c.end_date && new Date(c.end_date).toLocaleDateString()].filter(Boolean).join(" – ")}
                />
              )}
              <DetailRow label="Reason" value={c.reason} />
              {c.transportation.length > 0 && (
                <DetailRow label="Transportation" value={c.transportation.join(", ")} />
              )}
              {c.has_companions && <DetailRow label="Companions" value={c.companions_detail} />}
              {c.has_foreign_contacts && <DetailRow label="Foreign Contacts" value={c.contacts_detail} />}
            </div>
          ))}
        </>
      )}

      <SectionHeading>Emergency Contact</SectionHeading>
      <DetailRow label="Name" value={`${report.emergency_first_name} ${report.emergency_last_name}`} />
      <DetailRow label="Phone" value={report.emergency_phone} />

      {uploads.length > 0 && (
        <>
          <SectionHeading>Attachments</SectionHeading>
          {uploads.map((u) => (
            <a
              key={u.id}
              href={getTravelUploadDownloadUrl(u.id)}
              target="_blank"
              rel="noreferrer"
              style={{ display: "block", fontSize: 13, color: "var(--color-primary)", textDecoration: "underline", marginBottom: 4 }}
            >
              {u.file_name}
            </a>
          ))}
        </>
      )}
    </div>
  );
}

const reportingForLabels: Record<string, string> = {
  self: "Self",
  other: "On behalf of another",
  fcl: "FCL Reporting",
};

function ReportDetails({ report }: { report: Report }) {
  return (
    <div style={{ borderTop: "1px solid var(--color-border)", paddingTop: 12, marginTop: 4 }}>
      <SectionHeading>Report Details</SectionHeading>
      <DetailRow label="Submitted By" value={report.creator_name} />
      <DetailRow label="Email" value={report.creator_email} />
      <DetailRow label="Report Type" value={formatReportType(report.report_type)} />
      <DetailRow label="Reporting For" value={reportingForLabels[report.reporting_for] ?? report.reporting_for} />
      {report.subject_name && <DetailRow label="Subject" value={report.subject_name} />}
      <SectionHeading>Details</SectionHeading>
      <p style={{ fontSize: 13, whiteSpace: "pre-wrap", background: "var(--color-bg-elevated)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", padding: "0.6rem 0.75rem", lineHeight: 1.6, margin: 0 }}>
        {report.details}
      </p>
    </div>
  );
}

function TaskSubmissionDetails({ sub }: { sub: AdminSubmission }) {
  const responseMap = Object.fromEntries((sub.responses ?? []).map(r => [r.requirement_id, r]));
  const uploadMap: Record<string, typeof sub.uploads> = {};
  for (const u of (sub.uploads ?? [])) {
    uploadMap[u.requirement_id] = [...(uploadMap[u.requirement_id] ?? []), u];
  }

  return (
    <div style={{ borderTop: "1px solid var(--color-border)", paddingTop: 12, marginTop: 4 }}>
      <SectionHeading>Submission Details</SectionHeading>
      <DetailRow label="Submitted By" value={sub.user_name} />
      <DetailRow label="Email" value={sub.user_email} />
      <DetailRow label="Task" value={sub.task_title} />
      {sub.submitted_at && <DetailRow label="Submitted" value={formatDate(sub.submitted_at)} />}
      {(sub.requirements ?? []).length > 0 && (
        <>
          <SectionHeading>Responses</SectionHeading>
          {(sub.requirements ?? []).map(req => {
            const response = responseMap[req.id];
            const uploads = uploadMap[req.id] ?? [];
            return (
              <div key={req.id} style={{ marginBottom: 10 }}>
                <div style={{ fontSize: 12, fontWeight: 600, color: "var(--color-text-muted)", marginBottom: 4 }}>{req.label}</div>
                {req.kind === "checkbox" && (
                  <span style={{ fontSize: 13 }}>{response?.bool_value ? "✓ Checked" : "Not checked"}</span>
                )}
                {(req.kind === "text" || req.kind === "textarea") && (
                  <p style={{ margin: 0, fontSize: 13, whiteSpace: "pre-wrap", color: response?.text_value ? "var(--color-text)" : "var(--color-text-muted)" }}>
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
                    : <span style={{ fontSize: 13, color: "var(--color-text-muted)" }}>No file uploaded</span>
                )}
              </div>
            );
          })}
        </>
      )}
    </div>
  );
}

const DEBRIEF_QUESTIONS = [
  { answerKey: "q1_foreign_contact" as const, detailKey: "q1_details" as const, text: "Approached or contacted by a foreign national seeking sensitive information?" },
  { answerKey: "q2_surveillance" as const, detailKey: "q2_details" as const, text: "Observed any suspicious surveillance or monitoring?" },
  { answerKey: "q3_equipment_loss" as const, detailKey: "q3_details" as const, text: "Equipment, device, or sensitive material lost, stolen, or compromised?" },
  { answerKey: "q4_unusual_requests" as const, detailKey: "q4_details" as const, text: "Received unusual or unexpected requests for information or access?" },
];

function TravelDebriefDetails({ debrief }: { debrief: TravelDebrief }) {
  return (
    <div style={{ borderTop: "1px solid var(--color-border)", paddingTop: 12, marginTop: 4 }}>
      <SectionHeading>Debrief Details</SectionHeading>
      <DetailRow label="Trip" value={debrief.trip_name} />
      {debrief.submitted_at && (
        <DetailRow label="Submitted" value={formatDate(debrief.submitted_at)} />
      )}
      <SectionHeading>Responses</SectionHeading>
      {DEBRIEF_QUESTIONS.map((q, i) => {
        const answer = debrief[q.answerKey];
        const detail = debrief[q.detailKey];
        return (
          <div key={q.answerKey} style={{ marginBottom: 12 }}>
            <div style={{ fontSize: 12, fontWeight: 600, color: "var(--color-text-muted)", marginBottom: 4 }}>
              {i + 1}. {q.text}
            </div>
            <div style={{ display: "flex", alignItems: "flex-start", gap: 8, fontSize: 13 }}>
              <span style={{
                fontWeight: 600,
                color: answer === true ? "var(--color-danger)" : answer === false ? "var(--color-success)" : "var(--color-text-muted)",
              }}>
                {answer === null || answer === undefined ? "—" : answer ? "Yes" : "No"}
              </span>
              {answer === true && detail && (
                <span style={{ color: "var(--color-text)" }}>{detail}</span>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

export function ViewActionItemModal({ open, onClose, itemId, onUpdated }: ViewActionItemModalProps) {
  const role = useAuthStore((s) => s.user?.role);
  const readOnly = isReadOnlyFSO(role);
  const [item, setItem] = useState<ActionItem | null>(null);
  const [visitRequest, setVisitRequest] = useState<VisitRequest | null>(null);
  const [travelReport, setTravelReport] = useState<TravelReportDetail | null>(null);
  const [travelDebrief, setTravelDebrief] = useState<TravelDebrief | null>(null);
  const [report, setReport] = useState<Report | null>(null);
  const [taskSubmission, setTaskSubmission] = useState<AdminSubmission | null>(null);
  const [notes, setNotes] = useState("");
  const [savingNotes, setSavingNotes] = useState(false);
  const [notesSaved, setNotesSaved] = useState(false);
  const [error, setError] = useState("");
  const [updating, setUpdating] = useState(false);
  const [fsoUsers, setFSOUsers] = useState<FSOUser[]>([]);
  const [reassigning, setReassigning] = useState(false);

  useEffect(() => {
    if (!open || !itemId) return;
    setError("");
    setVisitRequest(null);
    setTravelReport(null);
    setTravelDebrief(null);
    setReport(null);
    setTaskSubmission(null);
    if (!readOnly) {
      listFSOUsers().then(r => setFSOUsers(r.users)).catch(() => {/* best effort */});
    }

    getActionItem(itemId).then(ai => {
      setItem(ai);
      setNotes(ai.notes || "");

      if (ai.source_id) {
        if (ai.source_type === "visit_request") {
          getAdminVisit(ai.source_id).then(setVisitRequest).catch(() => {/* best effort */});
        } else if (ai.source_type === "travel_report") {
          getTravelReport(ai.source_id).then(setTravelReport).catch(() => {/* best effort */});
        } else if (ai.source_type === "report") {
          getAdminReport(ai.source_id).then(setReport).catch(() => {/* best effort */});
        } else if (ai.source_type === "task_submission") {
          getAdminSubmission(ai.source_id).then(setTaskSubmission).catch(() => {/* best effort */});
        } else if (ai.source_type === "travel_debrief") {
          getAdminDebrief(ai.source_id).then(setTravelDebrief).catch(() => {/* best effort */});
        }
      }
    }).catch(() => setError("Failed to load action item"));
  }, [open, itemId]);

  async function handleSaveNotes() {
    if (!item) return;
    setSavingNotes(true);
    try {
      await updateActionItemNotes(item.id, notes);
      setNotesSaved(true);
      setTimeout(() => setNotesSaved(false), 2000);
    } catch {
      setError("Failed to save notes");
    } finally {
      setSavingNotes(false);
    }
  }

  async function handleReassign(userId: string) {
    if (!item) return;
    setReassigning(true);
    try {
      const updated = await reassignActionItem(item.id, userId || null);
      setItem(updated);
      onUpdated();
    } catch {
      setError("Failed to reassign action item");
    } finally {
      setReassigning(false);
    }
  }

  async function handleStatusUpdate(status: string) {
    if (!item) return;
    setUpdating(true);
    try {
      await updateActionItemStatus(item.id, status);
      const updated = await getActionItem(item.id);
      setItem(updated);
      onUpdated();
    } catch {
      setError("Failed to update status");
    } finally {
      setUpdating(false);
    }
  }

  if (!item) return null;

  const sourceTypeLabels: Record<string, string> = {
    task_submission: "Task Submission",
    visit_request: "Visit Request",
    travel_report: "Travel Report",
    travel_debrief: "Travel Debrief",
    incident_report: "Incident Report",
    clearance_renewal: "Clearance Renewal",
    sf86_submission: "SF-86 Submission",
    report: "Life Event Report",
  };

  return (
    <Modal open={open} onClose={onClose} title={item.title} size="wide">
      {error && <div className="alert alert-error">{error}</div>}

      <div style={{ display: "flex", gap: 12, marginBottom: 16, fontSize: 13 }}>
        <StatusBadge status={item.status} />
        <StatusBadge status={item.priority} />
        <span style={{ color: "var(--color-text-muted)" }}>{sourceTypeLabels[item.source_type] || item.source_type}</span>
      </div>

      {item.description && (
        <p style={{ fontSize: 14, color: "var(--color-text-muted)", marginBottom: 16 }}>{item.description}</p>
      )}

      <div style={{ marginBottom: 16 }}>
        <label style={{ display: "block", fontSize: 12, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--color-text-muted)", marginBottom: 6 }}>Assigned FSO</label>
        {readOnly ? (
          <span style={{ fontSize: 13 }}>{item.assignee_name ?? "Unassigned"}</span>
        ) : (
          <select
            value={item.assigned_to ?? ""}
            onChange={e => handleReassign(e.target.value)}
            disabled={reassigning}
            style={{ fontSize: 13, padding: "4px 8px", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", background: "var(--color-surface)", color: "var(--color-text)", minWidth: 200 }}
          >
            <option value="">Unassigned</option>
            {fsoUsers.map(u => (
              <option key={u.user_id} value={u.user_id}>{u.name}</option>
            ))}
          </select>
        )}
      </div>

      {visitRequest && <VisitRequestDetails vr={visitRequest} />}
      {travelReport && <TravelReportDetails detail={travelReport} />}
      {travelDebrief && <TravelDebriefDetails debrief={travelDebrief} />}
      {report && <ReportDetails report={report} />}
      {taskSubmission && <TaskSubmissionDetails sub={taskSubmission} />}

      <div style={{ marginTop: 16 }}>
        <label style={{ display: "block", fontSize: 12, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--color-text-muted)", marginBottom: 6 }}>Notes</label>
        <textarea
          style={{ width: "100%", minHeight: 80, padding: "0.6rem 0.75rem", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", fontFamily: "inherit", fontSize: "0.9rem", resize: "vertical", background: "var(--color-surface)", color: "var(--color-text)", boxSizing: "border-box" }}
          value={notes}
          onChange={e => { setNotes(e.target.value); setNotesSaved(false); }}
          placeholder={readOnly ? "No notes." : "Add notes..."}
          readOnly={readOnly}
        />
        {!readOnly && (
          <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 6 }}>
            <button className="btn btn-sm" onClick={handleSaveNotes} disabled={savingNotes}>
              {notesSaved ? "Saved!" : savingNotes ? "Saving..." : "Save Note"}
            </button>
          </div>
        )}
      </div>

      {!readOnly && (
        <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, paddingTop: 16, borderTop: "1px solid var(--color-border)" }}>
          {item.status === "pending" && (
            <>
              <button className="btn btn-sm btn-danger" onClick={() => handleStatusUpdate("rejected")} disabled={updating}>Reject</button>
              <button className="btn btn-sm" style={{ color: "#3A7CA5", borderColor: "#3A7CA5" }} onClick={() => handleStatusUpdate("under_review")} disabled={updating}>Under Review</button>
              <button className="btn btn-primary" style={{ width: "auto" }} onClick={() => handleStatusUpdate("processed")} disabled={updating}>Approve</button>
            </>
          )}
          {item.status === "under_review" && (
            <>
              <button className="btn btn-sm btn-danger" onClick={() => handleStatusUpdate("rejected")} disabled={updating}>Reject</button>
              <button className="btn btn-primary" style={{ width: "auto" }} onClick={() => handleStatusUpdate("processed")} disabled={updating}>Approve</button>
            </>
          )}
        </div>
      )}
    </Modal>
  );
}
