import { useState, useEffect, useCallback } from "react";
import { Building2, Clock, CheckCircle, Send, Plus, AlertCircle, Download } from "lucide-react";
import { useAuthStore } from "../../stores/authStore";
import { PageHeader } from "../../components/ui/PageHeader";
import { SummaryCards } from "../../components/ui/SummaryCards";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { FilterBar } from "../../components/ui/FilterBar";
import { SubOrgFilter } from "../../components/ui/SubOrgFilter";
import { Button } from "../../components/ui/Button";
import { CreateVisitWizard } from "../../components/visits/CreateVisitWizard";
import { ViewVisitModal } from "../../components/visits/ViewVisitModal";
import { ExportVisitsModal } from "../../components/visits/ExportVisitsModal";
import { isAdmin } from "../../utils/roles";
import * as visitsApi from "../../api/visits";
import type { VisitRequestRow, VisitRequestStats } from "../../api/visits";
import { formatAccessLevel } from "../../api/visits";

// --- Admin View ---

function AdminVisitsView() {
  const [requests, setRequests] = useState<VisitRequestRow[]>([]);
  const [stats, setStats] = useState<VisitRequestStats | null>(null);
  const [, setLoading] = useState(true);
  const [statusFilter, setStatusFilter] = useState("");
  const [search, setSearch] = useState("");
  const [subOrgId, setSubOrgId] = useState("");
  const [viewId, setViewId] = useState<string | null>(null);
  const [exportOpen, setExportOpen] = useState(false);

  const loadData = useCallback(async () => {
    try {
      const [listRes, statsRes] = await Promise.all([
        visitsApi.listAdminVisits({ status: statusFilter || undefined, search: search || undefined, sub_org_id: subOrgId || undefined }),
        visitsApi.getAdminVisitStats(subOrgId || undefined),
      ]);
      setRequests(listRes.requests ?? []);
      setStats(statsRes);
    } catch (err) {
      console.error("Failed to load visit requests", err);
    } finally {
      setLoading(false);
    }
  }, [search, statusFilter, subOrgId]);

  useEffect(() => { void loadData(); }, [loadData]);

  return (
    <>
      <PageHeader
        title="Visit Requests"
        description="Review and manage visit requests"
        actions={
          <Button size="sm" variant="secondary" onClick={() => setExportOpen(true)}>
            <Download size={16} style={{ marginRight: 6 }} /> Export
          </Button>
        }
      />

      {stats && (
        <SummaryCards
          cards={[
            { label: "Total", value: stats.total, icon: Building2 },
            { label: "Submitted", value: stats.submitted, icon: Send, variant: "info" },
            { label: "Under Review", value: stats.under_review, icon: Clock, variant: "warning" },
            { label: "Approved", value: stats.approved, icon: CheckCircle, variant: "success" },
            { label: "Rejected", value: stats.rejected, icon: AlertCircle, variant: "danger" },
          ]}
        />
      )}

      <FilterBar>
        <FilterBar.Select
          value={statusFilter}
          onChange={setStatusFilter}
          placeholder="All Statuses"
          options={[
            { label: "Submitted", value: "submitted" },
            { label: "Under Review", value: "under_review" },
            { label: "Approved", value: "approved" },
            { label: "Rejected", value: "rejected" },
            { label: "Cancelled", value: "cancelled" },
          ]}
        />
        <SubOrgFilter value={subOrgId} onChange={setSubOrgId} />
        <FilterBar.Search value={search} onChange={setSearch} placeholder="Search requests..." />
      </FilterBar>

      <div className="task-list">
        {requests.map((req) => (
          <div
            key={req.id}
            className="task-row"
            onClick={() => setViewId(req.id)}
          >
            <div className="task-row-title">{req.destination_name}</div>
            <div className="task-row-meta">
              <StatusBadge status={req.status} />
              <span>{req.submitter_name}</span>
              <span>{formatAccessLevel(req.access_level)}</span>
              <span>{new Date(req.visit_start_date).toLocaleDateString()} – {new Date(req.visit_end_date).toLocaleDateString()}</span>
            </div>
          </div>
        ))}
        {requests.length === 0 && (
          <p style={{ textAlign: "center", color: "var(--color-text-muted)", padding: 24 }}>
            No visit requests found.
          </p>
        )}
      </div>

      <ViewVisitModal
        open={!!viewId}
        onClose={() => { setViewId(null); void loadData(); }}
        visitId={viewId}
        onUpdated={loadData}
        isAdminView
      />

      <ExportVisitsModal
        open={exportOpen}
        onClose={() => setExportOpen(false)}
      />
    </>
  );
}

