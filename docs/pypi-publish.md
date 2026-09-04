# PyPI publish — org-cost-mcp

How to release `org-cost-mcp` to PyPI. CI publishes automatically on version tags; use this doc for manual releases or first-time setup.

## Package

| Field | Value |
|-------|-------|
| Name | `org-cost-mcp` |
| Source | [mcp-server/](../mcp-server/) |
| Entry points | `org-cost-mcp` (stdio MCP), `org-cost-chat` (HTTP chat agent) |

## Automated release (recommended)

1. Bump version in [mcp-server/pyproject.toml](../mcp-server/pyproject.toml).
2. Update [CHANGELOG.md](../CHANGELOG.md).
3. Tag and push:

```bash
git tag v0.2.0
git push origin v0.2.0
```

The [publish workflow](../.github/workflows/publish.yml) builds Docker images **and** publishes to PyPI when `PYPI_API_TOKEN` is configured as a GitHub secret.

### GitHub secret

1. Create a PyPI API token at https://pypi.org/manage/account/token/ (scope: `org-cost-mcp` or entire account).
2. Add repository secret **`PYPI_API_TOKEN`** in GitHub → Settings → Secrets → Actions.

The workflow uses `pypa/gh-action-pypi-publish@release/v1`.

## Manual release

```bash
cd mcp-server
python3 -m venv .venv && .venv/bin/pip install build twine
python -m build
# Test upload:
twine upload --repository testpypi dist/*
# Production:
twine upload dist/*
```

Verify:

```bash
pip install org-cost-mcp
export ORG_COST_API_URL=http://localhost:8080
org-cost-mcp   # Ctrl+C to exit
```

## CI smoke test (local)

```bash
cd mcp-server
pip install build
python -m build
pip install dist/org_cost_mcp-*.whl
pytest
```

## Version policy

- **Patch** — bug fixes, dependency bumps
- **Minor** — new MCP tools, chat features (backward compatible)
- **Major** — breaking MCP tool schemas or config discovery

Keep [mcp-server/pyproject.toml](../mcp-server/pyproject.toml) version in sync with git tags for Docker + PyPI releases.
