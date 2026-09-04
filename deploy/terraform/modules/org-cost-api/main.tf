terraform {
  required_version = ">= 1.3"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = ">= 2.10"
    }
  }
}

data "aws_caller_identity" "current" {}

data "aws_iam_policy_document" "org_cost_api" {
  statement {
    sid    = "CostExplorerRead"
    effect = "Allow"
    actions = [
      "ce:GetCostAndUsage",
      "ce:GetCostAndUsageWithResources",
      "ce:GetDimensionValues",
      "ce:GetTags",
    ]
    resources = ["*"]
  }

  statement {
    sid    = "EC2InventoryRead"
    effect = "Allow"
    actions = [
      "ec2:DescribeVolumes",
      "ec2:DescribeSnapshots",
      "ec2:DescribeInstances",
      "ec2:DescribeRegions",
    ]
    resources = ["*"]
  }

  statement {
    sid    = "CloudWatchMetricsRead"
    effect = "Allow"
    actions = [
      "cloudwatch:GetMetricData",
      "cloudwatch:ListMetrics",
    ]
    resources = ["*"]
  }

  statement {
    sid    = "OrganizationsList"
    effect = "Allow"
    actions = [
      "organizations:ListAccounts",
      "organizations:DescribeOrganization",
    ]
    resources = ["*"]
  }

  statement {
    sid    = "AssumeMemberReadOnly"
    effect = "Allow"
    actions = ["sts:AssumeRole"]
    resources = [
      "arn:aws:iam::*:role/${var.member_role_name}",
    ]
  }

  statement {
    sid    = "STSIdentity"
    effect = "Allow"
    actions = ["sts:GetCallerIdentity"]
    resources = ["*"]
  }
}

resource "aws_iam_policy" "org_cost_api" {
  name        = var.policy_name
  description = "org-cost-api payer IRSA policy (CE, ListAccounts, AssumeRole to member accounts)"
  policy      = data.aws_iam_policy_document.org_cost_api.json
}

module "irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.39"

  role_name = var.role_name

  role_policy_arns = [
    aws_iam_policy.org_cost_api.arn,
  ]

  oidc_providers = {
    main = {
      provider_arn               = var.oidc_provider_arn
      namespace_service_accounts = ["${var.namespace}:${var.service_account_name}"]
    }
  }

  tags = var.tags
}

locals {
  helm_chart = var.helm_chart_path != "" ? var.helm_chart_path : "${path.module}/../../../helm/org-cost-api"
}

resource "helm_release" "org_cost_api" {
  count = var.install_helm_chart ? 1 : 0

  name      = var.helm_release_name
  namespace = var.namespace
  chart     = local.helm_chart

  create_namespace = var.create_namespace
  wait             = var.helm_wait
  timeout          = var.helm_timeout

  values = concat(
    [
      yamlencode({
        serviceAccount = {
          create = true
          name   = var.service_account_name
          annotations = {
            "eks.amazonaws.com/role-arn" = module.irsa.iam_role_arn
          }
        }
      }),
    ],
    var.helm_values,
  )
}
