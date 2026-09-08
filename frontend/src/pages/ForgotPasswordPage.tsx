import { useState } from "react";
import { Link } from "react-router";
import { AuthLayout } from "../components/layout/AuthLayout";
import { Button } from "../components/ui/Button";
import { FormField } from "../components/ui/FormField";
import { Alert } from "../components/ui/Alert";
import { requestPasswordReset } from "../api/auth";

export function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setSubmitting(true);
    try {
      await requestPasswordReset(email);
      setSent(true);
    } catch {
      setError("Something went wrong. Please try again.");
    } finally {
      setSubmitting(false);
    }
  }

  if (sent) {
    return (
      <AuthLayout title="Check your email">
        <p className="auth-subtitle">
          If <strong>{email}</strong> has an account, a reset link has been sent. Check your inbox.
        </p>
        <Link to="/login" className="link-button" style={{ display: "block", textAlign: "center", marginTop: 16 }}>
          Back to sign in
        </Link>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout title="Reset your password">
      <p className="auth-subtitle">
        Enter your email and we'll send you a link to reset your password.
      </p>
      {error && <Alert variant="error" onDismiss={() => setError("")}>{error}</Alert>}
      <form className="login-form" onSubmit={handleSubmit}>
        <FormField label="Email" htmlFor="forgot-email">
          <input
            id="forgot-email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            autoFocus
            autoComplete="email"
          />
        </FormField>
        <Button type="submit" variant="primary" loading={submitting} loadingText="Sending...">
          Send reset link
        </Button>
      </form>
      <Link to="/login" className="link-button" style={{ display: "block", textAlign: "center", marginTop: 16 }}>
        Back to sign in
      </Link>
    </AuthLayout>
  );
}
