# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Public demo deploy: `deploy/fly.toml`, `scripts/deploy-demo.sh`, GitHub Actions demo deploy workflow
- Live demo URL: https://org-cost-api-demo.fly.dev
- `scripts/operate-bootstrap.sh` — bootstrap + verify helper for Operate path
- HTTP/SSE MCP transport (`MCP_TRANSPORT=sse|streamable-http`) and `docker-compose.mcp-http.yml`
- Complete [openapi.yaml](../openapi.yaml) with response schemas and CI validation
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

### Changed

- README: troubleshooting section for $0 costs, clearer Operate path, live demo deploy notes
- Multi-org SSO example uses sanitized placeholder profiles only

## [0.1.0] — 2025-01-01

Initial public release: Go Cost Explorer API, React dashboard, MCP server, demo mode, Hermes skill.
