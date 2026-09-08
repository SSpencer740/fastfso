import { useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router";
import { useAuthStore } from "../../stores/authStore";
import { Button } from "../ui/Button";
import { FormField } from "../ui/FormField";
import { Alert } from "../ui/Alert";

export function LoginForm() {
  const loginEmail = useAuthStore((s) => s.loginEmail);
  const loginMethods = useAuthStore((s) => s.loginMethods);
  const ssoOptions = useAuthStore((s) => s.ssoOptions);

  if (loginEmail) {
    return (
      <MethodStep
        email={loginEmail}
        methods={loginMethods}
        ssoOptions={ssoOptions}
      />
    );
  }

  return <EmailStep />;
}

function EmailStep() {
  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const identify = useAuthStore((s) => s.identify);
  const error = useAuthStore((s) => s.error);
  const clearError = useAuthStore((s) => s.clearError);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    try {
      await identify(email);
    } catch {
      // Error is set in the store
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="login-form" onSubmit={handleSubmit}>
      {error && (
        <Alert variant="error" onDismiss={clearError}>
          {error}
        </Alert>
      )}
      <FormField label="Email" htmlFor="email">
        <input
          id="email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          autoComplete="email"
          autoFocus
        />
      </FormField>
      <Button
        type="submit"
        variant="primary"
        loading={submitting}
        loadingText="Continuing..."
      >
        Continue
      </Button>
    </form>
  );
}

interface MethodStepProps {
  email: string;
  methods: string[];
  ssoOptions: { tenant_name: string; sso_url: string }[];
}

function MethodStep({ email, methods, ssoOptions }: MethodStepProps) {
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const login = useAuthStore((s) => s.login);
  const resetLogin = useAuthStore((s) => s.resetLogin);
  const error = useAuthStore((s) => s.error);
  const clearError = useAuthStore((s) => s.clearError);

  const hasPassword = methods.includes("password");

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!hasPassword) return;
    setSubmitting(true);
    try {
      await login(email, password);
    } catch {
      // Error is set in the store
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="login-form">
      {error && (
        <Alert variant="error" onDismiss={clearError}>
          {error}
        </Alert>
      )}
      <div className="login-email-display">
        <span>{email}</span>
        <button type="button" className="link-button" onClick={resetLogin}>
          Change
        </button>
      </div>

      {hasPassword && (
        <form onSubmit={handleSubmit}>
          <FormField
            label="Password"
            htmlFor="password"
            labelAction={
              <Link to="/forgot-password" className="link-button" tabIndex={-1} style={{ fontSize: 13 }}>
                Forgot password?
              </Link>
            }
          >
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="current-password"
              autoFocus
            />
          </FormField>
          <Button
            type="submit"
            variant="primary"
            loading={submitting}
            loadingText="Signing in..."
          >
            Sign in
          </Button>
        </form>
      )}

      {hasPassword && ssoOptions.length > 0 && (
        <div className="auth-divider">
          <span>or</span>
        </div>
      )}

      {ssoOptions.map((opt) => (
        <Button
          key={opt.sso_url}
          variant="secondary"
          style={{ width: "100%", marginBottom: "0.5rem" }}
          onClick={() => {
            window.location.href = opt.sso_url;
          }}
        >
          Sign in with {opt.tenant_name} SSO
        </Button>
      ))}
    </div>
  );
}
