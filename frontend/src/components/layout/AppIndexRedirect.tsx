import { Navigate } from "react-router";
import { useAuthStore } from "../../stores/authStore";

export function AppIndexRedirect() {
  const isSuperAdmin = useAuthStore((s) => s.isSuperAdmin);
  const user = useAuthStore((s) => s.user);

  if (isSuperAdmin && !user) {
    return <Navigate to="/admin" replace />;
  }
  return <Navigate to="/app/dashboard" replace />;
}
