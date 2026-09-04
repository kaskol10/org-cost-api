# Integrate org-cost-api (MCP quickstart)

One page to connect **Hermes**, **Cursor**, or any MCP host to org-cost-api in under 15 minutes.

For developers wiring agents to **their own** deployment — not platform embedding guides.

## Prerequisites

| Piece | Demo | Production |
|-------|------|------------|
| API | [Live demo](https://org-cost-api-demo.fly.dev) or `docker compose -f docker-compose.demo.yml up` | `docker compose -f docker-compose.prod.yml up` |
| AWS | Not needed | [iam-minimal-setup.md](./iam-minimal-setup.md) + `config.yaml` |
| MCP package | `pip install org-cost-mcp` | Same |

Set `ORG_COST_API_URL` to your API base (default `http://localhost:8080`).

---

## 0. Try the public demo (optional)

No local setup — synthetic data only:

```bash
export ORG_COST_API_URL=https://org-cost-api-demo.fly.dev
curl -s "$ORG_COST_API_URL/api/meta" | jq .
# { "demo": true }
```

Use this URL in MCP `env.ORG_COST_API_URL` for a quick Cursor or Hermes smoke test. No token required.

---

## 1. Smoke test the API

```bash
curl -s http://localhost:8080/api/health | jq .
curl -s http://localhost:8080/api/ready | jq .
curl -s http://localhost:8080/api/meta | jq .
```

If `api_token_env` is set in config:

```bash
export ORG_COST_API_TOKEN='your-token'
curl -s -H "Authorization: Bearer $ORG_COST_API_TOKEN" http://localhost:8080/api/dashboard | head
```

Public API contract: [openapi.yaml](../openapi.yaml)

---

## 2. Install MCP server

```bash
pip install org-cost-mcp
# or from repo:
cd mcp-server && pip install -e .
```

Quick client check:

```bash
export ORG_COST_API_URL=http://localhost:8080
python3 -c "
from org_cost_mcp.client import health_status
s = health_status()
print(s)
assert s['healthy'], s
"
```

---

## 3. Cursor

1. Copy [cursor/mcp-org-cost.example.json](../cursor/mcp-org-cost.example.json) into Cursor MCP settings (or merge into `~/.cursor/mcp.json`).
2. Set absolute paths for `command` and `CONFIG_PATH` (production only).
3. Set `ORG_COST_API_URL` and `ORG_COST_API_TOKEN` if auth is enabled.

```json
{
  "mcpServers": {
    "org-cost-api": {
      "command": "/path/to/.venv/bin/org-cost-mcp",
      "env": {
        "ORG_COST_API_URL": "http://127.0.0.1:8080",
        "ORG_COST_API_TOKEN": ""
      }
    }
  }
}
```

For zero-setup against the public demo:

```json
"ORG_COST_API_URL": "https://org-cost-api-demo.fly.dev"
```

4. Restart Cursor → MCP panel should show **org-cost-api** green.
5. Ask: *"Call check_api_health, then get_org_summary with refresh false."*

**Idle disconnect:** Cursor may show red after idle — run `check_mcp_alive` or toggle the server. See [mcp-setup.md](./mcp-setup.md#troubleshooting).

---

## 4. Hermes

1. Add to `~/.hermes/config.yaml`:

```yaml
mcp_servers:
  org-cost-api:
    command: org-cost-mcp
    env:
      ORG_COST_API_URL: http://127.0.0.1:8080
      ORG_COST_API_TOKEN: ""   # if auth enabled
```

Use full path to `org-cost-mcp` if not on `PATH`. For remote Hermes pointing at a self-hosted API on another machine, see [Remote MCP (HTTP/SSE)](./mcp-setup.md#9-remote-mcp-httpsse) in mcp-setup.md.

2. Optional skill:

```bash
ln -sf "$(pwd)/skills/org-cost-api" ~/.hermes/skills/org-cost-api
```

3. Verify:

```bash
ORG_COST_API_URL=http://127.0.0.1:8080 ./scripts/verify-hermes-costs.sh
hermes mcp list
```

4. In Hermes chat: *"Use org-cost-api MCP; call get_org_summary (refresh=false)."*

Full Hermes doc: [hermes-setup.md](./hermes-setup.md) · Nous deploy: [hermes-nous-deployment.md](./hermes-nous-deployment.md).

---

## 5. Claude Desktop / other MCP hosts

Same env vars as Cursor:

| Variable | Purpose |
|----------|---------|
| `ORG_COST_API_URL` | Go API base URL |
| `ORG_COST_API_TOKEN` | Bearer token when `api_token_env` is set |
| `CONFIG_PATH` | Optional; MCP reads same YAML as Go API |

Default command: `org-cost-mcp` (**stdio** transport).

For HTTP/SSE (remote hosts, org VM): set `MCP_TRANSPORT=sse` or `streamable-http` — see [mcp-setup.md § Remote MCP](./mcp-setup.md#9-remote-mcp-httpsse).

---

## 6. Tool smoke script

Run all three layers (API → MCP client → optional Hermes config):

```bash
./scripts/verify-hermes-costs.sh
```

Against the public demo:

```bash
ORG_COST_API_URL=https://org-cost-api-demo.fly.dev ./scripts/verify-hermes-costs.sh
```

Recommended agent prompt snippet:

> Use the org-cost-api MCP tools for AWS spend questions. Default to `refresh=false` unless the user asks for live data. Start with `check_api_health`, then `get_org_summary` or `ask_cost_question`.

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| MCP red / disconnected | Restart MCP; run `check_api_health`; see [mcp-setup.md](./mcp-setup.md) |
| `healthy: false` | Start API; check `curl /api/ready`; set token if auth on |
| Costs $0 | [README § Costs show $0](../README.md#costs-show-0) |
| Hermes can't find server | Full path in `command`; `hermes mcp list` |

More: [mcp-setup.md](./mcp-setup.md) · [KNOWN-ISSUES.md](./KNOWN-ISSUES.md).
