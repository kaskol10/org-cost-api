# PyPI publish — org-cost-mcp

How to release `org-cost-mcp` to PyPI. CI publishes automatically on version tags.

## Package

| Field | Value |
|-------|-------|
| Name | `org-cost-mcp` |
| Source | [mcp-server/](../mcp-server/) |
| Entry points | `org-cost-mcp` (stdio MCP), `org-cost-chat` (HTTP chat agent) |

## Automated release (recommended)

1. Bump version in [mcp-server/pyproject.toml](../mcp-server/pyproject.toml) (must match the tag intent).
2. Update [CHANGELOG.md](../CHANGELOG.md).
3. Tag and push:

```bash
git tag v0.0.1
git push origin v0.0.1
```

The [publish workflow](../.github/workflows/publish.yml) builds Docker images (demo / prod / api / **chat**) and publishes to PyPI via **Trusted Publishing (OIDC)**.

### One-time: PyPI trusted publisher

1. Create the project on PyPI if needed: https://pypi.org/manage/projects/ (or let the first OIDC upload create it).
2. Project → **Publishing** → **Add a new pending publisher** (or trusted publisher):

| Field | Value |
|-------|-------|
| PyPI project name | `org-cost-mcp` |
| Owner | `kaskol10` |
| Repository | `org-cost-api` |
| Workflow name | `publish.yml` |
| Environment name | *(leave empty unless you use GitHub Environments)* |

3. Ensure the workflow has `permissions: id-token: write` (already set in this repo).

No `PYPI_API_TOKEN` secret is required for trusted publishing.

## Docker images (GHCR)

On each `v*` tag, images are pushed to `ghcr.io/kaskol10/org-cost-api`:

| Tag | Contents |
|-----|----------|
| `:demo` / `:latest` | Demo API + UI |
| `:prod` | Production API + UI |
| `:api` | API only |
| `:chat` / `:org-cost-chat` | Chat agent (`mcp-server/Dockerfile`) |
| Helm OCI `.../charts/org-cost-api` | Chart from `deploy/helm/org-cost-api` (version = tag without `v`) |

Versioned aliases are also pushed (e.g. `:chat-v0.0.1`, `:prod-v0.0.1`).

Pull (may need `docker login ghcr.io`):

```bash
docker pull ghcr.io/kaskol10/org-cost-api:chat
docker pull ghcr.io/kaskol10/org-cost-api:prod
```

## Manual release

```bash
cd mcp-server
python3 -m venv .venv && .venv/bin/pip install build twine
python -m build
twine upload dist/*
```

Verify:

```bash
pip install org-cost-mcp
export ORG_COST_API_URL=http://localhost:8080
org-cost-mcp   # Ctrl+C to exit
```

## Version policy

- **Patch** — bug fixes, dependency bumps
- **Minor** — new MCP tools, chat features (backward compatible)
- **Major** — breaking MCP tool schemas or config discovery

Keep [mcp-server/pyproject.toml](../mcp-server/pyproject.toml) version in sync with git tags for Docker + PyPI releases.
