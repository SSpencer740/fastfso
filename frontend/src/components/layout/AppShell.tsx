import { Outlet, useNavigate } from "react-router";
import { useAuthStore } from "../../stores/authStore";
import { Button } from "../ui/Button";
import { Sidebar } from "./Sidebar";

export function AppShell() {
  const identity = useAuthStore((s) => s.identity);
  const user = useAuthStore((s) => s.user);
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
    <div className="app-shell">
      <header className="app-header">
        <div style={{ flex: 1 }} />
        <div className="dashboard-user">
          <span
            className="dashboard-user-name"
            onClick={() => navigate("/app/settings")}
          >
            {identity?.name ?? user?.name}
          </span>
          {user && (
            <span
              className={`user-role ${user.role === "administrator" || user.role === "fso" ? "user-role-admin" : ""}`}
            >
              {user.role.replace(/_/g, " ")}
            </span>
          )}
          {canSwitchContext && (
            <Button size="sm" onClick={handleSwitch}>
              Switch
            </Button>
          )}
          <Button size="sm" variant="secondary" onClick={handleLogout}>
            Sign out
          </Button>
        </div>
      </header>
      <div className="app-body">
        <Sidebar />
        <main className="app-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
