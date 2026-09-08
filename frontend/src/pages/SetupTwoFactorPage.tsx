import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { AuthLayout } from "../components/layout/AuthLayout";
import { Alert } from "../components/ui/Alert";
import { Spinner } from "../components/ui/Spinner";
import { Button } from "../components/ui/Button";
import { FormField } from "../components/ui/FormField";
import { useAuthStore } from "../stores/authStore";
import { enrollTOTP, confirmTOTP, type TOTPEnrollment } from "../api/settings";

export function SetupTwoFactorPage() {
  const [enrollment, setEnrollment] = useState<TOTPEnrollment | null>(null);
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const setup2FAComplete = useAuthStore((s) => s.setup2FAComplete);
  const challengeComplete = useAuthStore((s) => s.challengeComplete);
  const navigate = useNavigate();

  useEffect(() => {
    setLoading(true);
    enrollTOTP()
      .then(setEnrollment)
      .catch(() => setError("Failed to start 2FA setup. Please refresh."))
      .finally(() => setLoading(false));
  }, []);

  async function handleConfirm(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await confirmTOTP(code);
      setup2FAComplete();
      challengeComplete("pre_tenant");
      navigate("/select-tenant");
    } catch {
      setError("Invalid code. Please try again.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthLayout title="Set up two-factor authentication">
      <p className="auth-subtitle">
        Your account requires two-factor authentication. Scan the QR code with
        an authenticator app, then enter the code to continue.
      </p>
      {error && <Alert variant="error" onDismiss={() => setError("")}>{error}</Alert>}
      {loading && !enrollment && <Spinner text="Generating..." />}
      {enrollment && (
        <>
          <div style={{ display: "flex", justifyContent: "center", margin: "16px 0" }}>
            <img
              src={`https://api.qrserver.com/v1/create-qr-code/?size=180x180&data=${encodeURIComponent(enrollment.url)}`}
              alt="TOTP QR code"
              width={180}
              height={180}
            />
          </div>
          <p style={{ fontSize: 12, color: "var(--color-text-muted)", textAlign: "center", marginBottom: 16 }}>
            Can't scan? Enter this key manually:<br />
            <code style={{ fontSize: 13, letterSpacing: 2 }}>{enrollment.secret}</code>
          </p>
          <form className="login-form" onSubmit={handleConfirm}>
            <FormField label="Verification code" htmlFor="setup-totp-code">
              <input
                id="setup-totp-code"
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
            <Button type="submit" variant="primary" loading={loading} loadingText="Verifying...">
              Enable two-factor authentication
            </Button>
          </form>
        </>
      )}
    </AuthLayout>
  );
}
