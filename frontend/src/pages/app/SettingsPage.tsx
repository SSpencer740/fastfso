import { useEffect, useState } from "react";
import { Moon, Sun } from "lucide-react";
import { useAuthStore } from "../../stores/authStore";
import { useThemeStore } from "../../stores/themeStore";
import { PageHeader } from "../../components/ui/PageHeader";
import { Alert } from "../../components/ui/Alert";
import { Button } from "../../components/ui/Button";
import { FormField } from "../../components/ui/FormField";
import {
  type SecurityOverview,
  type Passkey,
  type TOTPEnrollment,
  type UserSettings,
  getSecurityOverview,
  changePassword,
  listPasskeys,
  registerPasskey,
  removePasskey,
  enrollTOTP,
  confirmTOTP,
  removeTOTP,
  getUserSettings,
  updateUserSettings,
} from "../../api/settings";
import { revokeAllOtherSessions } from "../../api/auth";
import { ApiError } from "../../api/client";

export function SettingsPage() {
  const identity = useAuthStore((s) => s.identity);
  const { theme, toggleTheme } = useThemeStore();

  const [security, setSecurity] = useState<SecurityOverview | null>(null);

  // User settings
  const [userSettings, setUserSettings] = useState<UserSettings | null>(null);
  const [settingsSaving, setSettingsSaving] = useState(false);
  const [settingsSuccess, setSettingsSuccess] = useState("");

  // Password
  const [currentPw, setCurrentPw] = useState("");
  const [newPw, setNewPw] = useState("");
  const [confirmPw, setConfirmPw] = useState("");
  const [pwError, setPwError] = useState("");
  const [pwSuccess, setPwSuccess] = useState("");

  // Passkeys
  const [passkeys, setPasskeys] = useState<Passkey[]>([]);
  const [passkeyName, setPasskeyName] = useState("");
  const [showAddPasskey, setShowAddPasskey] = useState(false);
  const [passkeyError, setPasskeyError] = useState("");
  const [passkeyLoading, setPasskeyLoading] = useState(false);

  // TOTP
  const [totpEnrollment, setTotpEnrollment] = useState<TOTPEnrollment | null>(
    null,
  );
  const [totpCode, setTotpCode] = useState("");
  const [totpError, setTotpError] = useState("");
  const [totpLoading, setTotpLoading] = useState(false);

  // Sign out everywhere else
  const [revokeOthersLoading, setRevokeOthersLoading] = useState(false);
  const [revokeOthersMessage, setRevokeOthersMessage] = useState("");
  const [revokeOthersError, setRevokeOthersError] = useState("");

  useEffect(() => {
    loadSecurityOverview();
    loadPasskeys();
    getUserSettings().then(setUserSettings).catch(() => {});
  }, []);

  async function loadSecurityOverview() {
    try {
      const data = await getSecurityOverview();
      setSecurity(data);
    } catch {
      // Failed to load
    }
  }

  async function loadPasskeys() {
    try {
      const data = await listPasskeys();
      setPasskeys(data.passkeys ?? []);
    } catch {
      // Failed to load
    }
  }

  // --- Notification frequency ---

  async function handleSaveNotificationFrequency(frequency: UserSettings["notification_frequency"]) {
    if (!userSettings) return;
    setSettingsSaving(true);
    setSettingsSuccess("");
    try {
      const updated = await updateUserSettings({ ...userSettings, notification_frequency: frequency });
      setUserSettings(updated);
      setSettingsSuccess("Preferences saved");
      setTimeout(() => setSettingsSuccess(""), 2500);
    } catch {
      // ignore for now
    } finally {
      setSettingsSaving(false);
    }
  }

  // --- Password ---

  async function handleChangePassword(e: React.FormEvent) {
    e.preventDefault();
    setPwError("");
    setPwSuccess("");

    if (newPw !== confirmPw) {
      setPwError("New passwords do not match");
      return;
    }

    try {
      await changePassword(currentPw, newPw);
      setPwSuccess("Password changed successfully");
      setCurrentPw("");
      setNewPw("");
      setConfirmPw("");
    } catch (err) {
      setPwError(err instanceof ApiError ? err.message : "Failed to change password");
    }
  }

  // --- Passkeys ---

  async function handleAddPasskey(e: React.FormEvent) {
    e.preventDefault();
    setPasskeyError("");
    setPasskeyLoading(true);
    try {
      await registerPasskey(passkeyName || "My Passkey");
      setPasskeyName("");
      setShowAddPasskey(false);
      await loadPasskeys();
      await loadSecurityOverview();
    } catch (err) {
      console.error("Passkey registration failed:", err);
      setPasskeyError(
        err instanceof ApiError ? err.message : "Failed to register passkey",
      );
    } finally {
      setPasskeyLoading(false);
    }
  }

  async function handleRemovePasskey(id: string) {
    setPasskeyError("");
    try {
      await removePasskey(id);
      setPasskeys((prev) => prev.filter((p) => p.id !== id));
      await loadSecurityOverview();
    } catch (err) {
      setPasskeyError(
        err instanceof ApiError ? err.message : "Failed to remove passkey",
      );
    }
  }

  // --- TOTP ---

  async function handleEnrollTOTP() {
    setTotpError("");
    setTotpLoading(true);
    try {
      const data = await enrollTOTP();
      setTotpEnrollment(data);
    } catch (err) {
      setTotpError(
        err instanceof ApiError ? err.message : "Failed to start TOTP setup",
      );
    } finally {
      setTotpLoading(false);
    }
  }

  async function handleConfirmTOTP(e: React.FormEvent) {
    e.preventDefault();
    setTotpError("");
    setTotpLoading(true);
    try {
      await confirmTOTP(totpCode);
      setTotpEnrollment(null);
      setTotpCode("");
      await loadSecurityOverview();
    } catch (err) {
      setTotpError(
        err instanceof ApiError ? err.message : "Invalid code",
      );
    } finally {
      setTotpLoading(false);
    }
  }

  async function handleRemoveTOTP() {
    setTotpError("");
    try {
      await removeTOTP();
      await loadSecurityOverview();
    } catch (err) {
      setTotpError(
        err instanceof ApiError ? err.message : "Failed to remove TOTP",
      );
    }
  }

  // --- Sign out everywhere else ---

  async function handleRevokeOtherSessions() {
    setRevokeOthersError("");
    setRevokeOthersMessage("");
    setRevokeOthersLoading(true);
    try {
      const res = await revokeAllOtherSessions();
      const n = res.revoked ?? 0;
      setRevokeOthersMessage(
        n === 0
          ? "No other active sessions to revoke."
          : `Signed out of ${n} other ${n === 1 ? "session" : "sessions"}.`,
      );
    } catch (err) {
      setRevokeOthersError(
        err instanceof ApiError ? err.message : "Failed to sign out other sessions",
      );
    } finally {
      setRevokeOthersLoading(false);
    }
  }

  return (
    <>
      <PageHeader title="Settings" description={identity?.email} />

      {/* --- Appearance --- */}
      <div className="settings-section">
        <h3>Appearance</h3>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", maxWidth: 360 }}>
          <div>
            <div style={{ fontWeight: 500, fontSize: 14 }}>Dark mode</div>
            <div style={{ fontSize: 13, color: "var(--color-text-muted)", marginTop: 2 }}>
              {theme === "dark" ? "Currently using dark theme" : "Currently using light theme"}
            </div>
          </div>
          <button
            type="button"
            onClick={toggleTheme}
            style={{
              display: "flex", alignItems: "center", gap: 8,
              padding: "7px 14px", fontSize: 13, fontWeight: 500,
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-md)",
              background: "var(--color-bg-elevated)",
              color: "var(--color-text)",
              cursor: "pointer",
            }}
          >
            {theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}
            {theme === "dark" ? "Switch to light" : "Switch to dark"}
          </button>
        </div>
      </div>

      {/* --- Notification Preferences --- */}
      <div className="settings-section">
        <h3>Email Notifications</h3>
        {settingsSuccess && <Alert variant="success">{settingsSuccess}</Alert>}
        <p style={{ fontSize: 13, color: "var(--color-text-muted)", marginBottom: 12 }}>
          Choose how often you receive notification emails. Emails contain counts and event types only — never names, titles, or other details.
        </p>
        <div style={{ display: "flex", flexDirection: "column", gap: 10, maxWidth: 440 }}>
          {(["every_task", "daily_summary"] as const).map(freq => {
            const selected = userSettings?.notification_frequency === freq;
            return (
              <button
                key={freq}
                type="button"
                onClick={() => handleSaveNotificationFrequency(freq)}
                disabled={settingsSaving || !userSettings}
                style={{
                  display: "flex", flexDirection: "column", alignItems: "flex-start",
                  padding: "12px 14px", textAlign: "left",
                  border: `2px solid ${selected ? "var(--color-primary)" : "var(--color-border)"}`,
                  borderRadius: "var(--radius-md)",
                  background: selected ? "var(--color-primary-muted)" : "var(--color-bg-elevated)",
                  cursor: "pointer",
                  transition: "border-color 0.15s",
                }}
              >
                <span style={{ fontWeight: 600, fontSize: 14, color: "var(--color-text)" }}>
                  {freq === "every_task" ? "As things happen" : "Daily summary"}
                </span>
                <span style={{ fontSize: 13, color: "var(--color-text-muted)", marginTop: 2 }}>
                  {freq === "every_task"
                    ? "Email me as soon as a task or action item is ready for me."
                    : "Bundle everything into one email per day, sent only when there's something new."}
                </span>
              </button>
            );
          })}
        </div>
      </div>

      {/* --- Change Password --- */}
      <div className="settings-section">
        <h3>Change Password</h3>
        {security && !security.has_password ? (
          <p>
            Your account uses SSO and does not have a password set.
          </p>
        ) : (
          <form onSubmit={handleChangePassword}>
            {pwError && <Alert variant="error">{pwError}</Alert>}
            {pwSuccess && <Alert variant="success">{pwSuccess}</Alert>}
            <FormField label="Current Password">
              <input
                type="password"
                value={currentPw}
                onChange={(e) => setCurrentPw(e.target.value)}
                required
              />
            </FormField>
            <FormField label="New Password">
              <input
                type="password"
                value={newPw}
                onChange={(e) => setNewPw(e.target.value)}
                required
                minLength={8}
              />
            </FormField>
            <FormField label="Confirm New Password">
              <input
                type="password"
                value={confirmPw}
                onChange={(e) => setConfirmPw(e.target.value)}
                required
                minLength={8}
              />
            </FormField>
            <Button type="submit" size="sm">
              Change Password
            </Button>
          </form>
        )}
      </div>

      {/* --- Passkeys --- */}
      <div className="settings-section">
        <h3>Passkeys</h3>
        {passkeyError && <Alert variant="error">{passkeyError}</Alert>}
        {passkeys.length > 0 ? (
          <div className="passkey-list">
            {passkeys.map((pk) => (
              <div key={pk.id} className="passkey-item">
                <div>
                  <strong>{pk.friendly_name || "Passkey"}</strong>
                  <br />
                  <small>
                    Added {new Date(pk.created_at).toLocaleDateString()}
                    {pk.last_used_at &&
                      ` \u00b7 Last used ${new Date(pk.last_used_at).toLocaleDateString()}`}
                  </small>
                </div>
                <Button
                  size="sm"
                  className="btn-danger"
                  onClick={() => handleRemovePasskey(pk.id)}
                >
                  Remove
                </Button>
              </div>
            ))}
          </div>
        ) : (
          <p>No passkeys registered.</p>
        )}

        {showAddPasskey ? (
          <form onSubmit={handleAddPasskey} className="admin-inline-form">
            <input
              type="text"
              placeholder="Passkey name (optional)"
              value={passkeyName}
              onChange={(e) => setPasskeyName(e.target.value)}
            />
            <Button
              type="submit"
              size="sm"
              loading={passkeyLoading}
              loadingText="Registering..."
            >
              Register
            </Button>
            <Button
              type="button"
              size="sm"
              onClick={() => setShowAddPasskey(false)}
            >
              Cancel
            </Button>
          </form>
        ) : (
          <Button size="sm" onClick={() => setShowAddPasskey(true)}>
            Add passkey
          </Button>
        )}
      </div>

      {/* --- TOTP --- */}
      <div className="settings-section">
        <h3>Authenticator App</h3>
        {totpError && <Alert variant="error">{totpError}</Alert>}

        {security?.has_totp ? (
          <div>
            <p>
              Authenticator app is <strong>configured</strong>.
            </p>
            <Button size="sm" className="btn-danger" onClick={handleRemoveTOTP}>
              Remove authenticator
            </Button>
          </div>
        ) : totpEnrollment ? (
          <div>
            <p>
              Enter this secret in your authenticator app:
            </p>
            <code className="totp-secret">{totpEnrollment.secret}</code>
            <form onSubmit={handleConfirmTOTP} className="admin-inline-form">
              <input
                type="text"
                placeholder="6-digit code"
                value={totpCode}
                onChange={(e) => setTotpCode(e.target.value)}
                required
                maxLength={6}
                pattern="[0-9]{6}"
              />
              <Button
                type="submit"
                size="sm"
                loading={totpLoading}
                loadingText="Verifying..."
              >
                Verify
              </Button>
              <Button
                type="button"
                size="sm"
                onClick={() => {
                  setTotpEnrollment(null);
                  setTotpCode("");
                }}
              >
                Cancel
              </Button>
            </form>
          </div>
        ) : (
          <div>
            <p>No authenticator app configured.</p>
            <Button
              size="sm"
              onClick={handleEnrollTOTP}
              loading={totpLoading}
              loadingText="Setting up..."
            >
              Set up authenticator
            </Button>
          </div>
        )}
      </div>

      {/* --- Active Sessions --- */}
      <div className="settings-section">
        <h3>Active Sessions</h3>
        {revokeOthersError && (
          <Alert variant="error" onDismiss={() => setRevokeOthersError("")}>
            {revokeOthersError}
          </Alert>
        )}
        {revokeOthersMessage && (
          <Alert variant="success" onDismiss={() => setRevokeOthersMessage("")}>
            {revokeOthersMessage}
          </Alert>
        )}
        <p style={{ fontSize: 13, color: "var(--color-text-muted)", marginBottom: 12 }}>
          If you have signed in on another device — for example a public computer
          or a phone you no longer have — sign out of every other session in one
          click. Your current session here stays signed in.
        </p>
        <Button
          size="sm"
          className="btn-danger"
          onClick={handleRevokeOtherSessions}
          loading={revokeOthersLoading}
          loadingText="Signing out..."
        >
          Sign out everywhere else
        </Button>
      </div>
    </>
  );
}

export default SettingsPage;
