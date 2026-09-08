import type { ReactNode } from "react";
import { useNavigate } from "react-router";
import { useAuthStore } from "../../stores/authStore";
import { Button } from "../ui/Button";

interface DashboardLayoutProps {
  title?: string;
  children: ReactNode;
}

export function DashboardLayout({
  title = "fastFSO",
  children,
}: DashboardLayoutProps) {
  const identity = useAuthStore((s) => s.identity);
  const user = useAuthStore((s) => s.user);
  const isSuperAdmin = useAuthStore((s) => s.isSuperAdmin);
  const canSwitchContext = useAuthStore((s) => s.canSwitchContext);
  const switchContext = useAuthStore((s) => s.switchContext);
  const logout = useAuthStore((s) => s.logout);
  const navigate = useNavigate();

  async function handleLogout() {
    await logout();
    navigate("/login");
  }

  async function handleSwitch() {
    await switchContext();
    navigate("/select-tenant");
  }

  return (
    <div className="dashboard">
      <header className="dashboard-header">
        <h1>{title}</h1>
        <div className="dashboard-user">
          <span
            className="dashboard-user-name"
            onClick={() => navigate("/app/settings")}
          >
            {identity?.name ?? user?.name}
          </span>
          {user && <span className="user-role">{user.role}</span>}
          {isSuperAdmin && <span className="user-role">Super Admin</span>}
          {canSwitchContext && (
            <Button size="sm" onClick={handleSwitch}>
              Switch
            </Button>
          )}
          <Button size="sm" onClick={handleLogout}>
            Sign out
          </Button>
        </div>
      </header>
      <main className="dashboard-main">{children}</main>
    </div>
  );
}
