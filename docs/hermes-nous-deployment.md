# Hermes at Nous Research (org deployment)

How to connect **org-hosted [Hermes Agent](https://github.com/nousresearch/hermes-agent)** to this **AWS Org Cost Explorer** when users are not running everything on a laptop.

Local setup: [hermes-setup.md](./hermes-setup.md).

## Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│  Nous / org Hermes (gateway, CLI, Slack, …)                 │
│    └── spawns MCP: org-cost-api (stdio)                     │
│              └── HTTP ──► org-cost-api (Go)                   │
│                              └── AWS (CE + EC2 read-only)   │
└─────────────────────────────────────────────────────────────┘
```

Hermes discovers tools from the MCP server ([MCP docs](https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp)). The MCP server only calls **your** Go API — not AWS directly — so the LLM never holds AWS credentials.

**Important:** Phase 1 MCP uses **stdio** (a subprocess on the **same machine** as Hermes). The API URL must be reachable from that host (`ORG_COST_API_URL`), not `http://localhost:8080` on a developer laptop.

| Deployment style | What to do |
|------------------|------------|
| Hermes runner **co-located** with the API | Same VM/pod: MCP `command` points at venv; `ORG_COST_API_URL=http://127.0.0.1:8080` |
| Hermes **remote**, API **internal** | Run MCP next to the API; give Hermes stdio on that host via SSH/session, **or** add an HTTP MCP wrapper (not in Phase 1) |
| Many Nous users | Publish catalog entry + org skill; central API URL in config template |

## Roles

| Role | Responsibility |
|------|----------------|
| **Platform / Hermes admins** (Nous) | Hermes install, `mcp_servers` in org config, model provider ([Nous Portal](https://hermes-agent.nousresearch.com/docs/getting-started/quickstart) or keys), gateway channels |
| **FinOps / cloud team** (you) | Deploy org-cost-api + config.yaml, AWS IAM, VPN/network rules |
| **End users** | Chat in Hermes CLI, gateway (Slack, etc.), or the web **Ask** tab in the React dashboard |

## Step 1 — Deploy the cost API (your side)

1. Run the Go server where it can reach AWS with **org credentials** (not each user’s laptop):
   - Kubernetes + IRSA, or
   - Internal VM with instance profile / `credential_process` for Granted on that host, or
   - CI-style role assumed at runtime.

2. Mount **`config.yaml`** (accounts, `billing_profile`, member `billing_profile: account` as needed).

3. Expose internally, e.g. `https://cost-explorer.internal.yourorg.com` (TLS, behind SSO or mTLS if required).

4. Probes: liveness `GET /api/health`, readiness `GET /api/ready` (auth-exempt). `billing_check: skipped` means no payer configured; member-only ready re-STSes `accounts[0]` and reports `accounts_check` (`ok` / `failed`).

5. Optional: deploy the React frontend on the same ingress for humans; Hermes only needs the API.

**AWS:** Payer/member Cost Explorer + `ec2:DescribeVolumes` / `DescribeSnapshots` as in the main README.

## Step 2 — Install MCP on the Hermes execution host

On every host where Hermes **spawns** the MCP subprocess:

```bash
git clone <org-cost-api-repo> /opt/org-cost-api
cd /opt/org-cost-api/mcp-server
python3 -m venv .venv
.venv/bin/pip install -e .
```

Set environment for the API (not localhost unless API is local):

```bash
export ORG_COST_API_URL=https://cost-explorer.internal.yourorg.com
```

Smoke test from that host:

```bash
curl -s "$ORG_COST_API_URL/api/health"
curl -s "$ORG_COST_API_URL/api/ready"
.venv/bin/python -c "from org_cost_mcp.client import health_check; print(health_check())"
```

## Step 3 — Register MCP in org Hermes config

Hermes reads `mcp_servers` from `~/.hermes/config.yaml` (per user) or from an **org template** your Nous admins maintain.

**Stdio (Phase 1 — co-located):**

```yaml
mcp_servers:
  org-cost-api:
    command: /opt/org-cost-api/mcp-server/.venv/bin/org-cost-mcp
    env:
      ORG_COST_API_URL: https://cost-explorer.internal.yourorg.com
      # Required when API auth is enabled (api_token_env / api_token on the backend):
      # ORG_COST_API_TOKEN: "..."
    enabled: true
    tools:
      include:
        - check_api_health
        - list_accounts
        - get_org_summary
        - get_account_costs
        - get_waste_signals
        - get_service_detail
        - get_service_tag_delta
        - get_service_tag_totals
        - get_cost_trends
        - get_cost_suggestions
        - get_cost_report
        - ask_cost_question
```

Admins install tools via:

```bash
hermes mcp configure org-cost-api   # if registered via catalog
```

Reload MCP / restart Hermes after config changes ([config reload notes](https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp)).

**Remote HTTP MCP:** Hermes supports `url:` for hosted MCP ([HTTP servers](https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp)). This repo does not ship an HTTP MCP endpoint yet; use co-location for Phase 1, or add a thin SSE/HTTP bridge in a later phase.

## Step 4 — Distribute the skill

Org-wide skill content lives in this repo: `skills/org-cost-api/SKILL.md`.

**Per user (quick):**

```bash
ln -sf /opt/org-cost-api/skills/org-cost-api ~/.hermes/skills/org-cost-api
```

**Org-wide (better):**

- Nous admins bake the skill into the standard Hermes image / dotfiles, or
- Publish to [Skills Hub](https://github.com/nousresearch/hermes-agent) / internal skill registry if your org uses one.

Users invoke with `/org-cost-api` or natural language after the skill is loaded.

## Step 5 — Models and Nous Portal

On Nous-hosted Hermes, teams often use **Nous Portal** for one subscription (models + tool gateway):

```bash
hermes setup --portal
```

That is separate from AWS cost data — you still need the org-cost-api for dollar amounts. Portal covers the LLM; MCP covers grounded cost tools.

## Step 6 — Gateway for non-technical users (optional)

If your org runs **`hermes gateway`** ([messaging guide](https://hermes-agent.nousresearch.com/docs/user-guide/messaging)):

1. Platform enables Slack / Teams / Telegram with allowlists ([security](https://hermes-agent.nousresearch.com/docs/user-guide/security)).
2. Same `mcp_servers` + skill on the gateway host.
3. Optional: [cron](https://hermes-agent.nousresearch.com/docs/user-guide/features/cron) for weekly “org cost digest” to a channel.

Finance users ask in Slack; Hermes calls `get_org_summary` / `get_waste_signals` — no Cost Explorer UI required.

## Step 7 — Nous MCP catalog (optional, org-wide rollout)

Nous maintains approved MCPs under `optional-mcps/` in [hermes-agent](https://github.com/nousresearch/hermes-agent). To make install one command for all Nous users:

1. Open a PR adding `optional-mcps/org-cost-api/manifest.yaml` (clone, bootstrap venv, `transport.command`, required env `ORG_COST_API_URL`).
2. After merge: `hermes mcp install org-cost-api` on user machines (or golden image).

Until that PR exists, use manual `mcp_servers` config above.

## Security checklist

- [ ] API is **internal only** (VPN / private network / SSO proxy).
- [ ] MCP tools are **read-only** (no shell, no AWS CLI in Hermes for cost questions).
- [ ] `refresh=true` is rate-limited; prefer cached dashboard for casual chat.
- [ ] Hermes **DM pairing** / allowlists enabled on gateway.
- [ ] AWS role on API host is **read-only** CE + EC2 describe.
- [ ] Secrets in `~/.hermes/.env` or K8s secrets — not in git.
- [ ] Optional `api_token` on API + `ORG_COST_API_TOKEN` in MCP env.

## Troubleshooting (org)

| Symptom | Likely cause |
|---------|----------------|
| `check_api_health` false from Hermes | API URL wrong from Hermes host; firewall; API down |
| Works locally, fails on Nous host | `ORG_COST_API_URL` still points at laptop localhost |
| All costs $0 | API host AWS auth or `billing_profile` / member `account` settings |
| OAuth MCP confusion | org-cost-api uses stdio, not OAuth — ignore OAuth flow for this server |

## Summary timeline

1. **You:** Deploy API + AWS auth → internal URL.
2. **You + platform:** Install MCP on Hermes runner(s) with that URL.
3. **Nous admins:** Add `mcp_servers` + model provider to org Hermes config.
4. **You:** Ship `org-cost-api` skill org-wide.
5. **Optional:** Gateway + cron; catalog PR for `hermes mcp install`.

The in-app **Ask** tab (shipped) calls `POST /api/ask` on the same backend. Hermes is optional for browser users; MCP remains the richer integration for agents.
