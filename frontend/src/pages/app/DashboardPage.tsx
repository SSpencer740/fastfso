import { useState, useEffect, useCallback } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer, Legend } from "recharts";
import { useAuthStore } from "../../stores/authStore";
import { PageHeader } from "../../components/ui/PageHeader";
import { SummaryCards } from "../../components/ui/SummaryCards";
import { isIC } from "../../utils/roles";
import { getMyTaskStats, getTaskStats } from "../../api/tasks";
import { getMyTravelStats, getTravelStats, getMyDebriefs } from "../../api/travel";
import type { TravelDebrief } from "../../api/travel";
import { getMyVisitStats, getAdminVisitStats } from "../../api/visits";
import { getMyReportStats, getAdminReportStats } from "../../api/reports";
import { getActionItemStats } from "../../api/actionitems";
import { getMyFSO, listSubOrgs } from "../../api/customerAdmin";
import type { MySummaryStats, SummaryStats } from "../../api/tasks";
import type { TravelStats } from "../../api/travel";
import type { VisitRequestStats } from "../../api/visits";
import type { ReportStats } from "../../api/reports";
import type { ActionItemStats } from "../../api/actionitems";
import { FileText, Plane, Building2, ClipboardList, CheckSquare } from "lucide-react";

// --- Date range helpers ---

type Preset = "30d" | "90d" | "1y" | "all" | "custom";

function toISO(d: Date) {
  return d.toISOString().split("T")[0];
}

function presetDates(preset: Preset): { from: string; to: string } {
  const now = new Date();
  const to = toISO(now);
  if (preset === "30d") return { from: toISO(new Date(now.getTime() - 30 * 86400000)), to };
  if (preset === "90d") return { from: toISO(new Date(now.getTime() - 90 * 86400000)), to };
  if (preset === "1y") return { from: toISO(new Date(now.getTime() - 365 * 86400000)), to };
  return { from: "", to: "" }; // "all"
}

// --- Chart colours ---

const IC_COLORS: Record<string, string> = {
  "To Do": "#94a3b8",
  "In Progress": "#3b82f6",
  "Submitted": "#f59e0b",
  "Approved": "#22c55e",
};

const ADMIN_COLORS: Record<string, string> = {
  "Active": "#3b82f6",
  "Draft": "#94a3b8",
  "Needs Review": "#f59e0b",
  "Archived": "#64748b",
};

// --- Date range picker ---

interface DateRangePickerProps {
  preset: Preset;
  from: string;
  to: string;
  onPreset: (p: Preset) => void;
  onFrom: (v: string) => void;
  onTo: (v: string) => void;
}

