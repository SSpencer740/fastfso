import type { FileStatus } from "./ReportTravelWizard";

export function FileStatusBadge({ status }: { status: FileStatus }) {
  if (status.kind === "idle") return null;
  if (status.kind === "scanning") {
    return (
      <span style={{ fontSize: "0.75rem", fontStyle: "italic", color: "var(--color-text-muted)" }}>
        Scanning…
      </span>
    );
  }
  if (status.kind === "clean") {
    return (
      <span style={{ fontSize: "0.75rem", color: "#1F7A46" }}>✓ Scanned</span>
    );
  }
  return (
    <span
      title={status.error}
      style={{
        fontSize: "0.75rem",
        color: "#B01A1F",
        maxWidth: 260,
        overflow: "hidden",
        textOverflow: "ellipsis",
        whiteSpace: "nowrap",
      }}
    >
      ✗ {status.error}
    </span>
  );
}
