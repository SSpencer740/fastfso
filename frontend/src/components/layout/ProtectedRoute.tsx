import { Navigate } from "react-router";
import type { ReactNode } from "react";
import { useAuthStore } from "../../stores/authStore";
import { Spinner } from "../ui/Spinner";

interface ProtectedRouteProps {
  children: ReactNode;
  allowedStates: string[];
  redirectTo?: string;
}

export function ProtectedRoute({
  children,
  allowedStates,
  redirectTo,
}: ProtectedRouteProps) {
  const state = useAuthStore((s) => s.state);

  if (state === "loading") {
    return <Spinner fullPage />;
  }

  if (!allowedStates.includes(state)) {
    const target = redirectTo ?? getDefaultRedirect(state);
    return <Navigate to={target} replace />;
  }

  return <>{children}</>;
}

function getDefaultRedirect(state: string): string {
  switch (state) {
    case "unauthenticated":
      return "/login";
    case "pre_auth":
      return "/challenge";
    case "setup_2fa":
      return "/setup-2fa";
    case "pre_tenant":
      return "/select-tenant";
    case "authenticated":
      return "/app";
    default:
      return "/login";
  }
}
