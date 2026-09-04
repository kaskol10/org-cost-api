import { useEffect, useState } from "react";
import { chatEnabled, fetchChatHealth, type ChatHealthResponse } from "../api";

const CHAT_SETUP_URL =
  "https://github.com/kaskol10/org-cost-api/blob/main/docs/chat-setup.md";

type Props = {
  apiOk: boolean;
  apiError?: string | null;
};

type Status = "ok" | "warn" | "error" | "unknown";

function pill(status: Status, label: string, detail?: string) {
  return (
    <span className={`chat-status-pill chat-status-${status}`} title={detail}>
      {label}
    </span>
  );
}

export default function ChatStatusBanner({ apiOk, apiError }: Props) {
  const [health, setHealth] = useState<ChatHealthResponse | null>(null);
  const [chatReachable, setChatReachable] = useState<boolean | null>(null);
  const [chatError, setChatError] = useState<string | null>(null);

  const chatConfigured = chatEnabled();

  useEffect(() => {
    if (!chatConfigured) {
      setChatReachable(false);
      return;
    }
    let cancelled = false;
    fetchChatHealth()
      .then((h) => {
        if (!cancelled) {
          setHealth(h);
          setChatReachable(true);
          setChatError(null);
        }
      })
      .catch((e) => {
        if (!cancelled) {
          setChatReachable(false);
          setChatError(e instanceof Error ? e.message : "Chat unreachable");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [chatConfigured]);

  const llmOk =
    health?.suggestions_llm === "true" || health?.llm_configured === "true";

  const apiStatus: Status = apiOk ? "ok" : apiError ? "error" : "unknown";
  const chatStatus: Status = !chatConfigured
    ? "warn"
    : chatReachable === null
      ? "unknown"
      : chatReachable
        ? "ok"
        : "error";
  const llmStatus: Status = !chatConfigured
    ? "warn"
    : chatReachable && llmOk
      ? "ok"
      : chatReachable
        ? "warn"
        : "unknown";

  return (
    <div className="chat-status-banner" role="status">
      <span className="chat-status-label">Setup</span>
      {pill(
        apiStatus,
        apiOk ? "API connected" : "API issue",
        apiError ?? undefined
      )}
      {pill(
        chatStatus,
        chatConfigured
          ? chatReachable
            ? "Chat agent ok"
            : chatReachable === false
              ? "Chat unreachable"
              : "Checking chat…"
          : "Chat not configured",
        chatError ?? undefined
      )}
      {pill(
        llmStatus,
        llmOk ? "LLM ready" : chatConfigured ? "LLM not configured" : "LLM —",
        health?.hint
      )}
      {!chatConfigured && (
        <a className="chat-status-link" href={CHAT_SETUP_URL} target="_blank" rel="noreferrer">
          Enable chat
        </a>
      )}
      {chatConfigured && chatReachable && !llmOk && health?.hint && (
        <span className="chat-status-hint">{health.hint}</span>
      )}
    </div>
  );
}
