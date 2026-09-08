import { useState, useEffect, useCallback } from "react";
import { FileText, Clock, CheckCircle, Plus, AlertCircle } from "lucide-react";
import { useAuthStore } from "../../stores/authStore";
import { PageHeader } from "../../components/ui/PageHeader";
import { SummaryCards } from "../../components/ui/SummaryCards";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { FilterBar } from "../../components/ui/FilterBar";
import { SubOrgFilter } from "../../components/ui/SubOrgFilter";
import { Button } from "../../components/ui/Button";
import { ReportWizard } from "../../components/reports/ReportWizard";
import { ViewReportModal } from "../../components/reports/ViewReportModal";
import { isAdmin, isIC } from "../../utils/roles";
import * as reportsApi from "../../api/reports";

const REPORTING_REQUIREMENTS = `As a cleared individual, you are required to report changes in personal status, adverse information, foreign travel, foreign contacts, security incidents, and suspicious contacts. Conditions to report include:

• Marriage or cohabitation
• Change of name
• Foreign travel (foreign contacts) — business or pleasure
• Termination of employment
• Any change in naturalized citizenship of you or your spouse
• Becoming a Representative of a Foreign Interest (RFI)
• Any intention to marry or cohabitate with a foreign national
• Media contact related to your job or organization
• Bankruptcy
• Unusual infusion of assets of $10,000 or greater (e.g. inheritance)
• Lawsuits where you could lose more money than you can afford
• Any affiliation with a foreign interest`;

const FCL_REQUIREMENTS = `Named officers in the firm are required to notify their FSO of any of the following changes:

• Change of ownership — Note: Any discussion of the sale of the firm to a foreign owner, even if no sale occurs, must be reported.
• Address change of company.
• The firm has become unable to meet the requirements to safeguard classified information.`;

// --- Admin View ---

function AdminReportsView() {
  const [reports, setReports] = useState<reportsApi.ReportRow[]>([]);
  const [stats, setStats] = useState<reportsApi.ReportStats | null>(null);
  const [total, setTotal] = useState(0);
  const [status, setStatus] = useState("");
  const [search, setSearch] = useState("");
  const [subOrgId, setSubOrgId] = useState("");
  const [page, setPage] = useState(0);
  const [viewId, setViewId] = useState<string | null>(null);
  const [, setLoading] = useState(true);
  const totalPages = Math.ceil(total / 20);

  const loadData = useCallback(async () => {
    try {
      const [listRes, statsRes] = await Promise.all([
        reportsApi.listAdminReports({ status: status || undefined, search: search || undefined, sub_org_id: subOrgId || undefined, limit: 20, offset: page * 20 }),
        reportsApi.getAdminReportStats(subOrgId || undefined),
      ]);
      setReports(listRes.reports ?? []);
      setTotal(listRes.total ?? 0);
      setStats(statsRes);
    } catch (err) {
      console.error("Failed to load reports", err);
    } finally {
      setLoading(false);
    }
  }, [status, search, subOrgId, page]);

  useEffect(() => { void loadData(); }, [loadData]);

  return (
    <>
      <PageHeader title="Reports" description="Foreign contact and activity reports" />

      {stats && (
        <SummaryCards cards={[
          { label: "Total", value: stats.total, icon: FileText },
          { label: "Unreviewed", value: stats.unreviewed, icon: AlertCircle, variant: "warning" },
          { label: "Under Review", value: stats.under_review, icon: Clock, variant: "info" },
          { label: "Processed", value: stats.processed, icon: CheckCircle, variant: "success" },
        ]} />
      )}

      <FilterBar>
        <FilterBar.Select
          value={status}
          onChange={v => { setStatus(v); setPage(0); }}
          placeholder="All Statuses"
          options={[
            { label: "Unreviewed", value: "unreviewed" },
            { label: "Under Review", value: "under_review" },
            { label: "Processed", value: "processed" },
          ]}
        />
        <SubOrgFilter value={subOrgId} onChange={v => { setSubOrgId(v); setPage(0); }} />
        <FilterBar.Search value={search} onChange={v => { setSearch(v); setPage(0); }} placeholder="Search reports..." />
      </FilterBar>

      <div className="task-list">
        {reports.map(r => (
          <div key={r.id} className="task-row" onClick={() => setViewId(r.id)}>
            <div className="task-row-title">{reportsApi.formatReportType(r.report_type)}</div>
            <div className="task-row-meta">
              <StatusBadge status={r.status} />
              <span>{r.creator_name}</span>
              {r.subject_name && <span>re: {r.subject_name}</span>}
              <span>{new Date(r.created_at).toLocaleDateString()}</span>
            </div>
          </div>
        ))}
        {reports.length === 0 && (
          <p style={{ textAlign: "center", color: "var(--color-text-muted)", padding: 24 }}>No reports found.</p>
        )}
      </div>

      {totalPages > 1 && (
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 16, fontSize: 14 }}>
          <button onClick={() => setPage(p => p - 1)} disabled={page === 0} className="btn btn-secondary">Previous</button>
          <span style={{ color: "var(--color-text-muted)" }}>Page {page + 1} of {totalPages} ({total} reports)</span>
          <button onClick={() => setPage(p => p + 1)} disabled={page >= totalPages - 1} className="btn btn-secondary">Next</button>
        </div>
      )}

      <ViewReportModal
        open={!!viewId}
        onClose={() => { setViewId(null); void loadData(); }}
        reportId={viewId}
        onUpdated={loadData}
        isAdminView
      />
    </>
  );
}

