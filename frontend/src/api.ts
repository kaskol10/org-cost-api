import type {
  AskResponse,
  ReportResponse,
  ServiceDetail,
  ServiceTagDeltaResponse,
  ServiceTagTotalsResponse,
  SuggestionsResponse,
} from "./types";

export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

/** Base URL for API calls. Empty string uses same-origin relative paths. */
export function apiBase(): string {
  const raw = import.meta.env.VITE_API_URL?.trim();
  if (!raw) return "";
  return raw.replace(/\/+$/, "");
}

/** Build a full API URL from a path (e.g. "/api/report"). */
export function apiUrl(path: string): string {
  const base = apiBase();
  if (!base) return path;
  return `${base}${path}`;
}

async function parseError(res: Response): Promise<never> {
  const body = await res.json().catch(() => ({}));
  const msg = (body as { error?: string }).error ?? `Request failed (${res.status})`;
  throw new ApiError(msg, res.status);
}

export function apiHeaders(extra: Record<string, string> = {}): Record<string, string> {
  const headers: Record<string, string> = { ...extra };
  const token = import.meta.env.VITE_API_TOKEN?.trim();
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return headers;
}

export async function fetchMeta(): Promise<{ demo: boolean }> {
  const res = await fetch(apiUrl("/api/meta"), { headers: apiHeaders() });
  if (!res.ok) await parseError(res);
  return res.json();
}

export async function fetchReport(): Promise<ReportResponse> {
  const res = await fetch(apiUrl("/api/report"), { headers: apiHeaders() });
  if (!res.ok) await parseError(res);
  return res.json();
}

export async function fetchReportFresh(): Promise<ReportResponse> {
  const res = await fetch(apiUrl("/api/report?refresh=1"), { headers: apiHeaders() });
  if (!res.ok) await parseError(res);
  return res.json();
}

export async function fetchAsk(
  question: string,
  refresh = false,
  signal?: AbortSignal
): Promise<AskResponse> {
  const res = await fetch(apiUrl("/api/ask"), {
    method: "POST",
    headers: apiHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify({ question, refresh }),
    signal,
  });
  if (!res.ok) await parseError(res);
  return res.json();
}

/** Base URL for the chat agent. Empty when conversational Ask is disabled. */
export function chatBase(): string {
  const raw = import.meta.env.VITE_CHAT_URL?.trim();
  if (raw) return raw.replace(/\/+$/, "");
  // Vite dev proxy: /chat → localhost:8090 (see vite.config.ts)
  if (import.meta.env.DEV) return "/chat";
  return "";
}

export function chatEnabled(): boolean {
  return chatBase() !== "";
}

export type ChatHealthResponse = {
  status: string;
  suggestions_llm?: string;
  llm_configured?: string;
  llm_provider?: string;
  llm_model?: string;
  llm_base_url?: string;
  hint?: string;
};

export async function fetchChatHealth(signal?: AbortSignal): Promise<ChatHealthResponse> {
  const base = chatBase();
  if (!base) {
    throw new Error("Chat is not configured (set VITE_CHAT_URL)");
  }
  const res = await fetch(`${base}/v1/chat/health`, { signal });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { detail?: string };
    const msg = body.detail ?? `Chat health failed (${res.status})`;
    throw new ApiError(msg, res.status);
  }
  return res.json();
}

export type ChatEvent =
  | { type: "session"; session_id: string }
  | { type: "delta"; text: string }
  | { type: "tool_start"; name: string }
  | { type: "tool_end"; name: string }
  | { type: "done"; tools_called: string[]; ce_calls_used: number }
  | { type: "error"; message: string };

/** Parse one SSE block (event + data lines) into a ChatEvent. */
export function parseSSEBlock(block: string): ChatEvent | null {
  let eventName = "message";
  let dataLine = "";
  for (const line of block.split("\n")) {
    if (line.startsWith("event:")) {
      eventName = line.slice(6).trim();
    } else if (line.startsWith("data:")) {
      dataLine = line.slice(5).trim();
    }
  }
  if (!dataLine) return null;
  let payload: Record<string, unknown>;
  try {
    payload = JSON.parse(dataLine) as Record<string, unknown>;
  } catch {
    return null;
  }
  switch (eventName) {
    case "session":
      return { type: "session", session_id: String(payload.session_id ?? "") };
    case "delta":
      return { type: "delta", text: String(payload.text ?? "") };
    case "tool_start":
      return { type: "tool_start", name: String(payload.name ?? "") };
    case "tool_end":
      return { type: "tool_end", name: String(payload.name ?? "") };
    case "done":
      return {
        type: "done",
        tools_called: Array.isArray(payload.tools_called)
          ? payload.tools_called.map(String)
          : [],
        ce_calls_used: Number(payload.ce_calls_used ?? 0),
      };
    case "error":
      return { type: "error", message: String(payload.message ?? "Chat error") };
    default:
      return null;
  }
}

