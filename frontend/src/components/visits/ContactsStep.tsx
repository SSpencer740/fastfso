import { FormField } from "../ui/FormField";

interface ContactsStepProps {
  pocName: string;
  onPocNameChange: (v: string) => void;
  pocEmail: string;
  onPocEmailChange: (v: string) => void;
  pocPhone: string;
  onPocPhoneChange: (v: string) => void;
  securityPocName: string;
  onSecurityPocNameChange: (v: string) => void;
  securityPocEmail: string;
  onSecurityPocEmailChange: (v: string) => void;
  securityPocPhone: string;
  onSecurityPocPhoneChange: (v: string) => void;
}

export function ContactsStep({
  pocName,
  onPocNameChange,
  pocEmail,
  onPocEmailChange,
  pocPhone,
  onPocPhoneChange,
  securityPocName,
  onSecurityPocNameChange,
  securityPocEmail,
  onSecurityPocEmailChange,
  securityPocPhone,
  onSecurityPocPhoneChange,
}: ContactsStepProps) {
  return (
    <div>
      <h4 style={{ margin: "0 0 1rem" }}>Point of Contact</h4>

      <FormField label="Name" htmlFor="poc-name">
        <input
          id="poc-name"
          type="text"
          value={pocName}
          onChange={(e) => onPocNameChange(e.target.value)}
          required
        />
      </FormField>

      <FormField label="Email" htmlFor="poc-email">
        <input
          id="poc-email"
          type="email"
          value={pocEmail}
          onChange={(e) => onPocEmailChange(e.target.value)}
          required
        />
      </FormField>

      <FormField label="Phone" htmlFor="poc-phone">
        <input
          id="poc-phone"
          type="tel"
          value={pocPhone}
          onChange={(e) => onPocPhoneChange(e.target.value)}
          required
        />
      </FormField>

      <h4 style={{ margin: "1.5rem 0 1rem", paddingTop: "1rem", borderTop: "1px solid var(--color-border)" }}>
        Security Point of Contact
      </h4>

      <FormField label="Name" htmlFor="security-poc-name">
        <input
          id="security-poc-name"
          type="text"
          value={securityPocName}
          onChange={(e) => onSecurityPocNameChange(e.target.value)}
          required
        />
      </FormField>

      <FormField label="Email" htmlFor="security-poc-email">
        <input
          id="security-poc-email"
          type="email"
          value={securityPocEmail}
          onChange={(e) => onSecurityPocEmailChange(e.target.value)}
          required
        />
      </FormField>

      <FormField label="Phone" htmlFor="security-poc-phone">
        <input
          id="security-poc-phone"
          type="tel"
          value={securityPocPhone}
          onChange={(e) => onSecurityPocPhoneChange(e.target.value)}
          required
        />
      </FormField>
    </div>
  );
}
