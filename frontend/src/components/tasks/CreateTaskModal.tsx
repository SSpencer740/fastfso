import { useState, useEffect } from "react";
import { Modal } from "../ui/Modal";
import { PeoplePicker } from "./PeoplePicker";
import { RequirementsBuilder, type RequirementDraft } from "./RequirementsBuilder";
import { useAuthStore } from "../../stores/authStore";
import { isAdministrator } from "../../utils/roles";
import type { TenantUser, SubOrgOption } from "../../api/tasks";
import { createTask, listSubOrgs } from "../../api/tasks";

interface CreateTaskModalProps {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}

type AssignTab = "people" | "sub_org" | "everyone";

export function CreateTaskModal({ open, onClose, onCreated }: CreateTaskModalProps) {
  const role = useAuthStore(s => s.user?.role);
  const userIsAdmin = isAdministrator(role);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState("medium");
  const [dueDate, setDueDate] = useState("");
  const [assignTab, setAssignTab] = useState<AssignTab>("people");
  const [selectedPeople, setSelectedPeople] = useState<TenantUser[]>([]);
  const [subOrgs, setSubOrgs] = useState<SubOrgOption[]>([]);
  const [selectedSubOrg, setSelectedSubOrg] = useState("");
  const [requirements, setRequirements] = useState<RequirementDraft[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (open) {
      listSubOrgs().then(r => setSubOrgs(r || [])).catch(() => {});
    }
  }, [open]);

  function reset() {
    setTitle(""); setDescription(""); setPriority("medium"); setDueDate("");
    setAssignTab("people"); setSelectedPeople([]); setSelectedSubOrg("");
    setRequirements([]); setError("");
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) { setError("Title is required"); return; }

    // FSOs see "Everyone" labelled as "everyone in your sub-org" and the
    // backend rejects org-wide rules from non-admins, so map their "everyone"
    // tab to a sub_org rule pointing at their own sub-org. listSubOrgs
    // already returns only their sub-org for non-admins.
    const fsoSubOrg = !userIsAdmin && subOrgs.length > 0 ? subOrgs[0].id : "";
    const rules = assignTab === "people"
      ? selectedPeople.map(u => ({ rule_type: "user" as const, target_id: u.user_id }))
      : assignTab === "sub_org" && selectedSubOrg
        ? [{ rule_type: "sub_org" as const, target_id: selectedSubOrg }]
        : !userIsAdmin && fsoSubOrg
          ? [{ rule_type: "sub_org" as const, target_id: fsoSubOrg }]
          : [{ rule_type: "org" as const }];

    const validRequirements = requirements.filter(r => r.label.trim());
    // Enforce: if AI verification is checked, criteria must be filled in.
    // Native `required` on the textarea catches Enter-key submits but not
    // programmatic clicks on Save, so we validate again here.
    const missingCriteria = validRequirements.find(
      r => r.kind === "file_upload" && r.ai_enabled && !r.ai_verification_criteria.trim(),
    );
    if (missingCriteria) {
      setError(`"${missingCriteria.label.trim()}": AI verification is enabled, so a verification description is required.`);
      return;
    }

    setSubmitting(true);
    setError("");
    try {
      await createTask({
        title: title.trim(),
        description,
        priority,
        due_date: dueDate || undefined,
        sub_org_id: assignTab === "sub_org" && selectedSubOrg
          ? selectedSubOrg
          : assignTab === "people" && selectedPeople.length > 0 && selectedPeople.every(u => u.sub_org_id && u.sub_org_id === selectedPeople[0].sub_org_id)
            ? selectedPeople[0].sub_org_id
            : undefined,
        requirements: validRequirements.map((r, i) => ({
          kind: r.kind,
          label: r.label.trim(),
          description: r.description,
          required: r.required,
          sort_order: i,
          // Send criteria only when the AI checkbox is on. The backend
          // treats empty criteria as "AI disabled."
          ai_verification_criteria: r.ai_enabled ? r.ai_verification_criteria : "",
        })),
        rules,
      });
      reset();
      onCreated();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create task");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Create New Task">
      <form onSubmit={handleSubmit}>
        {error && <div className="alert alert-error">{error}</div>}

        <div className="section-header">BASIC INFO</div>

        <div className="form-field">
          <label>Task Title</label>
          <input value={title} onChange={e => setTitle(e.target.value)} placeholder="Enter task title..." />
        </div>

        <div className="form-field">
          <label>Description / Instructions</label>
          <textarea
            style={{ width: "100%", minHeight: 80, padding: "0.6rem 0.75rem", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", fontFamily: "inherit", fontSize: "0.9rem", resize: "vertical", background: "var(--color-surface)", color: "var(--color-text)" }}
            value={description}
            onChange={e => setDescription(e.target.value)}
            placeholder="Add a description or instructions for this task..."
          />
        </div>

        <div style={{ display: "flex", gap: 16, marginBottom: 16 }}>
          <div className="form-field" style={{ flex: 1 }}>
            <label>Priority</label>
            <select className="filter-select" style={{ width: "100%" }} value={priority} onChange={e => setPriority(e.target.value)}>
              <option value="low">Low</option>
              <option value="medium">Medium</option>
              <option value="high">High</option>
              <option value="urgent">Urgent</option>
            </select>
          </div>
          <div className="form-field" style={{ flex: 1 }}>
            <label>Due Date</label>
            <input type="date" value={dueDate} onChange={e => setDueDate(e.target.value)} />
          </div>
        </div>

        <div className="section-header">ASSIGNMENT</div>
        <div style={{ display: "flex", gap: 8, marginBottom: 12 }}>
          {(userIsAdmin
            ? [
                { value: "people",   label: "Specific People" },
                { value: "sub_org",  label: "By Sub-Org" },
                { value: "everyone", label: "Everyone" },
              ]
            : [
                { value: "people",   label: "Specific People" },
                { value: "everyone", label: "Everyone in your sub-org" },
              ]
          ).map(tab => {
            const selected = assignTab === tab.value;
            return (
              <button
                key={tab.value}
                type="button"
                onClick={() => setAssignTab(tab.value as AssignTab)}
                style={{
                  flex: 1,
                  padding: "8px 10px",
                  fontSize: 13,
                  fontWeight: selected ? 600 : 400,
                  borderRadius: "var(--radius-md)",
                  border: `2px solid ${selected ? "var(--color-primary)" : "var(--color-border)"}`,
                  background: selected ? "color-mix(in srgb, var(--color-primary) 10%, transparent)" : "var(--color-bg-elevated)",
                  color: selected ? "var(--color-primary)" : "var(--color-text)",
                  cursor: "pointer",
                  transition: "border-color 0.15s, background 0.15s",
                }}
              >
                {tab.label}
              </button>
            );
          })}
        </div>

        {assignTab === "people" && <PeoplePicker selected={selectedPeople} onChange={setSelectedPeople} />}
        {assignTab === "sub_org" && userIsAdmin && (
          <select className="filter-select" style={{ width: "100%" }} value={selectedSubOrg} onChange={e => setSelectedSubOrg(e.target.value)}>
            <option value="">Select sub-organization...</option>
            {subOrgs.map(o => <option key={o.id} value={o.id}>{o.name}</option>)}
          </select>
        )}
        {assignTab === "everyone" && (
          <p style={{ fontSize: 13, color: "var(--color-text-muted)", margin: "8px 0" }}>
            {userIsAdmin
              ? "Task will be assigned to all users in your organization."
              : "Task will be assigned to everyone in your sub-organization."}
          </p>
        )}

        <div className="section-header" style={{ marginTop: 20 }}>TASK REQUIREMENTS ({requirements.length})</div>
        <p style={{ fontSize: 13, color: "#6B8294", margin: "0 0 8px" }}>Define what the assignee needs to provide</p>
        <RequirementsBuilder items={requirements} onChange={setRequirements} />

        <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, marginTop: 24, paddingTop: 16, borderTop: "1px solid var(--color-border)" }}>
          <button type="button" className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn btn-primary" style={{ width: "auto" }} disabled={submitting}>
            {submitting ? "Creating..." : "Create Task"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
