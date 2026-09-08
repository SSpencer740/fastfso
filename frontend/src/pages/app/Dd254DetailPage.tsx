import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { ArrowLeft, Trash2, UserPlus } from "lucide-react";
import { PageHeader } from "../../components/ui/PageHeader";
import { Button } from "../../components/ui/Button";
import { Spinner } from "../../components/ui/Spinner";
import { ClearanceBadge } from "../../components/ui/ClearanceBadge";
import { PeriodProgressBar } from "../../components/ui/PeriodProgressBar";
import { FormField } from "../../components/ui/FormField";
import { Modal } from "../../components/ui/Modal";
import { useAuthStore } from "../../stores/authStore";
import {
  dd254FileUrl,
  deleteDd254,
  getDd254,
  grantDd254Access,
  revokeDd254Access,
  type Dd254Detail,
} from "../../api/dd254";
import { listTeam, type Member } from "../../api/team";

function fmtDate(s?: string | null): string {
  return s ? new Date(s).toLocaleDateString() : "—";
}

export function Dd254DetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const role = useAuthStore((s) => s.user?.role);
  const canEdit = role === "administrator" || role === "fso";

  const [detail, setDetail] = useState<Dd254Detail | null>(null);
  const [loading, setLoading] = useState(true);
  const [addAccessOpen, setAddAccessOpen] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const d = await getDd254(id);
      setDetail(d);
    } catch (err) {
      console.error("Failed to load DD254", err);
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { void load(); }, [load]);

  async function handleDelete() {
    if (!detail) return;
    if (!confirm("Mark this DD254 as superseded? It will no longer appear in active lists.")) return;
    await deleteDd254(detail.id);
    navigate("/app/dd254");
  }

  async function handleRevoke(userId: string) {
    if (!detail) return;
    if (!confirm("Remove read-on access for this user?")) return;
    await revokeDd254Access(detail.id, userId);
    void load();
  }

  if (loading) return <Spinner fullPage />;
  if (!detail) return <p>DD254 not found.</p>;

  return (
    <>
      <Button variant="secondary" size="sm" onClick={() => navigate("/app/dd254")}>
        <ArrowLeft size={14} style={{ marginRight: 4 }} /> Back to DD254s
      </Button>

      <PageHeader
        title={detail.contract_number}
        description={detail.prime_contractor || "—"}
        actions={
          canEdit && detail.status === "active" && (
            <Button size="sm" variant="danger" onClick={handleDelete}>
              <Trash2 size={14} style={{ marginRight: 4 }} /> Mark superseded
            </Button>
          )
        }
      />

      <div className="dd254-detail-grid">
        <div className="dd254-pdf">
          <iframe
            src={dd254FileUrl(detail.id)}
            title={`DD254 ${detail.contract_number}`}
            style={{ width: "100%", height: "100%", border: 0 }}
          />
        </div>

        <div className="dd254-meta">
          <section className="team-section">
            <div className="team-section-header"><h4>Details</h4></div>
            <div className="team-card">
              <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                <ClearanceBadge level={detail.classification_max} />
                <span style={{ color: "var(--color-text-muted)", textTransform: "capitalize" }}>
                  {detail.status}
                </span>
              </div>
              <div style={{ marginTop: 8, fontSize: 13, color: "var(--color-text-secondary)" }}>
                Period: {fmtDate(detail.period_start)} → {fmtDate(detail.period_end)}
              </div>
              {detail.period_start && detail.period_end && (
                <PeriodProgressBar start={detail.period_start} end={detail.period_end} />
              )}
              <div style={{ marginTop: 8, fontSize: 13, color: "var(--color-text-secondary)" }}>
                Sub-org: {detail.sub_org_name ?? "Tenant-wide"}
              </div>
              <div style={{ fontSize: 13, color: "var(--color-text-secondary)" }}>
                Uploaded by {detail.uploaded_by_name} on {fmtDate(detail.created_at)}
              </div>
              <div style={{ fontSize: 12, color: "var(--color-text-muted)", marginTop: 4 }}>
                Markings: {detail.markings} · {(detail.size_bytes / 1024).toFixed(0)} KB
              </div>
            </div>
          </section>

          <section className="team-section">
            <div className="team-section-header">
              <h4>Read-on users ({(detail.access_grants ?? []).filter((g) => !g.debriefed_at).length})</h4>
              {canEdit && (
                <Button size="sm" variant="secondary" onClick={() => setAddAccessOpen(true)}>
                  <UserPlus size={14} style={{ marginRight: 4 }} /> Add
                </Button>
              )}
            </div>
            <div className="team-card">
              {(detail.access_grants ?? []).length === 0 && (
                <p style={{ color: "var(--color-text-muted)", fontSize: 13 }}>
                  No users have been briefed on this DD254.
                </p>
              )}
              {(detail.access_grants ?? []).map((g) => (
                <div key={g.user_id} className="history-row" style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                  <div>
                    <div>{g.name}</div>
                    <div style={{ fontSize: 12, color: "var(--color-text-muted)" }}>
                      {g.email}
                      {g.briefed_at && ` · briefed ${fmtDate(g.briefed_at)}`}
                      {g.debriefed_at && ` · debriefed ${fmtDate(g.debriefed_at)}`}
                    </div>
                  </div>
                  {canEdit && (
                    <Button size="sm" variant="secondary" onClick={() => handleRevoke(g.user_id)}>
                      Remove
                    </Button>
                  )}
                </div>
              ))}
            </div>
          </section>
        </div>
      </div>

      <AddAccessModal
        open={addAccessOpen}
        onClose={() => setAddAccessOpen(false)}
        dd254Id={detail.id}
        existingUserIds={new Set((detail.access_grants ?? []).map((g) => g.user_id))}
        onAdded={() => { setAddAccessOpen(false); void load(); }}
      />
    </>
  );
}

