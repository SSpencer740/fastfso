import { api } from "./client";
import { base64urlDecode, base64urlEncode } from "../utils/webauthn";

// --- Security Overview ---

export interface SecurityOverview {
  has_password: boolean;
  has_totp: boolean;
  passkey_count: number;
}

export function getSecurityOverview() {
  return api<SecurityOverview>("/api/auth/security");
}

// --- Change Password ---

export function changePassword(currentPassword: string, newPassword: string) {
  return api<{ message: string }>("/api/auth/change-password", {
    method: "POST",
    body: JSON.stringify({
      current_password: currentPassword,
      new_password: newPassword,
    }),
  });
}

// --- Passkeys ---

export interface Passkey {
  id: string;
  friendly_name: string;
  created_at: string;
  last_used_at: string;
}

export function listPasskeys(basePath = "/api/auth") {
  return api<{ passkeys: Passkey[] }>(`${basePath}/passkeys`);
}

export function removePasskey(id: string, basePath = "/api/auth") {
  return api<{ message: string }>(`${basePath}/passkeys/${id}`, {
    method: "DELETE",
  });
}

// --- Passkey Registration ---

interface PublicKeyCredentialCreationOptionsJSON {
  challenge: string;
  rp: { name: string; id?: string };
  user: { id: string; name: string; displayName: string };
  pubKeyCredParams: { type: string; alg: number }[];
  timeout?: number;
  attestation?: AttestationConveyancePreference;
  authenticatorSelection?: {
    authenticatorAttachment?: AuthenticatorAttachment;
    residentKey?: ResidentKeyRequirement;
    requireResidentKey?: boolean;
    userVerification?: UserVerificationRequirement;
  };
  excludeCredentials?: { id: string; type: string; transports?: string[] }[];
}

function deserializeCreationOptions(
  json: PublicKeyCredentialCreationOptionsJSON,
): PublicKeyCredentialCreationOptions {
  return {
    challenge: base64urlDecode(json.challenge),
    rp: json.rp,
    user: {
      id: base64urlDecode(json.user.id),
      name: json.user.name,
      displayName: json.user.displayName,
    },
    pubKeyCredParams: json.pubKeyCredParams.map((p) => ({
      type: p.type as "public-key",
      alg: p.alg,
    })),
    timeout: json.timeout,
    attestation: json.attestation,
    authenticatorSelection: json.authenticatorSelection,
    excludeCredentials: json.excludeCredentials?.map((c) => ({
      id: base64urlDecode(c.id),
      type: c.type as "public-key",
      transports: c.transports as AuthenticatorTransport[],
    })),
  };
}

function serializeAttestation(credential: PublicKeyCredential) {
  const response = credential.response as AuthenticatorAttestationResponse;
  return {
    id: credential.id,
    rawId: base64urlEncode(credential.rawId),
    type: credential.type,
    response: {
      attestationObject: base64urlEncode(response.attestationObject),
      clientDataJSON: base64urlEncode(response.clientDataJSON),
    },
  };
}

export async function registerPasskey(friendlyName: string, basePath = "/api/auth") {
  const resp =
    await api<{ publicKey: PublicKeyCredentialCreationOptionsJSON }>(
      `${basePath}/passkeys/register/begin`,
      { method: "POST" },
    );
  const options = resp.publicKey;

  const credential = await navigator.credentials.create({
    publicKey: deserializeCreationOptions(options),
  });

  if (!credential) throw new Error("No credential returned");

  await api(
    `${basePath}/passkeys/register/finish?friendly_name=${encodeURIComponent(friendlyName)}`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(
        serializeAttestation(credential as PublicKeyCredential),
      ),
    },
  );
}

// --- User Settings ---

export interface UserSettings {
  notification_frequency: "every_task" | "daily_summary";
}

export function getUserSettings() {
  return api<UserSettings>("/api/auth/user-settings");
}

export function updateUserSettings(settings: UserSettings) {
  return api<UserSettings>("/api/auth/user-settings", {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

// --- TOTP ---

export interface TOTPEnrollment {
  secret: string;
  url: string;
}

export function enrollTOTP() {
  return api<TOTPEnrollment>("/api/auth/totp/enroll", { method: "POST" });
}

export function confirmTOTP(code: string) {
  return api<{ message: string }>("/api/auth/totp/confirm", {
    method: "POST",
    body: JSON.stringify({ code }),
  });
}

export function removeTOTP() {
  return api<{ message: string }>("/api/auth/totp", { method: "DELETE" });
}
