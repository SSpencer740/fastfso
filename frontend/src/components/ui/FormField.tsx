import type { ReactNode } from "react";

interface FormFieldProps {
  label: string;
  htmlFor?: string;
  children: ReactNode;
  labelAction?: ReactNode;
}

export function FormField({ label, htmlFor, children, labelAction }: FormFieldProps) {
  return (
    <div className="form-field">
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
        <label htmlFor={htmlFor}>{label}</label>
        {labelAction}
      </div>
      {children}
    </div>
  );
}
