import { useEffect, useState } from "react";
import { Modal } from "../ui/Modal";
import { Spinner } from "../ui/Spinner";
import { Button } from "../ui/Button";
import { ClearanceBadge } from "../ui/ClearanceBadge";
import { InvestigationDueCell } from "../ui/InvestigationDueCell";
import { PeriodProgressBar } from "../ui/PeriodProgressBar";
import { ClearanceEditForm } from "./ClearanceEditForm";
import { getMember, clearanceLabels, type MemberDetail } from "../../api/team";
import { useAuthStore } from "../../stores/authStore";

interface ViewMemberModalProps {
  open: boolean;
  userId: string | null;
  onClose: () => void;
}

export function ViewMemberModal({ open, userId, onClose }: ViewMemberModalProps) {
  const [detail, setDetail] = useState<MemberDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState(false);
  const role = useAuthStore((s) => s.user?.role);
  const canEdit = role === "administrator" || role === "fso";

  useEffect(() => {
    if (!open || !userId) {
      setDetail(null);
      setEditing(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    getMember(userId)
      .then((d) => { if (!cancelled) setDetail(d); })
      .catch(() => { /* surface via empty state */ })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [open, userId]);

  async function refresh() {
    if (!userId) return;
    setLoading(true);
    try {
      const d = await getMember(userId);
      setDetail(d);
    } finally {
      setLoading(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={detail?.name ?? "Team member"}>
      {loading && <Spinner />}
      {!loading && detail && (
        <div>
          <div style={{ color: "var(--color-text-secondary)", marginBottom: 16 }}>
            {detail.email} · {detail.role.replace(/_/g, " ")}
            {detail.suspended && <span style={{ color: "var(--color-error)" }}> · suspended</span>}
          </div>

          <section className="team-section">
            <div className="team-section-header">
              <h4>Clearance</h4>
              {canEdit && !editing && (
                <Button size="sm" variant="secondary" onClick={() => setEditing(true)}>Edit</Button>
              )}
            </div>
            {editing ? (
              <ClearanceEditForm
                member={detail}
                onSaved={() => { setEditing(false); void refresh(); }}
                onCancel={() => setEditing(false)}
              />
            ) : (
              <div className="team-card">
                <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                  <ClearanceBadge level={detail.clearance} />
                  {detail.investigation_type && (
                    <span style={{ color: "var(--color-text-muted)" }}>
                      {detail.investigation_type}
                      {detail.last_investigation_date && ` (${detail.last_investigation_date})`}
                    </span>
                  )}
                </div>
                <div style={{ marginTop: 8 }}>
                  {detail.eligibility_date && (
                    <div style={{ fontSize: 13, color: "var(--color-text-secondary)" }}>
                      Eligibility: {detail.eligibility_date}
                    </div>
                  )}
                  <div style={{ fontSize: 13, color: "var(--color-text-secondary)" }}>
                    Next investigation: <InvestigationDueCell date={detail.next_investigation_date} />
                  </div>
                  {detail.last_investigation_date && detail.next_investigation_date && (
                    <PeriodProgressBar
                      start={detail.last_investigation_date}
                      end={detail.next_investigation_date}
                    />
                  )}
                </div>
              </div>
            )}
          </section>

          <section className="team-section">
            <div className="team-section-header">
              <h4>Sub-organizations</h4>
            </div>
            <div className="team-card">
              {(detail.sub_orgs ?? []).length === 0 && <span style={{ color: "var(--color-text-muted)" }}>None</span>}
              {(detail.sub_orgs ?? []).map((s) => (
                <span key={s.id} className="chip">{s.name}</span>
              ))}
            </div>
          </section>

          {(detail.history ?? []).length > 0 && (
            <section className="team-section">
              <div className="team-section-header">
                <h4>Clearance history ({detail.history.length})</h4>
              </div>
              <div className="team-card">
                {detail.history.map((rec) => (
                  <div key={rec.id} className="history-row">
                    <div style={{ display: "flex", justifyContent: "space-between" }}>
                      <span>
                        <ClearanceBadge level={rec.clearance} />
                        {rec.investigation_type && (
                          <span style={{ marginLeft: 8, color: "var(--color-text-muted)" }}>
                            {rec.investigation_type}
                          </span>
                        )}
                      </span>
                      <span style={{ color: "var(--color-text-muted)", fontSize: 12 }}>
                        {new Date(rec.recorded_at).toLocaleDateString()} · {rec.recorded_by_name}
                        {rec.superseded_at && " · superseded"}
                      </span>
                    </div>
                    {rec.notes && (
                      <div style={{ fontSize: 12, color: "var(--color-text-secondary)", marginTop: 4 }}>
                        {rec.notes}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </section>
          )}

          {!detail.clearance && (
            <p style={{ color: "var(--color-text-muted)", fontSize: 13 }}>
              No clearance recorded. {canEdit ? `Use "Edit" above to record an initial clearance (${clearanceLabels.secret}, ${clearanceLabels.top_secret}, …).` : ""}
            </p>
          )}
        </div>
      )}
      {!loading && !detail && <p style={{ color: "var(--color-text-muted)" }}>Member not found.</p>}
    </Modal>
  );
}
