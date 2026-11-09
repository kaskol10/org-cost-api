import type {
  AskResponse,
  ReportResponse,
  ServiceDetail,
  ServiceTagDeltaResponse,
  ServiceTagTotalsResponse,
  SuggestionsResponse,
} from "./types";
import type { components, paths } from "./generated/openapi";

/** Versioned base path of the org-cost API (see openapi.yaml). `/api` is a legacy alias. */
export const API_PREFIX = "/api/v1";

/** Spec paths that have a GET operation. */
type GetPath = {
  [P in keyof paths]: NonNullable<paths[P]["get"]> extends never ? never : P;
}[keyof paths];

/** Query parameters the spec declares for a GET path. */
type GetQuery<P extends GetPath> = NonNullable<paths[P]["get"]> extends {
  parameters: { query?: infer Q };
}
  ? Q
  : never;

type AskRequestBody = components["schemas"]["AskRequest"];

function queryString(query: object | undefined): string {
  if (!query) return "";
  const q = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === "") continue;
    q.set(key, String(value));
  }
  const s = q.toString();
  return s ? `?${s}` : "";
}

/**
 * GET a spec-declared path. `path` and `query` are type-checked against
 * src/generated/openapi.ts (regenerate with `npm run gen:api`), so the UI can
 * only call endpoints and parameters the public API contract documents.
 * Response bodies are typed by the callers with the richer types in ./types.
 */
export async function apiGet<P extends GetPath>(
  path: P,
  query?: GetQuery<P>,
  signal?: AbortSignal
): Promise<unknown> {
  const res = await fetch(apiUrl(`${path}${queryString(query ?? undefined)}`), {
    headers: apiHeaders(),
    signal,
  });
  if (!res.ok) await parseError(res);
  return res.json();
}

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

/** Build a full API URL from a path (e.g. "/api/v1/report"). */
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
  return (await apiGet("/api/v1/meta")) as { demo: boolean };
}

export type PeriodPreset = "30d" | "mtd";

export async function fetchReport(period: PeriodPreset = "30d"): Promise<ReportResponse> {
  return (await apiGet("/api/v1/report", { period })) as ReportResponse;
}

export async function fetchReportFresh(period: PeriodPreset = "30d"): Promise<ReportResponse> {
  return (await apiGet("/api/v1/report", { period, refresh: "1" })) as ReportResponse;
}

export async function fetchAsk(
  question: string,
  refresh = false,
  signal?: AbortSignal
): Promise<AskResponse> {
  const body: AskRequestBody = { question, refresh };
  const res = await fetch(apiUrl(`${API_PREFIX}/ask`), {
    method: "POST",
    headers: apiHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify(body),
    signal,
  });
  if (!res.ok) await parseError(res);
  return res.json();
}

const SAME_ORIGIN_CHAT = new Set(["same-origin", ".", "/"]);

function chatMode(): { enabled: boolean; prefix: string } {
  if (import.meta.env.VITE_CHAT_DISABLED === "true") {
    return { enabled: false, prefix: "" };
  }
  const raw = import.meta.env.VITE_CHAT_URL?.trim();
  if (raw && SAME_ORIGIN_CHAT.has(raw.replace(/\/+$/, "") || raw)) {
    // Production Gateway: browser origin already routes /v1/chat to the chat Service.
    return { enabled: true, prefix: "" };
  }
  if (raw) {
    return { enabled: true, prefix: raw.replace(/\/+$/, "") };
  }
  // Vite dev proxy: /chat → localhost:8090 (see vite.config.ts)
  if (import.meta.env.DEV) return { enabled: true, prefix: "/chat" };
  return { enabled: false, prefix: "" };
}

/** Base URL for the chat agent. Empty for same-origin `/v1/chat` or when disabled. */
export function chatBase(): string {
  return chatMode().prefix;
}

export function chatEnabled(): boolean {
  return chatMode().enabled;
}

/** Browser path for a chat API route (e.g. `/v1/chat/health`). */
export function chatUrl(path: string): string {
  return `${chatMode().prefix}${path}`;
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
  if (!chatEnabled()) {
    throw new Error("Chat is not configured (set VITE_CHAT_URL or same-origin)");
  }
  const res = await fetch(chatUrl("/v1/chat/health"), { signal });
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
  if (!chatEnabled()) {
    throw new Error("Chat is not configured (set VITE_CHAT_URL or same-origin)");
  }
  const res = await fetch(chatUrl("/v1/chat"), {
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
  if (!chatEnabled()) {
    throw new Error("Chat is not configured (set VITE_CHAT_URL or same-origin)");
  }
  const res = await fetch(chatUrl("/v1/suggestions/explain"), {
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
  return (await apiGet("/api/v1/service-detail", {
    account_id: params.account_id,
    service: params.service,
    start: params.start,
    end: params.end,
  })) as ServiceDetail;
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
  return (await apiGet(
    "/api/v1/service-tag-delta",
    {
      service: params.service,
      account_id: params.account_id,
      start: params.start,
      end: params.end,
      tag_key: params.tag_key ?? "Name",
      top_accounts: params.top_accounts,
    },
    params.signal
  )) as ServiceTagDeltaResponse;
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
  return (await apiGet(
    "/api/v1/service-tag-totals",
    {
      service: params.service,
      account_id: params.account_id,
      start: params.start,
      end: params.end,
      tag_key: params.tag_key ?? "Name",
      top_buckets: params.top_buckets,
    },
    params.signal
  )) as ServiceTagTotalsResponse;
}
