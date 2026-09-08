import type { ReactNode } from "react";

interface AuthLayoutProps {
  children: ReactNode;
  title?: string;
}

export function AuthLayout({ children, title }: AuthLayoutProps) {
  return (
    <div className="auth-layout">
      <div className="auth-card">
        <div className="auth-logo">fastFSO</div>
        {title && <h1 className="auth-title">{title}</h1>}
        {children}
      </div>
    </div>
  );
}
