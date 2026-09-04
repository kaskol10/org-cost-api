# Conversational Ask (chat agent)

Multi-turn **Ask** in the browser uses a Python chat service (`org-cost-chat`) that calls an **OpenAI-compatible LLM** (org **LiteLLM** proxy, local **vLLM**, or similar) and the existing Go API tools. AWS credentials stay on the API host — not in the browser or LLM process.

When `VITE_CHAT_URL` is unset, the Ask tab falls back to rule-based [`POST /api/ask`](../backend/internal/handlers/ask.go) (no LLM).

For Hermes / Cursor agents, see [mcp-setup.md](./mcp-setup.md).

---

## Architecture

```text
Browser (Ask tab)  --SSE-->  org-cost-chat (:8090)
                                  |
                                  +--> LLM (LiteLLM proxy / vLLM / OpenAI-compatible)
                                  +--> Go org-cost-api (:8080)
```

---

## 1. Prerequisites

| Piece | Requirement |
|-------|-------------|
| Go API | Running (demo or production) — [getting-started.md](./getting-started.md) |
| LLM | LiteLLM org proxy **or** vLLM OpenAI server **or** any OpenAI-compatible endpoint |
| Model | Tool-calling capable model recommended (see provider sections below) |

---

## 2. Config auto-discovery

`org-cost-chat` and `org-cost-mcp` read the same **`config.yaml`** as the Go API when env vars are unset. Search order (first match wins):

1. `CONFIG_PATH` or `ORG_COST_CONFIG` env
2. `config.yaml` in the current directory, then each parent up to the repo root
3. `config.demo.yaml` (when `ORG_COST_DEMO=1`)
4. `~/.org-cost/config.yaml`
5. `~/.config/org-cost-api/config.yaml`

Optional `chat:` block in `config.yaml` (see [config.example.yaml](../config.example.yaml)):

```yaml
listen_addr: ":8080"
api_token_env: ORG_COST_API_TOKEN

chat:
  provider: litellm
  base_url: https://litellm.internal/v1
  api_key_env: LITELLM_API_KEY
  model: claude-sonnet-4-20250514
  # api_url: http://127.0.0.1:8080  # optional override
```

`ORG_COST_API_URL` defaults from `listen_addr`. **Env vars always override** YAML.

The Go server uses the same discovery — run from `backend/` without `-config`:

```bash
cd backend && go run ./cmd/server
```

---

## 3. LLM providers

