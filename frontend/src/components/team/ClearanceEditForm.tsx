import { useState } from "react";
import {
  clearanceLabels,
  setClearance,
  type ClearanceLevel,
  type InvestigationType,
  type MemberDetail,
} from "../../api/team";
import { FormField } from "../ui/FormField";
import { Button } from "../ui/Button";

interface ClearanceEditFormProps {
  member: MemberDetail;
  onSaved: () => void;
  onCancel: () => void;
}

const levels: ClearanceLevel[] = ["none", "confidential", "secret", "top_secret", "ts_sci"];
const types: InvestigationType[] = ["T3", "T3R", "T5", "T5R"];

export function ClearanceEditForm({ member, onSaved, onCancel }: ClearanceEditFormProps) {
  const [clearance, setClearanceLevel] = useState<ClearanceLevel>(
    (member.clearance || "none") as ClearanceLevel,
  );
  const [investigationType, setInvestigationType] = useState<InvestigationType | "">(
    member.investigation_type ?? "",
  );
  const [eligibility, setEligibility] = useState(member.eligibility_date ?? "");
  const [lastInvest, setLastInvest] = useState(member.last_investigation_date ?? "");
  const [nextInvest, setNextInvest] = useState(member.next_investigation_date ?? "");
  const [notes, setNotes] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await setClearance(member.user_id, {
        clearance,
        investigation_type: investigationType || null,
        eligibility_date: eligibility || null,
        last_investigation_date: lastInvest || null,
        next_investigation_date: nextInvest || null,
        notes,
      });
      onSaved();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to save";
      setError(message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <form onSubmit={handleSubmit}>
      <FormField label="Clearance level" htmlFor="cl-level">
        <select
          id="cl-level"
          className="filter-select"
          value={clearance}
          onChange={(e) => setClearanceLevel(e.target.value as ClearanceLevel)}
        >
          {levels.map((l) => (
            <option key={l} value={l}>{clearanceLabels[l]}</option>
          ))}
        </select>
      </FormField>

      <FormField label="Investigation type" htmlFor="cl-itype">
        <select
          id="cl-itype"
          className="filter-select"
          value={investigationType}
          onChange={(e) => setInvestigationType(e.target.value as InvestigationType | "")}
        >
          <option value="">—</option>
          {types.map((t) => (
            <option key={t} value={t}>{t}</option>
          ))}
        </select>
      </FormField>

      <FormField label="Eligibility date" htmlFor="cl-elig">
        <input
          id="cl-elig"
          type="date"
          className="filter-input"
          value={eligibility}
          onChange={(e) => setEligibility(e.target.value)}
        />
      </FormField>

      <FormField label="Last investigation date" htmlFor="cl-last">
        <input
          id="cl-last"
          type="date"
          className="filter-input"
          value={lastInvest}
          onChange={(e) => setLastInvest(e.target.value)}
        />
      </FormField>

      <FormField label="Next investigation date" htmlFor="cl-next">
        <input
          id="cl-next"
          type="date"
          className="filter-input"
          value={nextInvest}
          onChange={(e) => setNextInvest(e.target.value)}
        />
      </FormField>

      <FormField label="Notes (optional)" htmlFor="cl-notes">
        <textarea
          id="cl-notes"
          className="filter-input"
          rows={2}
          value={notes}
          onChange={(e) => setNotes(e.target.value)}
          placeholder="Context for this update (e.g. crossover from prior employer)"
        />
      </FormField>

      {error && <p style={{ color: "var(--color-error)" }}>{error}</p>}

      <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, marginTop: 16 }}>
        <Button variant="secondary" type="button" onClick={onCancel} disabled={saving}>Cancel</Button>
        <Button type="submit" disabled={saving}>{saving ? "Saving…" : "Save clearance"}</Button>
      </div>
    </form>
  );
}
