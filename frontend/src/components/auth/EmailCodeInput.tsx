import { useState } from "react";
import type { FormEvent } from "react";
import { api } from "../../api/client";
import { ApiError } from "../../api/client";
import { useAuthStore } from "../../stores/authStore";
import type { AuthState } from "../../stores/authStore";
import { Button } from "../ui/Button";
import { FormField } from "../ui/FormField";
import { Alert } from "../ui/Alert";

export function EmailCodeInput() {
  const [code, setCode] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sent, setSent] = useState(false);
  const challengeComplete = useAuthStore((s) => s.challengeComplete);

  async function handleSend() {
    setSending(true);
    setError(null);
    try {
      await api("/api/auth/2fa/email/send", { method: "POST" });
      setSent(true);
    } catch (err) {
      const message =
        err instanceof ApiError ? err.message : "Failed to send code";
      setError(message);
    } finally {
      setSending(false);
    }
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const res = await api<{ state: string }>("/api/auth/2fa/email/verify", {
        method: "POST",
        body: JSON.stringify({ code }),
      });
      challengeComplete(res.state as AuthState);
    } catch {
      setError("Invalid or expired code. Please try again.");
    } finally {
      setSubmitting(false);
    }
  }

  if (!sent) {
    return (
      <div>
        {error && <Alert variant="error">{error}</Alert>}
        <p className="auth-subtitle">
          We'll send a verification code to your email.
        </p>
        <Button
          variant="primary"
          onClick={handleSend}
          loading={sending}
          loadingText="Sending..."
        >
          Send Code
        </Button>
      </div>
    );
  }

  return (
    <form className="login-form" onSubmit={handleSubmit}>
      {error && <Alert variant="error">{error}</Alert>}
      <p className="auth-subtitle">Enter the code sent to your email.</p>
      <FormField label="Verification Code" htmlFor="email-code">
        <input
          id="email-code"
          type="text"
          inputMode="numeric"
          pattern="[0-9]*"
          maxLength={6}
          value={code}
          onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
          required
          autoFocus
          autoComplete="one-time-code"
          placeholder="000000"
        />
      </FormField>
      <Button
        type="submit"
        variant="primary"
        loading={submitting}
        loadingText="Verifying..."
      >
        Verify
      </Button>
    </form>
  );
}
