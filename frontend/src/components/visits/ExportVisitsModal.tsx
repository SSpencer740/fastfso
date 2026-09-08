import { useState } from "react";
import { Modal } from "../ui/Modal";
import { Button } from "../ui/Button";
import { Alert } from "../ui/Alert";
import { SubOrgFilter } from "../ui/SubOrgFilter";
import { FormField } from "../ui/FormField";
import { exportVisitsUrl } from "../../api/visits";

interface ExportVisitsModalProps {
  open: boolean;
  onClose: () => void;
}

const STATUS_OPTIONS = [
  { value: "submitted", label: "Submitted" },
  { value: "under_review", label: "Under Review" },
  { value: "approved", label: "Approved" },
  { value: "rejected", label: "Rejected" },
  { value: "cancelled", label: "Cancelled" },
];

// Default date range: last 90 days. Covers most quarterly audits without
// being too wide. FSO can adjust.
function defaultRange(): { from: string; to: string } {
  const today = new Date();
  const ninety = new Date();
  ninety.setDate(today.getDate() - 90);
  const fmt = (d: Date) => d.toISOString().slice(0, 10);
  return { from: fmt(ninety), to: fmt(today) };
}

export function ExportVisitsModal({ open, onClose }: ExportVisitsModalProps) {
  const [{ from, to }, setRange] = useState(defaultRange);
  const [statuses, setStatuses] = useState<string[]>([]);
  const [subOrgId, setSubOrgId] = useState("");
  const [detail, setDetail] = useState<"full" | "summary">("full");
  const [error, setError] = useState("");

  function toggleStatus(v: string) {
    setStatuses((cur) => (cur.includes(v) ? cur.filter((s) => s !== v) : [...cur, v]));
  }

  function handleDownload() {
    setError("");
    if (!from || !to) {
      setError("Please pick a date range.");
      return;
    }
    if (from > to) {
      setError("From date must be before To date.");
      return;
    }
    const url = exportVisitsUrl({
      from,
      to,
      status: statuses,
      sub_org_id: subOrgId || undefined,
      detail,
    });

    // Trigger the download via a transient anchor. window.location.href
    // would also work since Content-Disposition: attachment keeps the page,
    // but the anchor pattern is more reliable across browsers and doesn't
    // mess with browser history.
    const a = document.createElement("a");
    a.href = url;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);

    onClose();
  }

  return (
    <Modal open={open} onClose={onClose} title="Export Visit Requests">
      {error && <Alert variant="error" onDismiss={() => setError("")}>{error}</Alert>}

      <p style={{ marginTop: 0, fontSize: 13, color: "var(--color-text-secondary)" }}>
        Generate a CSV audit report. Visitor clearance and DD254 authorization are snapshotted
        as of each visit's review date — what authorized the decision, not what's true today.
      </p>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12, marginTop: 12 }}>
        <FormField label="From (review date)" htmlFor="export-from">
          <input
            id="export-from"
            type="date"
            value={from}
            onChange={(e) => setRange((r) => ({ ...r, from: e.target.value }))}
            style={inputStyle}
          />
        </FormField>
        <FormField label="To (review date)" htmlFor="export-to">
          <input
            id="export-to"
            type="date"
            value={to}
            onChange={(e) => setRange((r) => ({ ...r, to: e.target.value }))}
            style={inputStyle}
          />
        </FormField>
      </div>

      <FormField label="Statuses (leave empty for all)">
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          {STATUS_OPTIONS.map((s) => (
            <label
              key={s.value}
              style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13, whiteSpace: "nowrap" }}
            >
              <input
                type="checkbox"
                checked={statuses.includes(s.value)}
                onChange={() => toggleStatus(s.value)}
                style={checkInputStyle}
              />
              {s.label}
            </label>
          ))}
        </div>
      </FormField>

      <FormField label="Sub-organization">
        <SubOrgFilter value={subOrgId} onChange={setSubOrgId} />
      </FormField>

      <FormField label="Detail level">
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13, whiteSpace: "nowrap" }}>
            <input
              type="radio"
              name="detail"
              value="full"
              checked={detail === "full"}
              onChange={() => setDetail("full")}
              style={checkInputStyle}
            />
            Full (22 columns — full audit detail)
          </label>
          <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13, whiteSpace: "nowrap" }}>
            <input
              type="radio"
              name="detail"
              value="summary"
              checked={detail === "summary"}
              onChange={() => setDetail("summary")}
              style={checkInputStyle}
            />
            Summary (8 columns)
          </label>
        </div>
      </FormField>

      <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, marginTop: 16 }}>
        <Button variant="secondary" onClick={onClose}>Cancel</Button>
        <Button onClick={handleDownload}>Download CSV</Button>
      </div>
    </Modal>
  );
}

const inputStyle: React.CSSProperties = {
  width: "100%",
  padding: "0.6rem 0.75rem",
  border: "1px solid var(--color-border-strong)",
  borderRadius: "var(--radius-md)",
  background: "var(--color-bg-elevated)",
  color: "inherit",
  fontSize: "1rem",
};

// Counter the global `.form-field input { width: 100% }` rule so checkboxes
// and radio buttons render at their native size instead of stretching across
// the row and pushing labels off-screen.
const checkInputStyle: React.CSSProperties = {
  width: "auto",
  margin: 0,
  padding: 0,
};
