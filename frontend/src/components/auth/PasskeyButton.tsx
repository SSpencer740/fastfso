import { useState } from "react";
import { api } from "../../api/client";
import { useAuthStore } from "../../stores/authStore";
import { base64urlDecode, base64urlEncode } from "../../utils/webauthn";
import { Button } from "../ui/Button";
import { Alert } from "../ui/Alert";

interface PasskeyButtonProps {
  mode: "login" | "2fa";
}

export function PasskeyButton({ mode }: PasskeyButtonProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleClick() {
    setLoading(true);
    setError(null);
    try {
      let state: string;
      if (mode === "login") {
        state = await passkeyLogin();
      } else {
        state = await passkey2FA();
      }
      useAuthStore.setState({ state: state as "pre_tenant" | "authenticated" });
    } catch (err) {
      console.error("Passkey authentication failed:", err);
      setError("Passkey authentication failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div>
      {error && <Alert variant="error">{error}</Alert>}
      <Button
        variant="secondary"
        style={{ width: "100%" }}
        onClick={handleClick}
        loading={loading}
        loadingText="Authenticating..."
      >
        {mode === "login" ? "Sign in with Passkey" : "Verify with Passkey"}
      </Button>
    </div>
  );
}

async function passkeyLogin() {
  const resp = await api<{ publicKey: PublicKeyCredentialRequestOptionsJSON }>(
    "/api/auth/login/passkey/begin",
    { method: "POST" },
  );

  const credential = await navigator.credentials.get({
    publicKey: deserializeRequestOptions(resp.publicKey),
  });

  if (!credential) throw new Error("No credential returned");

  const result = await api<{ state: string }>("/api/auth/login/passkey/finish", {
    method: "POST",
    body: JSON.stringify(serializeAssertion(credential as PublicKeyCredential)),
  });
  return result.state;
}

async function passkey2FA() {
  const resp = await api<{ publicKey: PublicKeyCredentialRequestOptionsJSON }>(
    "/api/auth/2fa/passkey/begin",
    { method: "POST" },
  );

  const credential = await navigator.credentials.get({
    publicKey: deserializeRequestOptions(resp.publicKey),
  });

  if (!credential) throw new Error("No credential returned");

  const result = await api<{ state: string }>("/api/auth/2fa/passkey/finish", {
    method: "POST",
    body: JSON.stringify(serializeAssertion(credential as PublicKeyCredential)),
  });
  return result.state;
}

// WebAuthn JSON serialization helpers
interface PublicKeyCredentialRequestOptionsJSON {
  challenge: string;
  timeout?: number;
  rpId?: string;
  allowCredentials?: { id: string; type: string; transports?: string[] }[];
  userVerification?: UserVerificationRequirement;
}

function deserializeRequestOptions(
  json: PublicKeyCredentialRequestOptionsJSON,
): PublicKeyCredentialRequestOptions {
  return {
    challenge: base64urlDecode(json.challenge),
    timeout: json.timeout,
    rpId: json.rpId,
    allowCredentials: json.allowCredentials?.map((c) => ({
      id: base64urlDecode(c.id),
      type: c.type as "public-key",
      transports: c.transports as AuthenticatorTransport[],
    })),
    userVerification: json.userVerification,
  };
}

function serializeAssertion(credential: PublicKeyCredential) {
  const response = credential.response as AuthenticatorAssertionResponse;
  return {
    id: credential.id,
    rawId: base64urlEncode(credential.rawId),
    type: credential.type,
    response: {
      authenticatorData: base64urlEncode(response.authenticatorData),
      clientDataJSON: base64urlEncode(response.clientDataJSON),
      signature: base64urlEncode(response.signature),
      userHandle: response.userHandle
        ? base64urlEncode(response.userHandle)
        : null,
    },
  };
}
