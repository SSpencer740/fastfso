import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ListTodo, Clock, CheckCircle, AlertCircle, FileCheck, Plus, Download } from "lucide-react";
import { useAuthStore } from "../../stores/authStore";
import { PageHeader } from "../../components/ui/PageHeader";
import { SummaryCards } from "../../components/ui/SummaryCards";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { PriorityIndicator } from "../../components/ui/PriorityIndicator";
import { FilterBar } from "../../components/ui/FilterBar";
import { SubOrgFilter } from "../../components/ui/SubOrgFilter";
import { CreateTaskModal } from "../../components/tasks/CreateTaskModal";
import { CompleteTaskModal } from "../../components/tasks/CompleteTaskModal";
import { TaskDetailModal } from "../../components/tasks/TaskDetailModal";
import { isAdmin, isReadOnlyFSO } from "../../utils/roles";
import * as tasksApi from "../../api/tasks";

function today() {
  return new Date().toISOString().slice(0, 10);
}

function firstOfYear() {
  return new Date(new Date().getFullYear(), 0, 1).toISOString().slice(0, 10);
}

function ExportModal({ onClose }: { onClose: () => void }) {
  const [from, setFrom] = useState(firstOfYear());
  const [to, setTo] = useState(today());
  const [format, setFormat] = useState<"csv" | "pdf">("csv");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleExport() {
    setLoading(true);
    setError(null);
    try {
      const url = `/api/v1/admin/tasks/export?format=${format}&from=${from}&to=${to}`;
      const res = await fetch(url, { credentials: "include" });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error ?? "Export failed");
      }
      const blob = await res.blob();
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = `tasks_${from}_${to}.${format}`;
      a.click();
      URL.revokeObjectURL(a.href);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Export failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" style={{ maxWidth: 420 }} onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2 className="modal-title">Export Task Report</h2>
          <button className="modal-close" onClick={onClose}>×</button>
        </div>
        <div className="modal-body" style={{ display: "flex", flexDirection: "column", gap: 16 }}>
          <p style={{ margin: 0, fontSize: 13, color: "var(--color-text-muted)" }}>
            Exports all task assignments for tasks created within the selected date range.
          </p>
          <div>
            <label className="form-label">From</label>
            <input type="date" className="form-input" value={from} max={to} onChange={e => setFrom(e.target.value)} />
          </div>
          <div>
            <label className="form-label">To</label>
            <input type="date" className="form-input" value={to} min={from} max={today()} onChange={e => setTo(e.target.value)} />
          </div>
          <div>
            <label className="form-label">Format</label>
            <div style={{ display: "flex", gap: 12 }}>
              {(["csv", "pdf"] as const).map(f => (
                <label key={f} style={{ display: "flex", alignItems: "center", gap: 6, cursor: "pointer" }}>
                  <input type="radio" name="format" value={f} checked={format === f} onChange={() => setFormat(f)} />
                  {f.toUpperCase()}
                </label>
              ))}
            </div>
          </div>
          {error && <p style={{ color: "var(--color-error)", margin: 0, fontSize: 14 }}>{error}</p>}
        </div>
        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose} disabled={loading}>Cancel</button>
          <button className="btn btn-primary" onClick={handleExport} disabled={loading || !from || !to}>
            {loading ? "Exporting…" : "Download"}
          </button>
        </div>
      </div>
    </div>
  );
}

