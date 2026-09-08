import { useState } from "react";
import { useNavigate, useSearchParams, Link } from "react-router";
import { AuthLayout } from "../components/layout/AuthLayout";
import { Button } from "../components/ui/Button";
import { FormField } from "../components/ui/FormField";
import { Alert } from "../components/ui/Alert";
import { confirmPasswordReset } from "../api/auth";
import { ApiError } from "../api/client";

export function ResetPasswordPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const token = searchParams.get("token") ?? "";

  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [totpCode, setTotpCode] = useState("");
  const [needs2FA, setNeeds2FA] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  if (!token) {
    return (
      <AuthLayout title="Invalid link">
        <Alert variant="error">This reset link is missing a token.</Alert>
        <Link to="/forgot-password" className="link-button" style={{ display: "block", textAlign: "center", marginTop: 16 }}>
          Request a new link
        </Link>
      </AuthLayout>
    );
  }

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
    if (needs2FA && totpCode.trim().length === 0) {
      setError("Enter the code from your authenticator app.");
      return;
    }

    setSubmitting(true);
    try {
      await confirmPasswordReset(
        token,
        password,
        needs2FA ? totpCode.trim() : undefined,
      );
      navigate("/login?reset=success");
    } catch (err) {
      if (err instanceof ApiError && err.body?.requires_2fa) {
        setNeeds2FA(true);
        setError(
          needs2FA
            ? "That code didn't match. Try again."
            : "",
        );
      } else {
        setError("This reset link is invalid or has expired. Please request a new one.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <AuthLayout title={needs2FA ? "Enter your 2FA code" : "Choose a new password"}>
      {error && <Alert variant="error" onDismiss={() => setError("")}>{error}</Alert>}
      <form className="login-form" onSubmit={handleSubmit}>
        <FormField label="New password" htmlFor="reset-password">
          <input
            id="reset-password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
            autoFocus={!needs2FA}
            autoComplete="new-password"
            placeholder="Min 8 characters"
            readOnly={needs2FA}
          />
        </FormField>
        <FormField label="Confirm password" htmlFor="reset-confirm">
          <input
            id="reset-confirm"
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
            minLength={8}
            autoComplete="new-password"
            readOnly={needs2FA}
          />
        </FormField>
        {needs2FA && (
          <FormField label="Authenticator code" htmlFor="reset-totp">
            <input
              id="reset-totp"
              type="text"
              inputMode="numeric"
              pattern="[0-9]*"
              value={totpCode}
              onChange={(e) => setTotpCode(e.target.value)}
              required
              autoFocus
              autoComplete="one-time-code"
              placeholder="6-digit code"
            />
          </FormField>
        )}
        <Button type="submit" variant="primary" loading={submitting} loadingText="Resetting...">
          Reset password
        </Button>
      </form>
    </AuthLayout>
  );
}
