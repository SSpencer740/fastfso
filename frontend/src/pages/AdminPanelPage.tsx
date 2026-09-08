import { useEffect, useState } from "react";
import { Navigate, useNavigate, useLocation } from "react-router";
import { useAuthStore } from "../stores/authStore";
import { api } from "../api/client";
import {
  type AdminTenant,
  type AdminTenantDetail,
  type AdminIdentity,
  type AdminIdentityDetail,
  type AdminPasskey,
  type SSOConfig,
  type SSOConfigUpdate,
  type AIFeatureUsage,
  getAdminTenants,
  getTenantAIUsage,
  createTenant,
  getTenant,
  updateTenant,
  deleteTenant,
  suspendTenant,
  unsuspendTenant,
  inviteAdmin,
  getSSOConfig,
  updateSSOConfig,
  getAdminIdentities,
  createIdentity,
  getIdentity,
  resetPassword,
  setSuperAdmin,
  suspendIdentity,
  unsuspendIdentity,
  deleteIdentity,
  addIdentityToTenant,
  resendInvite,
  changeIdentityEmail,
  listIdentityPasskeys,
  deleteIdentityPasskey,
} from "../api/admin";
import {
  type Passkey,
  listPasskeys,
  registerPasskey,
  removePasskey,
} from "../api/settings";
import { ApiError } from "../api/client";
import { DashboardLayout } from "../components/layout/DashboardLayout";
import { Alert } from "../components/ui/Alert";
import { Button } from "../components/ui/Button";
import { Spinner } from "../components/ui/Spinner";
import { DataTable } from "../components/ui/DataTable";

const ADMIN_API = "/api/admin";

interface AdminSession {
  id: string;
  identity_id: string;
  state: string;
  ip_address: string;
  user_agent: string;
  auth_method: string;
  is_super_admin: boolean;
  created_at: string;
  last_active_at: string;
  email: string;
  name: string;
}

interface AuditEntry {
  id: string;
  action: string;
  ip_address: string;
  user_agent: string;
  metadata: Record<string, unknown>;
  created_at: string;
}

const TABS = ["sessions", "audit", "tenants", "identities", "security"] as const;
type Tab = (typeof TABS)[number];

const TAB_LABELS: Record<Tab, string> = {
  sessions: "Sessions",
  audit: "Audit Log",
  tenants: "Tenants",
  identities: "Identities",
  security: "My Security",
};

function parseAdminPath(pathname: string): {
  tab: Tab;
  resourceId: string | null;
} {
  const parts = pathname.replace(/^\/admin\/?/, "").split("/").filter(Boolean);
  const tab = TABS.includes(parts[0] as Tab) ? (parts[0] as Tab) : "sessions";
  const resourceId = parts[1] || null;
  return { tab, resourceId };
}