interface AddAccessModalProps {
  open: boolean;
  onClose: () => void;
  dd254Id: string;
  existingUserIds: Set<string>;
  onAdded: () => void;
}

function AddAccessModal({ open, onClose, dd254Id, existingUserIds, onAdded }: AddAccessModalProps) {
  const [members, setMembers] = useState<Member[]>([]);
  const [search, setSearch] = useState("");
  const [briefed, setBriefed] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    listTeam({ search: search || undefined })
      .then((r) => setMembers(r.members ?? []))
      .catch(() => setMembers([]));
  }, [open, search]);

  async function handleAdd(userId: string) {
    setSaving(true);
    try {
      await grantDd254Access(dd254Id, userId, briefed || null, null);
      onAdded();
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Add read-on user">
      <FormField label="Search team members" htmlFor="acc-search">
        <input
          id="acc-search"
          className="filter-input"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Name or email..."
        />
      </FormField>
      <FormField label="Briefed date (optional)" htmlFor="acc-brief">
        <input
          id="acc-brief"
          type="date"
          className="filter-input"
          value={briefed}
          onChange={(e) => setBriefed(e.target.value)}
        />
      </FormField>
      <div className="team-card" style={{ maxHeight: 280, overflowY: "auto" }}>
        {members
          .filter((m) => !existingUserIds.has(m.user_id))
          .map((m) => (
            <div key={m.user_id} className="history-row" style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
              <div>
                <div>{m.name}</div>
                <div style={{ fontSize: 12, color: "var(--color-text-muted)" }}>{m.email}</div>
              </div>
              <Button size="sm" onClick={() => handleAdd(m.user_id)} disabled={saving}>
                Add
              </Button>
            </div>
          ))}
        {members.filter((m) => !existingUserIds.has(m.user_id)).length === 0 && (
          <p style={{ color: "var(--color-text-muted)", fontSize: 13 }}>No additional users found.</p>
        )}
      </div>
    </Modal>
  );
}

export default Dd254DetailPage;