// --- IC View ---

function ICReportsView() {
  const role = useAuthStore(s => s.user?.role);
  const showFCL = !isIC(role);

  const [reports, setReports] = useState<reportsApi.ReportRow[]>([]);
  const [stats, setStats] = useState<reportsApi.ReportStats | null>(null);
  const [total, setTotal] = useState(0);
  const [status, setStatus] = useState("");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const [wizardOpen, setWizardOpen] = useState(false);
  const [viewId, setViewId] = useState<string | null>(null);
  const [, setLoading] = useState(true);
  const totalPages = Math.ceil(total / 20);

  const loadData = useCallback(async () => {
    try {
      const [listRes, statsRes] = await Promise.all([
        reportsApi.listMyReports({ status: status || undefined, search: search || undefined, limit: 20, offset: page * 20 }),
        reportsApi.getMyReportStats(),
      ]);
      setReports(listRes.reports ?? []);
      setTotal(listRes.total ?? 0);
      setStats(statsRes);
    } catch (err) {
      console.error("Failed to load reports", err);
    } finally {
      setLoading(false);
    }
  }, [status, search, page]);

  useEffect(() => { void loadData(); }, [loadData]);

  return (
    <>
      <PageHeader
        title="Reports"
        actions={
          <Button size="sm" onClick={() => setWizardOpen(true)}>
            <Plus size={16} style={{ marginRight: 6 }} /> Report Life Event
          </Button>
        }
      />

      {/* Reporting requirements notice */}
      <div style={{ background: "var(--color-bg-elevated)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", padding: "1rem 1.25rem", marginBottom: 24 }}>
        <h4 style={{ margin: "0 0 8px", fontSize: 15 }}>Reporting for Cleared Personnel</h4>
        <p style={{ margin: "0 0 8px", fontSize: 13, color: "var(--color-text-muted)", whiteSpace: "pre-line", lineHeight: 1.7 }}>
          {REPORTING_REQUIREMENTS}
        </p>
        <p style={{ margin: 0, fontSize: 13, color: "var(--color-text-muted)", lineHeight: 1.7 }}>
          <a href="https://www.dni.gov/files/NCSC/documents/Regulations/SEAD-3-Reporting-U.pdf" target="_blank" rel="noreferrer">Security Executive Agent Directive 3 (SEAD 3)</a> further outlines reporting requirements.
        </p>
      </div>

      {showFCL && (
        <div style={{ background: "var(--color-bg-elevated)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", padding: "1rem 1.25rem", marginBottom: 24 }}>
          <h4 style={{ margin: "0 0 8px", fontSize: 15 }}>Facilities Clearance Reporting</h4>
          <p style={{ margin: 0, fontSize: 13, color: "var(--color-text-muted)", whiteSpace: "pre-line", lineHeight: 1.7 }}>
            {FCL_REQUIREMENTS}
          </p>
        </div>
      )}

      {stats && (
        <SummaryCards cards={[
          { label: "Total", value: stats.total, icon: FileText },
          { label: "Unreviewed", value: stats.unreviewed, icon: AlertCircle, variant: "warning" },
          { label: "Under Review", value: stats.under_review, icon: Clock, variant: "info" },
          { label: "Processed", value: stats.processed, icon: CheckCircle, variant: "success" },
        ]} />
      )}

      <FilterBar>
        <FilterBar.Select
          value={status}
          onChange={v => { setStatus(v); setPage(0); }}
          placeholder="All Statuses"
          options={[
            { label: "Unreviewed", value: "unreviewed" },
            { label: "Under Review", value: "under_review" },
            { label: "Processed", value: "processed" },
          ]}
        />
        <FilterBar.Search value={search} onChange={v => { setSearch(v); setPage(0); }} placeholder="Search reports..." />
      </FilterBar>

      <div className="task-list">
        {reports.map(r => (
          <div key={r.id} className="task-row" onClick={() => setViewId(r.id)}>
            <div className="task-row-title">{reportsApi.formatReportType(r.report_type)}</div>
            <div className="task-row-meta">
              <StatusBadge status={r.status} />
              {r.subject_name && <span>re: {r.subject_name}</span>}
              <span>{new Date(r.created_at).toLocaleDateString()}</span>
            </div>
          </div>
        ))}
        {reports.length === 0 && (
          <p style={{ textAlign: "center", color: "var(--color-text-muted)", padding: 24 }}>
            No reports yet. Use &quot;Report Life Event&quot; to submit one.
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

      <ReportWizard open={wizardOpen} onClose={() => setWizardOpen(false)} onCreated={loadData} showFCL={showFCL} />
      <ViewReportModal open={!!viewId} onClose={() => { setViewId(null); void loadData(); }} reportId={viewId} onUpdated={loadData} />
    </>
  );
}

// --- Main Page ---

export function ReportsPage() {
  const role = useAuthStore(s => s.user?.role);
  return isAdmin(role) ? <AdminReportsView /> : <ICReportsView />;
}

export default ReportsPage;
