import { useCallback, useEffect, useRef, useState } from "react";
import Markdown from "./Markdown";
import { ApiError, chatEnabled, fetchAsk, streamChat } from "../api";

const SESSION_KEY = "org-cost-chat-session";
const MCP_SETUP_URL =
  "https://github.com/kaskol10/org-cost-api/blob/main/docs/mcp-setup.md";

const SUGGESTED_PROMPTS = [
  "What are our top cost drivers?",
  "Where can we save money?",
  "How did spend change vs last month?",
  "Any unattached EBS volumes?",
];

type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  tools?: string[];
  ceCallsUsed?: number;
};

export type AskRequest = {
  id: number;
  question: string;
};

interface Props {
  askRequest?: AskRequest | null;
  onAskRequestHandled?: () => void;
}

function newId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

export default function AskPanel({ askRequest, onAskRequestHandled }: Props) {
  const [question, setQuestion] = useState("");
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [refresh, setRefresh] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sessionId, setSessionId] = useState<string | null>(() =>
    sessionStorage.getItem(SESSION_KEY)
  );
  const abortRef = useRef<AbortController | null>(null);
  const threadEndRef = useRef<HTMLDivElement | null>(null);
  const useChat = chatEnabled();

  function persistSession(id: string) {
    setSessionId(id);
    sessionStorage.setItem(SESSION_KEY, id);
  }

  const submit = useCallback(
    async (q: string) => {
      const trimmed = q.trim();
      if (!trimmed) return;
      abortRef.current?.abort();
      const ac = new AbortController();
      abortRef.current = ac;
      setLoading(true);
      setError(null);
      setQuestion("");
      setMessages((prev) => [...prev, { id: newId(), role: "user", content: trimmed }]);
      try {
        if (useChat) {
          const assistantId = newId();
          setMessages((prev) => [
            ...prev,
            { id: assistantId, role: "assistant", content: "" },
          ]);
          await streamChat(
            sessionId,
            trimmed,
            refresh,
            (ev) => {
              if (ev.type === "error") throw new Error(ev.message);
              if (ac.signal.aborted) return;
              if (ev.type === "session" && ev.session_id) {
                persistSession(ev.session_id);
                return;
              }
              if (ev.type === "delta" && ev.text) {
                setMessages((prev) =>
                  prev.map((m) =>
                    m.id === assistantId ? { ...m, content: m.content + ev.text } : m
                  )
                );
              }
              if (ev.type === "done") {
                setMessages((prev) =>
                  prev.map((m) =>
                    m.id === assistantId
                      ? {
                          ...m,
                          tools:
                            ev.tools_called.length > 0 ? ev.tools_called : m.tools,
                          ceCallsUsed: ev.ce_calls_used,
                        }
                      : m
                  )
                );
              }
            },
            ac.signal
          );
        } else {
          const res = await fetchAsk(trimmed, refresh, ac.signal);
          if (ac.signal.aborted) return;
          setMessages((prev) => [
            ...prev,
            {
              id: newId(),
              role: "assistant",
              content: res.answer,
              tools: res.intent ? [res.intent] : undefined,
              ceCallsUsed: res.ce_calls_used ?? 0,
            },
          ]);
        }
      } catch (e) {
        if (ac.signal.aborted || (e instanceof DOMException && e.name === "AbortError")) {
          return;
        }
        if (e instanceof ApiError && e.status === 401) {
          setError(
            "Authentication failed — check VITE_API_TOKEN matches the backend api_token."
          );
        } else {
          setError(e instanceof Error ? e.message : "Failed to get answer");
        }
        setMessages((prev) => {
          const last = prev[prev.length - 1];
          if (last?.role === "assistant" && last.content === "") {
            return prev.slice(0, -1);
          }
          return prev;
        });
      } finally {
        if (!ac.signal.aborted) {
          setLoading(false);
        }
      }
    },
    [refresh, sessionId, useChat]
  );

  useEffect(() => {
    return () => {
      abortRef.current?.abort();
    };
  }, []);

  useEffect(() => {
    threadEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, loading]);

  useEffect(() => {
    if (!askRequest?.question.trim()) return;
    void submit(askRequest.question).then(() => {
      onAskRequestHandled?.();
    });
    // Only re-run when a new highlight/suggestion injects a question.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [askRequest?.id]);

  function clearThread() {
    abortRef.current?.abort();
    setMessages([]);
    setSessionId(null);
    sessionStorage.removeItem(SESSION_KEY);
    setError(null);
  }

  return (
    <section className="panel ask-panel">
      <div className="ask-header">
        <div>
          <h2>Chat about your AWS costs</h2>
          <p className="category-desc">
            {useChat
              ? "Your main workspace — ask follow-ups, drill into accounts, and explore savings. Data comes from your org API + LLM."
              : "Ask in plain language — answers use cached dashboard data. Enable chat for multi-turn LLM."}
          </p>
        </div>
        {messages.length > 0 && (
          <button
            type="button"
            className="ask-clear-btn"
            onClick={clearThread}
            disabled={loading}
          >
            Clear chat
          </button>
        )}
      </div>

      {messages.length === 0 && !loading && (
        <p className="ask-empty-hint">
          Start from a highlight above, a suggested prompt below, or type your own question.
        </p>
      )}

      {messages.length > 0 && (
        <div className="ask-thread" role="log" aria-live="polite">
          {messages.map((msg) => (
            <div key={msg.id} className={`ask-message ask-message-${msg.role}`}>
              <span className="ask-message-role">
                {msg.role === "user" ? "You" : "Assistant"}
              </span>
              {msg.role === "assistant" && (msg.tools?.length || msg.ceCallsUsed) ? (
                <div className="ask-meta">
                  {msg.tools && msg.tools.length > 0 && (
                    <span>Tools: {msg.tools.join(", ")}</span>
                  )}
                  {msg.ceCallsUsed != null && msg.ceCallsUsed > 0 && (
                    <span>CE calls: {msg.ceCallsUsed}</span>
                  )}
                </div>
              ) : null}
              <div className="ask-markdown">
                {msg.content ? (
                  <Markdown className="ask-markdown-body">{msg.content}</Markdown>
                ) : loading ? (
                  <span className="ask-typing">Thinking…</span>
                ) : null}
              </div>
            </div>
          ))}
          <div ref={threadEndRef} />
        </div>
      )}

      <form
        className="ask-form"
        onSubmit={(e) => {
          e.preventDefault();
          void submit(question);
        }}
      >
        <input
          type="text"
          className="ask-input"
          placeholder="Ask about spend, trends, waste, or savings…"
          value={question}
          onChange={(e) => setQuestion(e.target.value)}
          disabled={loading}
        />
        <button className="btn" type="submit" disabled={loading || !question.trim()}>
          {loading ? "Working…" : "Send"}
        </button>
      </form>

      <label className="ask-refresh">
        <input
          type="checkbox"
          checked={refresh}
          onChange={(e) => setRefresh(e.target.checked)}
          disabled={loading}
        />
        Refresh from AWS (slower, uses Cost Explorer)
      </label>

      <div className="ask-chips">
        {SUGGESTED_PROMPTS.map((prompt) => (
          <button
            key={prompt}
            type="button"
            className="ask-chip"
            disabled={loading}
            onClick={() => void submit(prompt)}
          >
            {prompt}
          </button>
        ))}
      </div>

      {error && <div className="error">{error}</div>}

      {loading && messages.length === 0 && (
        <p className="loading">Thinking…</p>
      )}

      <p className="ask-footer">
        Using Hermes or Cursor?{" "}
        <a href={MCP_SETUP_URL} target="_blank" rel="noopener noreferrer">
          MCP setup
        </a>
      </p>
    </section>
  );
}
