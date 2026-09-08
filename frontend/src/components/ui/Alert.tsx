import type { ReactNode } from "react";

interface AlertProps {
  variant: "error" | "success" | "warning";
  onDismiss?: () => void;
  children: ReactNode;
}

export function Alert({ variant, onDismiss, children }: AlertProps) {
  return (
    <div
      className={`alert alert-${variant}`}
      onClick={onDismiss}
      role={onDismiss ? "button" : undefined}
    >
      {children}
    </div>
  );
}
