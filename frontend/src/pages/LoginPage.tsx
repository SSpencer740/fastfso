import { useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useAuthStore } from "../stores/authStore";
import { AuthLayout } from "../components/layout/AuthLayout";
import { LoginForm } from "../components/auth/LoginForm";
import { PasskeyButton } from "../components/auth/PasskeyButton";
import { Alert } from "../components/ui/Alert";

const SSO_ERRORS: Record<string, string> = {
  account_suspended: "Your account has been suspended.",
  org_suspended: "This organization has been suspended.",
  sso_failed: "SSO authentication failed.",
  sso_no_account: "No account found for this SSO identity.",
  sso_domain_not_allowed:
    "Your email domain is not allowed for this organization. Contact your administrator.",
};

export function LoginPage() {
  const state = useAuthStore((s) => s.state);
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const ssoError = searchParams.get("error");
  const resetSuccess = searchParams.get("reset") === "success";

  useEffect(() => {
    if (state === "pre_auth") {
      navigate("/challenge");
    } else if (state === "setup_2fa") {
      navigate("/setup-2fa");
    } else if (state === "pre_tenant") {
      navigate("/select-tenant");
    } else if (state === "authenticated") {
      navigate("/app");
    }
  }, [state, navigate]);

  return (
    <AuthLayout title="Sign in">
      {resetSuccess && (
        <Alert variant="success" onDismiss={() => setSearchParams({})}>
          Password reset successfully. Please sign in with your new password.
        </Alert>
      )}
      {ssoError && SSO_ERRORS[ssoError] && (
        <Alert
          variant="error"
          onDismiss={() => setSearchParams({})}
        >
          {SSO_ERRORS[ssoError]}
        </Alert>
      )}
      <LoginForm />
      <div className="auth-divider">
        <span>or</span>
      </div>
      <PasskeyButton mode="login" />
    </AuthLayout>
  );
}
