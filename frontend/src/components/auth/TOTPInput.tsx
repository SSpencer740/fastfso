import { useState } from "react";
import type { FormEvent } from "react";
import { api } from "../../api/client";
import { useAuthStore } from "../../stores/authStore";
import type { AuthState } from "../../stores/authStore";
import { Button } from "../ui/Button";
import { FormField } from "../ui/FormField";
import { Alert } from "../ui/Alert";

export function TOTPInput() {
  const [code, setCode] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const challengeComplete = useAuthStore((s) => s.challengeComplete);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const res = await api<{ state: string }>("/api/auth/2fa/totp/verify", {
        method: "POST",
        body: JSON.stringify({ code }),
      });
      challengeComplete(res.state as AuthState);
    } catch {
      setError("Invalid code. Please try again.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="login-form" onSubmit={handleSubmit}>
      {error && <Alert variant="error">{error}</Alert>}
      <FormField label="Authentication Code" htmlFor="totp-code">
        <input
          id="totp-code"
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
