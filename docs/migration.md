# Migration from ec2-other naming

If you previously used `ec2-other-costs` MCP or `~/.ec2-other/` paths, follow these steps to migrate to org-cost-api.

## 1. Update Hermes MCP config

- MCP server id → `org-cost-api`
- Command → `org-cost-mcp` (reinstall: `cd mcp-server && .venv/bin/pip install -e .`)

See [hermes-setup.md](hermes-setup.md) for full Hermes configuration.

## 2. Environment variables

| Old | New |
|-----|-----|
| `EC2_OTHER_API_URL` | `ORG_COST_API_URL` |
| (none) | `ORG_COST_API_TOKEN` (when API auth is enabled) |

The MCP client still accepts `EC2_OTHER_API_URL` with a deprecation warning, but you should switch to `ORG_COST_API_URL`.

## 3. History and reports directories

- New default: `~/.org-cost/history/` and `~/.org-cost/reports/`
- The backend auto-fallbacks to `~/.ec2-other/` until you copy data:

```bash
mkdir -p ~/.org-cost
cp -a ~/.ec2-other/history ~/.org-cost/ 2>/dev/null || true
cp -a ~/.ec2-other/reports ~/.org-cost/ 2>/dev/null || true
```

## 4. Frontend dev proxy

When `api_token` is configured in `config.yaml`, set the matching token for the Vite dev server:

```bash
VITE_API_TOKEN=your-token npm run dev
```

## 5. Verify

```bash
# API health
curl http://localhost:8080/api/health

# MCP health (from Hermes or stdio)
# check_api_health tool → {"healthy": true}
```