function DateRangePicker({ preset, from, to, onPreset, onFrom, onTo }: DateRangePickerProps) {
  const presets: { label: string; value: Preset }[] = [
    { label: "Last 30 days", value: "30d" },
    { label: "Last 90 days", value: "90d" },
    { label: "Last year", value: "1y" },
    { label: "All time", value: "all" },
    { label: "Custom", value: "custom" },
  ];

  return (
    <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
      {presets.map(p => (
        <button
          key={p.value}
          type="button"
          onClick={() => onPreset(p.value)}
          style={{
            padding: "4px 12px",
            fontSize: 13,
            borderRadius: "var(--radius-md)",
            border: "1px solid",
            cursor: "pointer",
            borderColor: preset === p.value ? "var(--color-primary)" : "var(--color-border)",
            background: preset === p.value ? "var(--color-primary)" : "transparent",
            color: preset === p.value ? "#fff" : "var(--color-text)",
          }}
        >
          {p.label}
        </button>
      ))}
      {preset === "custom" && (
        <div style={{ display: "flex", alignItems: "center", gap: 6, marginLeft: 4 }}>
          <input
            type="date"
            value={from}
            onChange={e => onFrom(e.target.value)}
            style={{ fontSize: 13, padding: "3px 8px", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", background: "var(--color-surface)", color: "var(--color-text)" }}
          />
          <span style={{ fontSize: 13, color: "var(--color-text-muted)" }}>to</span>
          <input
            type="date"
            value={to}
            onChange={e => onTo(e.target.value)}
            style={{ fontSize: 13, padding: "3px 8px", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", background: "var(--color-surface)", color: "var(--color-text)" }}
          />
        </div>
      )}
    </div>
  );
}

// --- Donut chart ---

interface DonutChartProps {
  data: { name: string; value: number }[];
  colors: Record<string, string>;
  total: number;
}

function DonutChart({ data, colors, total }: DonutChartProps) {
  const nonEmpty = data.filter(d => d.value > 0);
  const display = nonEmpty.length > 0 ? nonEmpty : [{ name: "No data", value: 1 }];
  const isEmpty = nonEmpty.length === 0;

  return (
    <div style={{ position: "relative", width: "100%", height: 280 }}>
      <ResponsiveContainer width="100%" height="100%">
        <PieChart>
          <Pie
            data={display}
            cx="50%"
            cy="50%"
            innerRadius={75}
            outerRadius={110}
            paddingAngle={isEmpty ? 0 : 3}
            dataKey="value"
            strokeWidth={0}
          >
            {display.map((entry) => (
              <Cell
                key={entry.name}
                fill={isEmpty ? "var(--color-border)" : (colors[entry.name] ?? "#94a3b8")}
              />
            ))}
          </Pie>
          {!isEmpty && <Tooltip formatter={(v) => [v, ""]} />}
          {!isEmpty && <Legend />}
        </PieChart>
      </ResponsiveContainer>
      {/* centre label */}
      <div style={{
        position: "absolute", top: "50%", left: "50%",
        transform: "translate(-50%, -50%)",
        textAlign: "center", pointerEvents: "none",
      }}>
        <div style={{ fontSize: 28, fontWeight: 700, lineHeight: 1 }}>{total}</div>
        <div style={{ fontSize: 12, color: "var(--color-text-muted)", marginTop: 2 }}>total</div>
      </div>
    </div>
  );
}

// --- IC Dashboard ---

function ICDashboard() {
  const navigate = useNavigate();
  const [preset, setPreset] = useState<Preset>("30d");
  const [from, setFrom] = useState(() => presetDates("30d").from);
  const [to, setTo] = useState(() => presetDates("30d").to);

  const { data: fsoData } = useQuery({
    queryKey: ["my-fso"],
    queryFn: getMyFSO,
  });
  const fso = fsoData?.fso ?? null;

  // Pending debriefs surface above the rest of the dashboard so an IC who
  // has just been assigned one doesn't have to discover the questionnaire
  // by drilling into the Travel page. The /api/v1/travel/debriefs endpoint
  // already returns only the user's own pending entries.
  const { data: debriefsData } = useQuery({
    queryKey: ["my-pending-debriefs"],
    queryFn: getMyDebriefs,
  });
  const pendingDebriefs: TravelDebrief[] = (debriefsData?.debriefs ?? []).filter(d => d.status === "pending");

  const [taskStats, setTaskStats] = useState<MySummaryStats | null>(null);
  const [travelStats, setTravelStats] = useState<TravelStats | null>(null);
  const [visitStats, setVisitStats] = useState<VisitRequestStats | null>(null);
  const [reportStats, setReportStats] = useState<ReportStats | null>(null);
  const [, setLoading] = useState(true);

  const loadData = useCallback(async () => {
    try {
      const [tasks, travel, visits, reports] = await Promise.all([
        getMyTaskStats(from || undefined, to || undefined),
        getMyTravelStats(),
        getMyVisitStats(),
        getMyReportStats(),
      ]);
      setTaskStats(tasks);
      setTravelStats(travel);
      setVisitStats(visits);
      setReportStats(reports);
    } catch (err) {
      console.error("Failed to load dashboard", err);
    } finally {
      setLoading(false);
    }
  }, [from, to]);

  useEffect(() => { void loadData(); }, [loadData]);

  function handlePreset(p: Preset) {
    setPreset(p);
    if (p !== "custom") {
      const dates = presetDates(p);
      setFrom(dates.from);
      setTo(dates.to);
    }
  }

  const chartData = taskStats ? [
    { name: "To Do", value: taskStats.to_do },
    { name: "In Progress", value: taskStats.in_progress },
    { name: "Submitted", value: taskStats.submitted },
    { name: "Approved", value: taskStats.approved },
  ] : [];

  return (
    <>
      <PageHeader title="Dashboard" />

      <div style={{ marginBottom: 24 }}>
        <DateRangePicker
          preset={preset} from={from} to={to}
          onPreset={handlePreset} onFrom={setFrom} onTo={setTo}
        />
      </div>

      {pendingDebriefs.length > 0 && (
        <div style={{ background: "color-mix(in srgb, var(--color-primary) 8%, var(--color-bg-elevated))", border: "1px solid var(--color-primary)", borderRadius: "var(--radius-md)", padding: "1rem 1.25rem", marginBottom: 24 }}>
          <h4 style={{ margin: "0 0 8px", fontSize: 15, color: "var(--color-text)" }}>
            Needs your attention
          </h4>
          <p style={{ margin: "0 0 12px", fontSize: 13, color: "var(--color-text-muted)" }}>
            {pendingDebriefs.length === 1
              ? "You have a post-travel debrief to complete."
              : `You have ${pendingDebriefs.length} post-travel debriefs to complete.`}
          </p>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            {pendingDebriefs.map(d => (
              <button
                key={d.id}
                type="button"
                onClick={() => navigate(`/app/travel?debrief=${d.id}`)}
                style={{
                  textAlign: "left",
                  padding: "8px 12px",
                  background: "var(--color-bg-elevated)",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-sm)",
                  cursor: "pointer",
                  fontSize: 13,
                  color: "var(--color-text)",
                }}
              >
                <strong>{d.trip_name}</strong>
                {d.due_date && (
                  <span style={{ color: "var(--color-text-muted)", marginLeft: 8 }}>
                    · due {new Date(d.due_date).toLocaleDateString()}
                  </span>
                )}
              </button>
            ))}
          </div>
        </div>
      )}

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 24, marginBottom: 24 }}>
        {/* Task donut */}
        <div style={{ background: "var(--color-bg-elevated)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", padding: "1.25rem" }}>
          <h4 style={{ margin: "0 0 16px", fontSize: 15 }}>My Tasks</h4>
          <DonutChart
            data={chartData}
            colors={IC_COLORS}
            total={taskStats?.total ?? 0}
          />
        </div>

        {/* Activity summary */}
        <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
          <SummaryCards cards={[
            { label: "Tasks", value: taskStats?.total ?? 0, icon: CheckSquare },
            { label: "Travel Reports", value: travelStats?.total ?? 0, icon: Plane },
          ]} />
          <SummaryCards cards={[
            { label: "Visit Requests", value: visitStats?.total ?? 0, icon: Building2 },
            { label: "Life Event Reports", value: reportStats?.total ?? 0, icon: FileText },
          ]} />
        </div>
      </div>

      {/* FSO contact */}
      <div style={{ background: "var(--color-bg-elevated)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", padding: "1rem 1.25rem" }}>
        <h4 style={{ margin: "0 0 8px", fontSize: 15 }}>Your FSO</h4>
        {fso ? (
          <div style={{ fontSize: 13 }}>
            <div style={{ fontWeight: 600, marginBottom: 2 }}>{fso.name}</div>
            <a href={`mailto:${fso.email}`} style={{ color: "var(--color-primary)" }}>{fso.email}</a>
          </div>
        ) : (
          <p style={{ margin: 0, fontSize: 13, color: "var(--color-text-muted)" }}>
            No primary FSO has been assigned to your sub-organization yet. Contact your administrator for assistance.
          </p>
        )}
      </div>
    </>
  );
}

// --- Admin / FSO Dashboard ---

function AdminDashboard() {
  const [preset, setPreset] = useState<Preset>("30d");
  const [from, setFrom] = useState(() => presetDates("30d").from);
  const [to, setTo] = useState(() => presetDates("30d").to);
  const [selectedSubOrgId, setSelectedSubOrgId] = useState<string>("");

  const { data: subOrgData } = useQuery({
    queryKey: ["sub-orgs"],
    queryFn: listSubOrgs,
  });
  const subOrgs = (subOrgData?.sub_orgs ?? []).filter(o => o.name !== "Default");

  const [taskStats, setTaskStats] = useState<SummaryStats | null>(null);
  const [travelStats, setTravelStats] = useState<TravelStats | null>(null);
  const [visitStats, setVisitStats] = useState<VisitRequestStats | null>(null);
  const [reportStats, setReportStats] = useState<ReportStats | null>(null);
  const [actionStats, setActionStats] = useState<ActionItemStats | null>(null);
  const [, setLoading] = useState(true);

  const subOrgId = selectedSubOrgId || undefined;

  const loadData = useCallback(async () => {
    try {
      const [tasks, travel, visits, reports, actions] = await Promise.all([
        getTaskStats(from || undefined, to || undefined, subOrgId),
        getTravelStats(subOrgId),
        getAdminVisitStats(subOrgId),
        getAdminReportStats(subOrgId),
        getActionItemStats(subOrgId),
      ]);
      setTaskStats(tasks);
      setTravelStats(travel);
      setVisitStats(visits);
      setReportStats(reports);
      setActionStats(actions);
    } catch (err) {
      console.error("Failed to load dashboard", err);
    } finally {
      setLoading(false);
    }
  }, [from, to, subOrgId]);

  useEffect(() => { void loadData(); }, [loadData]);

  function handlePreset(p: Preset) {
    setPreset(p);
    if (p !== "custom") {
      const dates = presetDates(p);
      setFrom(dates.from);
      setTo(dates.to);
    }
  }

  const selectedSubOrgName = subOrgs.find(o => o.id === selectedSubOrgId)?.name;
  const chartTitle = selectedSubOrgName ? `Tasks — ${selectedSubOrgName}` : "Tasks — Org Wide";

  const chartData = taskStats ? [
    { name: "Active", value: taskStats.active },
    { name: "Draft", value: taskStats.draft },
    { name: "Needs Review", value: taskStats.needs_review },
  ] : [];

  return (
    <>
      <PageHeader title="Dashboard" description="Organisation-wide overview" />

      <div style={{ display: "flex", alignItems: "center", gap: 16, marginBottom: 24, flexWrap: "wrap" }}>
        <DateRangePicker
          preset={preset} from={from} to={to}
          onPreset={handlePreset} onFrom={setFrom} onTo={setTo}
        />
        {subOrgs.length > 0 && (
          <select
            value={selectedSubOrgId}
            onChange={e => setSelectedSubOrgId(e.target.value)}
            style={{ fontSize: 13, padding: "4px 10px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-border)", background: "var(--color-surface)", color: "var(--color-text)", height: 32 }}
          >
            <option value="">All Sub-Organizations</option>
            {subOrgs.map(o => <option key={o.id} value={o.id}>{o.name}</option>)}
          </select>
        )}
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 24, marginBottom: 24 }}>
        {/* Task donut */}
        <div style={{ background: "var(--color-bg-elevated)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", padding: "1.25rem" }}>
          <h4 style={{ margin: "0 0 16px", fontSize: 15 }}>{chartTitle}</h4>
          <DonutChart
            data={chartData}
            colors={ADMIN_COLORS}
            total={taskStats?.total ?? 0}
          />
        </div>

        {/* Action items + activity */}
        <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
          <div style={{ background: "var(--color-bg-elevated)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", padding: "1rem 1.25rem" }}>
            <h4 style={{ margin: "0 0 12px", fontSize: 15 }}>Action Items</h4>
            <SummaryCards cards={[
              { label: "Pending", value: actionStats?.pending ?? 0, icon: ClipboardList, variant: "warning" },
              { label: "Under Review", value: actionStats?.under_review ?? 0, icon: ClipboardList, variant: "info" },
            ]} />
          </div>
          <SummaryCards cards={[
            { label: "Travel Reports", value: travelStats?.total ?? 0, icon: Plane },
            { label: "Visit Requests", value: visitStats?.total ?? 0, icon: Building2 },
          ]} />
          <SummaryCards cards={[
            { label: "Life Event Reports", value: reportStats?.total ?? 0, icon: FileText },
          ]} />
        </div>
      </div>
    </>
  );
}

// --- Main export ---

export function DashboardPage() {
  const role = useAuthStore(s => s.user?.role);
  return isIC(role) ? <ICDashboard /> : <AdminDashboard />;
}

export default DashboardPage;
