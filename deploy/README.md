# Deploy

## EKS (production Operate path)

Real AWS org on an existing EKS cluster — Gateway API `HTTPRoute`, IRSA, optional StackSet IAM:

| Component | Path |
|-----------|------|
| Org-wide IAM (StackSet) | [aws/README.md](./aws/README.md) |
| Helm chart | [helm/org-cost-api](./helm/org-cost-api) — also published to GHCR on `v*` tags |
| EKS prerequisites | [eks/README.md](./eks/README.md) |
| End-to-end guide | [docs/eks-operate.md](../docs/eks-operate.md) |
| Terraform (optional IRSA + Helm) | [terraform/README.md](./terraform/README.md) |

Install a released chart from GHCR (version matches the git tag, without the `v`):

```bash
helm registry login ghcr.io
helm upgrade --install org-cost-api \
  oci://ghcr.io/kaskol10/org-cost-api/charts/org-cost-api \
  --version 0.0.5 \
  --namespace monitoring --create-namespace \
  -f values-site.yaml
```

---

## Public demo (Fly.io)

The demo image serves **fixture data only** (no AWS credentials). It includes the React UI and API in one container.

**Live URL:** [https://org-cost-api-demo.fly.dev](https://org-cost-api-demo.fly.dev) (canonical value in [DEMO_URL](./DEMO_URL))

## Docker (local or any host)

```bash
docker compose -f docker-compose.demo.yml up --build
# open http://localhost:8080
```

Or pull from GHCR after a release tag:

```bash
docker run -p 8080:8080 ghcr.io/kaskol10/org-cost-api:demo      # synthetic demo
docker run -p 8080:8080 -v ./config.yaml:/config/config.yaml:ro \
  -v ~/.aws:/home/app/.aws:ro ghcr.io/kaskol10/org-cost-api:prod  # real AWS
```

## Fly.io (public demo)

### One-command deploy

```bash
flyctl auth login
./scripts/deploy-demo.sh
```

Uses [fly.toml](./fly.toml). After deploy, the script writes the URL to [DEMO_URL](./DEMO_URL) and prints smoke-test commands.

### CI deploy

GitHub Actions workflow [`.github/workflows/demo-deploy.yml`](../.github/workflows/demo-deploy.yml) deploys on push to `main` (demo-related paths) or manual dispatch. Requires repository secret `FLY_API_TOKEN`.

### Manual steps

1. Install [flyctl](https://fly.io/docs/hands-on/install-flyctl/) and `fly auth login`.
2. From repo root:

```bash
flyctl deploy --config deploy/fly.toml
```

3. Confirm `https://org-cost-api-demo.fly.dev/api/meta` returns `{ "demo": true }`.
4. Update [README.md](../README.md) if the app name or URL changes.

## Environment

| Variable | Demo value |
|----------|------------|
| `ORG_COST_DEMO` | `1` |
| `CONFIG_PATH` | `/config/demo.yaml` |
| `STATIC_DIR` | `/app/static` |
| `HISTORY_DIR` | `/data/history` (optional volume) |

Do **not** mount real AWS credentials or production `config.yaml` on the public demo.

## Rate limiting (recommended for public demo)

The API has no built-in HTTP rate limit. For a public demo, protect expensive routes at the edge:

| Route | Risk |
|-------|------|
| `GET /api/report?refresh=1` | Triggers live Cost Explorer calls |
| `POST /api/ask` with `refresh: true` | Same |

**nginx example** (limit refresh to 5 req/min per IP):

```nginx
location ~ ^/api/(report|ask) {
  limit_req zone=demo_refresh burst=2 nodelay;
  proxy_pass http://127.0.0.1:8080;
}
```

**Fly.io / Cloudflare:** use their rate-limit or WAF rules on `/api/report?refresh` and `/api/ask`.

Demo mode uses synthetic data — rate limits still prevent abuse of CPU and bandwidth.
