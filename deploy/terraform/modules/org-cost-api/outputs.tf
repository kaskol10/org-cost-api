output "irsa_role_arn" {
  description = "ARN of the payer IRSA IAM role — use as StackSet TrustedPrincipalArn."
  value       = module.irsa.iam_role_arn
}

output "irsa_role_name" {
  description = "Name of the payer IRSA IAM role."
  value       = module.irsa.iam_role_name
}

output "helm_release_name" {
  description = "Helm release name (empty if install_helm_chart is false)."
  value       = try(helm_release.org_cost_api[0].name, null)
}

output "helm_release_namespace" {
  description = "Helm release namespace."
  value       = var.namespace
}

output "payer_account_id" {
  description = "Organization payer account ID passed to the module."
  value       = var.payer_account_id
}

output "deploy_account_id" {
  description = "AWS account ID where IRSA role was created (EKS account)."
  value       = data.aws_caller_identity.current.account_id
}
