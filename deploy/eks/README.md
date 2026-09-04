# EKS prerequisites — org-cost-api

Checklist before installing the [Helm chart](../helm/org-cost-api). This repo does **not** provision EKS, VPC, node groups, or a Gateway API controller — bring your own cluster.

End-to-end Operate path: [docs/eks-operate.md](../../docs/eks-operate.md).

## Cluster requirements

| Requirement | Why |
|-------------|-----|
| **Existing EKS cluster** | Workload runs as a standard Deployment |
| **OIDC provider** | Required for IRSA (`eks.amazonaws.com/role-arn` on ServiceAccount) |
| **Gateway API CRDs** | Chart creates `HTTPRoute` only — no `Ingress` |
| **Gateway API controller** | e.g. [AWS Gateway API Controller](https://github.com/aws/aws-application-networking-k8s) (ALB), [Envoy Gateway](https://gateway.envoyproxy.io/), or Istio Gateway API |
| **Existing `Gateway`** | HTTPRoute references your shared Gateway by name/namespace |
| **StorageClass** (optional) | PVC for `/data/history` trend snapshots — disable with `persistence.enabled=false` if not needed |

## AWS Organization IAM

| Step | Doc |
|------|-----|
| All IAM (StackSet + IRSA, one command) | [deploy/aws/onboard-iam.sh](../aws/onboard-iam.sh) · [README](../aws/README.md) |

Trust chain:

```text
Pod (IRSA) → payer IRSA role → sts:AssumeRole → OrgCostReadOnly (per account)
```

## Gateway API

1. Confirm CRDs are installed:

   ```bash
   kubectl get crd gateways.gateway.networking.k8s.io httproutes.gateway.networking.k8s.io
   ```

2. Note your Gateway name and namespace (chart defaults: `shared-gateway` in `gateway-system`).

3. Set Helm values:

   ```yaml
   gateway:
     name: shared-gateway
     namespace: gateway-system
     sectionName: https   # optional listener name
   hostnames:
     - costs.internal.example.com
   ```

4. After install, verify the HTTPRoute:

   ```bash
   kubectl get httproute -n <release-namespace>
   ```

## IRSA (EKS pod identity)

Created automatically by the StackSet in the **deploy account** when `OidcProviderArn` is set:

```bash
./deploy/aws/onboard-iam.sh --profile Master.AdministratorAccess ...
```

Annotate the Helm ServiceAccount with the printed IRSA ARN:

```yaml
serviceAccount:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::444455556666:role/org-cost-api-irsa
```

Trust policy reference: [deploy/aws/irsa-trust-policy.json](../aws/irsa-trust-policy.json).

## Config modes

| Mode | Helm values | When |
|------|-------------|------|
| **Static** | `staticConfig.data` or `existingConfigSecret` | Fixed account list, member-billed rows you edit by hand |
| **Bootstrap Job** | `bootstrapJob.enabled: true` | Standard org — `ListAccounts` → ConfigMap with `role_arn` per account |

Bootstrap mirrors [`scripts/bootstrap-config.sh`](../../scripts/bootstrap-config.sh). Member-billed accounts still need manual `billing_profile: account` rows — see [docs/member-billed-account.md](../../docs/member-billed-account.md).

## Pre-flight verify

Run locally or from a one-off Job before cutover:

```bash
./scripts/verify-aws-auth.sh config.yaml --format markdown
```

## Quick install

```bash
helm upgrade --install org-cost-api ./deploy/helm/org-cost-api \
  --namespace org-cost --create-namespace \
  -f my-values.yaml
```

Chart defaults live in [`values.yaml`](../helm/org-cost-api/values.yaml) (generic). Site overlays (e.g. [`values-site.example.yaml`](../helm/org-cost-api/values-site.example.yaml)) only set IRSA, images, Gateway, and LLM.

See [docs/eks-operate.md](../../docs/eks-operate.md) for the full checklist.
