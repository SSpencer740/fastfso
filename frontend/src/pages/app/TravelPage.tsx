import { useState, useEffect, useCallback } from "react";
import { useSearchParams } from "react-router";
import { Plane, Clock, CheckCircle, AlertCircle, Send, Plus, ClipboardList } from "lucide-react";
import { useAuthStore } from "../../stores/authStore";
import { PageHeader } from "../../components/ui/PageHeader";
import { SummaryCards } from "../../components/ui/SummaryCards";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { FilterBar } from "../../components/ui/FilterBar";
import { SubOrgFilter } from "../../components/ui/SubOrgFilter";
import { Button } from "../../components/ui/Button";
import { ReportTravelWizard } from "../../components/travel/ReportTravelWizard";
import { TravelReportDetailModal } from "../../components/travel/TravelReportDetailModal";
import { TravelDebriefModal } from "../../components/travel/TravelDebriefModal";
import { isAdmin } from "../../utils/roles";
import * as travelApi from "../../api/travel";

function AdminTravelView() {
  const [reports, setReports] = useState<travelApi.TravelReportRow[]>([]);
  const [stats, setStats] = useState<travelApi.TravelStats | null>(null);
  const [total, setTotal] = useState(0);
  const totalPages = Math.ceil(total / 20);
  const [status, setStatus] = useState("");
  const [search, setSearch] = useState("");
  const [subOrgId, setSubOrgId] = useState("");
  const [page, setPage] = useState(0);
  const [detailReportId, setDetailReportId] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  const load = useCallback(async () => {
    const [listRes, statsRes] = await Promise.all([
      travelApi.listTravelReports({ status: status || undefined, search: search || undefined, sub_org_id: subOrgId || undefined, limit: 20, offset: page * 20 }),
      travelApi.getTravelStats(subOrgId || undefined),
    ]);
    setReports(listRes.reports || []);
    setTotal(listRes.total);
    setStats(statsRes);
  }, [status, search, subOrgId, page]);

  useEffect(() => { void load(); }, [load, refreshKey]); // eslint-disable-line react-hooks/set-state-in-effect

  return (
    <>
      <PageHeader
        title="Travel"
        description="Review and manage foreign travel reports"
      />

      {stats && (
        <SummaryCards cards={[
          { label: "Total", value: stats.total, icon: Plane },
          { label: "Submitted", value: stats.submitted, icon: Send, variant: "info" },
          { label: "Under Review", value: stats.under_review, icon: Clock, variant: "warning" },
          { label: "Approved", value: stats.approved, icon: CheckCircle, variant: "success" },
        ]} />
      )}

      <FilterBar>
        <FilterBar.Select
          value={status}
          onChange={v => { setStatus(v); setPage(0); }}
          placeholder="All Statuses"
          options={[
            { label: "Submitted", value: "submitted" },
            { label: "Under Review", value: "under_review" },
            { label: "Approved", value: "approved" },
            { label: "Rejected", value: "rejected" },
          ]}
        />
        <SubOrgFilter value={subOrgId} onChange={v => { setSubOrgId(v); setPage(0); }} />
        <FilterBar.Search value={search} onChange={v => { setSearch(v); setPage(0); }} placeholder="Search trips..." />
      </FilterBar>

      <div className="task-list">
        {reports.map(report => (
          <div key={report.id} className="task-row" onClick={() => setDetailReportId(report.id)}>
            <div className="task-row-title">{report.trip_name}</div>
            <div className="task-row-meta">
              <StatusBadge status={report.status} />
              <span>{report.countries}</span>
              <span>{report.creator_name}</span>
              {report.earliest_date && <span>{new Date(report.earliest_date).toLocaleDateString()}</span>}
            </div>
          </div>
        ))}
        {reports.length === 0 && (
          <p style={{ textAlign: "center", color: "var(--color-text-muted)", padding: 24 }}>
            No travel reports found.
          </p>
        )}
      </div>

      {totalPages > 1 && (
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 16, fontSize: 14 }}>
          <button onClick={() => setPage(p => p - 1)} disabled={page === 0} className="btn btn-secondary">Previous</button>
          <span style={{ color: "var(--color-text-muted)" }}>Page {page + 1} of {totalPages} ({total} reports)</span>
          <button onClick={() => setPage(p => p + 1)} disabled={page >= totalPages - 1} className="btn btn-secondary">Next</button>
        </div>
      )}

      <TravelReportDetailModal
        open={!!detailReportId}
        onClose={() => { setDetailReportId(null); setRefreshKey(k => k + 1); }}
        reportId={detailReportId}
        onUpdated={() => setRefreshKey(k => k + 1)}
      />
    </>
  );
}

