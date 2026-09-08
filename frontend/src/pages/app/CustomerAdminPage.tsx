import { useState, useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { UserPlus, Trash2, Pencil, Plus, AlertTriangle } from "lucide-react";
import { PageHeader } from "../../components/ui/PageHeader";
import { FilterBar } from "../../components/ui/FilterBar";
import { Button } from "../../components/ui/Button";
import { Modal } from "../../components/ui/Modal";
import { FormField } from "../../components/ui/FormField";
import { Alert } from "../../components/ui/Alert";
import {
  listMembers,
  listSubOrgs,
  inviteMember,
  bulkInviteMembers,
  updateMemberRole,
  setMemberSubOrg,
  removeMember,
  createSubOrg,
  updateSubOrg,
  deleteSubOrg,
  setPrimaryFSO,
  getTenantSSOConfig,
  updateTenantSSOConfig,
  listSSOEmailDomains,
  addSSOEmailDomain,
  removeSSOEmailDomain,
  ROLE_LABELS,
  type Member,
  type MemberRole,
  type SubOrg,
  type BulkInviteRow,
  type BulkInviteResult,
  type TenantSSOConfigUpdate,
  type SSOEmailDomain,
} from "../../api/customerAdmin";
import { ApiError } from "../../api/client";

const ALL_ROLES: MemberRole[] = [
  "administrator",
  "fso",
  "read_only_fso",
  "individual_contributor",
];

// --- Invite Modal ---

function InviteModal({ open, onClose, onInvited, subOrgs }: { open: boolean; onClose: () => void; onInvited: () => void; subOrgs: SubOrg[] }) {
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [role, setRole] = useState<MemberRole>("individual_contributor");
  const [subOrgId, setSubOrgId] = useState<string>("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  // "Default" is hidden from the picker (matches the existing member dropdown
  // logic) — admins choose a real sub-org or leave blank to land in Default
  // via the existing trigger.
  const realSubOrgs = subOrgs.filter(o => o.name !== "Default");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setSubmitting(true);
    try {
      await inviteMember(email, name, role, subOrgId || undefined);
      setEmail(""); setName(""); setRole("individual_contributor"); setSubOrgId("");
      onInvited();
      onClose();
    } catch {
      setError("Failed to send invite. The user may already be a member.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Invite Member">
      {error && <Alert variant="error" onDismiss={() => setError("")}>{error}</Alert>}
      <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        <FormField label="Email" htmlFor="invite-email">
          <input id="invite-email" type="email" value={email} onChange={e => setEmail(e.target.value)} required autoFocus />
        </FormField>
        <FormField label="Name (optional)" htmlFor="invite-name">
          <input id="invite-name" type="text" value={name} onChange={e => setName(e.target.value)} placeholder="Derived from email if blank" />
        </FormField>
        <FormField label="Role" htmlFor="invite-role">
          <select id="invite-role" value={role} onChange={e => setRole(e.target.value as MemberRole)}>
            {ALL_ROLES.map(r => <option key={r} value={r}>{ROLE_LABELS[r]}</option>)}
          </select>
        </FormField>
        {realSubOrgs.length > 0 && (
          <FormField label="Sub-organization (optional)" htmlFor="invite-suborg">
            <select id="invite-suborg" value={subOrgId} onChange={e => setSubOrgId(e.target.value)}>
              <option value="">Default</option>
              {realSubOrgs.map(o => <option key={o.id} value={o.id}>{o.name}</option>)}
            </select>
          </FormField>
        )}
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Button type="button" variant="secondary" onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="primary" loading={submitting} loadingText="Sending...">Send Invite</Button>
        </div>
      </form>
    </Modal>
  );
}

// --- Bulk Invite Modal ---

interface ParsedRow {
  email: string;
  name: string;
  role: MemberRole;
  subOrgName: string; // empty = Default
  error?: string;
}

// parseCSV reads pasted rows in the form `email, name, role, sub-org name`.
// Name, role, and sub-org are all optional — when name is absent the backend
// derives one from the email local-part, but admins can override here for
// IdP-asserted accounts where the email local-part isn't a usable display
// name (e.g. "ext-vendor-1234@acme.com"). Header rows are tolerated. Returns
// per-row errors so the admin can fix bad lines without losing the whole
// paste.
function parseCSV(input: string, validRoles: Set<string>, knownSubOrgs: Set<string>): ParsedRow[] {
  const rows: ParsedRow[] = [];
  const lines = input.split(/\r?\n/).map(l => l.trim()).filter(Boolean);
  for (const line of lines) {
    const cols = line.split(",").map(c => c.trim());
    // Skip likely header.
    if (cols[0]?.toLowerCase() === "email") continue;
    const [email = "", name = "", role = "individual_contributor", subOrgName = ""] = cols;
    const row: ParsedRow = { email, name, role: role as MemberRole, subOrgName };
    if (!email || !email.includes("@")) {
      row.error = "invalid email";
    } else if (!validRoles.has(role)) {
      row.error = `unknown role "${role}"`;
    } else if (subOrgName && !knownSubOrgs.has(subOrgName)) {
      row.error = `unknown sub-org "${subOrgName}"`;
    }
    rows.push(row);
  }
  return rows;
}

function BulkInviteModal({ open, onClose, onInvited, subOrgs }: { open: boolean; onClose: () => void; onInvited: () => void; subOrgs: SubOrg[] }) {
  const [csv, setCsv] = useState("");
  const [skipEmail, setSkipEmail] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<BulkInviteResult | null>(null);
  const [error, setError] = useState("");

  const validRoles = new Set<string>(ALL_ROLES);
  const knownSubOrgNames = new Set(subOrgs.map(o => o.name));
  const parsed = parseCSV(csv, validRoles, knownSubOrgNames);
  const validRows = parsed.filter(r => !r.error);
  const invalidRows = parsed.filter(r => r.error);

  // Map sub-org name → id for the API payload. "Default" maps to no
  // sub_org_id (server falls back to the trigger).
  const nameToID = new Map(subOrgs.map(o => [o.name, o.id]));

  async function handleSubmit() {
    setError("");
    setResult(null);
    setSubmitting(true);
    try {
      const invites: BulkInviteRow[] = validRows.map(r => ({
        email: r.email,
        name: r.name || undefined,
        role: r.role,
        sub_org_id: r.subOrgName && r.subOrgName !== "Default" ? nameToID.get(r.subOrgName) : null,
      }));
      const res = await bulkInviteMembers(invites, skipEmail);
      setResult(res);
      if (res.succeeded > 0) onInvited();
    } catch {
      setError("Bulk invite failed. Try again or shrink the batch.");
    } finally {
      setSubmitting(false);
    }
  }

  function reset() {
    setCsv(""); setResult(null); setError(""); setSkipEmail(false);
  }

  return (
    <Modal open={open} onClose={() => { reset(); onClose(); }} title="Bulk Invite">
      {error && <Alert variant="error" onDismiss={() => setError("")}>{error}</Alert>}

      {!result ? (
        <>
          <p style={{ fontSize: 13, color: "var(--color-text-muted)", marginBottom: 12 }}>
            Paste rows or upload a CSV as <code>email, name, role, sub-organization</code>. Name, role, and sub-organization are optional — leave name blank to derive it from the email. Header row is OK.
          </p>
          <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
            <input
              type="file"
              accept=".csv,text/csv,text/plain"
              onChange={e => {
                const file = e.target.files?.[0];
                if (!file) return;
                const reader = new FileReader();
                reader.onload = () => {
                  const text = typeof reader.result === "string" ? reader.result : "";
                  setCsv(text);
                };
                reader.readAsText(file);
                // Reset so the same file can be re-selected after edits.
                e.target.value = "";
              }}
              style={{ fontSize: 13 }}
            />
          </div>
          <textarea
            value={csv}
            onChange={e => setCsv(e.target.value)}
            placeholder={"alice@acme.com, Alice Park, fso, Engineering\nbob@acme.com, , individual_contributor, Sales"}
            style={{ width: "100%", minHeight: 160, padding: "0.6rem 0.75rem", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", fontFamily: "monospace", fontSize: 13, resize: "vertical", background: "var(--color-surface)", color: "var(--color-text)" }}
          />
          {parsed.length > 0 && (
            <div style={{ marginTop: 12, fontSize: 13 }}>
              <div style={{ marginBottom: 6 }}>
                <strong>{validRows.length}</strong> ready, <strong>{invalidRows.length}</strong> invalid
              </div>
              {invalidRows.length > 0 && (
                <ul style={{ margin: 0, paddingLeft: 20, color: "var(--color-danger)" }}>
                  {invalidRows.slice(0, 5).map((r, i) => (
                    <li key={i}>{r.email || "(empty)"} — {r.error}</li>
                  ))}
                  {invalidRows.length > 5 && <li>+ {invalidRows.length - 5} more</li>}
                </ul>
              )}
            </div>
          )}
          <label style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 12, fontSize: 13 }}>
            <input type="checkbox" checked={skipEmail} onChange={e => setSkipEmail(e.target.checked)} />
            Skip emails — silently pre-provision (useful for SSO rollouts)
          </label>
          <div style={{ display: "flex", gap: 8, justifyContent: "flex-end", marginTop: 16 }}>
            <Button type="button" variant="secondary" onClick={() => { reset(); onClose(); }}>Cancel</Button>
            <Button type="button" variant="primary" loading={submitting} loadingText="Inviting..." onClick={handleSubmit} disabled={validRows.length === 0}>
              Invite {validRows.length}
            </Button>
          </div>
        </>
      ) : (
        <>
          <p style={{ fontSize: 14, marginBottom: 12 }}>
            <strong>{result.succeeded}</strong> succeeded, <strong>{result.failed.length}</strong> failed.
          </p>
          {result.failed.length > 0 && (
            <ul style={{ margin: 0, paddingLeft: 20, fontSize: 13, color: "var(--color-danger)", maxHeight: 200, overflowY: "auto" }}>
              {result.failed.map((f, i) => <li key={i}>{f.email} — {f.reason}</li>)}
            </ul>
          )}
          <div style={{ display: "flex", gap: 8, justifyContent: "flex-end", marginTop: 16 }}>
            <Button type="button" variant="primary" onClick={() => { reset(); onClose(); }}>Close</Button>
          </div>
        </>
      )}
    </Modal>
  );
}

// --- Confirm Remove Modal ---

function ConfirmRemoveModal({ member, onConfirm, onCancel }: { member: Member | null; onConfirm: () => void; onCancel: () => void }) {
  return (
    <Modal open={!!member} onClose={onCancel} title="Remove Member">
      <p style={{ marginBottom: 20 }}>
        Remove <strong>{member?.name}</strong> ({member?.email}) from this organization? They will lose access immediately.
      </p>
      <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
        <Button variant="secondary" onClick={onCancel}>Cancel</Button>
        <Button variant="primary" onClick={onConfirm} style={{ background: "var(--color-danger)" }}>Remove</Button>
      </div>
    </Modal>
  );
}

// --- Members Tab ---

function MembersTab({ subOrgs }: { subOrgs: SubOrg[] }) {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [inviteOpen, setInviteOpen] = useState(false);
  const [bulkInviteOpen, setBulkInviteOpen] = useState(false);
  const [memberToRemove, setMemberToRemove] = useState<Member | null>(null);
  const [error, setError] = useState("");

  const { data } = useQuery({
    queryKey: ["members", search],
    queryFn: () => listMembers(search || undefined),
  });
  const members = data?.members ?? [];

  function invalidate() {
    void queryClient.invalidateQueries({ queryKey: ["members"] });
    // Sub-org member counts are derived from membership rows, so any change
    // to the members list (invite, role change, sub-org reassignment,
    // removal) shifts counts in the SubOrgs tab. Without invalidating both,
    // the counts there stay stale until a full reload.
    void queryClient.invalidateQueries({ queryKey: ["sub-orgs"] });
  }

  async function handleRoleChange(userId: string, role: MemberRole) {
    setError("");
    try {
      await updateMemberRole(userId, role);
      invalidate();
    } catch {
      setError("Failed to update role.");
    }
  }

  async function handleSubOrgChange(userId: string, subOrgId: string) {
    setError("");
    try {
      await setMemberSubOrg(userId, subOrgId);
      invalidate();
    } catch {
      setError("Failed to update sub-organization.");
    }
  }

  async function handleRemoveConfirm() {
    if (!memberToRemove) return;
    setError("");
    try {
      await removeMember(memberToRemove.user_id);
      setMemberToRemove(null);
      invalidate();
    } catch {
      setError("Failed to remove member.");
      setMemberToRemove(null);
    }
  }

  // Default is hidden from the dropdown's options because most tenants treat
  // it as the implicit catch-all and listing it next to real sub-orgs is
  // confusing. The carve-out (see optionsForMember) is the same as the wiki
  // editor's: if a member is *currently* in Default, keep Default visible
  // for that one row so the admin can see where they are and move them
  // intentionally rather than silently into the first sub-org alphabetically.
  const nonDefaultSubOrgs = subOrgs.filter(o => o.name !== "Default");
  const showSubOrgColumn = nonDefaultSubOrgs.length > 0;
  function optionsForMember(member: Member): SubOrg[] {
    return subOrgs.filter(o => o.name !== "Default" || o.id === member.sub_org_id);
  }

  return (
    <>
      {error && <Alert variant="error" onDismiss={() => setError("")}>{error}</Alert>}

      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
        <FilterBar>
          <FilterBar.Search value={search} onChange={setSearch} placeholder="Search members..." />
        </FilterBar>
        <div style={{ display: "flex", gap: 8 }}>
          <Button size="sm" variant="secondary" onClick={() => setBulkInviteOpen(true)}>
            Bulk Invite
          </Button>
          <Button size="sm" onClick={() => setInviteOpen(true)}>
            <UserPlus size={16} style={{ marginRight: 6 }} /> Invite Member
          </Button>
        </div>
      </div>

      <div className="task-list">
        {members.map(member => (
          <div key={member.user_id} className="task-row" style={{ alignItems: "center" }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div className="task-row-title" style={{ marginBottom: 2 }}>{member.name}</div>
              <div style={{ fontSize: 12, color: "var(--color-text-muted)" }}>{member.email}</div>
            </div>
            {showSubOrgColumn && member.role !== "administrator" && (
              <select
                value={member.sub_org_id ?? ""}
                onChange={e => handleSubOrgChange(member.user_id, e.target.value)}
                style={{ fontSize: 13, padding: "4px 8px", borderRadius: 6, border: "1px solid var(--color-border)", background: "var(--color-bg-elevated)", color: "var(--color-text)" }}
                title="Sub-organization"
              >
                {optionsForMember(member).map(o => <option key={o.id} value={o.id}>{o.name}</option>)}
              </select>
            )}
            <select
              value={member.role}
              onChange={e => handleRoleChange(member.user_id, e.target.value as MemberRole)}
              style={{ fontSize: 13, padding: "4px 8px", borderRadius: 6, border: "1px solid var(--color-border)", background: "var(--color-bg-elevated)", color: "var(--color-text)" }}
            >
              {ALL_ROLES.map(r => <option key={r} value={r}>{ROLE_LABELS[r]}</option>)}
            </select>
            <button
              className="btn btn-sm btn-danger"
              style={{ marginLeft: 8 }}
              onClick={() => setMemberToRemove(member)}
              title="Remove member"
            >
              <Trash2 size={14} />
            </button>
          </div>
        ))}
        {members.length === 0 && (
          <p style={{ textAlign: "center", color: "var(--color-text-muted)", padding: 24 }}>
            No members found. Invite your first team member to get started.
          </p>
        )}
      </div>

      <InviteModal open={inviteOpen} onClose={() => setInviteOpen(false)} onInvited={invalidate} subOrgs={subOrgs} />
      <BulkInviteModal open={bulkInviteOpen} onClose={() => setBulkInviteOpen(false)} onInvited={invalidate} subOrgs={subOrgs} />
      <ConfirmRemoveModal member={memberToRemove} onConfirm={handleRemoveConfirm} onCancel={() => setMemberToRemove(null)} />
    </>
  );
}

// --- Sub-Orgs Tab ---

function SubOrgsTab() {
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [editOrg, setEditOrg] = useState<SubOrg | null>(null);
  const [orgToDelete, setOrgToDelete] = useState<SubOrg | null>(null);
  const [newName, setNewName] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const { data } = useQuery({
    queryKey: ["sub-orgs"],
    queryFn: listSubOrgs,
  });
  const orgs = data?.sub_orgs ?? [];

  const { data: membersData } = useQuery({
    queryKey: ["members"],
    queryFn: () => listMembers(),
  });
  const allMembers = membersData?.members ?? [];

  // Primary FSO is a responsibility, not a membership — any FSO/admin in the
  // tenant is eligible regardless of which sub-org they currently sit in.
  const fsoCandidates = allMembers.filter(
    m => m.role === "fso" || m.role === "read_only_fso" || m.role === "administrator",
  );

  async function handleFSOChange(orgId: string, userId: string) {
    setError("");
    try {
      await setPrimaryFSO(orgId, userId || null);
      void queryClient.invalidateQueries({ queryKey: ["sub-orgs"] });
    } catch {
      setError("Failed to update primary FSO.");
    }
  }

  function invalidate() {
    void queryClient.invalidateQueries({ queryKey: ["sub-orgs"] });
    void queryClient.invalidateQueries({ queryKey: ["members"] });
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      await createSubOrg(newName.trim());
      setNewName("");
      setCreateOpen(false);
      invalidate();
    } catch {
      setError("Failed to create sub-organization. The name may already be taken.");
    } finally {
      setSaving(false);
    }
  }

  async function handleUpdate(e: React.FormEvent) {
    e.preventDefault();
    if (!editOrg) return;
    setError("");
    setSaving(true);
    try {
      await updateSubOrg(editOrg.id, newName.trim());
      setEditOrg(null);
      setNewName("");
      invalidate();
    } catch {
      setError("Failed to rename sub-organization. The name may already be taken.");
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!orgToDelete) return;
    setError("");
    try {
      await deleteSubOrg(orgToDelete.id);
      setOrgToDelete(null);
      invalidate();
    } catch {
      setError("Failed to delete sub-organization.");
      setOrgToDelete(null);
    }
  }

  const isDefault = (o: SubOrg) => o.name === "Default";
  const missingPrimaryFSO = orgs.filter(o => !isDefault(o) && !o.primary_fso_user_id);

  return (
    <>
      {error && <Alert variant="error" onDismiss={() => setError("")}>{error}</Alert>}

      {missingPrimaryFSO.length > 0 && (
        <Alert variant="warning">
          {missingPrimaryFSO.length === 1
            ? `${missingPrimaryFSO[0].name} has no primary FSO.`
            : `${missingPrimaryFSO.length} sub-organizations have no primary FSO.`}{" "}
          Action items created against {missingPrimaryFSO.length === 1 ? "it" : "them"} will fall back to all administrators until one is assigned.
        </Alert>
      )}

      <div style={{ display: "flex", justifyContent: "flex-end", marginBottom: 16 }}>
        <Button size="sm" onClick={() => { setNewName(""); setCreateOpen(true); }}>
          <Plus size={16} style={{ marginRight: 6 }} /> New Sub-Organization
        </Button>
      </div>

      <div className="task-list">
        {orgs.map(org => {
          return (
            <div key={org.id} className="task-row" style={{ alignItems: "center" }}>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div className="task-row-title" style={{ marginBottom: 2, display: "flex", alignItems: "center", gap: 6 }}>
                  {org.name}
                  {isDefault(org) && (
                    <span style={{ fontSize: 11, fontWeight: 500, color: "var(--color-text-muted)", background: "var(--color-bg-elevated)", border: "1px solid var(--color-border)", borderRadius: 4, padding: "1px 6px" }}>
                      default
                    </span>
                  )}
                  {!isDefault(org) && !org.primary_fso_user_id && (
                    <AlertTriangle size={14} color="var(--color-warning)" aria-label="No primary FSO assigned" />
                  )}
                </div>
                <div style={{ fontSize: 12, color: "var(--color-text-muted)" }}>
                  {org.member_count} {org.member_count === 1 ? "member" : "members"}
                </div>
              </div>
              <div style={{ display: "flex", alignItems: "center", gap: 6, marginRight: 8 }}>
                <span style={{ fontSize: 12, color: "var(--color-text-muted)", whiteSpace: "nowrap" }}>Primary FSO:</span>
                <select
                  value={org.primary_fso_user_id ?? ""}
                  onChange={e => handleFSOChange(org.id, e.target.value)}
                  disabled={fsoCandidates.length === 0}
                  title={fsoCandidates.length === 0 ? "No FSOs or administrators in this organization yet" : "Select primary FSO"}
                  style={{ fontSize: 13, padding: "4px 8px", borderRadius: 6, border: "1px solid var(--color-border)", background: "var(--color-bg-elevated)", color: "var(--color-text)" }}
                >
                  <option value="">None</option>
                  {fsoCandidates.map(m => (
                    <option key={m.user_id} value={m.user_id}>{m.name}</option>
                  ))}
                </select>
              </div>
              <button
                className="btn btn-sm"
                onClick={() => { setEditOrg(org); setNewName(org.name); }}
                title="Rename"
                style={{ marginRight: 6 }}
              >
                <Pencil size={14} />
              </button>
              <button
                className="btn btn-sm btn-danger"
                onClick={() => setOrgToDelete(org)}
                disabled={isDefault(org)}
                title={isDefault(org) ? "The Default sub-organization cannot be deleted" : "Delete"}
              >
                <Trash2 size={14} />
              </button>
            </div>
          );
        })}
        {orgs.length === 0 && (
          <p style={{ textAlign: "center", color: "var(--color-text-muted)", padding: 24 }}>
            No sub-organizations yet.
          </p>
        )}
      </div>

      {/* Create modal */}
      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="New Sub-Organization">
        <form onSubmit={handleCreate} style={{ display: "flex", flexDirection: "column", gap: 16 }}>
          <FormField label="Name" htmlFor="suborg-name">
            <input id="suborg-name" type="text" value={newName} onChange={e => setNewName(e.target.value)} required autoFocus placeholder="e.g. Engineering" />
          </FormField>
          <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
            <Button type="button" variant="secondary" onClick={() => setCreateOpen(false)}>Cancel</Button>
            <Button type="submit" variant="primary" loading={saving} loadingText="Creating...">Create</Button>
          </div>
        </form>
      </Modal>

      {/* Rename modal */}
      <Modal open={!!editOrg} onClose={() => setEditOrg(null)} title="Rename Sub-Organization">
        <form onSubmit={handleUpdate} style={{ display: "flex", flexDirection: "column", gap: 16 }}>
          <FormField label="Name" htmlFor="suborg-edit-name">
            <input id="suborg-edit-name" type="text" value={newName} onChange={e => setNewName(e.target.value)} required autoFocus />
          </FormField>
          <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
            <Button type="button" variant="secondary" onClick={() => setEditOrg(null)}>Cancel</Button>
            <Button type="submit" variant="primary" loading={saving} loadingText="Saving...">Save</Button>
          </div>
        </form>
      </Modal>

      {/* Delete confirm modal */}
      <Modal open={!!orgToDelete} onClose={() => setOrgToDelete(null)} title="Delete Sub-Organization">
        <p style={{ marginBottom: 20 }}>
          Delete <strong>{orgToDelete?.name}</strong>?{" "}
          {(orgToDelete?.member_count ?? 0) > 0
            ? `Its ${orgToDelete?.member_count} member(s) will be moved to the Default sub-organization.`
            : "It has no members."}
        </p>
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Button variant="secondary" onClick={() => setOrgToDelete(null)}>Cancel</Button>
          <Button variant="primary" onClick={handleDelete} style={{ background: "var(--color-danger)" }}>Delete</Button>
        </div>
      </Modal>
    </>
  );
}

// --- SSO Tab ---

const ROLE_OPTIONS: MemberRole[] = ["administrator", "fso", "read_only_fso", "individual_contributor"];

function emptySSOForm(): TenantSSOConfigUpdate {
  return {
    protocol: "oidc",
    enabled: false,
    auto_provision: false,
    default_role: "individual_contributor",
  };
}

function SSOTab() {
  const queryClient = useQueryClient();
  const [error, setError] = useState("");
  const [showSaved, setShowSaved] = useState(false);
  const [form, setForm] = useState<TenantSSOConfigUpdate>(emptySSOForm());
  const [newDomain, setNewDomain] = useState("");
  const [domainError, setDomainError] = useState("");

  const { data: cfg, isLoading: cfgLoading } = useQuery({
    queryKey: ["sso-config"],
    queryFn: async () => {
      try {
        return await getTenantSSOConfig();
      } catch (err) {
        // 404 is the expected "not yet configured" state — surface as null
        // rather than a thrown error so the form just shows empty fields.
        if (err instanceof ApiError && err.status === 404) return null;
        throw err;
      }
    },
  });

  // The domains list endpoint 404s if there's no SSO config yet — guard so
  // the empty-state copy is "Configure SSO first" rather than a generic err.
  const { data: domainsData } = useQuery({
    queryKey: ["sso-domains"],
    queryFn: async () => {
      try {
        return await listSSOEmailDomains();
      } catch (err) {
        if (err instanceof ApiError && err.status === 404) return { domains: [] as SSOEmailDomain[] };
        throw err;
      }
    },
    enabled: !!cfg,
  });
  const domains = domainsData?.domains ?? [];

  // Hydrate the form once cfg loads, but only on the first hydration —
  // we don't want to clobber an in-progress edit on a refetch.
  const [hydrated, setHydrated] = useState(false);
  useEffect(() => {
    if (hydrated || cfgLoading) return;
    if (cfg) {
      setForm({
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
    }
    setHydrated(true);
  }, [cfg, cfgLoading, hydrated]);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      await updateTenantSSOConfig(form);
      setShowSaved(true);
      setTimeout(() => setShowSaved(false), 4000);
      void queryClient.invalidateQueries({ queryKey: ["sso-config"] });
      void queryClient.invalidateQueries({ queryKey: ["sso-domains"] });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save SSO configuration");
    }
  }

  async function handleAddDomain(e: React.FormEvent) {
    e.preventDefault();
    setDomainError("");
    const trimmed = newDomain.trim().toLowerCase();
    if (!trimmed) return;
    try {
      await addSSOEmailDomain(trimmed);
      setNewDomain("");
      void queryClient.invalidateQueries({ queryKey: ["sso-domains"] });
    } catch (err) {
      setDomainError(err instanceof Error ? err.message : "Failed to add domain");
    }
  }

  async function handleRemoveDomain(domain: string) {
    setDomainError("");
    try {
      await removeSSOEmailDomain(domain);
      void queryClient.invalidateQueries({ queryKey: ["sso-domains"] });
    } catch (err) {
      setDomainError(err instanceof Error ? err.message : "Failed to remove domain");
    }
  }

  // The redirect URL the customer's IdP needs to whitelist. Origin is the
  // current backend, which the frontend reaches via its own origin in cloud
  // (same LB) but localhost:8080 in dev — read from window.location so the
  // URL is always copy-pasteable from whatever environment the admin's on.
  const callbackURL = `${window.location.origin}/api/auth/sso/callback/oidc`;

  if (cfgLoading) {
    return <p style={{ color: "var(--color-text-muted)", padding: 24 }}>Loading SSO configuration…</p>;
  }

  return (
    <>
      {error && <Alert variant="error" onDismiss={() => setError("")}>{error}</Alert>}

      <div style={{ background: "var(--color-bg-elevated)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", padding: "1rem 1.25rem", marginBottom: 20, fontSize: 13, color: "var(--color-text-muted)" }}>
        <strong style={{ color: "var(--color-text)" }}>Redirect URL for your identity provider</strong>
        <p style={{ margin: "6px 0 0" }}>Add this exact URL as an authorized redirect URI in your OAuth client (Google / Okta / Azure AD):</p>
        <code style={{ display: "inline-block", marginTop: 6, padding: "4px 8px", background: "var(--color-surface)", border: "1px solid var(--color-border)", borderRadius: 4, fontSize: 13 }}>{callbackURL}</code>
      </div>

      <form onSubmit={handleSave} style={{ display: "flex", flexDirection: "column", gap: 16, marginBottom: 24 }}>
        <h3 style={{ margin: 0, fontSize: 15 }}>OIDC configuration</h3>

        <FormField label="Issuer URL" htmlFor="sso-issuer">
          <input
            id="sso-issuer"
            type="url"
            value={form.issuer_url ?? ""}
            onChange={e => setForm({ ...form, issuer_url: e.target.value })}
            placeholder="https://accounts.google.com"
            required
          />
        </FormField>
        <FormField label="Client ID" htmlFor="sso-client-id">
          <input
            id="sso-client-id"
            type="text"
            value={form.client_id ?? ""}
            onChange={e => setForm({ ...form, client_id: e.target.value })}
            placeholder="123456789.apps.googleusercontent.com"
            required
          />
        </FormField>
        <FormField label={cfg?.client_id ? "Client Secret (leave blank to keep existing)" : "Client Secret"} htmlFor="sso-client-secret">
          <input
            id="sso-client-secret"
            type="password"
            value={form.client_secret ?? ""}
            onChange={e => setForm({ ...form, client_secret: e.target.value })}
            placeholder={cfg?.client_id ? "••••••••" : ""}
            required={!cfg?.client_id}
          />
        </FormField>
        <FormField label="Default role for SSO-provisioned users" htmlFor="sso-default-role">
          <select
            id="sso-default-role"
            value={form.default_role}
            onChange={e => setForm({ ...form, default_role: e.target.value as MemberRole })}
          >
            {ROLE_OPTIONS.map(r => <option key={r} value={r}>{ROLE_LABELS[r]}</option>)}
          </select>
        </FormField>

        <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 14 }}>
          <input
            type="checkbox"
            checked={form.enabled}
            onChange={e => setForm({ ...form, enabled: e.target.checked })}
          />
          <span><strong>Enabled</strong> — sign-in via this SSO is allowed</span>
        </label>
        <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 14 }}>
          <input
            type="checkbox"
            checked={form.auto_provision}
            onChange={e => setForm({ ...form, auto_provision: e.target.checked })}
          />
          <span><strong>Auto-provision users</strong> — anyone in the SSO domain can sign in even if not pre-invited (recommended OFF for enterprise)</span>
        </label>

        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <Button type="submit" variant="primary">Save</Button>
          {showSaved && (
            <span style={{ fontSize: 13, color: "var(--color-success, #1F7A46)" }}>✓ Saved</span>
          )}
        </div>
      </form>

      <div style={{ borderTop: "1px solid var(--color-border)", paddingTop: 20 }}>
        <h3 style={{ margin: "0 0 4px", fontSize: 15 }}>Email domains</h3>
        <p style={{ margin: "0 0 12px", fontSize: 13, color: "var(--color-text-muted)" }}>
          Users whose email addresses match one of these domains are routed to your SSO on sign-in. Add domains in lowercase, no <code>@</code>.
        </p>

        {!cfg && (
          <p style={{ fontSize: 13, color: "var(--color-text-muted)" }}>Save an SSO configuration above before adding email domains.</p>
        )}

        {cfg && (
          <>
            {domainError && <Alert variant="error" onDismiss={() => setDomainError("")}>{domainError}</Alert>}
            <form onSubmit={handleAddDomain} style={{ display: "flex", gap: 8, marginBottom: 12 }}>
              <input
                type="text"
                value={newDomain}
                onChange={e => setNewDomain(e.target.value)}
                placeholder="acmecorp.com"
                style={{ flex: 1 }}
              />
              <Button type="submit" size="sm" variant="primary" disabled={!newDomain.trim()}>Add</Button>
            </form>
            {domains.length === 0 ? (
              <p style={{ fontSize: 13, color: "var(--color-text-muted)" }}>No domains added yet.</p>
            ) : (
              <ul style={{ margin: 0, padding: 0, listStyle: "none", display: "flex", flexDirection: "column", gap: 4 }}>
                {domains.map(d => (
                  <li key={d.id} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "6px 10px", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", fontSize: 13 }}>
                    <span>{d.domain}</span>
                    <button
                      type="button"
                      onClick={() => handleRemoveDomain(d.domain)}
                      className="people-chip-remove"
                      title="Remove"
                    >
                      <Trash2 size={12} />
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </div>
    </>
  );
}

// --- Main Page ---

type Tab = "members" | "sub-orgs" | "sso";

export function CustomerAdminPage() {
  const [activeTab, setActiveTab] = useState<Tab>("members");

  const { data: subOrgData } = useQuery({
    queryKey: ["sub-orgs"],
    queryFn: listSubOrgs,
  });
  const subOrgs = subOrgData?.sub_orgs ?? [];

  return (
    <>
      <PageHeader
        title="Team Management"
        description="Invite and manage your organization's members"
      />

      <div style={{ display: "flex", gap: 0, borderBottom: "1px solid var(--color-border)", marginBottom: 20 }}>
        {(["members", "sub-orgs", "sso"] as Tab[]).map(tab => (
          <button
            key={tab}
            type="button"
            onClick={() => setActiveTab(tab)}
            style={{
              padding: "8px 18px",
              fontSize: 14,
              fontWeight: activeTab === tab ? 600 : 400,
              background: "none",
              border: "none",
              borderBottom: activeTab === tab ? "2px solid var(--color-primary)" : "2px solid transparent",
              color: activeTab === tab ? "var(--color-primary)" : "var(--color-text-muted)",
              cursor: "pointer",
              marginBottom: -1,
            }}
          >
            {tab === "members" ? "Members" : tab === "sub-orgs" ? "Sub-Organizations" : "Single Sign-On"}
          </button>
        ))}
      </div>

      {activeTab === "members" && <MembersTab subOrgs={subOrgs} />}
      {activeTab === "sub-orgs" && <SubOrgsTab />}
      {activeTab === "sso" && <SSOTab />}
    </>
  );
}

export default CustomerAdminPage;
