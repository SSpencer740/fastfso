import { AlertCircle, AlertTriangle } from "lucide-react";

interface InvestigationDueCellProps {
  date?: string | null;
}

// Returns a colored date with an inline countdown. The cell flips to:
//  - red    when overdue
//  - amber  when within 90 days
//  - normal otherwise
// "Not recorded" placeholder when no date is set.
export function InvestigationDueCell({ date }: InvestigationDueCellProps) {
  if (!date) {
    return <span className="invest-due invest-due-missing">not recorded</span>;
  }
  const due = new Date(date);
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const diffDays = Math.round((due.getTime() - today.getTime()) / (1000 * 60 * 60 * 24));

  let tone: "ok" | "warning" | "overdue" = "ok";
  if (diffDays < 0) tone = "overdue";
  else if (diffDays <= 90) tone = "warning";

  const formatted = due.toLocaleDateString();
  const suffix =
    tone === "overdue" ? `${Math.abs(diffDays)}d overdue` :
    tone === "warning" ? `${diffDays}d` : null;

  return (
    <span className={`invest-due invest-due-${tone}`}>
      {tone === "overdue" && <AlertCircle size={14} aria-hidden="true" style={{ marginRight: 4, verticalAlign: "-2px" }} />}
      {tone === "warning" && <AlertTriangle size={14} aria-hidden="true" style={{ marginRight: 4, verticalAlign: "-2px" }} />}
      {formatted}
      {suffix && <span className="invest-due-suffix"> · {suffix}</span>}
    </span>
  );
}
