# org-cost-api MCP server

Read-only [Model Context Protocol](https://modelcontextprotocol.io) server for [Hermes Agent](https://github.com/nousresearch/hermes-agent), Cursor, and other MCP hosts. Wraps the Go dashboard API so agents can explain AWS org costs **without AWS credentials in the LLM process**.

## Install (PyPI)

```bash
pip install org-cost-mcp
```

## Quick start

```bash
# Point at a running API (demo or production)
export ORG_COST_API_URL=http://localhost:8080
# When API auth is enabled:
# export ORG_COST_API_TOKEN=your-token

org-cost-mcp
```

**Try without AWS:** run the demo API first:

```bash
docker compose -f docker-compose.demo.yml up --build
export ORG_COST_API_URL=http://localhost:8080
org-cost-mcp
```

## Development install

```bash
cd mcp-server
python3 -m venv .venv
source .venv/bin/activate
pip install -e ".[dev]"
pytest
```

See **[docs/integrate-quickstart.md](../docs/integrate-quickstart.md)** (15 min) · details: **[docs/mcp-setup.md](../docs/mcp-setup.md)**

Public demo API: `https://org-cost-api-demo.fly.dev` (no AWS, no token).

## Transport

| Mode | Command / endpoint |
|------|---------------------|
| **stdio** (default) | `org-cost-mcp` — Cursor, Hermes, Claude Desktop |
| **SSE** | `MCP_TRANSPORT=sse org-cost-mcp` → `http://host:8000/sse` |
| **Streamable HTTP** | `MCP_TRANSPORT=streamable-http org-cost-mcp` → `http://host:8000/mcp` |

Remote HTTP: `docker compose -f docker-compose.mcp-http.yml up` — see [mcp-setup.md § Remote MCP](../docs/mcp-setup.md#9-remote-mcp-httpsse).

## Tools

| Tool | Description |
|------|-------------|
| `check_mcp_alive` | Ping MCP process only (no API call) |
| `check_api_health` | API health, readiness, demo flag, and auth (when token set) |
| `check_chat_health` | Ask / LLM config status (no LLM call) |
| `list_accounts` | Configured account names and IDs |
| `get_org_summary` | Org totals + top services |
| `get_cost_trends` | Current vs prior period (history-first) |
| `get_cost_suggestions` | Ranked cost optimization suggestions |
| `get_cost_report` | Markdown + Mermaid; writes `~/.org-cost/reports/latest.html` |
| `get_account_costs` | One account's spend and top services |
| `get_waste_signals` | Unattached EBS, snapshots by account |
| `get_service_detail` | Region / usage type / tag Name breakdown |
| `get_service_tag_delta` | Tag-bucket spend changes vs prior period |
| `get_service_tag_totals` | Top tag buckets for a service |
| `ask_cost_question` | Natural-language Q&A (same API as browser Ask tab) |

**CE discipline:** default `refresh=false`. Use refresh sparingly — each refresh can trigger live AWS API calls in production mode.

## Conversational Ask (`org-cost-chat`)

HTTP chat agent for multi-turn browser Ask. Supports **org LiteLLM proxy**, **direct vLLM**, or any OpenAI-compatible endpoint:

```bash
# LiteLLM org proxy (default)
export ORG_COST_API_URL=http://localhost:8080
export LLM_BASE_URL=https://litellm.internal/v1
export LLM_API_KEY=your-key
export LLM_MODEL=claude-sonnet-4-20250514

# Or local vLLM
export LLM_PROVIDER=vllm
export LLM_MODEL=meta-llama/Meta-Llama-3-8B-Instruct
export VLLM_BASE_URL=http://localhost:8000/v1

org-cost-chat
```

Frontend: `VITE_CHAT_URL=http://localhost:8090`. Full guide: [docs/chat-setup.md](../docs/chat-setup.md).

Docker: `docker compose -f docker-compose.chat.yml up --build`

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `ORG_COST_API_URL` | `http://localhost:8080` | Go API base URL |
| `ORG_COST_API_TOKEN` | (empty) | Bearer token when API auth is on |
| `ORG_COST_API_TIMEOUT_SECONDS` | `120` | Read timeout for slow dashboard builds |
| `MCP_TRANSPORT` | `stdio` | `stdio`, `sse`, or `streamable-http` |
| `MCP_HOST` | `127.0.0.1` | Bind address (`0.0.0.0` for HTTP transports) |
| `MCP_PORT` | `8000` | Port for HTTP transports |
| `MCP_ALLOWED_HOSTS` | (empty) | Comma-separated public Host headers for Streamable HTTP behind a proxy |

## Agent setup (Hermes / Cursor)

See **[docs/integrate-quickstart.md](../docs/integrate-quickstart.md)** — 15-minute MCP wiring for Hermes and Cursor.
Details: **[docs/mcp-setup.md](../docs/mcp-setup.md)** ([Hermes](../docs/mcp-setup.md#3-hermes-setup), [Cursor](../docs/mcp-setup.md#4-cursor-setup), [Remote MCP](../docs/mcp-setup.md#9-remote-mcp-httpsse)).

Org-hosted Hermes: [docs/hermes-nous-deployment.md](../docs/hermes-nous-deployment.md).

## License

MIT — see [LICENSE](../LICENSE).
