# Known issues and limitations

Honest list for contributors and self-hosters. Demo/public-sharing gaps are being addressed; production hardening is documented here for transparency.

## Public demo

- Demo mode uses **synthetic data** — not suitable for financial decisions.
- Public demo should run **without** `api_token_env`; do not expose real org data on the internet.
- Light edge rate limiting on refresh is recommended for hosted demos — see [deploy/README.md](./deploy/README.md#rate-limiting-recommended-for-public-demo).

## Security (self-hosted)

- **API auth is optional** unless you set `api_token_env`. Misconfigured deployments expose org spend on the network.
- **`VITE_API_TOKEN` is not real auth** — it is baked into the static JS bundle. Use VPN/network isolation or an OAuth reverse proxy for browser users. See [browser-users.md](./browser-users.md).
- Server error logs may include more detail than client responses (client bodies are sanitized).

## AWS integration

- AWS clients are created **at boot** — expired SSO sessions require a process restart.
- **No HTTP rate limiting** on `?refresh=1` or `POST /api/ask` with `refresh: true` (internal CE semaphore only).
- Readiness may not re-probe all member accounts when billing profile is configured (member-only path probes `accounts[0]`).

## API contract

- Public routes are documented in [openapi.yaml](../openapi.yaml) and validated in CI.
- Breaking changes to `/api/*` should bump `info.version` in openapi.yaml and note in [CHANGELOG.md](../CHANGELOG.md).

## Operations

- History snapshots are plain JSON files — no built-in backup/restore (see [migration.md](./migration.md) for manual copy).
- Docker `HEALTHCHECK` on `:prod` / `:demo` images uses `/api/ready` (readiness). The `:api` image (API-only) still uses `/api/health` for liveness-only deployments.
- No Prometheus metrics or structured request logging yet.

## Frontend

- No root error boundary — an uncaught render error can white-screen the app.
- Large JS bundle (~800 KB) — vgpu, recharts, and Ask load on first paint.

## MCP

- `check_api_health` now probes `/api/ready` and auth when a token is set; stdio MCP has no transport-level auth (trust the host that spawns the process).

## Chat agent (conversational Ask)

- Sessions are **in-memory** — not HA; restart clears conversation history.
- LLM may **hallucinate** if it skips tools — mitigated by system prompt; tool names shown in UI when available.
- **No per-user auth** on chat v1 — rely on VPN/network + Go API token.
- LiteLLM keys must stay on the chat server — never in `VITE_*` env vars.
- **Suggestions LLM enrichment** adds narrative explanations on top of rule-based items — may still miss nuance or skip items if the model omits an id; rule-based cards remain the source of truth for titles and estimates.

Contributions welcome — see [CONTRIBUTING.md](../CONTRIBUTING.md).
