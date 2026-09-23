# Operate on EKS — org-cost-api

Deploy org-cost-api on an **existing** EKS cluster with **Gateway API** (`HTTPRoute`) and **IRSA** — no `~/.aws` mount in the pod.

| Layer | Tool | Doc |
|-------|------|-----|
| All IAM (StackSet + IRSA) | CloudFormation | [deploy/aws/README.md](../deploy/aws/README.md) |
| App + HTTPRoute | Helm | [deploy/helm/org-cost-api](../deploy/helm/org-cost-api) |
| IRSA + Helm (optional) | Terraform | [deploy/terraform/modules/org-cost-api](../deploy/terraform/modules/org-cost-api) |
| Prerequisites | EKS checklist | [deploy/eks/README.md](../deploy/eks/README.md) |

For Docker Compose or local dev, use [getting-started.md](./getting-started.md) instead.

---

## 1. Prerequisites

- [ ] EKS cluster with OIDC provider enabled
- [ ] Gateway API CRDs + controller installed; an existing `Gateway` resource
- [ ] Org management access to deploy a StackSet (or delegated admin)
- [ ] Cost Explorer org view enabled on the payer account ([iam-minimal-setup.md](./iam-minimal-setup.md))

Details: [deploy/eks/README.md](../deploy/eks/README.md).

---

## 2. IAM onboarding (two commands)

Split so the EKS team and the org-management team can each use only their own credentials. Run **deploy first**.

```bash
# OIDC provider ARN (deploy / EKS account)
ISSUER=$(aws eks describe-cluster --name YOUR_CLUSTER \
  --profile Shared-Services.AdministratorAccess \
  --query 'cluster.identity.oidc.issuer' --output text)
OIDC_ARN="arn:aws:iam::444455556666:oidc-provider/${ISSUER#https://}"

./deploy/aws/onboard-iam.sh deploy \
  --profile Shared-Services.AdministratorAccess \
  --deploy-account-id 444455556666 \
  --oidc-provider-arn "$OIDC_ARN" \
  --namespace monitoring

./deploy/aws/onboard-iam.sh payer \
  --profile Master.AdministratorAccess \
  --payer-account-id 111122223333 \
  --deploy-account-id 444455556666 \
  --ou-id ou-abcd-12345678
```

Replace `ou-abcd-12345678` with your OU ID (`ou-...`, not organization ID `o-...`). For a pilot, use `--accounts 444455556666,111122223333` instead of `--ou-id`.

If one person has both profiles, omit the subcommand and pass `--deploy-account-profile` (see [deploy/aws/README.md](../deploy/aws/README.md)).

The script prints the Helm `serviceAccount` annotation when complete.

Full reference: [deploy/aws/README.md](../deploy/aws/README.md).

---

## 3. Install Helm chart

Example `values-eks.yaml`:

```yaml
serviceAccount:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::444455556666:role/org-cost-api-irsa

gateway:
  name: shared-gateway
  namespace: gateway-system
  sectionName: https

hostnames:
  - costs.internal.example.com

bootstrapJob:
  enabled: true
  defaultRegion: eu-west-1
  corsOrigin: "https://costs.internal.example.com"
  memberRoleName: OrgCostReadOnly

staticConfig:
  enabled: true
  data: |
    listen_addr: ":8080"
    cors_origin: "https://costs.internal.example.com"
    cost_lookback_days: 30
    billing_profile: ""
    api_token_env: ORG_COST_API_TOKEN
    accounts: []

apiToken: "change-me-in-production"

persistence:
  enabled: true
  storageClass: gp3
```

Install:

```bash
helm upgrade --install org-cost-api ./deploy/helm/org-cost-api \
  --namespace org-cost --create-namespace \
  -f values-eks.yaml
```

After bootstrap Job completes, restart the Deployment to pick up the new ConfigMap:

```bash
kubectl -n org-cost rollout restart deployment/org-cost-api
```

---

## 4. Verify HTTPRoute and readiness

```bash
kubectl -n org-cost get httproute,svc,pods
curl -s https://costs.internal.example.com/api/health
curl -s https://costs.internal.example.com/api/ready
```

