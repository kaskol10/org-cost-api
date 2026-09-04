# org-cost-api Terraform module

Creates the **payer IRSA role** in the EKS account and optionally installs the [Helm chart](../../../helm/org-cost-api).

## Inputs

| Name | Description | Default |
|------|-------------|---------|
| `cluster_name` | EKS cluster name (tag) | required |
| `oidc_provider_arn` | EKS OIDC provider ARN | required |
| `payer_account_id` | Org management account ID | required |
| `namespace` | Kubernetes namespace | `org-cost` |
| `member_role_name` | StackSet role name in member accounts | `OrgCostReadOnly` |
| `install_helm_chart` | Run `helm_release` | `true` |
| `helm_values` | Extra values YAML strings | `[]` |

## Outputs

- `irsa_role_arn` — pass to StackSet `TrustedPrincipalArn`
- `helm_release_name` / `helm_release_namespace`

## StackSet order

1. Apply this module (or create IRSA role manually) → note `irsa_role_arn`
2. Deploy [StackSet](../../../aws/stackset-iam.yaml) with `TrustedPrincipalArn = irsa_role_arn`
3. Re-run Helm upgrade if bootstrap Job was waiting on member roles

See [docs/eks-operate.md](../../../../docs/eks-operate.md).
