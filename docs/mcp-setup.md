# MCP setup — Hermes and Cursor

Connect **org-cost-api** to an AI agent via [Model Context Protocol](https://modelcontextprotocol.io). The MCP server calls your Go API over HTTP — the LLM never gets AWS credentials.

| Host | Best for |
|------|----------|
| [Hermes Agent](https://github.com/nousresearch/hermes-agent) | Terminal / Desktop agent with skills and org deploy |
| [Cursor](https://cursor.com) | IDE-integrated agent while you code |

Org-hosted Hermes at Nous: see [hermes-nous-deployment.md](./hermes-nous-deployment.md). Hermes-only details: [hermes-setup.md](./hermes-setup.md).

---

## 1. Start the API

Pick **demo** (no AWS) or **production** (your org).

### Demo (try in 2 minutes)

```bash
docker compose -f docker-compose.demo.yml up --build
# API + UI at http://localhost:8080
```

Or API only:

```bash
ORG_COST_DEMO=1 go run ./backend/cmd/server
# config.demo.yaml is auto-discovered when ORG_COST_DEMO=1
```

No `ORG_COST_API_TOKEN` needed in demo mode.

### Production (real AWS data)

```bash
granted sso login   # or ensure ~/.aws/credentials profiles work
cd backend && go run ./cmd/server
```

Verify:

```bash
curl -s http://localhost:8080/api/health
curl -s http://localhost:8080/api/ready
```

If `api_token_env` is set in config, export the same value for MCP:

```bash
export ORG_COST_API_TOKEN='your-token'
```

---

## 2. Install the MCP server

```bash
cd mcp-server
python3 -m venv .venv
source .venv/bin/activate   # Windows: .venv\Scripts\activate
pip install -e .
```

Or from PyPI (after publish):

```bash
pip install org-cost-mcp
```

Smoke test (waits on stdin — Ctrl+C to exit):

```bash
export ORG_COST_API_URL=http://localhost:8080
org-cost-mcp
```

---

## 3. Hermes setup

### Register MCP in `~/.hermes/config.yaml`

Replace `/absolute/path/to/org-cost-api` with your clone path.

**Option A — venv (recommended):**

```yaml
mcp_servers:
  org-cost-api:
    command: /absolute/path/to/org-cost-api/mcp-server/.venv/bin/org-cost-mcp
    env:
      ORG_COST_API_URL: http://localhost:8080
      # ORG_COST_API_TOKEN: "..."   # required when api_token_env is set on the API
```

**Option B — uv:**

```yaml
mcp_servers:
  org-cost-api:
    command: uv
    args:
      - run
      - --directory
      - /absolute/path/to/org-cost-api/mcp-server
      - org-cost-mcp
    env:
      ORG_COST_API_URL: http://localhost:8080
```

See [hermes/mcp-org-cost.example.yaml](../hermes/mcp-org-cost.example.yaml).

### Install the skill (recommended)

Teaches the agent when and how to use cost tools:

```bash
ln -sf "$(pwd)/skills/org-cost-api" ~/.hermes/skills/org-cost-api
```

### Reload and verify

```bash
# New terminal tab if `hermes` not found:
source ~/.zshrc

~/.local/bin/hermes mcp list          # org-cost-api should show enabled
./scripts/verify-hermes-costs.sh      # API + MCP + Hermes config
```

**Hermes Desktop:** quit the app completely, ensure the API is running, start a **new chat**, then ask:

> Use the org-cost-api MCP tool `check_api_health`, then `get_org_summary`.

Cost tools (`get_org_summary`, etc.) are **MCP tools** — they do not work as shell commands.

More troubleshooting: [hermes-setup.md](./hermes-setup.md).

---

## 4. Cursor setup

### Install MCP in Cursor

1. Start the API (demo or production) — step 1 above.
2. Install the MCP package — step 2 above.
3. Open **Cursor Settings → MCP** (or edit the config file directly).
4. Add a server named `org-cost-api`.

**User config** (`~/.cursor/mcp.json` on macOS/Linux):

```json
{
  "mcpServers": {
    "org-cost-api": {
      "command": "/absolute/path/to/org-cost-api/mcp-server/.venv/bin/org-cost-mcp",
      "env": {
        "ORG_COST_API_URL": "http://localhost:8080"
      }
    }
  }
}
```

**Project config** (`.cursor/mcp.json` in this repo — good for team sharing; use your own paths):

Copy from [cursor/mcp-org-cost.example.json](../cursor/mcp-org-cost.example.json) and adjust the `command` path.

When API auth is enabled, add to `env`:

```json
"ORG_COST_API_TOKEN": "same-value-as-backend"
```

### Enable tools in chat

1. Open **Cursor Settings → MCP** and confirm `org-cost-api` is enabled (green).
2. In **Agent** or **Composer**, MCP tools appear when the server connects.
3. Ask naturally, e.g.:
   - “Use org-cost-api to call `check_api_health` and `get_org_summary`.”
   - “What are our top AWS cost drivers?” (agent should call `ask_cost_question` or `get_org_summary`.)

### Cursor + demo mode

```bash
docker compose -f docker-compose.demo.yml up --build
```

Point `ORG_COST_API_URL` at `http://localhost:8080`. No token. Example prompts work against synthetic fixture data.

### Cursor tips

| Topic | Note |
|-------|------|
| Path to `org-cost-mcp` | Use the **venv binary** (`mcp-server/.venv/bin/org-cost-mcp`), not `python -m` unless you activate the venv in `command` |
| API not reachable from Docker | If Cursor runs on the host, `localhost:8080` is correct. If the API is in Docker with a different network, use `host.docker.internal:8080` (macOS/Windows) |
| Tools greyed out | Restart Cursor after editing `mcp.json`; confirm API is up (`curl /api/health`) |
| 401 errors | Set `ORG_COST_API_TOKEN` in the MCP `env` block to match backend `api_token_env` |
| Refresh cost | Default `refresh=false`. Avoid asking the agent to refresh on every message — CE calls cost money in production |

---

## 5. Available MCP tools

| Tool | Use when |
|------|----------|
| `check_mcp_alive` | Ping MCP process only (no API call) — use when Cursor shows MCP disconnected |
| `check_api_health` | Verify API is up, ready, and authenticated |
| `check_chat_health` | Ask / LLM config status (no LLM call) |
| `list_accounts` | Resolve account names before drill-down |
| `get_org_summary` | Current org spend + top services |
| `get_cost_trends` | Period-over-period changes (history-first) |
| `get_cost_suggestions` | Ranked savings ideas |
| `get_cost_report` | Full report + HTML at `~/.org-cost/reports/latest.html` |
| `get_waste_signals` | Unattached EBS, snapshots |
| `get_account_costs` | Single-account breakdown |
| `get_service_detail` | Region / usage type / tag drill-down |
| `get_service_tag_delta` / `get_service_tag_totals` | Tag-bucket analysis |
| `ask_cost_question` | Natural-language Q&A |

**CE discipline:** prefer cached data (`refresh=false`). Run `get_org_summary` once daily or use `./scripts/daily-snapshot.sh` to build history in `~/.org-cost/history/`.

---

## 6. Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ORG_COST_API_URL` | No (default `http://localhost:8080`) | Go API base URL |
| `ORG_COST_API_TOKEN` | When API auth on | Bearer token matching backend |
| `ORG_COST_API_TIMEOUT_SECONDS` | No (default `120`) | HTTP read timeout for slow refreshes |

Legacy `EC2_OTHER_API_URL` still works with a deprecation warning.

---

## 7. Troubleshooting

| Issue | Fix |
|-------|-----|
| `check_api_health` → `healthy: false` | Start API; check `curl /api/ready`. If auth on, set `ORG_COST_API_TOKEN` |
| `ready: false` | AWS creds or config issue (production). Demo: should always be ready |
| Hermes: command not found | `~/.local/bin/hermes` or `source ~/.zshrc` |
| Cursor: no MCP tools | Edit `mcp.json`, restart Cursor, check MCP panel for errors |
| MCP shows red / disconnected after idle | Toggle MCP off/on in Cursor Settings, or restart Cursor. Try `check_mcp_alive` first — if it works, MCP is up and the issue is API/AWS |
| Tool errors after hours of use | AWS SSO may have expired — `granted sso login`, then `check_api_health` |
| Agent uses AWS CLI instead | Enable only `org-cost-api` MCP; install Hermes skill |
| All costs $0 | SSO login, `billing_profile`, or use demo mode to verify MCP wiring |
| Unknown account | Call `list_accounts` first; use config `name` fields |

Run the stack verifier:

```bash
./scripts/verify-hermes-costs.sh
```

Works for API + MCP install even if you only use Cursor (Hermes config check is optional).

---

## 8. Security

- MCP tools are **read-only** HTTP calls to your dashboard API.
- No AWS credentials are passed to the model.
- Demo mode uses **fake data only** — safe for public try-outs.
- Production: use `api_token_env` and restrict who can reach the API on your network.

See [SECURITY.md](../SECURITY.md) and [KNOWN-ISSUES.md](./KNOWN-ISSUES.md).

---

## 9. Remote MCP (HTTP/SSE)

By default `org-cost-mcp` uses **stdio** — Cursor and Hermes spawn it as a subprocess. For remote agents (org-hosted Hermes, VM, or any host that cannot run a local subprocess), use HTTP transport.

This is an MCP server feature for self-hosters — not a separate product offering.

### Start HTTP MCP

```bash
export ORG_COST_API_URL=http://localhost:8080   # or your remote API URL
export MCP_TRANSPORT=sse                         # or streamable-http
export MCP_HOST=0.0.0.0
export MCP_PORT=8000
org-cost-mcp
```

Or with Docker (API on host `:8080`):

```bash
docker compose -f docker-compose.mcp-http.yml up --build
```

| Transport | Endpoint | Notes |
|-----------|----------|-------|
| `stdio` (default) | subprocess pipes | Cursor, Hermes Desktop, Claude Desktop |
| `sse` | `http://host:8000/sse` | Legacy SSE clients; POST messages to `/messages/` |
| `streamable-http` | `http://host:8000/mcp` | MCP streamable HTTP spec |

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_TRANSPORT` | `stdio` | `stdio`, `sse`, or `streamable-http` |
| `MCP_HOST` | `127.0.0.1` (`0.0.0.0` for HTTP modes) | Bind address |
| `MCP_PORT` | `8000` | Listen port for HTTP transports |
| `MCP_ALLOWED_HOSTS` | (empty) | Comma-separated `Host` headers for Streamable HTTP (e.g. `costs.internal.resiz.es`). Required behind a Gateway; otherwise the SDK returns `421 Invalid Host header`. |
| `ORG_COST_API_URL` | `http://localhost:8080` | Go API the MCP tools call |

HTTP MCP still calls your Go API over HTTP — it does **not** embed AWS credentials. Protect the MCP port on your network; stdio remains the recommended path for local agents.

**Not the same as org-cost-chat:** the chat agent on `:8090` serves browser SSE at `/v1/chat` — that is conversational UI, not MCP protocol.

Remote Hermes (Nous): see [hermes-nous-deployment.md](./hermes-nous-deployment.md) — stdio subprocesses on the Hermes host cannot reach a laptop API; run the Go API + HTTP MCP on the same VM or use SSH port-forwarding.
