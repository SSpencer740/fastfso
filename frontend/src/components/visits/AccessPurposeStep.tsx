import { FormField } from "../ui/FormField";
import type { MyDd254Authorization } from "../../api/visits";

interface AccessPurposeStepProps {
  accessLevel: string;
  onAccessLevelChange: (v: string) => void;
  visitDescription: string;
  onVisitDescriptionChange: (v: string) => void;
  // Contract selection. Available list is the IC's read-on DD254s; we mark
  // entries that don't actually cover the chosen access level + visit dates
  // as disabled rather than hiding them — easier for the IC to see why an
  // option is unselectable.
  visitStartDate: string;
  visitEndDate: string;
  authorizations: MyDd254Authorization[];
  dd254Id: string;
  onDd254IdChange: (v: string) => void;
}

const accessLevels = [
  { value: "confidential", label: "Confidential" },
  { value: "secret", label: "Secret" },
  { value: "top_secret", label: "Top Secret" },
  { value: "top_secret_sci", label: "Top Secret/SCI" },
];

const classOrder: Record<string, number> = {
  none: 0,
  confidential: 1,
  secret: 2,
  top_secret: 3,
  ts_sci: 4,
};

function isHighLevel(level: string): boolean {
  return level === "top_secret" || level === "top_secret_sci";
}

// matchesVisit returns whether an authorization's classification + period
// could authorize a visit at the given access level and dates. Mirrors the
// server-side gates in dd254.SuggestForVisit so the IC sees the same
// candidate set the FSO will see at review time.
function matchesVisit(
  a: MyDd254Authorization,
  accessLevel: string,
  startDate: string,
  endDate: string,
): boolean {
  const required = classOrder[accessLevel === "top_secret_sci" ? "ts_sci" : accessLevel] ?? 0;
  const available = classOrder[a.classification_max] ?? 0;
  if (available < required) return false;
  if (a.period_start && startDate && a.period_start.slice(0, 10) > startDate) return false;
  if (a.period_end && endDate && a.period_end.slice(0, 10) < endDate) return false;
  return true;
}

export function AccessPurposeStep({
  accessLevel,
  onAccessLevelChange,
  visitDescription,
  onVisitDescriptionChange,
  visitStartDate,
  visitEndDate,
  authorizations,
  dd254Id,
  onDd254IdChange,
}: AccessPurposeStepProps) {
  return (
    <div>
      <FormField label="Access Level">
        <div className="radio-group">
          {accessLevels.map((level) => (
            <label key={level.value} className="radio-option">
              <input
                type="radio"
                name="access-level"
                value={level.value}
                checked={accessLevel === level.value}
                onChange={(e) => onAccessLevelChange(e.target.value)}
              />
              <span>{level.label}</span>
            </label>
          ))}
        </div>
      </FormField>

      {isHighLevel(accessLevel) && (
        <div className="cui-warning">
          This visit requires Top Secret or higher access. Ensure all CUI handling
          procedures are followed and that the destination facility has the appropriate
          clearance level.
        </div>
      )}

      <FormField label="Visiting under contract" htmlFor="dd254-select">
        <select
          id="dd254-select"
          value={dd254Id}
          onChange={(e) => onDd254IdChange(e.target.value)}
          style={{
            width: "100%",
            padding: "0.6rem 0.75rem",
            border: "1px solid var(--color-border-strong)",
            borderRadius: "var(--radius-md)",
            background: "var(--color-bg-elevated)",
            color: "inherit",
            fontSize: "1rem",
          }}
        >
          <option value="">— Not under a contract —</option>
          {authorizations.map((a) => {
            const ok = matchesVisit(a, accessLevel, visitStartDate, visitEndDate);
            return (
              <option key={a.id} value={a.id} disabled={!ok}>
                {a.contract_number}
                {a.prime_contractor ? ` · ${a.prime_contractor}` : ""}
                {" "}({a.classification_max})
                {!ok ? " — does not cover this visit" : ""}
              </option>
            );
          })}
        </select>
        <p style={{ marginTop: 6, fontSize: 12, color: "var(--color-text-muted)" }}>
          Pick the contract this visit is being made under, or select the first option for
          pre-contract visits. Your FSO uses this to confirm authorization.
        </p>
      </FormField>

      <FormField label="Visit Description / Purpose" htmlFor="visit-description">
        <textarea
          id="visit-description"
          rows={4}
          value={visitDescription}
          onChange={(e) => onVisitDescriptionChange(e.target.value)}
          required
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
    </div>
  );
}