function AdminTasksView() {
  const queryClient = useQueryClient();
  const role = useAuthStore((s) => s.user?.role);
  const readOnly = isReadOnlyFSO(role);
  const [status, setStatus] = useState("");
  const [priority, setPriority] = useState("");
  const [search, setSearch] = useState("");
  const [subOrgId, setSubOrgId] = useState("");
  const [page, setPage] = useState(0);
  const [createOpen, setCreateOpen] = useState(false);
  const [exportOpen, setExportOpen] = useState(false);
  const [detailTaskId, setDetailTaskId] = useState<string | null>(null);

  const { data: listData } = useQuery({
    queryKey: ["tasks", status, priority, search, subOrgId, page],
    queryFn: () => tasksApi.listTasks({ status: status || undefined, priority: priority || undefined, search: search || undefined, sub_org_id: subOrgId || undefined, limit: 20, offset: page * 20 }),
  });
  const { data: stats } = useQuery({
    queryKey: ["task-stats", subOrgId],
    queryFn: () => tasksApi.getTaskStats(undefined, undefined, subOrgId || undefined),
  });
  const tasks = listData?.tasks ?? [];
  const total = listData?.total ?? 0;
  const totalPages = Math.ceil(total / 20);

  function invalidate() {
    void queryClient.invalidateQueries({ queryKey: ["tasks"] });
    void queryClient.invalidateQueries({ queryKey: ["task-stats"] });
  }

  return (
    <>
      <PageHeader
        title="Tasks"
        description="Manage and assign security tasks"
        actions={
          <div style={{ display: "flex", gap: 8 }}>
            <button className="btn btn-secondary" style={{ width: "auto" }} onClick={() => setExportOpen(true)}>
              <Download size={16} style={{ marginRight: 6 }} /> Export
            </button>
            {!readOnly && (
              <button className="btn btn-primary" style={{ width: "auto" }} onClick={() => setCreateOpen(true)}>
                <Plus size={16} style={{ marginRight: 6 }} /> New Task
              </button>
            )}
          </div>
        }
      />

      {stats && (
        <SummaryCards cards={[
          { label: "Total Tasks", value: stats.total, icon: ListTodo },
          { label: "Active", value: stats.active, icon: CheckCircle, variant: "success" },
          { label: "Needs Review", value: stats.needs_review, icon: AlertCircle, variant: "warning" },
        ]} />
      )}

      <FilterBar>
        <FilterBar.Select
          value={status}
          onChange={v => { setStatus(v); setPage(0); }}
          placeholder="All Statuses"
          options={[
            { label: "Active", value: "active" },
            { label: "Archived", value: "archived" },
          ]}
        />
        <FilterBar.Select
          value={priority}
          onChange={v => { setPriority(v); setPage(0); }}
          placeholder="All Priorities"
          options={[
            { label: "Low", value: "low" },
            { label: "Medium", value: "medium" },
            { label: "High", value: "high" },
            { label: "Urgent", value: "urgent" },
          ]}
        />
        <SubOrgFilter value={subOrgId} onChange={v => { setSubOrgId(v); setPage(0); }} />
        <FilterBar.Search value={search} onChange={v => { setSearch(v); setPage(0); }} placeholder="Search tasks..." />
      </FilterBar>

      <div className="task-list">
        {tasks.map(task => (
          <PriorityIndicator key={task.id} priority={task.priority}>
            <div className="task-row" onClick={() => setDetailTaskId(task.id)}>
              <div className="task-row-title">{task.title}</div>
              <div className="task-row-meta">
                <StatusBadge status={task.status} />
                <span>{task.completed}/{task.total_assigned} completed</span>
                {task.due_date && <span>Due {new Date(task.due_date).toLocaleDateString()}</span>}
              </div>
            </div>
          </PriorityIndicator>
        ))}
        {tasks.length === 0 && (
          <p style={{ textAlign: "center", color: "var(--color-text-muted)", padding: 24 }}>
            No tasks found. Create your first task to get started.
          </p>
        )}
      </div>

      {totalPages > 1 && (
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 16, fontSize: 14 }}>
          <button onClick={() => setPage(p => p - 1)} disabled={page === 0} className="btn btn-secondary">Previous</button>
          <span style={{ color: "var(--color-text-muted)" }}>Page {page + 1} of {totalPages} ({total} tasks)</span>
          <button onClick={() => setPage(p => p + 1)} disabled={page >= totalPages - 1} className="btn btn-secondary">Next</button>
        </div>
      )}

      {exportOpen && <ExportModal onClose={() => setExportOpen(false)} />}
      <CreateTaskModal open={createOpen} onClose={() => setCreateOpen(false)} onCreated={invalidate} />
      <TaskDetailModal open={!!detailTaskId} onClose={() => setDetailTaskId(null)} taskId={detailTaskId} onUpdated={invalidate} />
    </>
  );
}

function ICTasksView() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);

  const { data: listData } = useQuery({
    queryKey: ["my-tasks", search],
    queryFn: () => tasksApi.listMyTasks(search || undefined),
  });
  const { data: stats } = useQuery({
    queryKey: ["my-task-stats"],
    queryFn: () => tasksApi.getMyTaskStats(),
  });
  const tasks = listData?.tasks ?? [];

  function invalidate() {
    void queryClient.invalidateQueries({ queryKey: ["my-tasks"] });
    void queryClient.invalidateQueries({ queryKey: ["my-task-stats"] });
  }

  return (
    <>
      <PageHeader title="My Action Items" description="Your assigned security tasks" />

      {stats && (
        <SummaryCards cards={[
          { label: "Total", value: stats.total, icon: ListTodo },
          { label: "To Do", value: stats.to_do, icon: Clock, variant: "info" },
          { label: "In Progress", value: stats.in_progress, icon: AlertCircle, variant: "warning" },
          { label: "Submitted", value: stats.submitted, icon: FileCheck, variant: "info" },
          { label: "Approved", value: stats.approved, icon: CheckCircle, variant: "success" },
        ]} />
      )}

      <FilterBar>
        <FilterBar.Search value={search} onChange={setSearch} placeholder="Search tasks..." />
      </FilterBar>

      <div className="task-list">
        {tasks.map(task => (
          <PriorityIndicator key={task.id} priority={task.priority}>
            <div className="task-row" onClick={() => setSelectedTaskId(task.id)}>
              <div className="task-row-title">{task.title}</div>
              <div className="task-row-meta">
                <StatusBadge status={task.status} />
                {task.due_date && <span>Due {new Date(task.due_date).toLocaleDateString()}</span>}
                <span>by {task.creator_name}</span>
              </div>
            </div>
          </PriorityIndicator>
        ))}
        {tasks.length === 0 && (
          <p style={{ textAlign: "center", color: "var(--color-text-muted)", padding: 24 }}>
            No tasks assigned to you yet.
          </p>
        )}
      </div>

      <CompleteTaskModal open={!!selectedTaskId} onClose={() => setSelectedTaskId(null)} taskId={selectedTaskId} onSubmitted={invalidate} />
    </>
  );
}

export function TasksPage() {
  const role = useAuthStore((s) => s.user?.role);
  return isAdmin(role) ? <AdminTasksView /> : <ICTasksView />;
}

export default TasksPage;
