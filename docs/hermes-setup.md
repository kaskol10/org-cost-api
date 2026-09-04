# Hermes Agent + AWS Org Cost Explorer (Phase 1)

Connect [Hermes Agent](https://github.com/nousresearch/hermes-agent) to this dashboard via MCP so you can ask questions in plain language.

**Also using Cursor?** See [mcp-setup.md](./mcp-setup.md) for Hermes and Cursor in one guide (shared API + MCP install).

**Org-hosted Hermes at Nous Research?** See [hermes-nous-deployment.md](./hermes-nous-deployment.md).

## Hermes Desktop (app) vs `hermes` in Terminal

**Hermes Desktop** and the **CLI** share the same install under `~/.hermes` and the same `~/.hermes/config.yaml`. Desktop does not add `hermes` to every app’s PATH (e.g. Cursor’s terminal may still say `command not found`).

| Symptom | Fix |
|---------|-----|
| `zsh: command not found: hermes` | Run `source ~/.zshrc` or open a **new** Terminal.app tab. Or use the full path: `~/.local/bin/hermes` |
| Desktop works but Terminal does not | Same — reload shell config; PATH line should be `export PATH="$HOME/.local/bin:$PATH"` |
| Agent runs `hermes mcp list` with full path | Normal; install is fine |

Check install:

```bash
~/.local/bin/hermes --version
~/.local/bin/hermes mcp list    # should show org-cost-api ✓ enabled
```

Cost tools (`get_org_summary`, etc.) are **MCP tools**, not shell commands. In chat, ask: “Use the org-cost-api MCP tool get_org_summary” — do not expect `get_org_summary` to work in bash.

### If Desktop says “MCP tools don’t exist”

Run the verifier:

```bash
chmod +x scripts/verify-hermes-costs.sh
./scripts/verify-hermes-costs.sh
```

If all checks pass:

1. **Quit Hermes Desktop** completely (not just close the window).
2. Ensure the Go API is running (`curl http://localhost:8080/api/health`).
3. Open Desktop → **new chat** (old threads may not load MCP tools).
4. Ask: “Use **org-cost-api** MCP: call `check_api_health`, then `get_org_summary`.”

**Reject AWS CLI** for org spend — it bypasses your dashboard and needs different credentials per account.

## 1. Start the cost API

```bash
granted sso login   # or ensure ~/.aws/credentials profiles work

cd backend && go run ./cmd/server
```

Verify: `curl -s http://localhost:8080/api/health`

## 2. Install the MCP server

```bash
cd mcp-server
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
org-cost-mcp   # should wait on stdin (stdio MCP) — Ctrl+C to exit
```

Or with [uv](https://github.com/astral-sh/uv): `uv sync && uv run org-cost-mcp`

## 3. Register MCP in Hermes

Add to `~/.hermes/config.yaml`:

**Option A — venv (recommended):**

```yaml
mcp_servers:
  org-cost-api:
    command: /absolute/path/to/org-cost-api/mcp-server/.venv/bin/org-cost-mcp
    env:
      ORG_COST_API_URL: http://localhost:8080
      # ORG_COST_API_TOKEN: ...   # if api_token configured on API
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

Restart Hermes or reload MCP after editing config.

## 4. Install the skill

```bash
ln -sf "$(pwd)/skills/org-cost-api" ~/.hermes/skills/org-cost-api
```

## 5. Try it

```bash
hermes
```

Example prompts:

- “What are we spending across AWS this month?”
- “How did Redshift costs change vs last month?” → `get_cost_trends service=Redshift`
- “What should we do to reduce AWS spend?” → `get_cost_suggestions`
- “Show me a visual cost report / charts” → `get_cost_report` (then open `~/.org-cost/reports/latest.html`)
- “Break down production account costs”

**CE API tip:** ask for trends/suggestions **without** `refresh=true`. Run `get_org_summary` once daily (or `./scripts/daily-snapshot.sh`) to build history in `~/.org-cost/history/`.

Enable only the **org-cost-api** MCP toolset for cost questions so the agent does not shell out to AWS CLI.

## Security notes

- MCP tools are **read-only** HTTP calls to your local dashboard.
- No AWS credentials are passed to the LLM.
- Optional `api_token` protects `/api/*` when configured.
- `refresh=true` triggers live Cost Explorer queries (rate limits apply).

## Troubleshooting

| Issue | Fix |
|-------|-----|
| `command not found: hermes` | `source ~/.zshrc` or `~/.local/bin/hermes` |
| Agent tries `get_org_summary` in shell | That is an MCP tool — enable org-cost-api; API must be running |
| MCP enabled but no cost data | Start Go API; `curl http://localhost:8080/api/health` |
| `check_api_health` unhealthy | Start Go backend on port 8080 |
| 401 from API | Set `ORG_COST_API_TOKEN` in MCP env to match `api_token` in config |
| All costs $0 | SSO login, click Refresh in UI, check `billing_profile` |
| Unknown account | Use `list_accounts`; names are config `name` fields |
| Service detail fails | Pass exact Cost Explorer `service` string from `top_services` |

## Migration from ec2-other-costs

- MCP id: `ec2-other-costs` → `org-cost-api`
- CLI: `ec2-other-mcp` → `org-cost-mcp`
- Env: `EC2_OTHER_API_URL` → `ORG_COST_API_URL` (old name still works with deprecation warning)
- History: `~/.ec2-other/history/` → `~/.org-cost/history/` (auto-fallback if new dir empty)
