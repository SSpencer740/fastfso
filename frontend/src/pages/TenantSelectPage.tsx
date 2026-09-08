import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { useAuthStore } from "../stores/authStore";
import { AuthLayout } from "../components/layout/AuthLayout";
import { Alert } from "../components/ui/Alert";
import { Spinner } from "../components/ui/Spinner";
import { Button } from "../components/ui/Button";
import { FormField } from "../components/ui/FormField";
import * as authApi from "../api/auth";
import type { Tenant } from "../api/auth";
import { enrollTOTP, confirmTOTP, type TOTPEnrollment } from "../api/settings";

// --- Mandatory 2FA setup gate ---

function Setup2FAGate({ onComplete }: { onComplete: () => void }) {
  const [enrollment, setEnrollment] = useState<TOTPEnrollment | null>(null);
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

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
      onComplete();
    } catch {
      setError("Invalid code. Please try again.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthLayout title="Set up two-factor authentication">
      <p className="auth-subtitle">
        Your organization requires two-factor authentication. Scan the QR code
        with an authenticator app, then enter the code to continue.
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

// --- Main tenant select page ---

export function TenantSelectPage() {
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [canAccessAdmin, setCanAccessAdmin] = useState(false);
  const [loading, setLoading] = useState(true);
  const selectTenant = useAuthStore((s) => s.selectTenant);
  const selectAdmin = useAuthStore((s) => s.selectAdmin);
  const state = useAuthStore((s) => s.state);
  const mustSetup2FA = useAuthStore((s) => s.mustSetup2FA);
  const setup2FAComplete = useAuthStore((s) => s.setup2FAComplete);
  const error = useAuthStore((s) => s.error);
  const navigate = useNavigate();

  useEffect(() => {
    if (state === "authenticated") {
      navigate("/app");
      return;
    }

    authApi
      .getTenants()
      .then((res) => {
        setTenants(res.tenants);
        setCanAccessAdmin(res.can_access_admin_panel);
      })
      .catch(() => {
        navigate("/login");
      })
      .finally(() => setLoading(false));
  }, [state, navigate]);

  if (mustSetup2FA) {
    return <Setup2FAGate onComplete={setup2FAComplete} />;
  }

  if (loading) {
    return (
      <AuthLayout title="Select Organization">
        <Spinner text="Loading..." />
      </AuthLayout>
    );
  }

  async function handleSelectTenant(tenantId: string) {
    try {
      await selectTenant(tenantId);
    } catch {
      // Error is set in the store
    }
  }

  async function handleSelectAdmin() {
    try {
      await selectAdmin();
      navigate("/admin/sessions");
    } catch {
      // Error is set in the store
    }
  }

  return (
    <AuthLayout title="Select Organization">
      {error && <Alert variant="error">{error}</Alert>}
      <div className="tenant-list">
        {tenants.map((tenant) => (
          <button
            key={tenant.id}
            className={`tenant-card${tenant.suspended ? " tenant-suspended" : ""}`}
            onClick={() => !tenant.suspended && handleSelectTenant(tenant.id)}
            disabled={tenant.suspended}
          >
            <span className="tenant-name">{tenant.name}</span>
            <span className={`tenant-role${tenant.suspended ? " status-suspended" : ""}`}>
              {tenant.suspended ? "Suspended" : tenant.role}
            </span>
          </button>
        ))}
        {tenants.length === 0 && !canAccessAdmin && (
          <p className="auth-subtitle">
            You are not a member of any organization.
          </p>
        )}
      </div>
      {canAccessAdmin && (
        <div className="admin-option">
          <Button variant="secondary" onClick={handleSelectAdmin}>
            Admin Panel
          </Button>
        </div>
      )}
    </AuthLayout>
  );
}
