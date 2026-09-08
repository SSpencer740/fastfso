import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { FileBadge, Clock, AlertCircle, UserMinus, Plus } from "lucide-react";
import { PageHeader } from "../../components/ui/PageHeader";
import { SummaryCards } from "../../components/ui/SummaryCards";
import { FilterBar } from "../../components/ui/FilterBar";
import { SubOrgFilter } from "../../components/ui/SubOrgFilter";
import { DataTable } from "../../components/ui/DataTable";
import { ClearanceBadge } from "../../components/ui/ClearanceBadge";
import { Button } from "../../components/ui/Button";
import { Alert } from "../../components/ui/Alert";
import { useAuthStore } from "../../stores/authStore";
import { UploadDd254Wizard } from "../../components/dd254/UploadDd254Wizard";
import {
  listDd254,
  getDd254Stats,
  type Dd254Form,
  type Dd254Stats,
} from "../../api/dd254";
import { clearanceLabels } from "../../api/team";

const statusOptions = [
  { label: "Active", value: "active" },
  { label: "Expired", value: "expired" },
  { label: "Superseded", value: "superseded" },
];

const classOptions = [
  { label: clearanceLabels.confidential, value: "confidential" },
  { label: clearanceLabels.secret, value: "secret" },
  { label: clearanceLabels.top_secret, value: "top_secret" },
  { label: clearanceLabels.ts_sci, value: "ts_sci" },
];

function fmtDate(s?: string | null): string {
  return s ? new Date(s).toLocaleDateString() : "—";
}

export function Dd254Page() {
  const navigate = useNavigate();
  const role = useAuthStore((s) => s.user?.role);
  const canUpload = role === "administrator" || role === "fso";

  const [forms, setForms] = useState<Dd254Form[]>([]);
  const [stats, setStats] = useState<Dd254Stats | null>(null);
  const [subOrgId, setSubOrgId] = useState("");
  const [status, setStatus] = useState("active");
  const [classFilter, setClassFilter] = useState("");
  const [search, setSearch] = useState("");
  const [uploadOpen, setUploadOpen] = useState(false);
  const [uploadToast, setUploadToast] = useState<{ message: string; variant: "success" | "warning" } | null>(null);

  useEffect(() => {
    if (!uploadToast) return;
    const id = window.setTimeout(() => setUploadToast(null), 5000);
    return () => window.clearTimeout(id);
  }, [uploadToast]);

  const loadData = useCallback(async () => {
    try {
      const [listRes, statsRes] = await Promise.all([
        listDd254({
          sub_org_id: subOrgId || undefined,
          status: status || undefined,
          class: classFilter || undefined,
          search: search || undefined,
        }),
        getDd254Stats(subOrgId || undefined),
      ]);
      setForms(listRes.forms ?? []);
      setStats(statsRes);
    } catch (err) {
      console.error("Failed to load DD254s", err);
    }
  }, [subOrgId, status, classFilter, search]);

  useEffect(() => { void loadData(); }, [loadData]);

  return (
    <>
      <PageHeader
        title="DD254s"
        description="Contract security classification specifications"
        actions={
          canUpload && (
            <Button size="sm" onClick={() => setUploadOpen(true)}>
              <Plus size={16} style={{ marginRight: 6 }} /> Upload DD254
            </Button>
          )
        }
      />

      {uploadToast && (
        <Alert variant={uploadToast.variant} onDismiss={() => setUploadToast(null)}>
          {uploadToast.message}
        </Alert>
      )}

      {stats && (
        <SummaryCards
          cards={[
            { label: "Active", value: stats.active, icon: FileBadge },
            { label: "Expiring ≤90d", value: stats.expiring_soon, icon: Clock, variant: "warning" },
            { label: "Expired", value: stats.expired, icon: AlertCircle, variant: "danger" },
            { label: "No read-on", value: stats.no_read_on, icon: UserMinus, variant: "info" },
          ]}
        />
      )}

      <FilterBar>
        <SubOrgFilter value={subOrgId} onChange={setSubOrgId} />
        <FilterBar.Select
          value={status}
          onChange={setStatus}
          placeholder="All statuses"
          options={statusOptions}
        />
        <FilterBar.Select
          value={classFilter}
          onChange={setClassFilter}
          placeholder="All classifications"
          options={classOptions}
        />
        <FilterBar.Search value={search} onChange={setSearch} placeholder="Contract # or prime..." />
      </FilterBar>

      <DataTable
        data={forms}
        rowKey={(f) => f.id}
        onRowClick={(f) => navigate(`/app/dd254/${f.id}`)}
        emptyMessage="No DD254s found."
        columns={[
          { header: "Contract #", render: (f) => f.contract_number },
          { header: "Prime", render: (f) => f.prime_contractor || "—" },
          {
            header: "Class. cap",
            render: (f) => <ClearanceBadge level={f.classification_max} />,
          },
          {
            header: "Period",
            render: (f) => `${fmtDate(f.period_start)} → ${fmtDate(f.period_end)}`,
          },
          { header: "Read-on", render: (f) => `${f.read_on_count} user${f.read_on_count === 1 ? "" : "s"}` },
          { header: "Sub-org", render: (f) => f.sub_org_name ?? "Tenant-wide" },
        ]}
      />

      <UploadDd254Wizard
        open={uploadOpen}
        onClose={() => setUploadOpen(false)}
        onUploaded={(result) => {
          setUploadOpen(false);
          setUploadToast(
            result.scanned
              ? { message: "DD254 uploaded and scanned for malware.", variant: "success" }
              : { message: "DD254 uploaded. Malware scanner not configured for this environment — production uploads are always scanned.", variant: "warning" },
          );
          void loadData();
        }}
      />
    </>
  );
}

export default Dd254Page;
