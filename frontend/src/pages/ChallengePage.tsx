import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { AuthLayout } from "../components/layout/AuthLayout";
import { TOTPInput } from "../components/auth/TOTPInput";
import { EmailCodeInput } from "../components/auth/EmailCodeInput";
import { PasskeyButton } from "../components/auth/PasskeyButton";
import { Spinner } from "../components/ui/Spinner";
import { Button } from "../components/ui/Button";
import { useAuthStore } from "../stores/authStore";
import * as authApi from "../api/auth";

export function ChallengePage() {
  const state = useAuthStore((s) => s.state);
  const [methods, setMethods] = useState<string[]>([]);
  const [selectedMethod, setSelectedMethod] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  useEffect(() => {
    if (state === "pre_tenant") {
      navigate("/select-tenant");
      return;
    }
    if (state === "authenticated") {
      navigate("/app");
      return;
    }

    authApi
      .get2FAMethods()
      .then((res) => {
        setMethods(res.methods);
        if (res.methods.length === 1) {
          setSelectedMethod(res.methods[0]);
        }
      })
      .catch(() => navigate("/login"))
      .finally(() => setLoading(false));
  }, [state, navigate]);

  if (loading) {
    return (
      <AuthLayout title="Two-Factor Authentication">
        <Spinner text="Loading..." />
      </AuthLayout>
    );
  }

  return (
    <AuthLayout title="Two-Factor Authentication">
      {!selectedMethod && methods.length > 1 && (
        <div className="tenant-list">
          {methods.includes("totp") && (
            <button
              className="tenant-card"
              onClick={() => setSelectedMethod("totp")}
            >
              <span className="tenant-name">Authenticator App</span>
            </button>
          )}
          {methods.includes("email") && (
            <button
              className="tenant-card"
              onClick={() => setSelectedMethod("email")}
            >
              <span className="tenant-name">Email Code</span>
            </button>
          )}
          {methods.includes("passkey") && (
            <button
              className="tenant-card"
              onClick={() => setSelectedMethod("passkey")}
            >
              <span className="tenant-name">Passkey</span>
            </button>
          )}
        </div>
      )}

      {selectedMethod === "totp" && <TOTPInput />}

      {selectedMethod === "email" && <EmailCodeInput />}

      {selectedMethod === "passkey" && <PasskeyButton mode="2fa" />}

      {selectedMethod && methods.length > 1 && (
        <Button
          variant="secondary"
          style={{ marginTop: "1rem", width: "100%" }}
          onClick={() => setSelectedMethod(null)}
        >
          Use a different method
        </Button>
      )}
    </AuthLayout>
  );
}
