# Browser users

How to give teammates dashboard access without Hermes or AWS credentials on their laptop.

---

## Architecture

```text
Browser ──► static React app ──► Go API ──► AWS (CE + EC2)
              (no AWS keys)         (org creds)
```

The frontend never holds AWS credentials. It calls your deployed API with an optional Bearer token.

---

## Deploy

1. **Run the Go API** where it can reach AWS (VM, Kubernetes, etc.) with org `config.yaml` mounted.

2. **Build the frontend** with API URL and token:

   ```bash
   cd frontend
   VITE_API_URL=https://costs.internal.example.com \
   VITE_API_TOKEN=your-shared-read-token \
   npm run build
   ```

   Serve `frontend/dist/` from nginx, S3+CloudFront, or the same ingress as the API.

3. **Configure backend**:

   ```yaml
   listen_addr: ":8080"
   cors_origin: "https://costs.internal.example.com"
   api_token_env: ORG_COST_API_TOKEN
   ```

   Set `ORG_COST_API_TOKEN` to the same value as `VITE_API_TOKEN`.

---

## Auth summary

| Setting | Where |
|---------|-------|
| `api_token_env: ORG_COST_API_TOKEN` | Backend `config.yaml` |
| `ORG_COST_API_TOKEN` | Backend runtime env |
| `VITE_API_TOKEN` | Frontend **build-time** env (internal shared secret only — not public-internet auth) |
| `cors_origin` | Backend `config.yaml` — must match browser origin |

`/api/health` and `/api/ready` stay unauthenticated for load balancers (liveness vs readiness).

Full table: [getting-started.md § Production auth](./getting-started.md#production-auth--one-page).

---

## Local dev

```bash
cp frontend/.env.example frontend/.env.local
# VITE_API_TOKEN=<same as ORG_COST_API_TOKEN>
npm run dev
```

Restart Vite after editing `.env.local`.

---

## What browser users cannot do

- Refresh AWS SSO — that's on the API host (run `granted sso login` there, or use IRSA/instance profile).
- Use MCP tools — see [hermes-setup.md](./hermes-setup.md) for agent access.

The **Ask** tab uses the same bounded API as MCP (no direct AWS access in the browser).