/** Consume an SSE response body and invoke onEvent for each parsed event. */
export async function parseSSEStream(
  body: ReadableStream<Uint8Array>,
  onEvent: (ev: ChatEvent) => void,
  signal?: AbortSignal
): Promise<void> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  try {
    while (true) {
      if (signal?.aborted) {
        throw new DOMException("Aborted", "AbortError");
      }
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const parts = buffer.split("\n\n");
      buffer = parts.pop() ?? "";
      for (const part of parts) {
        const trimmed = part.trim();
        if (!trimmed) continue;
        const ev = parseSSEBlock(trimmed);
        if (ev) onEvent(ev);
      }
    }
    const tail = buffer.trim();
    if (tail) {
      const ev = parseSSEBlock(tail);
      if (ev) onEvent(ev);
    }
  } finally {
    reader.releaseLock();
  }
}

export async function streamChat(
  sessionId: string | null,
  message: string,
  refresh: boolean,
  onEvent: (ev: ChatEvent) => void,
  signal?: AbortSignal
): Promise<void> {
  const base = chatBase();
  if (!base) {
    throw new Error("Chat is not configured (set VITE_CHAT_URL)");
  }
  const res = await fetch(`${base}/v1/chat`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ session_id: sessionId, message, refresh }),
    signal,
  });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { detail?: string };
    const msg = body.detail ?? `Chat failed (${res.status})`;
    throw new ApiError(msg, res.status);
  }
  if (!res.body) {
    throw new Error("Chat response had no body");
  }
  await parseSSEStream(res.body, onEvent, signal);
}

export async function fetchExplainedSuggestions(
  refresh = false,
  report?: Pick<ReportResponse, "dashboard" | "trends" | "suggestions">,
  signal?: AbortSignal
): Promise<SuggestionsResponse> {
  const base = chatBase();
  if (!base) {
    throw new Error("Chat is not configured (set VITE_CHAT_URL)");
  }
  const res = await fetch(`${base}/v1/suggestions/explain`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      refresh,
      report: report
        ? {
            dashboard: report.dashboard,
            trends: report.trends,
            suggestions: report.suggestions,
          }
        : undefined,
    }),
    signal,
  });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { detail?: string };
    const msg = body.detail ?? `Suggestions explain failed (${res.status})`;
    throw new ApiError(msg, res.status);
  }
  return res.json();
}

export async function fetchServiceDetail(params: {
  account_id: string;
  service: string;
  start?: string;
  end?: string;
}): Promise<ServiceDetail> {
  const q = new URLSearchParams();
  q.set("account_id", params.account_id);
  q.set("service", params.service);
  if (params.start) q.set("start", params.start);
  if (params.end) q.set("end", params.end);

  const res = await fetch(apiUrl(`/api/service-detail?${q.toString()}`), {
    headers: apiHeaders(),
  });
  if (!res.ok) await parseError(res);
  return res.json();
}

export async function fetchServiceTagDelta(params: {
  service: string;
  account_id?: string;
  start?: string;
  end?: string;
  tag_key?: string;
  top_accounts?: number;
  signal?: AbortSignal;
}): Promise<ServiceTagDeltaResponse> {
  const q = new URLSearchParams();
  q.set("service", params.service);
  if (params.account_id) q.set("account_id", params.account_id);
  if (params.start) q.set("start", params.start);
  if (params.end) q.set("end", params.end);
  q.set("tag_key", params.tag_key ?? "Name");
  if (params.top_accounts != null) q.set("top_accounts", String(params.top_accounts));

  const res = await fetch(apiUrl(`/api/service-tag-delta?${q.toString()}`), {
    headers: apiHeaders(),
    signal: params.signal,
  });
  if (!res.ok) await parseError(res);
  return res.json();
}

export async function fetchServiceTagTotals(params: {
  service: string;
  account_id?: string;
  start?: string;
  end?: string;
  tag_key?: string;
  top_buckets?: number;
  signal?: AbortSignal;
}): Promise<ServiceTagTotalsResponse> {
  const q = new URLSearchParams();
  q.set("service", params.service);
  if (params.account_id) q.set("account_id", params.account_id);
  if (params.start) q.set("start", params.start);
  if (params.end) q.set("end", params.end);
  q.set("tag_key", params.tag_key ?? "Name");
  if (params.top_buckets != null) q.set("top_buckets", String(params.top_buckets));

  const res = await fetch(apiUrl(`/api/service-tag-totals?${q.toString()}`), {
    headers: apiHeaders(),
    signal: params.signal,
  });
  if (!res.ok) await parseError(res);
  return res.json();
}
