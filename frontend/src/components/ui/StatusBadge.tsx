const statusLabels: Record<string, string> = {
  to_do: "To Do",
  in_progress: "In Progress",
  submitted: "Submitted",
  approved: "Approved",
  rejected: "Rejected",
  pending: "Pending",
  under_review: "Under Review",
  processed: "Processed",
  draft: "Draft",
  active: "Active",
  archived: "Archived",
};

interface StatusBadgeProps {
  status: string;
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const label = statusLabels[status] ?? status.replace(/_/g, " ");
  return (
    <span className={`status-badge status-badge-${status}`}>
      {label}
    </span>
  );
}
