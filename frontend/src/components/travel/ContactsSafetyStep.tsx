import { FormField } from "../ui/FormField";

interface ContactsSafetyStepProps {
  emergencyFirstName: string;
  onEmergencyFirstNameChange: (v: string) => void;
  emergencyLastName: string;
  onEmergencyLastNameChange: (v: string) => void;
  emergencyPhone: string;
  onEmergencyPhoneChange: (v: string) => void;
  additionalComments: string;
  onAdditionalCommentsChange: (v: string) => void;
}

export function ContactsSafetyStep({
  emergencyFirstName,
  onEmergencyFirstNameChange,
  emergencyLastName,
  onEmergencyLastNameChange,
  emergencyPhone,
  onEmergencyPhoneChange,
  additionalComments,
  onAdditionalCommentsChange,
}: ContactsSafetyStepProps) {
  return (
    <div>
      <h4 style={{ margin: "0 0 1rem", fontSize: "0.95rem" }}>Emergency Contact</h4>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "1rem" }}>
        <FormField label="First Name" htmlFor="emergency-first-name">
          <input
            id="emergency-first-name"
            type="text"
            value={emergencyFirstName}
            onChange={(e) => onEmergencyFirstNameChange(e.target.value)}
            placeholder="First name"
          />
        </FormField>

        <FormField label="Last Name" htmlFor="emergency-last-name">
          <input
            id="emergency-last-name"
            type="text"
            value={emergencyLastName}
            onChange={(e) => onEmergencyLastNameChange(e.target.value)}
            placeholder="Last name"
          />
        </FormField>
      </div>

      <FormField label="Phone Number" htmlFor="emergency-phone">
        <input
          id="emergency-phone"
          type="tel"
          value={emergencyPhone}
          onChange={(e) => onEmergencyPhoneChange(e.target.value)}
          placeholder="e.g. +1 (555) 123-4567"
        />
      </FormField>

      <div style={{ marginTop: "1.5rem" }}>
        <h4 style={{ margin: "0 0 1rem", fontSize: "0.95rem" }}>Additional Information</h4>
        <FormField label="Comments" htmlFor="additional-comments">
          <textarea
            id="additional-comments"
            value={additionalComments}
            onChange={(e) => onAdditionalCommentsChange(e.target.value)}
            placeholder="Any additional information the FSO should know about this trip..."
            rows={4}
            style={{
              width: "100%",
              padding: "0.6rem 0.75rem",
              border: "1px solid var(--color-border-strong)",
              borderRadius: "var(--radius-md)",
              background: "var(--color-bg-elevated)",
              color: "inherit",
              fontSize: "0.9rem",
              fontFamily: "inherit",
              resize: "vertical",
            }}
          />
        </FormField>
      </div>
    </div>
  );
}
