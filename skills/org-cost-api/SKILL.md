---
name: org-cost-api
description: Explain AWS organization spend using the org-cost-api MCP server (read-only). Use when the user asks about AWS bills, accounts, services, cost trends, or savings opportunities.
---

# AWS organization costs

Help users understand AWS spend and how it changes over time. **Never invent dollar amounts** — always call an MCP tool first.

## Where things live

| Layer | Location |
|-------|----------|
| Cost API (Go) | This repo → `backend/` (`http://localhost:8080`) |
| MCP server (Python) | This repo → `mcp-server/` |
| Cost history | `~/.org-cost/history/` |
| HTML reports | `~/.org-cost/reports/latest.html` |
| Hermes config | `~/.hermes/config.yaml` → `mcp_servers.org-cost-api` |

## MCP tools (prefer cheap defaults)

| Tool | When to use |
|------|-------------|
| `check_api_health` | First call if API might be down |
| `list_accounts` | List configured account names and IDs |
| `get_cost_report` | Charts, visual summary, dashboard in one call |
| `get_cost_trends` | Prior vs current period (e.g. Redshift +300%) |
| `get_cost_suggestions` | Ranked savings ideas |
| `get_org_summary` | Current org spend + top services |
| `get_account_costs` / `get_service_detail` | Drill-down |
| `get_waste_signals` | Unattached EBS, snapshots |
| `get_service_tag_delta` | Why a service changed (tag buckets) |
| `get_service_tag_totals` | Most expensive S3 buckets / tag buckets |
| `ask_cost_question` | Plain-language cost question (same as browser Ask tab) |

**Cost Explorer discipline:** default `refresh=false`. Only use `refresh=true` when the user explicitly wants live AWS data. Run `get_org_summary` once daily to build history in `~/.org-cost/history/`.

## Example flows

**“How did costs evolve?”**
1. `get_cost_trends` (no refresh)
2. For bucket-level detail: `get_service_tag_delta` with `service=Amazon Simple Storage Service`, `tag_key=Name`

**“How can we save money?”**
1. `get_cost_suggestions` (no refresh)
2. Deep-dive top item with `get_waste_signals` or `get_service_detail`

**“Show me charts”**
1. `get_cost_report` (no refresh) — includes Mermaid in chat + HTML path

## Plain language

- **Organization spend** = all AWS services (usage), default ~30-day window
- **Prior period** = same-length window immediately before the current period
- **EC2-Other** is one service line; most spend is often S3, RDS, data transfer, etc.
