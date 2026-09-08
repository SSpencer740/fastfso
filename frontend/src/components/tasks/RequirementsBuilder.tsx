import { X, Plus } from "lucide-react";

export interface RequirementDraft {
  kind: string;
  label: string;
  description: string;
  required: boolean;
  // ai_enabled is a transient UI-only flag. It controls whether the criteria
  // textarea is visible/required in the form. Stripped before sending to the
  // API — the backend treats criteria !== "" as "AI verification on."
  ai_enabled: boolean;
  ai_verification_criteria: string;
}

interface RequirementsBuilderProps {
  items: RequirementDraft[];
  onChange: (items: RequirementDraft[]) => void;
}

export function RequirementsBuilder({ items, onChange }: RequirementsBuilderProps) {
  function add() {
    onChange([...items, { kind: "text", label: "", description: "", required: true, ai_enabled: false, ai_verification_criteria: "" }]);
  }

  function update(index: number, field: keyof RequirementDraft, value: string | boolean) {
    const updated = items.map((item, i) => i === index ? { ...item, [field]: value } : item);
    onChange(updated);
  }

  // Toggling AI off clears the criteria so we never submit a row in an
  // inconsistent state (ai_enabled=false but criteria=non-empty).
  function toggleAI(index: number, enabled: boolean) {
    const updated = items.map((item, i) => i === index
      ? { ...item, ai_enabled: enabled, ai_verification_criteria: enabled ? item.ai_verification_criteria : "" }
      : item);
    onChange(updated);
  }

  function remove(index: number) {
    onChange(items.filter((_, i) => i !== index));
  }

  const kindLabels: Record<string, string> = {
    text: "Text field",
    textarea: "Text area",
    file_upload: "Document upload",
    checkbox: "Checkbox",
  };

  return (
    <div>
      {items.map((item, i) => (
        <div key={i} className="requirement-row" style={{ flexDirection: "column", alignItems: "stretch" }}>
          <div style={{ display: "flex", gap: 8, alignItems: "center", width: "100%" }}>
            <span className="requirement-row-number">{i + 1}</span>
            <div className="requirement-row-content" style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap", flex: 1 }}>
              <select
                className="filter-select"
                value={item.kind}
                onChange={e => update(i, "kind", e.target.value)}
              >
                {Object.entries(kindLabels).map(([val, label]) => (
                  <option key={val} value={val}>{label}</option>
                ))}
              </select>
              <input
                className="filter-input"
                style={{ flex: 1, minWidth: 150 }}
                placeholder="Label"
                value={item.label}
                onChange={e => update(i, "label", e.target.value)}
              />
              <label style={{ display: "flex", alignItems: "center", gap: 4, fontSize: 13, cursor: "pointer" }}>
                <input
                  type="checkbox"
                  checked={item.required}
                  onChange={e => update(i, "required", e.target.checked)}
                />
                Required
              </label>
            </div>
            <div className="requirement-row-actions">
              <button type="button" className="btn btn-sm btn-danger" onClick={() => remove(i)}>
                <X size={14} />
              </button>
            </div>
          </div>

          {item.kind === "file_upload" && (
            <div style={{ marginTop: 8, marginLeft: 32 }}>
              <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 13, cursor: "pointer" }}>
                <input
                  type="checkbox"
                  checked={item.ai_enabled}
                  onChange={e => toggleAI(i, e.target.checked)}
                />
                Enable AI verification of uploaded documents
              </label>
              {item.ai_enabled && (
                <div style={{ marginTop: 8 }}>
                  <label style={{ display: "block", fontSize: 12, color: "var(--color-text-muted)", marginBottom: 4 }}>
                    What should AI verify? <span style={{ color: "var(--color-danger)" }}>*</span>
                  </label>
                  <textarea
                    required
                    placeholder="e.g. The document is a Cyber Awareness training certificate, completion date within the last 12 months, and the name on the certificate matches the assignee."
                    value={item.ai_verification_criteria}
                    onChange={e => update(i, "ai_verification_criteria", e.target.value)}
                    style={{
                      width: "100%",
                      minHeight: 60,
                      padding: "0.5rem 0.6rem",
                      border: "1px solid var(--color-border)",
                      borderRadius: "var(--radius-md)",
                      fontFamily: "inherit",
                      fontSize: "0.85rem",
                      resize: "vertical",
                      background: "var(--color-surface)",
                      color: "var(--color-text)",
                    }}
                  />
                  <div style={{ marginTop: 4, fontSize: 12, color: "var(--color-text-muted)", lineHeight: 1.4 }}>
                    <div>Uploads to this requirement will be restricted to <strong>PDF, JPG, PNG, or WEBP</strong>, up to 18 MB.</div>
                    <div style={{ color: "var(--color-danger)", marginTop: 2 }}>
                      Do not enable on tasks where uploads may contain CUI or classified material.
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      ))}
      <button type="button" className="btn btn-sm" onClick={add} style={{ marginTop: 8 }}>
        <Plus size={14} style={{ marginRight: 4 }} /> Add Requirement
      </button>
    </div>
  );
}
