# Contributing

Thanks for helping improve org-cost-api.

## Try it first

No AWS credentials required:

```bash
ORG_COST_DEMO=1 go run ./backend/cmd/server -config config.demo.yaml
# or: docker compose -f docker-compose.demo.yml up
```

See [docs/getting-started.md](docs/getting-started.md) for the full demo vs real-AWS paths.

## Development setup

```bash
# Backend
cd backend && go test ./...

# Frontend
cd frontend && npm ci && npm test && npm run build

# MCP server
cd mcp-server && pip install -e ".[dev]" && pytest
```

CI runs all three on every PR (see `.github/workflows/ci.yml`).

## Pull requests

- Keep changes focused; match existing style in the touched package.
- Add or update tests for behavior changes.
- Do not commit `config.yaml`, secrets, or real account IDs.
- Update docs when you change user-facing behavior (README, `docs/`, OpenAPI).

## Release

- Docker: tag `v*` pushes `:demo`, `:prod`, `:api` to GHCR — see [.github/workflows/publish.yml](.github/workflows/publish.yml).
- PyPI: same tag publishes `org-cost-mcp` — see [docs/pypi-publish.md](docs/pypi-publish.md).

## Demo mode

Fixture data lives in `backend/internal/demo/`. When adding API fields, update demo fixtures so the public demo stays representative.

## Questions

Open a GitHub issue for bugs and feature ideas.
