# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- `openapi.yaml` now fully describes every response: 20 new schemas (cost summary, EBS volume and snapshot inventory, service detail…) replace the free-form `additionalProperties: true` placeholders, and `required` / `nullable` match what the server actually sends
- Two contract tests keep it that way: one compares every Go response type to its schema by reflection (fields, types, required, nullable), the other validates real responses against the spec and fails on undocumented fields
- Frontend API types are generated from the spec; `types.ts` is now aliases only

- Per-integration API tokens: `api_clients` in config (Helm: `apiClients`). Each client is named in a new one-line access log (`api_request client=…`), can be revoked on its own, and has its own refresh budget. The legacy `api_token` keeps working as client "default"
- `?refresh=1` no longer locks other clients out: the limit is one forced fetch per client per 5 minutes, and a refresh of data another client just refreshed is answered from that data (`X-Refresh-Coalesced: true`) without a new Cost Explorer call
- CI lints and renders the Helm chart

- Versioned API: every route is served under `/api/v1/...`; the unversioned `/api/...` paths remain a permanent alias, so existing integrations keep working. `openapi.yaml` now documents `/api/v1` (`info.version` 1.0.0) and states the v1 compatibility rule (additive changes only)
- Test that fails CI when `openapi.yaml` and the server's registered routes differ
- Frontend types generated from `openapi.yaml` (`npm run gen:api` → `src/generated/openapi.ts`); API calls are type-checked against the spec, and CI fails if the generated file is stale

- API computes what the web UI used to compute client-side, so third-party clients get the same numbers (additive; no existing field changed):
  - `report.highlights[]` and `report.headline` (rule-based one-sentence summary)
  - `dashboard.org_daily` and `dashboard.org_ec2_other_daily` (daily series summed across accounts)
  - `totals.volume_available_est_monthly_usd`, `totals.volume_available_by_type[]`, `totals.volume_available_pricing_basis`, and per-account `unattached_ebs_est_monthly_usd`
- `GET /api/dashboard` accepts the optional `period` query parameter (`30d` | `mtd`); omitting it keeps the 30d default
- Publish Helm chart `org-cost-api` to GHCR (`oci://ghcr.io/kaskol10/org-cost-api/charts`) on `v*` tags

### Changed

- Array and map fields the spec marks required are always sent as `[]` / `{}`, never `null` (e.g. `trends.top_decreases` with no decreases used to be `null`)

- Unattached EBS waste is priced per volume type (gp2, gp3, io1/io2, st1, sc1, standard list prices) instead of a flat gp3 $0.08/GiB-month, in suggestions, Ask answers, the UI and the MCP `get_waste_signals` tool. Volumes without a type breakdown still use the gp3 rate
- Web UI and MCP prefer the server-computed fields above and fall back to the old client-side logic against older APIs
- Split `onboard-iam.sh` into `deploy` (IRSA) and `payer` (StackSet + management OrgCostReadOnly) so each side can run with only its own credentials
- Optional `--name-prefix` / CloudFormation `NamePrefix` for customer-scoped IAM role, policy, and stack names
- Prod UI chat is same-origin (`VITE_CHAT_URL=same-origin`): the browser calls `/v1/chat` on the current hostname instead of a baked chat URL

### Fixed

- Multi-tenant Helm: a site on its own hostname no longer fetches another tenant’s chat URL (CORS) and no longer crashes when suggestion lists are missing (`e is not iterable`)

## [0.0.5] — 2026-09-10

### Fixed

- Streamable HTTP MCP behind Gateway API: allow public `Host` headers (`MCP_ALLOWED_HOSTS`, Helm `hostnames`) so `/mcp` no longer returns `421 Invalid Host header`

## [0.0.4] — 2026-09-10

### Added

- Helm chart deploys optional remote MCP (`mcp.enabled`) as Streamable HTTP on `/mcp`
- GHCR image `ghcr.io/kaskol10/org-cost-api:mcp` from `mcp-server/Dockerfile.mcp`

### Changed

- MCP container default transport is `streamable-http` (was `sse`)

## [0.0.3] — 2026-09-09

### Added

- Account movers on `TrendsResponse`: `account_trends`, `top_account_increases`, `top_account_decreases` (snapshot / CE `LINKED_ACCOUNT` prior / demo)
- `GetOrgAccountTotals` Cost Explorer helper; prior-period cache stores account totals
- Period presets (`30d` / `mtd`) with shared `CostContextBar` and prior-period comparison labels
- Explore **Account movers** strip (signed `$` deltas; falls back to top spend)
- Suggestion category filter chips (All + category)
- Clickable Top services → service explain; Clear dismisses explain panel
- Whole-card Ask on billing highlights; unified **Ask about this →** CTAs
- Chat thread persists across Chat / Explore tab switches

### Changed

- Explore layout: movers sit under the summary dashboard
- OpenAPI `TrendsResponse` documents account trend schemas

## [0.0.2] — 2026-09-04

### Added

- Bake `VITE_CHAT_URL` into the `:prod` GHCR image at publish time (default `https://costs.internal.resiz.es`; override with repo variable `VITE_CHAT_URL`)

## [0.0.1] — 2026-09-04

### Added

- Public demo deploy: `deploy/fly.toml`, `scripts/deploy-demo.sh`, GitHub Actions demo deploy workflow
- Live demo URL: https://org-cost-api-demo.fly.dev
- `scripts/operate-bootstrap.sh` — bootstrap + verify helper for Operate path
- HTTP/SSE MCP transport (`MCP_TRANSPORT=sse|streamable-http`) and `docker-compose.mcp-http.yml`
- Complete [openapi.yaml](openapi.yaml) with response schemas and CI validation
- `scripts/bootstrap-config.sh` — generate starter `config.yaml` from org payer profile
- `docker-compose.prod.yml` — production stack (API + UI, config + AWS mount)
- `docker-compose.full.yml` — API + UI + chat agent in one compose file
- Vite dev proxy for chat (`/chat` → `:8090`) — no `VITE_CHAT_URL` needed locally
- In-app Chat/LLM status banner on the Chat tab
- `docs/iam-minimal-setup.md` — IAM read-only setup for new adopters
- `docs/integrate-quickstart.md` — one-page MCP integration (Hermes + Cursor)
- `docs/pypi-publish.md` — PyPI release workflow for `org-cost-mcp`
- GitHub issue and PR templates, `CODE_OF_CONDUCT.md`
- Improved `verify-aws-auth` output (`--format json|markdown`)
- GHCR publish targets: `:prod`, `:api` (readiness healthcheck uses `/api/ready`)
- Initial public surface: Go Cost Explorer API, React dashboard, MCP server, demo mode, Hermes skill

### Changed

- README: troubleshooting section for $0 costs, clearer Operate path, live demo deploy notes
- Multi-org SSO example uses sanitized placeholder profiles only
