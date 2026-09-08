import { useEffect, useState } from "react";
import { CheckCircle, AlertTriangle } from "lucide-react";
import { useNavigate } from "react-router";
import { Button } from "../ui/Button";
import { Alert } from "../ui/Alert";
import { ClearanceBadge } from "../ui/ClearanceBadge";
import {
  getDd254Suggestions,
  linkDd254ToVisit,
  dd254FileUrl,
  type Dd254Suggestion,
} from "../../api/dd254";

interface DD254AuthorizationPanelProps {
  visitRequestId: string;
  // The DD254 currently linked to the visit — either the IC's declaration
  // at submit, or an FSO override at review. Empty until someone picks one.
  currentDD254Id?: string | null;
  visitAccessLevel: string;
  onChange?: () => void;
  // Drives the contextual "Approve" vs "Approve anyway" copy on the parent's
  // review buttons. "matched" only when the linked DD254 actually authorizes
  // this visit per all gates; everything else is "no_match".
  onAuthStateChange?: (state: AuthState) => void;
}

export type AuthState = "matched" | "no_match" | "loading";

// Panel meaning:
//   - If a DD254 is linked AND it passes all server-side gates (read-on,
//     dates, classification) it shows green "Authorization confirmed."
//   - If a DD254 is linked but is NOT among the candidates that pass the
//     gates, it shows a red warning — the IC declared a contract that
//     doesn't actually authorize this visit. FSO is expected to investigate.
//   - If nothing is linked, the panel lists the candidates (DD254s the
//     visitor is read-on to that pass dates+classification) and asks the
//     FSO to confirm which contract this visit is under. No auto-pick.
//   - Zero candidates + no link → amber warning, explicit override required.
//
// Auto-suggest was removed: the system used to claim a "match" whenever the
// visitor was read-on to *any* DD254 at the right classification for the
// right dates, even if the visit was actually being made under a different
// contract. That was a false positive. Now the visit carries the IC's
// declared contract, and the FSO confirms or overrides — no claim without
// an explicit pick.
export function DD254AuthorizationPanel({
  visitRequestId,
  currentDD254Id,
  onChange,
  onAuthStateChange,
}: DD254AuthorizationPanelProps) {
  const navigate = useNavigate();
  const [candidates, setCandidates] = useState<Dd254Suggestion[]>([]);
  const [loading, setLoading] = useState(true);
  const [showFull, setShowFull] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    onAuthStateChange?.("loading");
    getDd254Suggestions(visitRequestId)
      .then((res) => {
        if (cancelled) return;
        setCandidates(res.suggestions);
        // "matched" only when the linked DD254 is among the candidates.
        const linkedIsValid = !!currentDD254Id && res.suggestions.some((s) => s.id === currentDD254Id);
        onAuthStateChange?.(linkedIsValid ? "matched" : "no_match");
      })
      .catch(() => {
        if (!cancelled) {
          setCandidates([]);
          onAuthStateChange?.("no_match");
        }
      })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [visitRequestId, currentDD254Id, onAuthStateChange]);

  async function handleLink(id: string | null) {
    await linkDd254ToVisit(visitRequestId, id);
    onChange?.();
  }

  if (loading) {
    return (
      <div className="auth-panel">
        <p style={{ color: "var(--color-text-muted)" }}>Checking authorization…</p>
      </div>
    );
  }

  const linked = currentDD254Id ? candidates.find((s) => s.id === currentDD254Id) : null;
  const linkedButInvalid = !!currentDD254Id && !linked;

  return (
    <div className="auth-panel">
      <div className="team-section-header">
        <h4>Authorization</h4>
      </div>

      {/* Case A: linked + valid → green confirmation */}
      {linked && (
        <div className="team-card">
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <CheckCircle size={18} color="var(--color-success)" />
            <strong>{linked.contract_number}</strong>
            {linked.prime_contractor && <span>· {linked.prime_contractor}</span>}
            <ClearanceBadge level={linked.classification_max} />
          </div>
          <div style={{ fontSize: 13, color: "var(--color-text-secondary)", marginTop: 4 }}>
            Valid {formatDate(linked.period_start)} → {formatDate(linked.period_end)} ·
            authorization confirmed
          </div>
          <div style={{ marginTop: 8, display: "flex", gap: 8 }}>
            <Button size="sm" variant="secondary" onClick={() => setShowFull((v) => !v)}>
              {showFull ? "Hide preview" : "Preview DD254"}
            </Button>
            <Button size="sm" variant="secondary" onClick={() => navigate(`/app/dd254/${linked.id}`)}>
              Open full DD254
            </Button>
            <Button size="sm" variant="secondary" onClick={() => handleLink(null)}>
              Unlink
            </Button>
          </div>
          {showFull && (
            <iframe
              src={dd254FileUrl(linked.id)}
              title={`DD254 ${linked.contract_number}`}
              style={{ width: "100%", height: 400, border: "1px solid var(--color-border)", marginTop: 8 }}
            />
          )}
        </div>
      )}

      {/* Case B: linked but invalid — IC declared a contract that doesn't authorize this visit */}
      {linkedButInvalid && (
        <Alert variant="error">
          <AlertTriangle size={14} style={{ marginRight: 6, verticalAlign: "-2px" }} />
          The visitor declared a contract that does not authorize this visit (visitor is not
          read-on, or the period/classification does not cover the visit). Investigate before
          approving.
          <div style={{ marginTop: 8 }}>
            <Button size="sm" variant="secondary" onClick={() => handleLink(null)}>
              Clear declaration
            </Button>
          </div>
        </Alert>
      )}

      {/* Case C: no link — show candidates the FSO can confirm */}
      {!currentDD254Id && candidates.length > 0 && (
        <div className="team-card">
          <p style={{ margin: "0 0 8px 0", fontSize: 13, color: "var(--color-text-secondary)" }}>
            Visitor did not declare a contract at submission. {candidates.length} DD254
            {candidates.length === 1 ? " is" : "s are"} on file that could authorize this visit.
            Confirm which contract this visit is being made under:
          </p>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            {candidates.map((c) => (
              <div
                key={c.id}
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  padding: 8,
                  border: "1px solid var(--color-border)",
                  borderRadius: 6,
                }}
              >
                <div>
                  <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <strong>{c.contract_number}</strong>
                    {c.prime_contractor && <span>· {c.prime_contractor}</span>}
                    <ClearanceBadge level={c.classification_max} />
                  </div>
                  <div style={{ fontSize: 12, color: "var(--color-text-muted)", marginTop: 2 }}>
                    Valid {formatDate(c.period_start)} → {formatDate(c.period_end)}
                  </div>
                </div>
                <Button size="sm" variant="secondary" onClick={() => handleLink(c.id)}>
                  Link this contract
                </Button>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Case D: no link, no candidates — visitor has no authorization on file at all */}
      {!currentDD254Id && candidates.length === 0 && (
        <Alert variant="warning">
          <AlertTriangle size={14} style={{ marginRight: 6, verticalAlign: "-2px" }} />
          No DD254 on file that could authorize this visit. The visitor is not read-on to any
          active contract that covers this access level and date range. Approving will require
          explicit override.
        </Alert>
      )}
    </div>
  );
}

// Period start/end come back as ISO date strings (date-only on the DB, but
// pgx/JSON renders them as "2026-10-15T00:00:00Z"). Slice off the time portion
// for display — the FSO only cares about the day.
function formatDate(iso?: string | null): string {
  if (!iso) return "—";
  return iso.slice(0, 10);
}
