import { useAuthStore } from "../stores/authStore";

type ErrorSource =
  | "window.onerror"
  | "unhandledrejection"
  | "error_boundary"
  | "manual";

interface LastApiCall {
  method: string;
  url: string;
  status: number;
  response_body: string | null;
}

interface ErrorReport {
  message: string;
  stack: string | null;
  url: string;
  timestamp: string;
  source: ErrorSource;
  identity_id: string | null;
  user_id: string | null;
  tenant_id: string | null;
  last_api_call: LastApiCall | null;
  component_name: string | null;
}

// Module-level state
let lastApiCall: LastApiCall | null = null;
const rateLimitTimestamps: number[] = [];
let dedupFingerprints = new Set<string>();

const RATE_LIMIT_MAX = 5;
const RATE_LIMIT_WINDOW_MS = 60_000;

export function setLastApiCall(call: LastApiCall): void {
  lastApiCall = call;
}

function fingerprint(message: string, stack: string | null): string {
  return `${message}::${stack?.split("\n")[0] ?? ""}`;
}

function isRateLimited(): boolean {
  const now = Date.now();
  // Remove timestamps outside the window
  while (rateLimitTimestamps.length > 0 && rateLimitTimestamps[0] < now - RATE_LIMIT_WINDOW_MS) {
    rateLimitTimestamps.shift();
  }
  if (rateLimitTimestamps.length >= RATE_LIMIT_MAX) {
    return true;
  }
  rateLimitTimestamps.push(now);
  return false;
}

export function reportError(
  error: unknown,
  source: ErrorSource,
  componentName?: string,
): void {
  if (import.meta.env.DEV) {
    console.error(`[${source}]`, error);
    return;
  }

  const message =
    error instanceof Error ? error.message : String(error);
  const stack =
    error instanceof Error ? error.stack ?? null : null;

  const fp = fingerprint(message, stack);
  if (dedupFingerprints.has(fp)) return;
  if (isRateLimited()) return;
  dedupFingerprints.add(fp);

  const auth = useAuthStore.getState();

  const report: ErrorReport = {
    message,
    stack,
    url: window.location.href,
    timestamp: new Date().toISOString(),
    source,
    identity_id: auth.identity?.id ?? null,
    user_id: auth.user?.id ?? null,
    tenant_id: auth.user?.tenant_id ?? null,
    last_api_call: lastApiCall,
    component_name: componentName ?? null,
  };

  fetch("/api/v1/errors", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(report),
    keepalive: true,
  }).catch(() => {
    // Silently ignore reporting failures
  });
}

export function initErrorReporting(): void {
  if (import.meta.env.DEV) return;

  window.onerror = (
    message: string | Event,
    _source?: string,
    _lineno?: number,
    _colno?: number,
    error?: Error,
  ) => {
    reportError(error ?? message, "window.onerror");
  };

  window.onunhandledrejection = (event: PromiseRejectionEvent) => {
    reportError(event.reason, "unhandledrejection");
  };

  // Clear dedup fingerprints every 60 seconds
  setInterval(() => {
    dedupFingerprints = new Set();
  }, RATE_LIMIT_WINDOW_MS);
}
