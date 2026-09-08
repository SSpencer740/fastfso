import { useState } from "react";
import { Clipboard, Check } from "lucide-react";
import { StatusBadge } from "../ui/StatusBadge";
import { formatAccessLevel } from "../../api/visits";
import type { VisitRequest } from "../../api/visits";

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

export function VisitRequestDetails({ vr }: { vr: VisitRequest }) {
  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", marginBottom: "1.25rem" }}>
        <h4 style={{ margin: 0 }}>{vr.destination_name}</h4>
        <StatusBadge status={vr.status} />
      </div>

      <SectionHeading>Destination</SectionHeading>
      <DetailRow label="DISS SMO Code" value={vr.diss_smo_code} />
      <DetailRow label="Address" value={vr.visit_address} />
      <DetailRow
        label="Dates"
        value={`${new Date(vr.visit_start_date).toLocaleDateString()} – ${new Date(vr.visit_end_date).toLocaleDateString()}`}
      />

      <SectionHeading>Access & Purpose</SectionHeading>
      <DetailRow label="Access Level" value={formatAccessLevel(vr.access_level)} />
      <DetailRow label="Description" value={vr.visit_description} />

      <SectionHeading>Point of Contact</SectionHeading>
      <DetailRow label="Name" value={vr.poc_name} />
      <DetailRow label="Email" value={vr.poc_email} />
      <DetailRow label="Phone" value={vr.poc_phone} />

      <SectionHeading>Security Point of Contact</SectionHeading>
      <DetailRow label="Name" value={vr.security_poc_name} />
      <DetailRow label="Email" value={vr.security_poc_email} />
      <DetailRow label="Phone" value={vr.security_poc_phone} />

      {vr.reviewed_at && (
        <>
          <SectionHeading>Review</SectionHeading>
          <DetailRow label="Reviewed" value={new Date(vr.reviewed_at).toLocaleDateString()} />
          {vr.reviewer_notes && <DetailRow label="Notes" value={vr.reviewer_notes} />}
        </>
      )}
    </div>
  );
}