// --- IC View ---

function ICVisitsView() {
  const [requests, setRequests] = useState<VisitRequestRow[]>([]);
  const [stats, setStats] = useState<VisitRequestStats | null>(null);
  const [, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [viewId, setViewId] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    try {
      const [listRes, statsRes] = await Promise.all([
        visitsApi.listMyVisits({ status: statusFilter || undefined, search: search || undefined }),
        visitsApi.getMyVisitStats(),
      ]);
      setRequests(listRes.requests ?? []);
      setStats(statsRes);
    } catch (err) {
      console.error("Failed to load visit requests", err);
    } finally {
      setLoading(false);
    }
  }, [search, statusFilter]);

  useEffect(() => { void loadData(); }, [loadData]);

  return (
    <>
      <PageHeader
        title="Visit Requests"
        description="Submit and track your visit requests"
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus size={16} style={{ marginRight: 6 }} /> New Request
          </Button>
        }
      />

      {stats && (
        <SummaryCards
          cards={[
            { label: "Total", value: stats.total, icon: Building2 },
            { label: "Submitted", value: stats.submitted, icon: Send, variant: "info" },
            { label: "Under Review", value: stats.under_review, icon: Clock, variant: "warning" },
            { label: "Approved", value: stats.approved, icon: CheckCircle, variant: "success" },
            { label: "Rejected", value: stats.rejected, icon: AlertCircle, variant: "danger" },
          ]}
        />
      )}

      <FilterBar>
        <FilterBar.Select
          value={statusFilter}
          onChange={setStatusFilter}
          placeholder="All Statuses"
          options={[
            { label: "Submitted", value: "submitted" },
            { label: "Under Review", value: "under_review" },
            { label: "Approved", value: "approved" },
            { label: "Rejected", value: "rejected" },
          ]}
        />
        <FilterBar.Search value={search} onChange={setSearch} placeholder="Search requests..." />
      </FilterBar>

      <div className="task-list">
        {requests.map((req) => (
          <div
            key={req.id}
            className="task-row"
            onClick={() => setViewId(req.id)}
          >
            <div className="task-row-title">{req.destination_name}</div>
            <div className="task-row-meta">
              <StatusBadge status={req.status} />
              <span>{formatAccessLevel(req.access_level)}</span>
              <span>{new Date(req.visit_start_date).toLocaleDateString()} – {new Date(req.visit_end_date).toLocaleDateString()}</span>
            </div>
          </div>
        ))}
        {requests.length === 0 && (
          <p style={{ textAlign: "center", color: "var(--color-text-muted)", padding: 24 }}>
            No visit requests yet. Create your first request to get started.
          </p>
        )}
      </div>

      <CreateVisitWizard
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={loadData}
      />

      <ViewVisitModal
        open={!!viewId}
        onClose={() => { setViewId(null); void loadData(); }}
        visitId={viewId}
        onUpdated={loadData}
      />
    </>
  );
}

// --- Main Page ---

export function VisitsPage() {
  const role = useAuthStore((s) => s.user?.role);
  return isAdmin(role) ? <AdminVisitsView /> : <ICVisitsView />;
}

export default VisitsPage;
