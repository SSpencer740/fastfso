import { useCallback, useEffect, useState } from "react";
import { UserCog, AlertCircle, AlertTriangle } from "lucide-react";
import { PageHeader } from "../../components/ui/PageHeader";
import { SummaryCards } from "../../components/ui/SummaryCards";
import { FilterBar } from "../../components/ui/FilterBar";
import { SubOrgFilter } from "../../components/ui/SubOrgFilter";
import { DataTable } from "../../components/ui/DataTable";
import { ClearanceBadge } from "../../components/ui/ClearanceBadge";
import { InvestigationDueCell } from "../../components/ui/InvestigationDueCell";
import { ViewMemberModal } from "../../components/team/ViewMemberModal";
import {
  clearanceLabels,
  getTeamStats,
  listTeam,
  type Member,
  type TeamStats,
  type ClearanceLevel,
} from "../../api/team";

const clearanceFilterOptions: { label: string; value: string }[] = [
  { label: "None or missing", value: "none_or_missing" },
  { label: clearanceLabels.confidential, value: "confidential" },
  { label: clearanceLabels.secret, value: "secret" },
  { label: clearanceLabels.top_secret, value: "top_secret" },
  { label: clearanceLabels.ts_sci, value: "ts_sci" },
];

const dueWithinOptions = [
  { label: "Any", value: "" },
  { label: "Overdue / 30 days", value: "30" },
  { label: "Within 90 days", value: "90" },
  { label: "Within 180 days", value: "180" },
];

const roleLabel = (role: string) =>
  ({
    administrator: "Administrator",
    fso: "FSO",
    read_only_fso: "Read-only FSO",
    individual_contributor: "IC",
  })[role] ?? role;

const PAGE_SIZE = 50;

export function TeamPage() {
  const [members, setMembers] = useState<Member[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [stats, setStats] = useState<TeamStats | null>(null);
  const [subOrgId, setSubOrgId] = useState("");
  const [clearance, setClearance] = useState("");
  const [dueWithin, setDueWithin] = useState("");
  const [search, setSearch] = useState("");
  const [viewUserId, setViewUserId] = useState<string | null>(null);

  const totalPages = Math.ceil(total / PAGE_SIZE);

  // Changing any filter resets to the first page so the offset stays valid.
  const resetPage = () => setPage(0);

  const loadData = useCallback(async () => {
    try {
      const [listRes, statsRes] = await Promise.all([
        listTeam({
          sub_org_id: subOrgId || undefined,
          clearance: clearance || undefined,
          search: search || undefined,
          due_within_days: dueWithin ? Number(dueWithin) : undefined,
          limit: PAGE_SIZE,
          offset: page * PAGE_SIZE,
        }),
        getTeamStats(subOrgId || undefined),
      ]);
      setMembers(listRes.members ?? []);
      setTotal(listRes.total ?? 0);
      setStats(statsRes);
    } catch (err) {
      console.error("Failed to load team", err);
    }
  }, [subOrgId, clearance, dueWithin, search, page]);

  useEffect(() => { void loadData(); }, [loadData]);

  return (
    <>
      <PageHeader title="Team" description="Roster and clearances" />

      {stats && (
        <SummaryCards
          cards={[
            { label: "Members", value: stats.members, icon: UserCog },
            { label: "Due ≤90d", value: stats.due_within_90, icon: AlertTriangle, variant: "warning" },
            { label: "Overdue", value: stats.overdue, icon: AlertCircle, variant: "danger" },
          ]}
        />
      )}

      <FilterBar>
        <SubOrgFilter value={subOrgId} onChange={(v) => { resetPage(); setSubOrgId(v); }} />
        <FilterBar.Select
          value={clearance}
          onChange={(v) => { resetPage(); setClearance(v); }}
          placeholder="All clearance levels"
          options={clearanceFilterOptions}
        />
        <FilterBar.Select
          value={dueWithin}
          onChange={(v) => { resetPage(); setDueWithin(v); }}
          placeholder="Investigation due"
          options={dueWithinOptions}
        />
        <FilterBar.Search value={search} onChange={(v) => { resetPage(); setSearch(v); }} placeholder="Search by name or email..." />
      </FilterBar>

      <DataTable
        data={members}
        rowKey={(m) => m.user_id}
        onRowClick={(m) => setViewUserId(m.user_id)}
        emptyMessage="No team members found."
        columns={[
          { header: "Name", render: (m) => m.name },
          { header: "Email", render: (m) => m.email },
          { header: "Role", render: (m) => roleLabel(m.role) },
          {
            header: "Sub-orgs",
            render: (m) =>
              m.sub_orgs.filter((s) => s.name !== "Default").map((s) => s.name).join(", ") || "—",
          },
          {
            header: "Clearance",
            render: (m) => <ClearanceBadge level={m.clearance as ClearanceLevel | ""} />,
          },
          {
            header: "Next investigation",
            render: (m) => <InvestigationDueCell date={m.next_investigation_date} />,
          },
        ]}
      />

      {totalPages > 1 && (
        <div style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 16, marginTop: 16 }}>
          <button onClick={() => setPage(p => p - 1)} disabled={page === 0} className="btn btn-secondary">Previous</button>
          <span style={{ color: "var(--color-text-muted)" }}>Page {page + 1} of {totalPages} ({total} members)</span>
          <button onClick={() => setPage(p => p + 1)} disabled={page >= totalPages - 1} className="btn btn-secondary">Next</button>
        </div>
      )}

      <ViewMemberModal
        open={!!viewUserId}
        userId={viewUserId}
        onClose={() => { setViewUserId(null); void loadData(); }}
      />
    </>
  );
}

export default TeamPage;
