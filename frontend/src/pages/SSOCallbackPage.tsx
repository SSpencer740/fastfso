import { useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useAuthStore } from "../stores/authStore";
import { AuthLayout } from "../components/layout/AuthLayout";
import { Alert } from "../components/ui/Alert";
import { Button } from "../components/ui/Button";

export function SSOCallbackPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const initialize = useAuthStore((s) => s.initialize);
  const error = searchParams.get("error");

  useEffect(() => {
    if (error) return;
    // After SSO redirect, the session cookie is set by the backend.
    // Re-initialize to pick up the session state.
    initialize().then(() => {
      navigate("/select-tenant");
    });
  }, [error, initialize, navigate]);

  if (error) {
    return (
      <AuthLayout title="SSO Error">
        <Alert variant="error">
          {error === "sso_no_account"
            ? "No account found for your SSO identity. Contact your administrator."
            : error === "sso_domain_not_allowed"
            ? "Your email domain is not allowed for this organization. Contact your administrator."
            : "SSO authentication failed. Please try again."}
        </Alert>
        <Button variant="primary" onClick={() => navigate("/login")}>
          Back to Login
        </Button>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout>
      <p className="auth-subtitle">Completing sign in...</p>
    </AuthLayout>
  );
}
