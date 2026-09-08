import { clearanceLabels, type ClearanceLevel } from "../../api/team";

interface ClearanceBadgeProps {
  level: ClearanceLevel | "";
}

export function ClearanceBadge({ level }: ClearanceBadgeProps) {
  const value = level || "missing";
  return (
    <span className={`clearance-badge clearance-badge-${value}`}>
      {level ? clearanceLabels[level] : "—"}
    </span>
  );
}
