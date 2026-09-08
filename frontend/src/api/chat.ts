function getCsrfToken(): string | null {
  const match = document.cookie.match(/(?:^|; )fastfso_csrf=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : null;
}

export interface HistoryMessage {
  role: "user" | "assistant";
  content: string;
  created_at: string;
}

export async function getChatHistory(): Promise<HistoryMessage[]> {
  const res = await fetch("/api/v1/me/chat/history", { credentials: "include" });
  if (!res.ok) return [];
  const data = await res.json() as { messages: HistoryMessage[] };
  return data.messages ?? [];
}

export interface ChatUsage {
  used: number;
  limit: number;
}

export async function getChatUsage(): Promise<ChatUsage | null> {
  const res = await fetch("/api/v1/me/chat/usage", { credentials: "include" });
  if (!res.ok) return null;
  return await res.json() as ChatUsage;
}

export class ChatLimitError extends Error {
  limit: number;
  constructor(limit: number) {
    super("Daily AI chat limit reached");
    this.limit = limit;
  }
}

export async function* streamChat(message: string): AsyncGenerator<string> {
  const csrf = getCsrfToken();
  const res = await fetch("/api/v1/me/chat", {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(csrf ? { "X-CSRF-Token": csrf } : {}),
    },
    body: JSON.stringify({ message }),
  });

  if (!res.ok) {
    if (res.status === 503) {
      throw new Error("The AI assistant is not available in this environment.");
    }
    if (res.status === 429) {
      const body = await res.json().catch(() => ({})) as { limit?: number };
      throw new ChatLimitError(body.limit ?? 0);
    }
    throw new Error(`Chat error: ${res.status}`);
  }

  const reader = res.body!.getReader();
  const decoder = new TextDecoder();
  let buf = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });

    const lines = buf.split("\n");
    buf = lines.pop() ?? "";

    for (const line of lines) {
      if (!line.startsWith("data: ")) continue;
      const payload = line.slice(6).trim();
      if (!payload) continue;
      try {
        const obj = JSON.parse(payload) as Record<string, string>;
        if (obj.done) return;
        if (obj.error) throw new Error(obj.error);
        if (obj.text) yield obj.text;
      } catch {
        // skip malformed chunks
      }
    }
  }
}
