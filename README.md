# AWS Org Cost Explorer (org-cost-api)

[![CI](https://github.com/kaskol10/org-cost-api/actions/workflows/ci.yml/badge.svg)](https://github.com/kaskol10/org-cost-api/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Understand **AWS organization spend** and find cost reduction opportunities — per account, per service, with waste signals (unattached EBS, snapshots) and natural-language Q&A via MCP or the web UI.

## Try it in 30 seconds (no AWS)

**Local Docker (recommended):**

```bash
docker compose -f docker-compose.demo.yml up --build
# open http://localhost:8080
```

**Or run the API only:**

```bash
ORG_COST_DEMO=1 go run ./backend/cmd/server
# or: go run ./backend/cmd/server -config config.demo.yaml
cd frontend && npm install && npm run dev
# open http://localhost:5173
```

Demo mode serves **synthetic fixture data** — no AWS credentials, no API token. A banner in the UI marks demo mode.

**Live demo:** [https://org-cost-api-demo.fly.dev](https://org-cost-api-demo.fly.dev) — synthetic fixture data, no AWS credentials. Self-host: [deploy/README.md](deploy/README.md) · `ghcr.io/kaskol10/org-cost-api:demo`

## Three paths

| Path | You are… | Start here |
|------|----------|------------|
| **Explore** | Curious, no AWS access | `docker compose -f docker-compose.demo.yml up` |
| **Integrate** | Hermes, Cursor, or other MCP hosts | [docs/integrate-quickstart.md](docs/integrate-quickstart.md) |
| **Operate** | Connecting your real AWS org | [docs/getting-started.md](docs/getting-started.md) · [EKS + Gateway API](docs/eks-operate.md) |

## What it does

| Area | Capability |
|------|------------|
| **Spend** | Per-account totals, EC2-Other breakdown, daily trends, top services with drill-down |
| **Waste** | Unattached EBS volumes, snapshot inventory, heuristic rightsizing hints |
| **Agents** | 14 MCP tools + Hermes skill — ask in plain language, no AWS creds in the LLM |
| **Ask chat** | Optional multi-turn browser chat via org LiteLLM — [docs/chat-setup.md](docs/chat-setup.md) |
| **History** | Daily snapshots for prior-period trends without repeat CE calls |

## MCP tools (Hermes / Cursor)

Full setup: **[docs/integrate-quickstart.md](docs/integrate-quickstart.md)** (15 min) · details: **[docs/mcp-setup.md](docs/mcp-setup.md)**

| Tool | What it does |
|------|----------------|
| `check_mcp_alive` | Ping MCP process (no API call) |
| `check_api_health` | API reachable, ready, and authed (when token set) |
| `check_chat_health` | Ask / LLM config status (no LLM call) |
| `list_accounts` | Configured account names and IDs |
| `get_org_summary` | Org totals + top services |
| `get_cost_trends` | Prior vs current period |
| `get_cost_suggestions` | Ranked savings opportunities |
| `get_cost_report` | Dashboard + trends + suggestions in one call |
| `get_waste_signals` | Unattached EBS, snapshots |
| `get_account_costs` / `get_service_detail` | Per-account drill-down |
| `get_service_tag_delta` / `get_service_tag_totals` | Tag-bucket spend changes |
| `ask_cost_question` | Natural-language Q&A |

```bash
pip install org-cost-mcp
export ORG_COST_API_URL=http://localhost:8080   # or your demo deploy URL
org-cost-mcp
```

**Hermes:** [docs/integrate-quickstart.md](docs/integrate-quickstart.md#4-hermes) · **Cursor:** [docs/integrate-quickstart.md](docs/integrate-quickstart.md#3-cursor) · example config: [cursor/mcp-org-cost.example.json](cursor/mcp-org-cost.example.json)

## Connect real AWS

```bash
./scripts/operate-bootstrap.sh --profile YOUR_PAYER_PROFILE --output config.yaml
aws sso login   # if using SSO profiles

# Docker (API + UI, mount config + ~/.aws)
docker compose -f docker-compose.prod.yml up --build

# Or local dev:
cd backend && go run ./cmd/server
cd frontend && npm run dev
```

IAM setup: [docs/iam-minimal-setup.md](docs/iam-minimal-setup.md) · billing model: [docs/onboarding-aws-auth.md](docs/onboarding-aws-auth.md) · full checklist: [docs/getting-started.md](docs/getting-started.md) · **EKS:** [docs/eks-operate.md](docs/eks-operate.md).

Optional API auth: `api_token_env: ORG_COST_API_TOKEN` — see [docs/getting-started.md](docs/getting-started.md).

## Costs show $0

If the dashboard loads but every account is **$0** or empty:

1. **Verify AWS auth first** — `./scripts/verify-aws-auth.sh config.yaml --format markdown` and fix any STS or CE failures.
2. **Billing model** — member-billed accounts need `billing_profile: account` on that row; see [docs/member-billed-account.md](docs/member-billed-account.md).
3. **SSO expired** — restart the server after `aws sso login` / `granted sso login`.
4. **Wrong payer profile** — `billing_profile` must be the org management account with Cost Explorer org view enabled.
5. **Date range** — CE can lag 24–48h; try **Refresh** after auth is confirmed.

More: [docs/onboarding-aws-auth.md](docs/onboarding-aws-auth.md#billing-model-payer-linked-vs-member-billed).

## Architecture

```text
Browser / MCP  →  Go API  →  AWS (CE, EC2, CloudWatch)
                 ↓
            history JSON (trends)
```

Full diagram: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). Known limitations: [docs/KNOWN-ISSUES.md](docs/KNOWN-ISSUES.md).

## Development

```bash
cd backend && go test ./...
cd frontend && npm test && npm run build
cd mcp-server && pip install -e ".[dev]" && pytest
```

See [CONTRIBUTING.md](CONTRIBUTING.md). Report issues via [GitHub Issues](https://github.com/kaskol10/org-cost-api/issues).

Org-specific Granted helpers (e.g. multi-portal SSO) live under [docs/examples/](docs/examples/) — not required for standard AWS Organizations setup.

## License

MIT — see [LICENSE](LICENSE).
