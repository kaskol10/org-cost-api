# Optional Terraform — IRSA + Helm

Thin wrapper for teams that already manage EKS with Terraform. **Helm is the portable contract**; this module creates the payer IRSA role and installs the chart.

## Module

[`modules/org-cost-api`](./modules/org-cost-api) — IRSA role + `helm_release` for `deploy/helm/org-cost-api`.

## Example

```hcl
module "org_cost_api" {
  source = "github.com/kaskol10/org-cost-api//deploy/terraform/modules/org-cost-api"

  cluster_name       = "my-eks"
  namespace          = "org-cost"
  oidc_provider_arn  = module.eks.oidc_provider_arn
  payer_account_id   = "111111111111"

  helm_values = [
    yamlencode({
      gateway = {
        name      = "shared-gateway"
        namespace = "gateway-system"
      }
      hostnames = ["costs.internal.example.com"]
      bootstrapJob = {
        enabled = true
      }
    })
  ]
}
```

Without Terraform: create the IRSA role manually and run `helm upgrade --install` — see [docs/eks-operate.md](../../docs/eks-operate.md).
