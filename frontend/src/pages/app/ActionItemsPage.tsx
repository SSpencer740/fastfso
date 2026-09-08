import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Inbox, Clock, Eye, CheckCircle } from "lucide-react";
import { PageHeader } from "../../components/ui/PageHeader";
import { SummaryCards } from "../../components/ui/SummaryCards";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { PriorityIndicator } from "../../components/ui/PriorityIndicator";
import { FilterBar } from "../../components/ui/FilterBar";
import { SubOrgFilter } from "../../components/ui/SubOrgFilter";
import { ViewActionItemModal } from "../../components/tasks/ViewActionItemModal";
import * as actionItemsApi from "../../api/actionitems";

const sourceTypeLabels: Record<string, string> = {
  task_submission: "Task Submission",
  visit_request: "Visit Request",
  travel_report: "Travel Report",
  travel_debrief: "Travel Debrief",
  incident_report: "Incident Report",
  clearance_renewal: "Clearance Renewal",
  sf86_submission: "SF-86 Submission",
};

export function ActionItemsPage() {
  const queryClient = useQueryClient();
  const [sourceType, setSourceType] = useState("");
  const [status, setStatus] = useState("");
  const [search, setSearch] = useState("");
  const [subOrgId, setSubOrgId] = useState("");
  const [page, setPage] = useState(0);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const { data: listData } = useQuery({
    queryKey: ["action-items", sourceType, status, search, subOrgId, page],
    queryFn: () => actionItemsApi.listActionItems({
      source_type: sourceType || undefined,
      status: status || undefined,
      search: search || undefined,
      sub_org_id: subOrgId || undefined,
      limit: 20,
      offset: page * 20,
    }),
  });
  const { data: stats } = useQuery({
    queryKey: ["action-item-stats", subOrgId],
    queryFn: () => actionItemsApi.getActionItemStats(subOrgId || undefined),
  });
  const items = listData?.items ?? [];
  const total = listData?.total ?? 0;
  const totalPages = Math.ceil(total / 20);

  function invalidate() {
    void queryClient.invalidateQueries({ queryKey: ["action-items"] });
    void queryClient.invalidateQueries({ queryKey: ["action-item-stats"] });
  }

  return (
    <>
      <PageHeader title="Action Items" description="Review and process incoming requests" />

      {stats && (
        <SummaryCards cards={[
          { label: "Total", value: stats.total, icon: Inbox },
          { label: "Pending", value: stats.pending, icon: Clock, variant: "warning" },
          { label: "Under Review", value: stats.under_review, icon: Eye, variant: "info" },
          { label: "Processed", value: stats.processed, icon: CheckCircle, variant: "success" },
        ]} />
      )}

      <FilterBar>
        <FilterBar.Select
          value={sourceType}
          onChange={v => { setSourceType(v); setPage(0); }}
          placeholder="All Types"
          options={Object.entries(sourceTypeLabels).map(([value, label]) => ({ value, label }))}
        />
        <FilterBar.Select
          value={status}
          onChange={v => { setStatus(v); setPage(0); }}
          placeholder="All Statuses"
          options={[
            { label: "Pending", value: "pending" },
            { label: "Under Review", value: "under_review" },
            { label: "Processed", value: "processed" },
            { label: "Rejected", value: "rejected" },
          ]}
        />
        <SubOrgFilter value={subOrgId} onChange={v => { setSubOrgId(v); setPage(0); }} />
        <FilterBar.Search value={search} onChange={v => { setSearch(v); setPage(0); }} placeholder="Search action items..." />
      </FilterBar>

      <div className="task-list">
        {items.map(item => (
          <PriorityIndicator key={item.id} priority={item.priority}>
            <div className="task-row" onClick={() => setSelectedId(item.id)}>
              <div className="task-row-title">{item.title}</div>
              <div className="task-row-meta">
                <StatusBadge status={item.status} />
                <span style={{ fontSize: 12, color: "var(--color-text-muted)" }}>
                  {sourceTypeLabels[item.source_type] || item.source_type}
                </span>
                {item.due_date && <span>Due {new Date(item.due_date).toLocaleDateString()}</span>}
              </div>
            </div>
          </PriorityIndicator>
        ))}
        {items.length === 0 && (
          <p style={{ textAlign: "center", color: "var(--color-text-muted)", padding: 24 }}>
            No action items found.
          </p>
        )}
      </div>

      {totalPages > 1 && (
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 16, fontSize: 14 }}>
          <button onClick={() => setPage(p => p - 1)} disabled={page === 0} className="btn btn-secondary">Previous</button>
          <span style={{ color: "var(--color-text-muted)" }}>Page {page + 1} of {totalPages} ({total} items)</span>
          <button onClick={() => setPage(p => p + 1)} disabled={page >= totalPages - 1} className="btn btn-secondary">Next</button>
        </div>
      )}

      <ViewActionItemModal
        open={!!selectedId}
        onClose={() => setSelectedId(null)}
        itemId={selectedId}
        onUpdated={invalidate}
      />
    </>
  );
}

export default ActionItemsPage;
