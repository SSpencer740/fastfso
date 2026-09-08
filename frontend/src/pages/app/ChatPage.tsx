import { useState, useRef, useEffect } from "react";
import { Send, Info, X } from "lucide-react";
import { PageHeader } from "../../components/ui/PageHeader";
import { streamChat, getChatHistory, getChatUsage, ChatLimitError, type ChatUsage } from "../../api/chat";

interface Message {
  role: "user" | "assistant";
  text: string;
}

export function ChatPage() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState("");
  const [loadingHistory, setLoadingHistory] = useState(true);
  const [usage, setUsage] = useState<ChatUsage | null>(null);
  const [showPrivacy, setShowPrivacy] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    getChatHistory()
      .then(history => {
        setMessages(history.map(m => ({ role: m.role, text: m.content })));
      })
      .finally(() => setLoadingHistory(false));
    void getChatUsage().then(setUsage);
  }, []);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  async function handleSend() {
    const text = input.trim();
    if (!text || streaming) return;

    setInput("");
    setError("");
    setMessages(prev => [...prev, { role: "user", text }]);
    setStreaming(true);

    setMessages(prev => [...prev, { role: "assistant", text: "" }]);

    try {
      for await (const chunk of streamChat(text)) {
        setMessages(prev => {
          const updated = [...prev];
          updated[updated.length - 1] = {
            role: "assistant",
            text: updated[updated.length - 1].text + chunk,
          };
          return updated;
        });
      }
    } catch (e) {
      if (e instanceof ChatLimitError) {
        setError(`Daily AI chat limit reached (${e.limit} messages). Limit resets at 00:00 UTC.`);
      } else {
        setError("Something went wrong. Please try again.");
      }
      setMessages(prev => prev.slice(0, -2));
    } finally {
      setStreaming(false);
      void getChatUsage().then(setUsage);
    }
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      void handleSend();
    }
  }

  const limitReached = usage !== null && usage.used >= usage.limit;

  return (
    <>
      <PageHeader
        title="FSO Assistant"
        description="Ask questions about security policies or your action items"
      />

      <div style={{
        display: "flex",
        alignItems: "center",
        gap: 12,
        marginBottom: 8,
        fontSize: 12,
        color: "var(--color-text-muted)",
      }}>
        {usage && (
          <span>
            {usage.used} of {usage.limit} messages today
          </span>
        )}
        <button
          type="button"
          onClick={() => setShowPrivacy(true)}
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 4,
            padding: "2px 6px",
            background: "transparent",
            border: "1px solid var(--color-border)",
            borderRadius: "var(--radius-sm)",
            color: "var(--color-text-muted)",
            cursor: "pointer",
            fontSize: 12,
          }}
        >
          <Info size={12} />
          Privacy
        </button>
      </div>

      <div style={{ display: "flex", flexDirection: "column", flex: 1, minHeight: 0 }}>
        {/* Message thread */}
        <div style={{
          flex: 1,
          overflowY: "auto",
          display: "flex",
          flexDirection: "column",
          gap: 16,
          padding: "4px 0 16px",
        }}>
          {loadingHistory && (
            <div style={{ color: "var(--color-text-muted)", fontSize: 14, textAlign: "center", marginTop: 48 }}>
              Loading conversation history…
            </div>
          )}
          {!loadingHistory && messages.length === 0 && (
            <div style={{ color: "var(--color-text-muted)", fontSize: 14, textAlign: "center", marginTop: 48 }}>
              Ask a question about security policies, reporting requirements, or your action items.
            </div>
          )}
          {messages.map((msg, i) => (
            <div key={i} style={{
              display: "flex",
              justifyContent: msg.role === "user" ? "flex-end" : "flex-start",
            }}>
              <div style={{
                maxWidth: "80%",
                padding: "10px 14px",
                borderRadius: msg.role === "user" ? "16px 16px 4px 16px" : "16px 16px 16px 4px",
                background: msg.role === "user" ? "var(--color-primary)" : "var(--color-bg-elevated)",
                color: msg.role === "user" ? "#fff" : "var(--color-text)",
                border: msg.role === "assistant" ? "1px solid var(--color-border)" : "none",
                fontSize: 14,
                lineHeight: 1.6,
                whiteSpace: "pre-wrap",
              }}>
                {msg.text || (msg.role === "assistant" && streaming ? (
                  <span style={{ opacity: 0.5 }}>Thinking…</span>
                ) : null)}
              </div>
            </div>
          ))}
          <div ref={bottomRef} />
        </div>

        {error && (
          <div style={{ color: "var(--color-danger)", fontSize: 13, marginBottom: 8 }}>{error}</div>
        )}

        {/* Input */}
        <div style={{
          display: "flex",
          gap: 8,
          alignItems: "flex-end",
          borderTop: "1px solid var(--color-border)",
          paddingTop: 12,
        }}>
          <textarea
            value={input}
            onChange={e => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={limitReached
              ? "Daily limit reached — resets at 00:00 UTC"
              : "Ask a question… (Enter to send, Shift+Enter for new line)"}
            disabled={streaming || limitReached}
            rows={2}
            style={{
              flex: 1,
              resize: "none",
              padding: "10px 12px",
              borderRadius: "var(--radius-md)",
              border: "1px solid var(--color-border)",
              background: "var(--color-bg-elevated)",
              color: "var(--color-text)",
              fontSize: 14,
              lineHeight: 1.5,
              fontFamily: "inherit",
            }}
          />
          <button
            onClick={() => void handleSend()}
            disabled={!input.trim() || streaming || limitReached}
            style={{
              padding: "10px 14px",
              borderRadius: "var(--radius-md)",
              background: "var(--color-primary)",
              color: "#fff",
              border: "none",
              cursor: input.trim() && !streaming && !limitReached ? "pointer" : "not-allowed",
              opacity: input.trim() && !streaming && !limitReached ? 1 : 0.5,
              display: "flex",
              alignItems: "center",
              gap: 6,
              fontSize: 14,
            }}
          >
            <Send size={16} />
          </button>
        </div>
      </div>

      {showPrivacy && (
        <div
          onClick={() => setShowPrivacy(false)}
          style={{
            position: "fixed",
            inset: 0,
            background: "rgba(0,0,0,0.5)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            zIndex: 1000,
          }}
        >
          <div
            onClick={e => e.stopPropagation()}
            style={{
              background: "var(--color-bg)",
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-md)",
              padding: 24,
              maxWidth: 520,
              width: "90%",
              maxHeight: "80vh",
              overflowY: "auto",
              position: "relative",
              fontSize: 14,
              lineHeight: 1.6,
            }}
          >
            <button
              onClick={() => setShowPrivacy(false)}
              style={{
                position: "absolute",
                top: 12,
                right: 12,
                background: "transparent",
                border: "none",
                cursor: "pointer",
                color: "var(--color-text-muted)",
              }}
              aria-label="Close"
            >
              <X size={18} />
            </button>
            <h3 style={{ marginTop: 0 }}>About your conversations with the AI assistant</h3>
            <ul style={{ paddingLeft: 20 }}>
              <li>
                Your messages are processed by Google Cloud's Vertex AI (Gemini), running in the
                same Google Cloud project that hosts fastFSO, within the US region.
              </li>
              <li>
                Under Google's enterprise terms, <strong>your prompts and responses are not used
                to train Google's AI models</strong>.
              </li>
              <li>No data is sent to consumer AI services or third-party AI vendors.</li>
              <li>
                Chat history is scoped to your tenant and stored in fastFSO's database. It is
                retained for 30 days.
              </li>
              <li>
                <strong>Do not paste classified information into the assistant.</strong> It is not
                approved for CUI or classified content.
              </li>
            </ul>
          </div>
        </div>
      )}
    </>
  );
}

export default ChatPage;
