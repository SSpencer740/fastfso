import { setLastApiCall } from "../lib/errorReporter";

function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`));
  return match ? decodeURIComponent(match[1]) : null;
}

export class ApiError extends Error {
  status: number;
  body: Record<string, unknown>;

  constructor(status: number, body: Record<string, unknown>) {
    super((body.error as string) || `API error ${status}`);
    this.status = status;
    this.body = body;
  }
}

export async function api<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };

  // Attach CSRF token for non-GET requests
  if (options.method && options.method !== "GET") {
    const csrf = getCookie("fastfso_csrf");
    if (csrf) {
      headers["X-CSRF-Token"] = csrf;
    }
  }

  const method = options.method ?? "GET";

  const res = await fetch(path, {
    ...options,
    headers,
    credentials: "include",
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: "unknown error" }));
    const bodyStr = JSON.stringify(body);
    setLastApiCall({
      method,
      url: path,
      status: res.status,
      response_body: bodyStr.length > 1024 ? bodyStr.slice(0, 1024) : bodyStr,
    });
    throw new ApiError(res.status, body);
  }

  setLastApiCall({ method, url: path, status: res.status, response_body: null });
  if (res.status === 204 || res.headers.get("content-length") === "0") {
    return undefined as T;
  }
  return res.json();
}
