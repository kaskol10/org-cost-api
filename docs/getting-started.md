# Getting started — first hour

One path from zero to a working dashboard, whether you use the browser, Hermes MCP, or both.

## Try without AWS (demo)

```bash
docker compose -f docker-compose.demo.yml up --build
# open http://localhost:8080 — synthetic fixture data, no credentials
```

See [ARCHITECTURE.md](./ARCHITECTURE.md#demo-vs-production-mode) for how demo mode differs from production.

---

## Checklist (real AWS)

- [ ] **1. Copy config**

  ```bash
  cp config.example.yaml config.yaml
  # or for multi-account orgs:
  cp config.example.org.yaml config.yaml
  ```

- [ ] **2. Generate starter config**

  ```bash
  chmod +x scripts/bootstrap-config.sh scripts/verify-aws-auth.sh scripts/operate-bootstrap.sh
  ./scripts/operate-bootstrap.sh --profile YOUR_PAYER_PROFILE --output config.yaml
  # or step-by-step: bootstrap-config.sh then verify-aws-auth.sh
  # Optional: --profile-map profiles.csv for bulk profile assignment
  ```

  Billing model per row: [onboarding-aws-auth.md](./onboarding-aws-auth.md#billing-model-payer-linked-vs-member-billed) · IAM: [iam-minimal-setup.md](./iam-minimal-setup.md).

- [ ] **3. Verify AWS auth**

  ```bash
  ./scripts/verify-aws-auth.sh config.yaml --format markdown
  ```

  Fix any STS or Cost Explorer failures before starting the server.

- [ ] **4. Start backend**

  ```bash
  cd backend && go run ./cmd/server
  ```

  `config.yaml` is auto-discovered from the repo root (or set `CONFIG_PATH` / `-config`).

  Smoke: `curl -s http://localhost:8080/api/health` and `curl -s http://localhost:8080/api/ready`
  (liveness vs readiness — see Production auth below)

- [ ] **5. Start frontend**

  ```bash
  cd frontend && npm install && npm run dev
  ```

  Open http://localhost:5173

- [ ] **6. Set frontend token (if API auth enabled)**

  When `api_token` or `api_token_env` is set in config:

  ```bash
  cp frontend/.env.example frontend/.env.local
  # Set VITE_API_TOKEN to the same value as ORG_COST_API_TOKEN
  npm run dev   # restart after editing .env.local
  ```

- [ ] **7. Confirm dashboard**

  - `GET /api/dashboard` returns account totals (not all $0).
  - Click **Refresh** in the UI if SSO session was stale.

- [ ] **8. Optional — Hermes or Cursor MCP**

  ```bash
  cd mcp-server && python3 -m venv .venv && .venv/bin/pip install -e .
  export ORG_COST_API_URL=http://127.0.0.1:8080
  # Set ORG_COST_API_TOKEN if api auth enabled — see table below
  ```

  Full wiring: [mcp-setup.md](./mcp-setup.md) (Hermes + Cursor)

---

## Production auth — one page

When `api_token_env: ORG_COST_API_TOKEN` (or inline `api_token`) is set, every `/api/*` route except `/api/health`, `/api/ready`, and `/api/meta` requires:

```http
Authorization: Bearer <token>
```

| Component | Variable / config | Notes |
|-----------|-------------------|-------|
| **Backend** | `api_token_env: ORG_COST_API_TOKEN` in `config.yaml` | Or inline `api_token:` (avoid in git). If `api_token_env` is set, the env var must be non-empty at startup or the process refuses to boot. |
| **Backend runtime** | `export ORG_COST_API_TOKEN='...'` | Must be set before starting the Go server |
| **Frontend (dev)** | `VITE_API_TOKEN` in `frontend/.env.local` | Same value as backend token; restart Vite after change |
| **Frontend (prod static)** | `VITE_API_TOKEN` at **build time** | Baked into the bundle; rebuild to rotate |
| **MCP server** | `ORG_COST_API_TOKEN` env var | Set where Hermes spawns the MCP subprocess |
| **Cron / scripts** | `ORG_COST_API_TOKEN` | e.g. `scripts/daily-snapshot.sh` forwards it automatically |
| **Liveness probe** | No token | `GET /api/health` — process up |
| **Readiness probe** | No token | `GET /api/ready` — accounts loaded; optional billing STS. `billing_check: skipped` means no payer configured; member-only setups re-STS `accounts[0]` and set `accounts_check` (`ok` / `failed`). Use health for liveness and ready for readiness in Kubernetes/LB. |

**CORS:** Set `cors_origin` in config to your frontend origin (e.g. `https://costs.internal.example.com`). Default `http://localhost:5173` is for local dev only.

**Browser-only users:** See [browser-users.md](./browser-users.md).

**Hermes-only users:** See [hermes-setup.md](./hermes-setup.md) and [hermes-nous-deployment.md](./hermes-nous-deployment.md).

---

## Operate on EKS (Gateway API + IRSA)

For an existing EKS cluster with Gateway API and IRSA (no `~/.aws` mount in the pod):

1. [deploy/aws/README.md](../deploy/aws/README.md) — StackSet `OrgCostReadOnly` across the org
2. [deploy/eks/README.md](../deploy/eks/README.md) — cluster prerequisites
3. [eks-operate.md](./eks-operate.md) — end-to-end checklist (Helm, HTTPRoute, optional bootstrap Job)

---

## Next steps

| Task | Doc |
|------|-----|
| EKS + Gateway API | [eks-operate.md](./eks-operate.md) |
| Add one account | [add-account.md](./add-account.md) |
| Member-billed account | [member-billed-account.md](./member-billed-account.md) |
| IAM policy for FinOps | [iam-readonly-policy.json](./iam-readonly-policy.json) |
| Costs show $0 | [README § Costs show $0](../README.md#costs-show-0) |