function ICTravelView() {
  const [reports, setReports] = useState<travelApi.TravelReportRow[]>([]);
  const [stats, setStats] = useState<travelApi.TravelStats | null>(null);
  const [debriefs, setDebriefs] = useState<travelApi.TravelDebrief[]>([]);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [wizardOpen, setWizardOpen] = useState(false);
  const [detailReportId, setDetailReportId] = useState<string | null>(null);
  const [activeDebrief, setActiveDebrief] = useState<travelApi.TravelDebrief | null>(null);

  const load = useCallback(async () => {
    const [listRes, statsRes, debriefRes] = await Promise.all([
      travelApi.listMyTravelReports({ status: status || undefined, search: search || undefined }),
      travelApi.getMyTravelStats(),
      travelApi.getMyDebriefs(),
    ]);
    setReports(listRes.reports || []);
    setStats(statsRes);
    setDebriefs(debriefRes.debriefs || []);
  }, [search, status]);

  useEffect(() => { void load(); }, [load]); // eslint-disable-line react-hooks/set-state-in-effect

  // When the IC arrives via the deep-link in their debrief email
  // (/app/travel?debrief=<id>), find the matching pending debrief and pop
  // the modal so they don't have to hunt for it.
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedDebriefID = searchParams.get("debrief");
  useEffect(() => {
    if (!requestedDebriefID || debriefs.length === 0) return;
    const target = debriefs.find(d => d.id === requestedDebriefID && d.status === "pending");
    if (target) {
      setActiveDebrief(target);
      // Strip the param so a subsequent close+reopen of the page doesn't
      // re-pop the modal.
      const next = new URLSearchParams(searchParams);
      next.delete("debrief");
      setSearchParams(next, { replace: true });
    }
  }, [requestedDebriefID, debriefs, searchParams, setSearchParams]);

  const pendingDebriefs = debriefs.filter(d => d.status === "pending");

  return (
    <>
      <PageHeader
        title="Travel"
        description="Foreign travel requests and briefings"
        actions={
          <Button size="sm" onClick={() => setWizardOpen(true)}>
            <Plus size={16} style={{ marginRight: 6 }} /> Report Travel
          </Button>
        }
      />

      {pendingDebriefs.length > 0 && (
        <div style={{
          background: "var(--color-warning-muted, rgba(255,193,7,0.1))",
          border: "1px solid var(--color-warning, #ffc107)",
          borderRadius: "var(--radius-md)",
          padding: "14px 16px",
          marginBottom: 20,
        }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 10 }}>
            <ClipboardList size={16} style={{ color: "var(--color-warning, #ffc107)", flexShrink: 0 }} />
            <span style={{ fontWeight: 600, fontSize: 14 }}>
              {pendingDebriefs.length === 1 ? "Post-Travel Debrief Required" : `${pendingDebriefs.length} Post-Travel Debriefs Required`}
            </span>
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {pendingDebriefs.map(d => {
              const isOverdue = d.due_date && new Date(d.due_date) < new Date();
              return (
                <div key={d.id} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", flexWrap: "wrap", gap: 8 }}>
                  <div>
                    <span style={{ fontSize: 13, fontWeight: 500 }}>{d.trip_name}</span>
                    {d.due_date && (
                      <span style={{ fontSize: 12, marginLeft: 10, color: isOverdue ? "var(--color-danger)" : "var(--color-text-muted)" }}>
                        Due {new Date(d.due_date).toLocaleDateString()}
                      </span>
                    )}
                  </div>
                  <Button size="sm" onClick={() => setActiveDebrief(d)}>
                    Complete Debrief
                  </Button>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {stats && (
        <SummaryCards cards={[
          { label: "Total", value: stats.total, icon: Plane },
          { label: "Draft", value: stats.draft, icon: Clock, variant: "info" },
          { label: "Submitted", value: stats.submitted, icon: Send, variant: "info" },
          { label: "Approved", value: stats.approved, icon: CheckCircle, variant: "success" },
          { label: "Rejected", value: stats.rejected, icon: AlertCircle, variant: "danger" },
        ]} />
      )}

      <FilterBar>
        <FilterBar.Select
          value={status}
          onChange={setStatus}
          placeholder="All Statuses"
          options={[
            { label: "Draft", value: "draft" },
            { label: "Submitted", value: "submitted" },
            { label: "Approved", value: "approved" },
            { label: "Rejected", value: "rejected" },
          ]}
        />
        <FilterBar.Search value={search} onChange={setSearch} placeholder="Search trips..." />
      </FilterBar>

      <div className="task-list">
        {reports.map(report => (
          <div key={report.id} className="task-row" onClick={() => setDetailReportId(report.id)}>
            <div className="task-row-title">{report.trip_name}</div>
            <div className="task-row-meta">
              <StatusBadge status={report.status} />
              <span>{report.countries}</span>
              {report.earliest_date && <span>{new Date(report.earliest_date).toLocaleDateString()}</span>}
            </div>
          </div>
        ))}
        {reports.length === 0 && (
          <p style={{ textAlign: "center", color: "var(--color-text-muted)", padding: 24 }}>
            No travel reports yet. Click &quot;Report Travel&quot; to get started.
          </p>
        )}
      </div>

      <ReportTravelWizard open={wizardOpen} onClose={() => setWizardOpen(false)} onCreated={load} />
      <TravelReportDetailModal open={!!detailReportId} onClose={() => setDetailReportId(null)} reportId={detailReportId} onUpdated={load} />
      {activeDebrief && (
        <TravelDebriefModal
          debrief={activeDebrief}
          onClose={() => setActiveDebrief(null)}
          onSubmitted={() => { setActiveDebrief(null); void load(); }}
        />
      )}
    </>
  );
}

export function TravelPage() {
  const role = useAuthStore((s) => s.user?.role);
  return isAdmin(role) ? <AdminTravelView /> : <ICTravelView />;
}

export default TravelPage;
