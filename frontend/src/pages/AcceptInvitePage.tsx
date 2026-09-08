import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useAuthStore } from "../stores/authStore";
import { AuthLayout } from "../components/layout/AuthLayout";
import { Alert } from "../components/ui/Alert";
import { Button } from "../components/ui/Button";
import { FormField } from "../components/ui/FormField";
import { getInviteInfo, acceptInvite } from "../api/auth";

export function AcceptInvitePage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const initialize = useAuthStore((s) => s.initialize);
  const token = searchParams.get("token") ?? "";

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [tenantName, setTenantName] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!token) {
      setError("No invite token provided.");
      setLoading(false);
      return;
    }
    getInviteInfo(token)
      .then((info) => {
        setEmail(info.email);
        setName(info.name);
        setTenantName(info.tenant_name);
      })
      .catch(() => {
        setError("This invite link is invalid or has expired.");
      })
      .finally(() => setLoading(false));
  }, [token]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (password.length < 8) {
      setError("Password must be at least 8 characters.");
      return;
    }
    if (password !== confirmPassword) {
      setError("Passwords do not match.");
      return;
    }

    setSubmitting(true);
    try {
      await acceptInvite(token, password, name);
      await initialize();
      navigate("/select-tenant");
    } catch {
      setError("Failed to set up your account. The invite may have expired.");
    } finally {
      setSubmitting(false);
    }
  }

  if (loading) {
    return (
      <AuthLayout>
        <p className="auth-subtitle">Validating invite...</p>
      </AuthLayout>
    );
  }

  if (error && !email) {
    return (
      <AuthLayout title="Invalid Invite">
        <Alert variant="error">{error}</Alert>
        <Button variant="primary" onClick={() => navigate("/login")}>
          Go to Login
        </Button>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout title="Set Up Your Account">
      {tenantName && (
        <p className="auth-subtitle">
          You've been invited to <strong>{tenantName}</strong>
        </p>
      )}
      {error && <Alert variant="error">{error}</Alert>}
      <form onSubmit={handleSubmit} className="login-form">
        <FormField label="Email" htmlFor="invite-email">
          <input id="invite-email" type="email" value={email} disabled />
        </FormField>
        <FormField label="Name" htmlFor="invite-name">
          <input
            id="invite-name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </FormField>
        <FormField label="Password" htmlFor="invite-password">
          <input
            id="invite-password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
            placeholder="Min 8 characters"
          />
        </FormField>
        <FormField label="Confirm Password" htmlFor="invite-confirm-password">
          <input
            id="invite-confirm-password"
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
            minLength={8}
          />
        </FormField>
        <Button
          type="submit"
          variant="primary"
          loading={submitting}
          loadingText="Setting up..."
        >
          Set Up Account
        </Button>
      </form>
    </AuthLayout>
  );
}