export function AdminPanelPage() {
  const isSuperAdmin = useAuthStore((s) => s.isSuperAdmin);
  const navigate = useNavigate();
  const location = useLocation();

  const { tab: activeTab, resourceId } = parseAdminPath(location.pathname);

  const [sessions, setSessions] = useState<AdminSession[]>([]);
  const [auditEntries, setAuditEntries] = useState<AuditEntry[]>([]);
  const [tenants, setTenants] = useState<AdminTenant[]>([]);
  const [identities, setIdentities] = useState<AdminIdentity[]>([]);
  const [loading, setLoading] = useState(false);

  // Detail views
  const [tenantDetail, setTenantDetail] = useState<AdminTenantDetail | null>(
    null,
  );
  const [identityDetail, setIdentityDetail] =
    useState<AdminIdentityDetail | null>(null);

  // Forms
  const [showNewTenant, setShowNewTenant] = useState(false);
  const [newTenantName, setNewTenantName] = useState("");
  const [showNewIdentity, setShowNewIdentity] = useState(false);
  const [newIdentityEmail, setNewIdentityEmail] = useState("");
  const [newIdentityName, setNewIdentityName] = useState("");
  const [newIdentityPassword, setNewIdentityPassword] = useState("");

  // Tenant rename
  const [editingTenantName, setEditingTenantName] = useState("");
  const [isEditingTenant, setIsEditingTenant] = useState(false);

  // AI usage rollup (current UTC month)
  const [aiUsage, setAIUsage] = useState<AIFeatureUsage[] | null>(null);
  const [aiUsageSince, setAIUsageSince] = useState<string>("");

  // SSO config
  const [ssoConfig, setSSOConfig] = useState<SSOConfig | null>(null);
  const [ssoLoading, setSSOLoading] = useState(false);
  const [showSSOForm, setShowSSOForm] = useState(false);
  const [ssoForm, setSSOForm] = useState<SSOConfigUpdate>({
    protocol: "oidc",
    enabled: false,
    auto_provision: false,
    default_role: "individual_contributor",
  });

  // Invite admin
  const [showInvite, setShowInvite] = useState(false);
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteName, setInviteName] = useState("");
  const [inviteSuccess, setInviteSuccess] = useState("");

  // Passkeys
  const [identityPasskeys, setIdentityPasskeys] = useState<AdminPasskey[]>([]);

  // Super-admin's own passkeys (Security tab)
  const [myPasskeys, setMyPasskeys] = useState<Passkey[]>([]);
  const [myPasskeyName, setMyPasskeyName] = useState("");
  const [myPasskeyAdding, setMyPasskeyAdding] = useState(false);
  const [myPasskeyError, setMyPasskeyError] = useState("");

  // Identity actions
  const [resetPw, setResetPw] = useState("");
  const [showResetPw, setShowResetPw] = useState(false);
  const [showAddToTenant, setShowAddToTenant] = useState(false);
  const [addTenantId, setAddTenantId] = useState("");
  const [addTenantRole, setAddTenantRole] = useState("read_only_fso");
  const [showChangeEmail, setShowChangeEmail] = useState(false);
  const [changeEmailValue, setChangeEmailValue] = useState("");

  const [error, setError] = useState("");

  // Load list data when returning to the list view (tab changes or detail → list)
  useEffect(() => {
    if (!resourceId) {
      loadTabData(activeTab);
    }
  }, [activeTab, resourceId]);

  // Auto-clear invite success message after 5 seconds
  useEffect(() => {
    if (!inviteSuccess) return;
    const timer = setTimeout(() => setInviteSuccess(""), 5000);
    return () => clearTimeout(timer);
  }, [inviteSuccess]);

  // Load detail data when resourceId changes
  useEffect(() => {
    if (activeTab === "tenants" && resourceId) {
      loadTenantDetail(resourceId);
    } else {
      setTenantDetail(null);
      setSSOConfig(null);
      setShowSSOForm(false);
      setShowInvite(false);
      setInviteSuccess("");
      setAIUsage(null);
      setAIUsageSince("");
    }
    if (activeTab === "identities" && resourceId) {
      loadIdentityDetail(resourceId);
    } else {
      setIdentityDetail(null);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeTab, resourceId]);

  async function loadTabData(tab: Tab) {
    setLoading(true);
    setError("");
    try {
      switch (tab) {
        case "sessions": {
          const res = await api<{ sessions: AdminSession[] }>(
            "/api/admin/sessions",
          );
          setSessions(res.sessions);
          break;
        }
        case "audit": {
          const res = await api<{ entries: AuditEntry[] }>(
            "/api/admin/audit",
          );
          setAuditEntries(res.entries ?? []);
          break;
        }
        case "tenants": {
          const res = await getAdminTenants();
          setTenants(res.tenants);
          break;
        }
        case "identities": {
          const res = await getAdminIdentities();
          setIdentities(res.identities);
          break;
        }
        case "security": {
          const res = await listPasskeys(ADMIN_API);
          setMyPasskeys(res.passkeys ?? []);
          break;
        }
      }
    } catch {
      // Error loading data
    } finally {
      setLoading(false);
    }
  }

  async function loadTenantDetail(id: string) {
    try {
      const res = await getTenant(id);
      setTenantDetail(res.tenant);
      setEditingTenantName(res.tenant.name);
      setIsEditingTenant(false);
      setShowInvite(false);
      loadSSOConfig(id);
      void getTenantAIUsage(id)
        .then(r => {
          setAIUsage(r.features);
          setAIUsageSince(r.since);
        })
        .catch(() => {
          setAIUsage([]);
          setAIUsageSince("");
        });
    } catch {
      setError("Failed to load tenant");
    }
  }

  async function loadSSOConfig(tenantId: string) {
    setSSOLoading(true);
    try {
      const cfg = await getSSOConfig(tenantId);
      setSSOConfig(cfg);
      setSSOForm({
        protocol: cfg.protocol,
        entity_id: cfg.entity_id,
        sso_url: cfg.sso_url,
        certificate: cfg.certificate,
        client_id: cfg.client_id,
        issuer_url: cfg.issuer_url,
        enabled: cfg.enabled,
        auto_provision: cfg.auto_provision,
        default_role: cfg.default_role,
      });
    } catch {
      setSSOConfig(null);
      setSSOForm({
        protocol: "oidc",
        enabled: false,
        auto_provision: false,
        default_role: "individual_contributor",
      });
    } finally {
      setSSOLoading(false);
    }
  }

  async function loadIdentityDetail(id: string) {
    try {
      const [identRes, passkeysRes] = await Promise.all([
        getIdentity(id),
        listIdentityPasskeys(id),
      ]);
      setIdentityDetail(identRes.identity);
      setIdentityPasskeys(passkeysRes.passkeys ?? []);
      setShowResetPw(false);
      setShowAddToTenant(false);
      setResetPw("");
      if (tenants.length === 0) {
        const tenantRes = await getAdminTenants();
        setTenants(tenantRes.tenants);
      }
    } catch {
      setError("Failed to load identity");
    }
  }

  async function handleDeletePasskey(passkeyId: string) {
    if (!identityDetail) return;
    if (!confirm("Remove this passkey? The user will need to register a new one.")) return;
    try {
      await deleteIdentityPasskey(identityDetail.id, passkeyId);
      setIdentityPasskeys((prev) => prev.filter((p) => p.id !== passkeyId));
    } catch {
      setError("Failed to remove passkey");
    }
  }

  async function handleAddMyPasskey(e: React.FormEvent) {
    e.preventDefault();
    setMyPasskeyError("");
    setMyPasskeyAdding(true);
    try {
      await registerPasskey(myPasskeyName || "My Passkey", ADMIN_API);
      setMyPasskeyName("");
      const res = await listPasskeys(ADMIN_API);
      setMyPasskeys(res.passkeys ?? []);
    } catch (err) {
      setMyPasskeyError(
        err instanceof ApiError ? err.message : "Failed to register passkey",
      );
    } finally {
      setMyPasskeyAdding(false);
    }
  }

  async function handleRemoveMyPasskey(id: string) {
    if (!confirm("Remove this passkey?")) return;
    setMyPasskeyError("");
    try {
      await removePasskey(id, ADMIN_API);
      setMyPasskeys((prev) => prev.filter((p) => p.id !== id));
    } catch (err) {
      setMyPasskeyError(
        err instanceof ApiError ? err.message : "Failed to remove passkey",
      );
    }
  }

  async function handleRevokeSession(sessionId: string) {
    try {
      await api(`/api/admin/sessions/${sessionId}`, { method: "DELETE" });
      setSessions((prev) => prev.filter((s) => s.id !== sessionId));
    } catch {
      // Error revoking
    }
  }

  async function handleCreateTenant(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      await createTenant(newTenantName);
      setNewTenantName("");
      setShowNewTenant(false);
      loadTabData("tenants");
    } catch {
      setError("Failed to create tenant");
    }
  }

  async function handleUpdateTenant() {
    if (!tenantDetail) return;
    setError("");
    try {
      await updateTenant(tenantDetail.id, editingTenantName);
      setIsEditingTenant(false);
      loadTenantDetail(tenantDetail.id);
      loadTabData("tenants");
    } catch {
      setError("Failed to update tenant");
    }
  }

  async function handleDeleteTenant() {
    if (!tenantDetail) return;
    if (
      !confirm(
        `Delete tenant "${tenantDetail.name}"? This cannot be undone from the UI.`,
      )
    )
      return;
    setError("");
    try {
      await deleteTenant(tenantDetail.id);
      navigate("/admin/tenants");
      loadTabData("tenants");
    } catch {
      setError("Failed to delete tenant");
    }
  }

  async function handleSuspendTenant() {
    if (!tenantDetail) return;
    if (
      !confirm(
        `Suspend tenant "${tenantDetail.name}"? All active sessions will be revoked.`,
      )
    )
      return;
    setError("");
    try {
      await suspendTenant(tenantDetail.id);
      loadTenantDetail(tenantDetail.id);
      loadTabData("tenants");
    } catch {
      setError("Failed to suspend tenant");
    }
  }

  async function handleUnsuspendTenant() {
    if (!tenantDetail) return;
    setError("");
    try {
      await unsuspendTenant(tenantDetail.id);
      loadTenantDetail(tenantDetail.id);
      loadTabData("tenants");
    } catch {
      setError("Failed to unsuspend tenant");
    }
  }

  async function handleSaveSSOConfig(e: React.FormEvent) {
    e.preventDefault();
    if (!tenantDetail) return;
    setError("");
    try {
      const cfg = await updateSSOConfig(tenantDetail.id, ssoForm);
      setSSOConfig(cfg);
      setShowSSOForm(false);
    } catch {
      setError("Failed to save SSO configuration");
    }
  }

  async function handleInviteAdmin(e: React.FormEvent) {
    e.preventDefault();
    if (!tenantDetail) return;
    setError("");
    setInviteSuccess("");
    try {
      const res = await inviteAdmin(tenantDetail.id, inviteEmail, inviteName);
      setInviteEmail("");
      setInviteName("");
      setShowInvite(false);
      setInviteSuccess(
        res.identity_created
          ? `New identity created and added as administrator. Invitation sent to ${inviteEmail}.`
          : `Existing identity added as administrator. Invitation sent to ${inviteEmail}.`,
      );
      loadTenantDetail(tenantDetail.id);
    } catch {
      setError("Failed to invite admin (may already be in this tenant)");
    }
  }

  async function handleCreateIdentity(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      await createIdentity(
        newIdentityEmail,
        newIdentityName,
        newIdentityPassword,
      );
      setNewIdentityEmail("");
      setNewIdentityName("");
      setNewIdentityPassword("");
      setShowNewIdentity(false);
      loadTabData("identities");
    } catch {
      setError("Failed to create identity");
    }
  }

  async function handleResetPassword(e: React.FormEvent) {
    e.preventDefault();
    if (!identityDetail) return;
    setError("");
    try {
      await resetPassword(identityDetail.id, resetPw);
      setResetPw("");
      setShowResetPw(false);
    } catch {
      setError("Failed to reset password");
    }
  }

  async function handleAddToTenant(e: React.FormEvent) {
    e.preventDefault();
    if (!identityDetail) return;
    setError("");
    try {
      await addIdentityToTenant(
        identityDetail.id,
        addTenantId,
        addTenantRole,
      );
      setShowAddToTenant(false);
      setAddTenantId("");
      setAddTenantRole("read_only_fso");
      loadIdentityDetail(identityDetail.id);
    } catch {
      setError("Failed to add identity to tenant");
    }
  }

  async function handleSuspendIdentity() {
    if (!identityDetail) return;
    if (
      !confirm(
        `Suspend identity "${identityDetail.email}"? All active sessions will be revoked.`,
      )
    )
      return;
    setError("");
    try {
      await suspendIdentity(identityDetail.id);
      loadIdentityDetail(identityDetail.id);
      loadTabData("identities");
    } catch {
      setError("Failed to suspend identity");
    }
  }

  async function handleUnsuspendIdentity() {
    if (!identityDetail) return;
    setError("");
    try {
      await unsuspendIdentity(identityDetail.id);
      loadIdentityDetail(identityDetail.id);
      loadTabData("identities");
    } catch {
      setError("Failed to unsuspend identity");
    }
  }

  async function handleDeleteIdentity() {
    if (!identityDetail) return;
    if (
      !confirm(
        `Delete identity "${identityDetail.email}"? This cannot be undone.`,
      )
    )
      return;
    setError("");
    try {
      await deleteIdentity(identityDetail.id);
      navigate("/admin/identities");
      loadTabData("identities");
    } catch {
      setError("Failed to delete identity");
    }
  }

  async function handleResendInvite() {
    if (!identityDetail) return;
    setError("");
    try {
      await resendInvite(identityDetail.id);
      setError("");
    } catch {
      setError("Failed to resend invite");
    }
  }

  async function handleChangeEmail(e: React.FormEvent) {
    e.preventDefault();
    if (!identityDetail) return;
    setError("");
    try {
      await changeIdentityEmail(identityDetail.id, changeEmailValue);
      setShowChangeEmail(false);
      setChangeEmailValue("");
      loadIdentityDetail(identityDetail.id);
      loadTabData("identities");
    } catch {
      setError("Failed to change email");
    }
  }

  if (!isSuperAdmin) {
    return <Navigate to="/app/dashboard" replace />;
  }

  return (
    <DashboardLayout title="fastFSO Admin">
      <div className="admin-tabs">
        {TABS.map((tab) => (
          <button
            key={tab}
            className={`admin-tab ${activeTab === tab ? "active" : ""}`}
            onClick={() => navigate(`/admin/${tab}`)}
          >
            {TAB_LABELS[tab]}
          </button>
        ))}
      </div>

      {error && (
        <Alert variant="error" onDismiss={() => setError("")}>
          {error}
        </Alert>
      )}

      {loading && <Spinner text="Loading..." />}

      {/* Sessions Tab */}
      {!loading && activeTab === "sessions" && (
        <DataTable<AdminSession>
          columns={[
            { header: "User", render: (s) => s.name },
            { header: "Email", render: (s) => s.email },
            { header: "State", render: (s) => s.state },
            { header: "IP", render: (s) => s.ip_address },
            { header: "Auth", render: (s) => s.auth_method },
            {
              header: "Last Active",
              render: (s) => new Date(s.last_active_at).toLocaleString(),
            },
          ]}
          data={sessions}
          rowKey={(s) => s.id}
          actions={(s) => (
            <Button size="sm" onClick={() => handleRevokeSession(s.id)}>
              Revoke
            </Button>
          )}
          emptyMessage="No active sessions"
        />
      )}

      {/* Audit Tab */}
      {!loading && activeTab === "audit" && (
        <DataTable<AuditEntry>
          columns={[
            { header: "Action", render: (e) => e.action },
            { header: "IP", render: (e) => e.ip_address },
            {
              header: "Time",
              render: (e) => new Date(e.created_at).toLocaleString(),
            },
            {
              header: "Details",
              render: (e) => <code>{JSON.stringify(e.metadata)}</code>,
            },
          ]}
          data={auditEntries}
          rowKey={(e) => e.id}
          emptyMessage="No audit entries"
        />
      )}

      {/* Tenants Tab - List */}
      {!loading && activeTab === "tenants" && !resourceId && (
        <div>
          <div className="admin-toolbar">
            <Button
              size="sm"
              onClick={() => setShowNewTenant(!showNewTenant)}
            >
              {showNewTenant ? "Cancel" : "New Tenant"}
            </Button>
          </div>
          {showNewTenant && (
            <form
              onSubmit={handleCreateTenant}
              className="admin-inline-form"
            >
              <input
                type="text"
                placeholder="Tenant name"
                value={newTenantName}
                onChange={(e) => setNewTenantName(e.target.value)}
                required
              />
              <Button type="submit" size="sm">
                Create
              </Button>
            </form>
          )}
          <DataTable<AdminTenant>
            columns={[
              { header: "Name", render: (t) => t.name },
              {
                header: "Status",
                render: (t) =>
                  t.suspended_at ? (
                    <span className="status-suspended">Suspended</span>
                  ) : (
                    "Active"
                  ),
              },
              { header: "Users", render: (t) => t.user_count },
              {
                header: "Created",
                render: (t) => new Date(t.created_at).toLocaleString(),
              },
            ]}
            data={tenants}
            rowKey={(t) => t.id}
            onRowClick={(t) => navigate(`/admin/tenants/${t.id}`)}
            emptyMessage="No tenants"
          />
        </div>
      )}

      {/* Tenant Detail */}
      {!loading && activeTab === "tenants" && resourceId && tenantDetail && (
        <div>
          <Button
            size="sm"
            onClick={() => navigate("/admin/tenants")}
          >
            Back to tenants
          </Button>
          <div className="admin-detail-header">
            {isEditingTenant ? (
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  handleUpdateTenant();
                }}
                className="admin-inline-form"
              >
                <input
                  type="text"
                  value={editingTenantName}
                  onChange={(e) => setEditingTenantName(e.target.value)}
                  required
                />
                <Button type="submit" size="sm">
                  Save
                </Button>
                <Button
                  type="button"
                  size="sm"
                  onClick={() => setIsEditingTenant(false)}
                >
                  Cancel
                </Button>
              </form>
            ) : (
              <>
                <h2>
                  {tenantDetail.name}
                  {tenantDetail.suspended_at && (
                    <span className="status-suspended"> (Suspended)</span>
                  )}
                </h2>
                <Button
                  size="sm"
                  onClick={() => setIsEditingTenant(true)}
                >
                  Rename
                </Button>
                {tenantDetail.suspended_at ? (
                  <Button
                    size="sm"
                    onClick={handleUnsuspendTenant}
                  >
                    Unsuspend
                  </Button>
                ) : (
                  <Button
                    size="sm"
                    className="btn-danger"
                    onClick={handleSuspendTenant}
                  >
                    Suspend
                  </Button>
                )}
                <Button
                  size="sm"
                  className="btn-danger"
                  onClick={handleDeleteTenant}
                >
                  Delete
                </Button>
              </>
            )}
          </div>

          {/* Invite Admin */}
          <div className="admin-toolbar">
            <Button
              size="sm"
              onClick={() => {
                setShowInvite(!showInvite);
                setInviteSuccess("");
              }}
            >
              {showInvite ? "Cancel" : "Invite Admin"}
            </Button>
          </div>
          {inviteSuccess && (
            <Alert variant="success">{inviteSuccess}</Alert>
          )}
          {showInvite && (
            <form
              onSubmit={handleInviteAdmin}
              className="admin-inline-form"
            >
              <input
                type="email"
                placeholder="Email (required)"
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                required
              />
              <input
                type="text"
                placeholder="Name (optional)"
                value={inviteName}
                onChange={(e) => setInviteName(e.target.value)}
              />
              <Button type="submit" size="sm">
                Invite
              </Button>
            </form>
          )}

          {/* AI Usage (current UTC month) */}
          <h3>AI Usage</h3>
          {aiUsage === null && <Spinner text="Loading AI usage..." />}
          {aiUsage !== null && aiUsage.length === 0 && (
            <p style={{ color: "var(--color-text-muted)", fontSize: 13 }}>
              No AI activity recorded for this tenant
              {aiUsageSince && ` since ${new Date(aiUsageSince).toLocaleDateString()}`}.
            </p>
          )}
          {aiUsage !== null && aiUsage.length > 0 && (
            <>
              <p style={{ color: "var(--color-text-muted)", fontSize: 12, marginBottom: 8 }}>
                Since {new Date(aiUsageSince).toLocaleDateString()} (current UTC month)
              </p>
              <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
                <thead>
                  <tr style={{ textAlign: "left", borderBottom: "1px solid var(--color-border)" }}>
                    <th style={{ padding: "6px 8px" }}>Feature</th>
                    <th style={{ padding: "6px 8px", textAlign: "right" }}>Calls</th>
                    <th style={{ padding: "6px 8px", textAlign: "right" }}>Prompt tokens</th>
                    <th style={{ padding: "6px 8px", textAlign: "right" }}>Response tokens</th>
                    <th style={{ padding: "6px 8px", textAlign: "right" }}>Total tokens</th>
                  </tr>
                </thead>
                <tbody>
                  {aiUsage.map(f => (
                    <tr key={f.feature} style={{ borderBottom: "1px solid var(--color-border)" }}>
                      <td style={{ padding: "6px 8px" }}>{f.feature}</td>
                      <td style={{ padding: "6px 8px", textAlign: "right" }}>{f.calls.toLocaleString()}</td>
                      <td style={{ padding: "6px 8px", textAlign: "right" }}>{f.prompt_tokens.toLocaleString()}</td>
                      <td style={{ padding: "6px 8px", textAlign: "right" }}>{f.response_tokens.toLocaleString()}</td>
                      <td style={{ padding: "6px 8px", textAlign: "right" }}>
                        {(f.prompt_tokens + f.response_tokens).toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </>
          )}

          {/* SSO Configuration */}
          <h3>SSO Configuration</h3>
          {ssoLoading && <Spinner text="Loading SSO config..." />}
          {!ssoLoading && !showSSOForm && ssoConfig && (
            <div>
              <p>
                Protocol: <strong>{ssoConfig.protocol.toUpperCase()}</strong>
                {" | "}
                {ssoConfig.enabled ? "Enabled" : "Disabled"}
                {" | "}
                Auto-provision: {ssoConfig.auto_provision ? "Yes" : "No"}
                {" | "}
                Default role: {ssoConfig.default_role}
              </p>
              <Button
                size="sm"
                onClick={() => setShowSSOForm(true)}
              >
                Edit SSO
              </Button>
            </div>
          )}
          {!ssoLoading && !showSSOForm && !ssoConfig && (
            <div>
              <p>No SSO configured for this tenant.</p>
              <Button
                size="sm"
                onClick={() => setShowSSOForm(true)}
              >
                Configure SSO
              </Button>
            </div>
          )}
          {showSSOForm && (
            <form onSubmit={handleSaveSSOConfig} className="admin-sso-form">
              <label>
                Protocol
                <select
                  value={ssoForm.protocol}
                  onChange={(e) =>
                    setSSOForm({ ...ssoForm, protocol: e.target.value })
                  }
                >
                  <option value="oidc">OIDC</option>
                  <option value="saml">SAML</option>
                </select>
              </label>

              {ssoForm.protocol === "oidc" && (
                <>
                  <label>
                    Client ID
                    <input
                      type="text"
                      value={ssoForm.client_id ?? ""}
                      onChange={(e) =>
                        setSSOForm({ ...ssoForm, client_id: e.target.value })
                      }
                    />
                  </label>
                  <label>
                    Client Secret
                    <input
                      type="password"
                      value={ssoForm.client_secret ?? ""}
                      onChange={(e) =>
                        setSSOForm({
                          ...ssoForm,
                          client_secret: e.target.value,
                        })
                      }
                      placeholder="Enter to change"
                    />
                  </label>
                  <label>
                    Issuer URL
                    <input
                      type="text"
                      value={ssoForm.issuer_url ?? ""}
                      onChange={(e) =>
                        setSSOForm({ ...ssoForm, issuer_url: e.target.value })
                      }
                    />
                  </label>
                </>
              )}

              {ssoForm.protocol === "saml" && (
                <>
                  <label>
                    Entity ID
                    <input
                      type="text"
                      value={ssoForm.entity_id ?? ""}
                      onChange={(e) =>
                        setSSOForm({ ...ssoForm, entity_id: e.target.value })
                      }
                    />
                  </label>
                  <label>
                    SSO URL
                    <input
                      type="text"
                      value={ssoForm.sso_url ?? ""}
                      onChange={(e) =>
                        setSSOForm({ ...ssoForm, sso_url: e.target.value })
                      }
                    />
                  </label>
                  <label>
                    Certificate
                    <textarea
                      value={ssoForm.certificate ?? ""}
                      onChange={(e) =>
                        setSSOForm({
                          ...ssoForm,
                          certificate: e.target.value,
                        })
                      }
                      rows={4}
                    />
                  </label>
                </>
              )}

              <label>
                <input
                  type="checkbox"
                  checked={ssoForm.enabled}
                  onChange={(e) =>
                    setSSOForm({ ...ssoForm, enabled: e.target.checked })
                  }
                />{" "}
                Enabled
              </label>
              <label>
                <input
                  type="checkbox"
                  checked={ssoForm.auto_provision}
                  onChange={(e) =>
                    setSSOForm({
                      ...ssoForm,
                      auto_provision: e.target.checked,
                    })
                  }
                />{" "}
                Auto-provision users
              </label>
              <label>
                Default Role
                <select
                  value={ssoForm.default_role}
                  onChange={(e) =>
                    setSSOForm({ ...ssoForm, default_role: e.target.value })
                  }
                >
                  <option value="administrator">Administrator</option>
                  <option value="fso">FSO</option>
                  <option value="read_only_fso">Read-Only FSO</option>
                  <option value="individual_contributor">
                    Individual Contributor
                  </option>
                </select>
              </label>

              <div className="admin-toolbar">
                <Button type="submit" size="sm">
                  Save
                </Button>
                <Button
                  type="button"
                  size="sm"
                  onClick={() => setShowSSOForm(false)}
                >
                  Cancel
                </Button>
              </div>
            </form>
          )}

          <h3>Users ({tenantDetail.users.length})</h3>
          <DataTable
            columns={[
              { header: "Name", render: (u: AdminTenantDetail["users"][0]) => u.name },
              { header: "Email", render: (u: AdminTenantDetail["users"][0]) => u.email },
              { header: "Role", render: (u: AdminTenantDetail["users"][0]) => u.role },
            ]}
            data={tenantDetail.users}
            rowKey={(u: AdminTenantDetail["users"][0]) => u.user_id}
            emptyMessage="No users in this tenant"
          />
        </div>
      )}

      {/* Identities Tab - List */}
      {!loading && activeTab === "identities" && !resourceId && (
        <div>
          <div className="admin-toolbar">
            <Button
              size="sm"
              onClick={() => setShowNewIdentity(!showNewIdentity)}
            >
              {showNewIdentity ? "Cancel" : "New Identity"}
            </Button>
          </div>
          {showNewIdentity && (
            <form
              onSubmit={handleCreateIdentity}
              className="admin-inline-form"
            >
              <input
                type="email"
                placeholder="Email"
                value={newIdentityEmail}
                onChange={(e) => setNewIdentityEmail(e.target.value)}
                required
              />
              <input
                type="text"
                placeholder="Name"
                value={newIdentityName}
                onChange={(e) => setNewIdentityName(e.target.value)}
                required
              />
              <input
                type="password"
                placeholder="Password (min 8)"
                value={newIdentityPassword}
                onChange={(e) => setNewIdentityPassword(e.target.value)}
                required
                minLength={8}
              />
              <Button type="submit" size="sm">
                Create
              </Button>
            </form>
          )}
          <DataTable<AdminIdentity>
            columns={[
              { header: "Email", render: (i) => i.email },
              { header: "Name", render: (i) => i.name },
              {
                header: "Status",
                render: (i) =>
                  i.suspended_at ? (
                    <span className="status-suspended">Suspended</span>
                  ) : (
                    "Active"
                  ),
              },
              {
                header: "Super Admin",
                render: (i) => (i.is_super_admin ? "Yes" : ""),
              },
              {
                header: "Activated",
                render: (i) => (i.activated ? "Yes" : ""),
              },
              { header: "Tenants", render: (i) => i.tenant_count },
              {
                header: "Created",
                render: (i) => new Date(i.created_at).toLocaleString(),
              },
            ]}
            data={identities}
            rowKey={(i) => i.id}
            onRowClick={(i) => navigate(`/admin/identities/${i.id}`)}
            emptyMessage="No identities"
          />
        </div>
      )}

      {/* Identity Detail */}
      {!loading &&
        activeTab === "identities" &&
        resourceId &&
        identityDetail && (
          <div>
            <Button
              size="sm"
              onClick={() => navigate("/admin/identities")}
            >
              Back to identities
            </Button>
            <div className="admin-detail-header">
              <h2>
                {identityDetail.name}
                {identityDetail.suspended_at && (
                  <span className="status-suspended"> (Suspended)</span>
                )}
              </h2>
              <span className="admin-detail-email">
                {identityDetail.email}
              </span>
              {identityDetail.is_super_admin && (
                <span className="user-role">Super Admin</span>
              )}
            </div>

            <div className="admin-toolbar">
              <Button
                size="sm"
                onClick={() => {
                  setShowResetPw(!showResetPw);
                  setShowAddToTenant(false);
                }}
              >
                {showResetPw ? "Cancel" : "Reset Password"}
              </Button>
              <Button
                size="sm"
                onClick={() => {
                  setShowAddToTenant(!showAddToTenant);
                  setShowResetPw(false);
                }}
              >
                {showAddToTenant ? "Cancel" : "Add to Tenant"}
              </Button>
              {identityDetail.suspended_at ? (
                <Button
                  size="sm"
                  onClick={handleUnsuspendIdentity}
                >
                  Unsuspend
                </Button>
              ) : (
                <Button
                  size="sm"
                  className="btn-danger"
                  onClick={handleSuspendIdentity}
                >
                  Suspend
                </Button>
              )}
              {identityDetail.is_super_admin ? (
                <Button
                  size="sm"
                  className="btn-danger"
                  onClick={async () => {
                    if (!confirm("Revoke super admin status from this identity?")) return;
                    try {
                      await setSuperAdmin(identityDetail.id, false);
                      loadIdentityDetail(identityDetail.id);
                    } catch {
                      setError("Failed to revoke super admin");
                    }
                  }}
                >
                  Revoke Super Admin
                </Button>
              ) : (
                <Button
                  size="sm"
                  onClick={async () => {
                    if (!confirm("Grant super admin status to this identity?")) return;
                    try {
                      await setSuperAdmin(identityDetail.id, true);
                      loadIdentityDetail(identityDetail.id);
                    } catch {
                      setError("Failed to grant super admin");
                    }
                  }}
                >
                  Grant Super Admin
                </Button>
              )}
              <Button
                size="sm"
                className="btn-danger"
                onClick={handleDeleteIdentity}
              >
                Delete
              </Button>
            </div>

            {!identityDetail.activated && (
              <div className="admin-toolbar">
                <Button size="sm" onClick={handleResendInvite}>
                  Resend Invite
                </Button>
                <Button
                  size="sm"
                  onClick={() => {
                    setShowChangeEmail(!showChangeEmail);
                    setChangeEmailValue(identityDetail.email);
                  }}
                >
                  {showChangeEmail ? "Cancel" : "Change Email"}
                </Button>
              </div>
            )}
            {showChangeEmail && !identityDetail.activated && (
              <form
                onSubmit={handleChangeEmail}
                className="admin-inline-form"
              >
                <input
                  type="email"
                  placeholder="New email"
                  value={changeEmailValue}
                  onChange={(e) => setChangeEmailValue(e.target.value)}
                  required
                />
                <Button type="submit" size="sm">
                  Update Email
                </Button>
              </form>
            )}

            {showResetPw && (
              <form
                onSubmit={handleResetPassword}
                className="admin-inline-form"
              >
                <input
                  type="password"
                  placeholder="New password (min 8)"
                  value={resetPw}
                  onChange={(e) => setResetPw(e.target.value)}
                  required
                  minLength={8}
                />
                <Button type="submit" size="sm">
                  Reset
                </Button>
              </form>
            )}

            {showAddToTenant && (
              <form
                onSubmit={handleAddToTenant}
                className="admin-inline-form"
              >
                <select
                  value={addTenantId}
                  onChange={(e) => setAddTenantId(e.target.value)}
                  required
                >
                  <option value="">Select tenant...</option>
                  {tenants.map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.name}
                    </option>
                  ))}
                </select>
                <select
                  value={addTenantRole}
                  onChange={(e) => setAddTenantRole(e.target.value)}
                >
                  <option value="administrator">Administrator</option>
                  <option value="fso">FSO</option>
                  <option value="read_only_fso">Read-Only FSO</option>
                  <option value="individual_contributor">
                    Individual Contributor
                  </option>
                </select>
                <Button type="submit" size="sm">
                  Add
                </Button>
              </form>
            )}

            <h3>
              Tenant Memberships ({identityDetail.tenants.length})
            </h3>
            <DataTable
              columns={[
                { header: "Tenant", render: (t: AdminIdentityDetail["tenants"][0]) => t.Name },
                { header: "Role", render: (t: AdminIdentityDetail["tenants"][0]) => t.Role },
              ]}
              data={identityDetail.tenants}
              rowKey={(t: AdminIdentityDetail["tenants"][0]) => t.TenantID}
              emptyMessage="No tenant memberships"
            />

            <h3>Passkeys ({identityPasskeys.length})</h3>
            <DataTable<AdminPasskey>
              columns={[
                {
                  header: "Name",
                  render: (p) => p.friendly_name ?? <em>Unnamed</em>,
                },
                {
                  header: "Last Used",
                  render: (p) =>
                    p.last_used_at
                      ? new Date(p.last_used_at).toLocaleString()
                      : "Never",
                },
                {
                  header: "Registered",
                  render: (p) => new Date(p.created_at).toLocaleString(),
                },
              ]}
              data={identityPasskeys}
              rowKey={(p) => p.id}
              actions={(p) => (
                <Button
                  size="sm"
                  className="btn-danger"
                  onClick={() => handleDeletePasskey(p.id)}
                >
                  Remove
                </Button>
              )}
              emptyMessage="No passkeys registered"
            />
          </div>
        )}

      {!loading && activeTab === "security" && (
        <div className="admin-section">
          <h2>My Passkeys</h2>
          <p style={{ color: "var(--color-text-muted)", fontSize: 13 }}>
            Passkeys let you sign in to this super-admin account without a password.
          </p>
          {myPasskeyError && <Alert variant="error">{myPasskeyError}</Alert>}
          <DataTable<Passkey>
            columns={[
              {
                header: "Name",
                render: (p) => p.friendly_name || <em>Unnamed</em>,
              },
              {
                header: "Last Used",
                render: (p) =>
                  p.last_used_at
                    ? new Date(p.last_used_at).toLocaleString()
                    : "Never",
              },
              {
                header: "Registered",
                render: (p) => new Date(p.created_at).toLocaleString(),
              },
            ]}
            data={myPasskeys}
            rowKey={(p) => p.id}
            actions={(p) => (
              <Button
                size="sm"
                className="btn-danger"
                onClick={() => handleRemoveMyPasskey(p.id)}
              >
                Remove
              </Button>
            )}
            emptyMessage="No passkeys registered"
          />
          <form onSubmit={handleAddMyPasskey} className="admin-inline-form" style={{ marginTop: 16 }}>
            <input
              type="text"
              placeholder="Passkey name (optional)"
              value={myPasskeyName}
              onChange={(e) => setMyPasskeyName(e.target.value)}
              disabled={myPasskeyAdding}
            />
            <Button type="submit" size="sm" loading={myPasskeyAdding} loadingText="Registering...">
              Add passkey
            </Button>
          </form>
        </div>
      )}
    </DashboardLayout>
  );
}

export default AdminPanelPage;