`/api/ready` should report accounts loaded. Fix IAM or config before exposing to users.

Pre-flight (local clone):

```bash
kubectl -n org-cost get configmap org-cost-api-config -o jsonpath='{.data.config\.yaml}' > /tmp/config.yaml
./scripts/verify-aws-auth.sh /tmp/config.yaml --format markdown
```

---

## 5. Production auth and CORS

Set `api_token_env: ORG_COST_API_TOKEN` in config and pass the token via Helm `apiToken` or an existing Secret. Set `cors_origin` to your Gateway hostname.

Probe table: [getting-started.md § Production auth](./getting-started.md#production-auth--one-page).

---

## 6. Optional — Chat / LLM

The Helm chart deploys **API+UI only** by default. Conversational Ask needs a second image (`org-cost-chat`) and a UI rebuild that bakes in `VITE_CHAT_URL`.

1. **Build and push** the chat image:

```bash
docker build --platform linux/arm64 \
  -t YOUR_REGISTRY/org-cost-chat:latest \
  -f mcp-server/Dockerfile mcp-server
# docker push …
```

2. **Rebuild API/UI** so the browser knows where chat lives (same Gateway host; HTTPRoute sends `/v1/chat` and `/v1/suggestions` to the chat Service):

```bash
docker build --platform linux/arm64 --target prod-ui \
  --build-arg VITE_CHAT_URL=https://costs.internal.example.com \
  -t YOUR_REGISTRY/org-cost-api:latest .
# docker push …  then bump image.tag in your values overlay
```

3. **Enable chat** in a site values file:

```yaml
chat:
  enabled: true
  image:
    repository: YOUR_REGISTRY/org-cost-chat
    tag: latest
  llm:
    provider: vllm   # or litellm
    baseUrl: https://vllm.example.com/v1
    model: your-model-id
    # apiKey: ""   # vLLM → EMPTY; set for LiteLLM
```

```bash
helm upgrade --install org-cost-api ./deploy/helm/org-cost-api \
  --namespace org-cost -f my-values.yaml
```

Site overlay example: `deploy/helm/org-cost-api/values-site.example.yaml`.

4. **Verify**:

```bash
curl -s https://costs.internal.example.com/v1/chat/health
# expect llm_provider / llm_model populated
```

Without `VITE_CHAT_URL` in the UI bundle, Ask stays on rule-based `/api/ask`. Full LLM options: [chat-setup.md](./chat-setup.md).

---

## 7. Optional — MCP

Point agents at the Gateway URL (stdio MCP on a bastion or HTTP MCP per [mcp-setup.md](./mcp-setup.md)):

```bash
export ORG_COST_API_URL=https://costs.internal.example.com
export ORG_COST_API_TOKEN=your-token
org-cost-mcp
```

---

## Member-billed accounts

Bootstrap and StackSet assume **payer-linked** CE. Accounts with their own billing need `billing_profile: account` on that row — edit the ConfigMap after bootstrap. See [member-billed-account.md](./member-billed-account.md).

---

## Troubleshooting

| Symptom | Check |
|---------|--------|
| StackSet `Invalid principal` | Run `./deploy/aws/onboard-iam.sh deploy` first so IRSA exists, then `payer` |
| `/api/ready` fails STS | IRSA annotation matches `org-cost-api-irsa`; ServiceAccount name/namespace match StackSet params |
| Empty account list | Bootstrap Job logs; `organizations:ListAccounts` on IRSA role |
| HTTPRoute not routing | Gateway listener `sectionName`, controller logs, hostname on HTTPRoute |
| All costs $0 | [README § Costs show $0](../README.md#costs-show-0), CE org view on payer |

---

## Related

| Task | Doc |
|------|-----|
| IAM policy reference | [iam-readonly-policy.json](./iam-readonly-policy.json) |
| Billing models | [onboarding-aws-auth.md](./onboarding-aws-auth.md) |
| Architecture | [ARCHITECTURE.md](./ARCHITECTURE.md) |