The chat agent uses [LiteLLM](https://docs.litellm.ai/) as the client library. Point it at your backend with `LLM_PROVIDER` and related env vars. Legacy `LITELLM_*` names still work.

### Option A — Org LiteLLM proxy (default)

```bash
export LLM_PROVIDER=litellm   # optional; this is the default
export LLM_BASE_URL=https://litellm.internal/v1
export LLM_API_KEY=your-proxy-key
export LLM_MODEL=claude-sonnet-4-20250514
```

Legacy aliases: `LITELLM_BASE_URL`, `LITELLM_API_KEY`, `LITELLM_MODEL`.

LiteLLM can route to Anthropic, OpenAI, Bedrock, **vLLM**, etc. — configure models on the proxy, not in org-cost-api.

### Option B — vLLM (direct OpenAI server)

Run [vLLM](https://docs.vllm.ai/) with the OpenAI-compatible API (default port **8000**):

```bash
vllm serve meta-llama/Meta-Llama-3-8B-Instruct --enable-auto-tool-choice --tool-call-parser llama3_json
```

Chat agent:

```bash
export LLM_PROVIDER=vllm
export LLM_MODEL=meta-llama/Meta-Llama-3-8B-Instruct
# optional if not on localhost:8000
export VLLM_BASE_URL=http://localhost:8000/v1
export VLLM_API_KEY=EMPTY

org-cost-chat
```

The agent adds the `hosted_vllm/` model prefix for LiteLLM. Use a model that supports **tool / function calling** — otherwise answers may not invoke cost tools reliably.

### Option C — LiteLLM proxy → vLLM backend

On your LiteLLM server, register the vLLM model, then use Option A with the model name LiteLLM exposes (e.g. `vllm-llama-3`). No `LLM_PROVIDER=vllm` needed on org-cost-chat — the proxy handles routing.

---

## 4. Install and run the chat agent

### Pip (local)

```bash
cd mcp-server
python3 -m venv .venv
source .venv/bin/activate   # Windows: .venv\Scripts\activate

# After activate, pip works inside .venv. If you skip activate, use:
# .venv/bin/python3 -m pip install -e .

pip install -e .

export ORG_COST_API_URL=http://127.0.0.1:8080
export LLM_BASE_URL=https://litellm.internal/v1
export LLM_API_KEY=your-proxy-key
export LLM_MODEL=claude-sonnet-4-20250514

org-cost-chat
# listens on http://0.0.0.0:8090
```

If the Go API uses `api_token_env`, also set:

```bash
export ORG_COST_API_TOKEN=your-token
```

### Docker Compose (full stack: API + UI + chat)

Production AWS + chat in one command:

```bash
cp .env.example .env   # set LLM_BASE_URL, LLM_API_KEY
cp config.example.yaml config.yaml   # or bootstrap-config.sh
docker compose -f docker-compose.full.yml up --build
# open http://localhost:8080
```

Demo API + chat only (no real AWS):

```bash
export LLM_BASE_URL=https://litellm.internal/v1
export LLM_API_KEY=your-proxy-key
export LLM_MODEL=claude-sonnet-4-20250514

docker compose -f docker-compose.chat.yml up --build
```

---

## 5. Enable chat in the frontend

**Local dev (recommended):** start `org-cost-chat` on `:8090`, then run Vite — no `VITE_CHAT_URL` needed. Vite proxies `/chat` → `http://localhost:8090`:

```bash
# terminal 1: Go API
cd backend && go run ./cmd/server

# terminal 2: chat agent
org-cost-chat

# terminal 3: frontend
cd frontend && npm run dev
# open http://localhost:5173 → Chat tab (setup banner shows API / chat / LLM status)
```

**Explicit URL** (production build or when not using the Vite proxy):

```bash
export VITE_CHAT_URL=http://localhost:8090
npm run dev
```

For production Docker builds, set `VITE_CHAT_URL` at build time via [docker-compose.full.yml](../docker-compose.full.yml) or build args.

---

## 6. Environment variables

### Chat agent

| Variable | Default | Description |
|----------|---------|-------------|
| `ORG_COST_API_URL` | `http://localhost:8080` | Go API base URL |
| `ORG_COST_API_TOKEN` | (empty) | Bearer token when API auth is on |
| `LLM_PROVIDER` | `litellm` | `litellm`, `vllm`, or `openai` |
| `LLM_BASE_URL` | (empty) | OpenAI-compatible API base (`…/v1`) |
| `LLM_API_KEY` | (empty) | API key (use `EMPTY` for local vLLM) |
| `LLM_MODEL` | provider default | Model name |
| `VLLM_BASE_URL` | `http://localhost:8000/v1` | vLLM server (when `LLM_PROVIDER=vllm`) |
| `VLLM_API_KEY` | `EMPTY` | vLLM API key if required |
| `LITELLM_*` | — | Legacy aliases for `LLM_*` |
| `CHAT_HOST` | `0.0.0.0` | Bind address |
| `CHAT_PORT` | `8090` | Listen port |
| `CHAT_CORS_ORIGINS` | localhost origins | Comma-separated browser origins |
| `CHAT_MAX_TOOL_ROUNDS` | `8` | Max tool-call loops per message |
| `CHAT_SESSION_TTL_SECONDS` | `3600` | In-memory session TTL |

`GET /v1/chat/health` returns `llm_provider`, `llm_model`, and `llm_base_url`.

### Frontend

| Variable | Description |
|----------|-------------|
| `VITE_CHAT_URL` | Chat agent base URL (e.g. `http://localhost:8090`). Empty = rule-based Ask only. |

---

## 7. API

### `POST /v1/chat` (SSE, default)

```json
{
  "session_id": "uuid-or-null",
  "message": "Why did Redshift go up?",
  "refresh": false
}
```

SSE events: `session`, `delta`, `tool_start`, `tool_end`, `done`, `error`.

### `POST /v1/chat?stream=false` (JSON, tests)

Returns `{ "session_id", "answer", "tools_called", "ce_calls_used" }`.

### `GET /v1/chat/health`

Liveness probe.

---

### `POST /v1/suggestions/explain` (JSON)

Enriches rule-based suggestions from the Go API with LLM explanations using full report context (dashboard + trends + waste signals).

```json
{ "refresh": false }
```

Returns the same shape as `/api/suggestions` plus optional fields:

- `llm_enriched: true`
- `narrative_summary` — leadership overview
- `suggestions[].explanation` — per-item markdown
- `suggestions[].confidence` — high | medium | low
- `additional_insights` — extra bullets grounded in data

When LLM is not configured, returns **503** — the UI keeps rule-based suggestions from `/api/report`.

The dashboard **Cost savings suggestions** panel calls this automatically when `VITE_CHAT_URL` is set.

---

## 8. Security

- Run the chat agent on the **same trust boundary** as the Go API (internal VPN).
- LLM keys are **server-side only** — never in `VITE_*` or the static bundle.
- `refresh=true` (user checkbox) can trigger live Cost Explorer calls — rate-limit at your proxy if needed.
- Chat v1 has **no per-user auth**; rely on network isolation + optional API token on the Go API.

---

## 9. Troubleshooting

| Symptom | Fix |
|---------|-----|
| Ask tab shows single-turn / "Routing…" only | Set `VITE_CHAT_URL` and rebuild or restart Vite |
| Chat error / connection refused | Ensure `org-cost-chat` is running on :8090 |
| LLM 401/403 | Check `LLM_BASE_URL` / `LLM_API_KEY` (or `LITELLM_*`) |
| vLLM: no tool calls | Use a tool-capable model; enable `--enable-auto-tool-choice` on vLLM |
| vLLM: connection refused | Check `VLLM_BASE_URL` (default `http://localhost:8000/v1`) |
| Tool errors / empty answers | Verify `ORG_COST_API_URL` from chat container (`http://api:8080` in compose) |
| Suggestions show rules only, no "AI explained" | Set `VITE_CHAT_URL` and ensure chat agent has `LLM_BASE_URL` / `chat:` in config.yaml |

---

## 10. Production tip: same-origin proxy

To avoid exposing a second origin in the browser, put nginx (or your ingress) in front:

```nginx
location /v1/chat {
    proxy_pass http://chat-agent:8090;
    proxy_buffering off;
    proxy_cache off;
}
```

Then set `VITE_CHAT_URL` to empty and extend the frontend to use relative `/v1/chat` — optional follow-up.
