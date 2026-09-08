import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "../../components/ui/PageHeader";
import { FilterBar } from "../../components/ui/FilterBar";
import { DataTable } from "../../components/ui/DataTable";
import { listTenantAuditLog, AUDIT_ACTIONS, type AuditEntry } from "../../api/auditLog";

const PAGE_SIZE = 50;

function formatAction(action: string): string {
  return action.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

function formatMetadata(meta: Record<string, unknown> | undefined): string {
  if (!meta) return "—";
  const entries = Object.entries(meta).filter(([, v]) => v != null && v !== "");
  if (entries.length === 0) return "—";
  return entries.map(([k, v]) => `${k.replace(/_/g, " ")}: ${String(v)}`).join(", ");
}

export function AuditLogPage() {
  const [action, setAction] = useState("");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);

  const { data, isLoading } = useQuery({
    queryKey: ["tenant-audit-log", action, search, page],
    queryFn: () => listTenantAuditLog({
      action: action || undefined,
      search: search || undefined,
      limit: PAGE_SIZE,
      offset: page * PAGE_SIZE,
    }),
  });

  const entries = data?.entries ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.ceil(total / PAGE_SIZE);

  const columns = [
    {
      header: "Time",
      render: (e: AuditEntry) => (
        <span style={{ whiteSpace: "nowrap", fontSize: 13 }}>
          {new Date(e.created_at).toLocaleString()}
        </span>
      ),
    },
    {
      header: "Action",
      render: (e: AuditEntry) => (
        <span style={{ fontWeight: 500 }}>{formatAction(e.action)}</span>
      ),
    },
    {
      header: "User",
      render: (e: AuditEntry) => (
        <span>
          {e.identity_name || e.identity_email || (
            <span style={{ color: "var(--color-text-muted)" }}>System</span>
          )}
          {e.identity_email && e.identity_name && (
            <span style={{ fontSize: 12, color: "var(--color-text-muted)", marginLeft: 4 }}>
              ({e.identity_email})
            </span>
          )}
        </span>
      ),
    },
    {
      header: "IP Address",
      render: (e: AuditEntry) => (
        <span style={{ fontSize: 13, fontFamily: "monospace" }}>{e.ip_address || "—"}</span>
      ),
    },
    {
      header: "Details",
      render: (e: AuditEntry) => (
        <span style={{ fontSize: 12, color: "var(--color-text-muted)" }}>
          {formatMetadata(e.metadata)}
        </span>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Audit Log"
        description="Security-relevant activity within your organization"
      />

      <FilterBar>
        <FilterBar.Select
          value={action}
          placeholder="All actions"
          options={AUDIT_ACTIONS.map((a) => ({ value: a, label: formatAction(a) }))}
          onChange={(v) => { setAction(v); setPage(0); }}
        />
        <FilterBar.Search
          value={search}
          onChange={(v) => { setSearch(v); setPage(0); }}
          placeholder="Search by user or IP..."
        />
      </FilterBar>

      {isLoading ? (
        <p style={{ color: "var(--color-text-muted)", padding: 24 }}>Loading…</p>
      ) : (
        <DataTable<AuditEntry>
          columns={columns}
          data={entries}
          rowKey={(e) => e.id}
          emptyMessage="No audit entries found."
        />
      )}

      {totalPages > 1 && (
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 16, fontSize: 14 }}>
          <button
            onClick={() => setPage((p) => p - 1)}
            disabled={page === 0}
            className="btn btn-secondary"
          >
            Previous
          </button>
          <span style={{ color: "var(--color-text-muted)" }}>
            Page {page + 1} of {totalPages} ({total} entries)
          </span>
          <button
            onClick={() => setPage((p) => p + 1)}
            disabled={page >= totalPages - 1}
            className="btn btn-secondary"
          >
            Next
          </button>
        </div>
      )}
    </>
  );
}

export default AuditLogPage;
