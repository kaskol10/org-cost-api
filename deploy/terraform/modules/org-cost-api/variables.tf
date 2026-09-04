variable "cluster_name" {
  description = "EKS cluster name (informational tag)."
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace for org-cost-api."
  type        = string
  default     = "org-cost"
}

variable "service_account_name" {
  description = "Kubernetes ServiceAccount name."
  type        = string
  default     = "org-cost-api"
}

variable "oidc_provider_arn" {
  description = "ARN of the EKS OIDC provider (for IRSA trust)."
  type        = string
}

variable "payer_account_id" {
  description = "Organization management account ID (for documentation/tags)."
  type        = string
}

variable "member_role_name" {
  description = "IAM role name in member accounts (StackSet OrgCostReadOnly)."
  type        = string
  default     = "OrgCostReadOnly"
}

variable "role_name" {
  description = "IAM role name for the payer IRSA role in the EKS account."
  type        = string
  default     = "org-cost-api-irsa"
}

variable "policy_name" {
  description = "IAM policy name attached to the IRSA role."
  type        = string
  default     = "org-cost-api-irsa"
}

variable "tags" {
  description = "Tags applied to IAM resources."
  type        = map(string)
  default     = {}
}

variable "install_helm_chart" {
  description = "When false, only create IRSA — install Helm separately."
  type        = bool
  default     = true
}

variable "helm_release_name" {
  description = "Helm release name."
  type        = string
  default     = "org-cost-api"
}

variable "helm_chart_path" {
  description = "Local path to the Helm chart. Defaults to deploy/helm/org-cost-api in this repo."
  type        = string
  default     = ""
}

variable "create_namespace" {
  description = "Create namespace on helm install."
  type        = bool
  default     = true
}

variable "helm_wait" {
  description = "Wait for Helm resources to be ready."
  type        = bool
  default     = true
}

variable "helm_timeout" {
  description = "Helm install timeout in seconds."
  type        = number
  default     = 600
}

variable "helm_values" {
  description = "Additional Helm values as YAML-encoded strings."
  type        = list(string)
  default     = []
}
