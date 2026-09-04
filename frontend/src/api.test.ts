import { afterEach, describe, expect, it, vi } from "vitest";
import {
  apiBase,
  apiHeaders,
  apiUrl,
  chatBase,
  chatEnabled,
  fetchAsk,
  fetchReport,
  fetchServiceTagTotals,
  parseSSEBlock,
  streamChat,
} from "./api";

afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("apiBase", () => {
  it("returns empty string when VITE_API_URL is unset", () => {
    vi.stubEnv("VITE_API_URL", "");
    expect(apiBase()).toBe("");
  });

  it("strips trailing slashes", () => {
    vi.stubEnv("VITE_API_URL", "https://cost.example.com///");
    expect(apiBase()).toBe("https://cost.example.com");
  });
});

describe("apiUrl", () => {
  it("returns path when base is empty", () => {
    vi.stubEnv("VITE_API_URL", "");
    expect(apiUrl("/api/report")).toBe("/api/report");
  });

  it("joins base and path without double slashes", () => {
    vi.stubEnv("VITE_API_URL", "https://cost.example.com/");
    expect(apiUrl("/api/report")).toBe("https://cost.example.com/api/report");
  });
});

describe("apiHeaders / fetchReport auth", () => {
  it("adds Authorization when VITE_API_TOKEN is set", () => {
    vi.stubEnv("VITE_API_TOKEN", "secret-token");
    expect(apiHeaders()).toEqual({ Authorization: "Bearer secret-token" });
  });

  it("sends Authorization on fetchReport", async () => {
    vi.stubEnv("VITE_API_URL", "https://cost.example.com");
    vi.stubEnv("VITE_API_TOKEN", "secret-token");
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ dashboard: null }),
    });
    vi.stubGlobal("fetch", fetchMock);

    await fetchReport();

    expect(fetchMock).toHaveBeenCalledWith(
      "https://cost.example.com/api/report",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer secret-token",
        }),
      })
    );
  });
});

describe("fetchAsk", () => {
  it("forwards AbortSignal to fetch", async () => {
    vi.stubEnv("VITE_API_URL", "");
    const ac = new AbortController();
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ answer: "ok", intent: "summary", sources: [], ce_calls_used: 0 }),
    });
    vi.stubGlobal("fetch", fetchMock);

    await fetchAsk("What are top drivers?", false, ac.signal);

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/ask",
      expect.objectContaining({
        method: "POST",
        signal: ac.signal,
      })
    );
  });
});

describe("chatBase / chatEnabled", () => {
  it("uses /chat dev proxy when VITE_CHAT_URL is unset in dev", () => {
    vi.stubEnv("VITE_CHAT_URL", "");
    expect(chatBase()).toBe("/chat");
    expect(chatEnabled()).toBe(true);
  });

  it("returns empty when VITE_CHAT_URL is unset in production build", () => {
    vi.stubEnv("VITE_CHAT_URL", "");
    vi.stubEnv("DEV", false);
    vi.stubEnv("PROD", true);
    expect(chatBase()).toBe("");
    expect(chatEnabled()).toBe(false);
  });

  it("strips trailing slashes", () => {
    vi.stubEnv("VITE_CHAT_URL", "http://localhost:8090/");
    expect(chatBase()).toBe("http://localhost:8090");
    expect(chatEnabled()).toBe(true);
  });
});

describe("parseSSEBlock", () => {
  it("parses session and delta events", () => {
    const session = parseSSEBlock(
      'event: session\ndata: {"session_id": "abc-123"}\n'
    );
    expect(session).toEqual({ type: "session", session_id: "abc-123" });

    const delta = parseSSEBlock('event: delta\ndata: {"text": "Hello"}\n');
    expect(delta).toEqual({ type: "delta", text: "Hello" });
  });

  it("parses done event", () => {
    const done = parseSSEBlock(
      'event: done\ndata: {"tools_called": ["get_org_summary"], "ce_calls_used": 0}\n'
    );
    expect(done).toEqual({
      type: "done",
      tools_called: ["get_org_summary"],
      ce_calls_used: 0,
    });
  });
});

describe("streamChat", () => {
  it("POSTs to chat URL with session and message", async () => {
    vi.stubEnv("VITE_CHAT_URL", "http://localhost:8090");
    const sse =
      'event: session\ndata: {"session_id": "s1"}\n\n' +
      'event: delta\ndata: {"text": "Hi"}\n\n' +
      'event: done\ndata: {"tools_called": [], "ce_calls_used": 0}\n\n';
    const encoder = new TextEncoder();
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(encoder.encode(sse));
        controller.close();
      },
    });
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      body: stream,
    });
    vi.stubGlobal("fetch", fetchMock);
    const events: string[] = [];

    await streamChat("s0", "Hello", false, (ev) => {
      events.push(ev.type);
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8090/v1/chat",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ session_id: "s0", message: "Hello", refresh: false }),
      })
    );
    expect(events).toEqual(["session", "delta", "done"]);
  });
});

describe("fetchServiceTagTotals", () => {
  it("includes tag_key and top_buckets in the URL", async () => {
    vi.stubEnv("VITE_API_URL", "");
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ buckets: [] }),
    });
    vi.stubGlobal("fetch", fetchMock);

    await fetchServiceTagTotals({
      service: "Amazon Simple Storage Service",
      start: "2026-01-01",
      end: "2026-01-31",
      tag_key: "Name",
      top_buckets: 10,
    });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const url = fetchMock.mock.calls[0][0] as string;
    expect(url).toContain("/api/service-tag-totals?");
    expect(url).toContain("tag_key=Name");
    expect(url).toContain("top_buckets=10");
    expect(url).toContain("service=Amazon+Simple+Storage+Service");
  });
});
