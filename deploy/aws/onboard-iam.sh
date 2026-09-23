#!/usr/bin/env bash
# IAM onboarding for org-cost-api on EKS — split by account access.
#
# Deploy account (EKS / shared-services) — IRSA only:
#   ./deploy/aws/onboard-iam.sh deploy \
#     --profile Shared-Services.AdministratorAccess \
#     --deploy-account-id 444455556666 \
#     --oidc-provider-arn 'arn:aws:iam::444455556666:oidc-provider/oidc.eks.eu-west-1.amazonaws.com/id/XXXX' \
#     --namespace monitoring \
#     --name-prefix acme
#
# Payer / org management — OrgCostReadOnly + StackSet (no deploy-account credentials):
#   ./deploy/aws/onboard-iam.sh payer \
#     --profile Master.AdministratorAccess \
#     --payer-account-id 111122223333 \
#     --deploy-account-id 444455556666 \
#     --ou-id r-xxxx \
#     --name-prefix acme
#
# Both from one machine (legacy): omit the subcommand and pass both profiles.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
STACKSET_TEMPLATE="$ROOT/deploy/aws/stackset-iam.yaml"
IRSA_TEMPLATE="$ROOT/deploy/aws/irsa-role.yaml"
PAYER_TEMPLATE="$ROOT/deploy/aws/payer-readonly-role.yaml"
NAME_PREFIX=""
IRSA_ROLE_BASE="org-cost-api-irsa"
MEMBER_ROLE_BASE="OrgCostReadOnly"
SIDE=""
PROFILE=""
DEPLOY_PROFILE=""
PAYER_ACCOUNT_ID=""
DEPLOY_ACCOUNT_ID=""
OIDC_PROVIDER_ARN=""
OU_ID=""
ACCOUNTS=""
REGION="eu-west-1"
K8S_NAMESPACE="monitoring"
SERVICE_ACCOUNT_NAME="org-cost-api"
TRUST_VERSION="3"

usage() {
  sed -n '2,18p' "$0" | sed 's/^# \?//'
  exit "${1:-0}"
}

if [[ $# -gt 0 && "$1" != -* ]]; then
  case "$1" in
    deploy|payer|all) SIDE="$1"; shift ;;
    -h|--help) usage 0 ;;
    *) echo "Unknown command: $1 (use deploy, payer, or all)" >&2; usage 1 ;;
  esac
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profile|-p) PROFILE="${2:?}"; shift 2 ;;
    --deploy-account-profile) DEPLOY_PROFILE="${2:?}"; shift 2 ;;
    --payer-account-id) PAYER_ACCOUNT_ID="${2:?}"; shift 2 ;;
    --deploy-account-id) DEPLOY_ACCOUNT_ID="${2:?}"; shift 2 ;;
    --oidc-provider-arn) OIDC_PROVIDER_ARN="${2:?}"; shift 2 ;;
    --ou-id) OU_ID="${2:?}"; shift 2 ;;
    --accounts) ACCOUNTS="${2:?}"; shift 2 ;;
    --region|-r) REGION="${2:?}"; shift 2 ;;
    --namespace) K8S_NAMESPACE="${2:?}"; shift 2 ;;
    --service-account) SERVICE_ACCOUNT_NAME="${2:?}"; shift 2 ;;
    --irsa-role-name) IRSA_ROLE_BASE="${2:?}"; shift 2 ;;
    --member-role-name) MEMBER_ROLE_BASE="${2:?}"; shift 2 ;;
    --name-prefix) NAME_PREFIX="${2:?}"; shift 2 ;;
    --trust-version) TRUST_VERSION="${2:?}"; shift 2 ;;
    -h|--help) usage 0 ;;
    *) echo "Unknown option: $1" >&2; usage 1 ;;
  esac
done

if [[ -z "$SIDE" ]]; then
  if [[ -n "$DEPLOY_PROFILE" ]]; then
    SIDE="all"
  else
    echo "ERROR: pass 'deploy' or 'payer', or provide both --profile and --deploy-account-profile" >&2
    usage 1
  fi
fi

if [[ "$SIDE" == "all" ]]; then
  [[ -n "$PROFILE" ]] || { echo "ERROR: --profile is required (org management account)" >&2; usage 1; }
  [[ -n "$DEPLOY_PROFILE" ]] || { echo "ERROR: --deploy-account-profile is required (EKS account)" >&2; usage 1; }
elif [[ "$SIDE" == "deploy" ]]; then
  [[ -n "$PROFILE" || -n "$DEPLOY_PROFILE" ]] || {
    echo "ERROR: --profile is required (EKS / deploy account)" >&2
    usage 1
  }
  DEPLOY_PROFILE="${DEPLOY_PROFILE:-$PROFILE}"
elif [[ "$SIDE" == "payer" ]]; then
  [[ -n "$PROFILE" ]] || { echo "ERROR: --profile is required (org management / payer account)" >&2; usage 1; }
fi

if [[ "$SIDE" == "all" || "$SIDE" == "payer" ]]; then
  [[ -n "$PAYER_ACCOUNT_ID" ]] || { echo "ERROR: --payer-account-id is required" >&2; usage 1; }
fi

[[ -n "$DEPLOY_ACCOUNT_ID" ]] || { echo "ERROR: --deploy-account-id is required" >&2; usage 1; }

if [[ "$SIDE" == "all" || "$SIDE" == "deploy" ]]; then
  [[ -n "$OIDC_PROVIDER_ARN" ]] || { echo "ERROR: --oidc-provider-arn is required" >&2; usage 1; }
  [[ "$OIDC_PROVIDER_ARN" == arn:aws:iam::*:oidc-provider/* ]] || {
    echo "ERROR: --oidc-provider-arn must look like arn:aws:iam::ACCOUNT:oidc-provider/..." >&2
    exit 1
  }
fi

if [[ "$SIDE" == "all" || "$SIDE" == "payer" ]]; then
  [[ -n "$OU_ID" || -n "$ACCOUNTS" ]] || { echo "ERROR: --ou-id or --accounts is required" >&2; usage 1; }
fi

if [[ -n "$NAME_PREFIX" ]]; then
  NAME_PREFIX="${NAME_PREFIX%-}-"
  if [[ ! "$NAME_PREFIX" =~ ^[A-Za-z0-9+=,.@_-]+$ ]]; then
    echo "ERROR: --name-prefix must use IAM-safe characters (A-Za-z0-9+=,.@_-) " >&2
    exit 1
  fi
fi
IRSA_ROLE_NAME="${NAME_PREFIX}${IRSA_ROLE_BASE}"
MEMBER_ROLE_NAME="${NAME_PREFIX}${MEMBER_ROLE_BASE}"
MEMBER_POLICY_NAME="${NAME_PREFIX}OrgCostReadOnly"
STACK_SET_NAME="${NAME_PREFIX}org-cost-api-iam"
IRSA_STACK_NAME="${NAME_PREFIX}org-cost-api-irsa"
PAYER_STACK_NAME="${NAME_PREFIX}org-cost-api-payer-readonly"
if (( ${#IRSA_ROLE_NAME} > 64 || ${#MEMBER_ROLE_NAME} > 64 )); then
  echo "ERROR: prefixed IAM role name exceeds 64 characters" >&2
  exit 1
fi

AWS_MASTER=()
AWS_DEPLOY=()
if [[ "$SIDE" == "all" || "$SIDE" == "payer" ]]; then
  AWS_MASTER=(aws --profile "$PROFILE" --region "$REGION")
fi
if [[ "$SIDE" == "all" || "$SIDE" == "deploy" ]]; then
  AWS_DEPLOY=(aws --profile "$DEPLOY_PROFILE" --region "$REGION")
fi

ORG_TARGET_OU="$OU_ID"
if [[ "$SIDE" != "deploy" && -z "$ORG_TARGET_OU" ]]; then
  ORG_TARGET_OU="$("${AWS_MASTER[@]}" organizations list-roots --query 'Roots[0].Id' --output text)"
fi

deployment_targets_json() {
  local mode="$1"
  case "$mode" in
    ou)
      ORG_TARGET_OU="$ORG_TARGET_OU" python3 -c '
import json, os
print(json.dumps({"OrganizationalUnitIds": [os.environ["ORG_TARGET_OU"]]}))
'
      ;;
    accounts)
      ACCOUNTS="$ACCOUNTS" ORG_TARGET_OU="$ORG_TARGET_OU" python3 -c '
import json, os
accounts = [a.strip() for a in os.environ["ACCOUNTS"].split(",") if a.strip()]
print(json.dumps({
    "Accounts": accounts,
    "OrganizationalUnitIds": [os.environ["ORG_TARGET_OU"]],
    "AccountFilterType": "INTERSECTION",
}))
'
      ;;
    *) echo "ERROR: unknown deployment mode: $mode" >&2; return 1 ;;
  esac
}

STACKSET_PARAMS=(
  "ParameterKey=DeployAccountId,ParameterValue=$DEPLOY_ACCOUNT_ID"
  "ParameterKey=PayerAccountId,ParameterValue=$PAYER_ACCOUNT_ID"
  "ParameterKey=NamePrefix,ParameterValue=$NAME_PREFIX"
  "ParameterKey=IrsaRoleName,ParameterValue=$IRSA_ROLE_BASE"
  "ParameterKey=RoleName,ParameterValue=$MEMBER_ROLE_BASE"
  "ParameterKey=TrustVersion,ParameterValue=$TRUST_VERSION"
)

wait_for_stackset_operation() {
  local op_id="$1"
  echo "Waiting for StackSet operation $op_id..." >&2
  while true; do
    STATUS="$("${AWS_MASTER[@]}" cloudformation describe-stack-set-operation \
      --stack-set-name "$STACK_SET_NAME" --operation-id "$op_id" \
      --query 'StackSetOperation.Status' --output text)"
    case "$STATUS" in
      SUCCEEDED) return 0 ;;
      FAILED|STOPPED)
        echo "ERROR: StackSet operation $STATUS" >&2
        "${AWS_MASTER[@]}" cloudformation describe-stack-set-operation \
          --stack-set-name "$STACK_SET_NAME" --operation-id "$op_id" --output json >&2 || true
        return 1
        ;;
      *) sleep 10 ;;
    esac
  done
}

deploy_irsa_role() {
  echo "Deploy: IRSA in EKS account ($DEPLOY_PROFILE)..." >&2
  "${AWS_DEPLOY[@]}" cloudformation deploy \
    --template-file "$IRSA_TEMPLATE" \
    --stack-name "$IRSA_STACK_NAME" \
    --capabilities CAPABILITY_NAMED_IAM \
    --parameter-overrides \
      "OidcProviderArn=$OIDC_PROVIDER_ARN" \
      "KubernetesNamespace=$K8S_NAMESPACE" \
      "ServiceAccountName=$SERVICE_ACCOUNT_NAME" \
      "NamePrefix=$NAME_PREFIX" \
      "IrsaRoleName=$IRSA_ROLE_BASE" \
      "MemberRoleName=$MEMBER_ROLE_BASE" \
    --no-fail-on-empty-changeset

  echo "Waiting for IRSA role to be visible..." >&2
  for _ in $(seq 1 30); do
    if "${AWS_DEPLOY[@]}" iam get-role --role-name "$IRSA_ROLE_NAME" >/dev/null 2>&1; then
      echo "IRSA role ready: arn:aws:iam::${DEPLOY_ACCOUNT_ID}:role/${IRSA_ROLE_NAME}" >&2
      return 0
    fi
    sleep 5
  done
  echo "ERROR: IRSA role $IRSA_ROLE_NAME not found after deploy" >&2
  exit 1
}

deploy_payer_role() {
  echo "Payer: OrgCostReadOnly in management account (direct CFN)..." >&2
  set +e
  DEPLOY_OUT="$("${AWS_MASTER[@]}" cloudformation deploy \
    --template-file "$PAYER_TEMPLATE" \
    --stack-name "$PAYER_STACK_NAME" \
    --capabilities CAPABILITY_NAMED_IAM \
    --parameter-overrides \
      "DeployAccountId=$DEPLOY_ACCOUNT_ID" \
      "NamePrefix=$NAME_PREFIX" \
      "IrsaRoleName=$IRSA_ROLE_BASE" \
      "RoleName=$MEMBER_ROLE_BASE" \
      "TrustVersion=$TRUST_VERSION" \
    --no-fail-on-empty-changeset 2>&1)"
  DEPLOY_RC=$?
  set -e

  if [[ $DEPLOY_RC -ne 0 ]]; then
    if echo "$DEPLOY_OUT" | grep -qiE 'already exists|AlreadyExists'; then
      echo "Role already exists outside this stack — updating trust + policy in place..." >&2
      TRUST_FILE="$(mktemp)"
      POLICY_FILE="$(mktemp)"
      cat >"$TRUST_FILE" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Sid": "TrustEksIrsa",
    "Effect": "Allow",
    "Principal": {
      "AWS": "arn:aws:iam::${DEPLOY_ACCOUNT_ID}:role/${IRSA_ROLE_NAME}"
    },
    "Action": "sts:AssumeRole"
  }]
}
EOF
      cat >"$POLICY_FILE" <<'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "CostExplorerRead",
      "Effect": "Allow",
      "Action": [
        "ce:GetCostAndUsage",
        "ce:GetCostAndUsageWithResources",
        "ce:GetDimensionValues",
        "ce:GetTags"
      ],
      "Resource": "*"
    },
    {
      "Sid": "EC2InventoryRead",
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeVolumes",
        "ec2:DescribeSnapshots",
        "ec2:DescribeInstances",
        "ec2:DescribeRegions"
      ],
      "Resource": "*"
    },
    {
      "Sid": "CloudWatchMetricsRead",
      "Effect": "Allow",
      "Action": ["cloudwatch:GetMetricData", "cloudwatch:ListMetrics"],
      "Resource": "*"
    },
    {
      "Sid": "STSIdentity",
      "Effect": "Allow",
      "Action": ["sts:GetCallerIdentity"],
      "Resource": "*"
    },
    {
      "Sid": "OrganizationsListPayer",
      "Effect": "Allow",
      "Action": [
        "organizations:ListAccounts",
        "organizations:DescribeOrganization"
      ],
      "Resource": "*"
    }
  ]
}
EOF
      "${AWS_MASTER[@]}" iam update-assume-role-policy \
        --role-name "$MEMBER_ROLE_NAME" \
        --policy-document "file://${TRUST_FILE}"
      "${AWS_MASTER[@]}" iam put-role-policy \
        --role-name "$MEMBER_ROLE_NAME" \
        --policy-name "$MEMBER_POLICY_NAME" \
        --policy-document "file://${POLICY_FILE}"
      rm -f "$TRUST_FILE" "$POLICY_FILE"
    else
      echo "ERROR: payer role deploy failed:" >&2
      echo "$DEPLOY_OUT" >&2
      exit 1
    fi
  fi

  echo "Verifying payer role trust..." >&2
  TRUST="$("${AWS_MASTER[@]}" iam get-role --role-name "$MEMBER_ROLE_NAME" \
    --query 'Role.AssumeRolePolicyDocument' --output json)"
  echo "$TRUST" >&2
  if ! echo "$TRUST" | grep -q "$IRSA_ROLE_NAME"; then
    echo "ERROR: payer OrgCostReadOnly does not trust ${IRSA_ROLE_NAME}" >&2
    exit 1
  fi
}

upsert_stack_set() {
  echo "Payer: update StackSet definition (member accounts)..." >&2
  "${AWS_MASTER[@]}" cloudformation enable-organizations-access >/dev/null 2>&1 || true

  if "${AWS_MASTER[@]}" cloudformation describe-stack-set --stack-set-name "$STACK_SET_NAME" >/dev/null 2>&1; then
    set +e
    UPDATE_SET_OUT="$("${AWS_MASTER[@]}" cloudformation update-stack-set \
      --stack-set-name "$STACK_SET_NAME" \
      --template-body "file://$STACKSET_TEMPLATE" \
      --capabilities CAPABILITY_NAMED_IAM \
      --parameters "${STACKSET_PARAMS[@]}" \
      --operation-preferences FailureToleranceCount=2,MaxConcurrentCount=5 \
      --query 'OperationId' --output text 2>&1)"
    UPDATE_SET_RC=$?
    set -e
    if [[ $UPDATE_SET_RC -eq 0 && -n "$UPDATE_SET_OUT" && "$UPDATE_SET_OUT" != "None" ]]; then
      wait_for_stackset_operation "$UPDATE_SET_OUT"
    elif echo "$UPDATE_SET_OUT" | grep -qi 'No updates are to be performed'; then
      echo "StackSet definition already current." >&2
    else
      echo "ERROR: update-stack-set failed:" >&2
      echo "$UPDATE_SET_OUT" >&2
      exit 1
    fi
  else
    "${AWS_MASTER[@]}" cloudformation create-stack-set \
      --stack-set-name "$STACK_SET_NAME" \
      --template-body "file://$STACKSET_TEMPLATE" \
      --permission-model SERVICE_MANAGED \
      --auto-deployment Enabled=true,RetainStacksOnAccountRemoval=false \
      --capabilities CAPABILITY_NAMED_IAM \
      --parameters "${STACKSET_PARAMS[@]}"
  fi
}

deploy_member_roles() {
  local mode="$1"
  local targets
  targets="$(deployment_targets_json "$mode")"
  echo "Payer: deploy/update OrgCostReadOnly via StackSet ($targets)..." >&2

  set +e
  UPDATE_OUT="$("${AWS_MASTER[@]}" cloudformation update-stack-instances \
    --stack-set-name "$STACK_SET_NAME" \
    --deployment-targets "$targets" \
    --regions "$REGION" \
    --parameter-overrides "ParameterKey=TrustVersion,ParameterValue=$TRUST_VERSION" \
    --operation-preferences FailureToleranceCount=2,MaxConcurrentCount=5 \
    --query 'OperationId' --output text 2>&1)"
  UPDATE_RC=$?
  set -e

  local op_id=""
  if [[ $UPDATE_RC -eq 0 ]]; then
    op_id="$UPDATE_OUT"
  elif echo "$UPDATE_OUT" | grep -qiE 'not found|No stack instances|does not exist'; then
    op_id="$("${AWS_MASTER[@]}" cloudformation create-stack-instances \
      --stack-set-name "$STACK_SET_NAME" \
      --deployment-targets "$targets" \
      --regions "$REGION" \
      --operation-preferences FailureToleranceCount=2,MaxConcurrentCount=5 \
      --query 'OperationId' --output text)"
  elif echo "$UPDATE_OUT" | grep -qiE 'already exist|AlreadyExists'; then
    op_id="$("${AWS_MASTER[@]}" cloudformation update-stack-instances \
      --stack-set-name "$STACK_SET_NAME" \
      --deployment-targets "$targets" \
      --regions "$REGION" \
      --parameter-overrides "ParameterKey=TrustVersion,ParameterValue=$TRUST_VERSION" \
      --operation-preferences FailureToleranceCount=2,MaxConcurrentCount=5 \
      --query 'OperationId' --output text)"
  else
    echo "ERROR: StackSet instance deploy failed:" >&2
    echo "$UPDATE_OUT" >&2
    exit 1
  fi

  wait_for_stackset_operation "$op_id"
}

print_helm_hint() {
  local irsa_arn="arn:aws:iam::${DEPLOY_ACCOUNT_ID}:role/${IRSA_ROLE_NAME}"
  echo ""
  echo "Helm SA annotation:"
  echo "  eks.amazonaws.com/role-arn: $irsa_arn"
  echo "  namespace / SA must be: ${K8S_NAMESPACE} / ${SERVICE_ACCOUNT_NAME}"
  echo "  Helm autoConfig.memberRoleName: ${MEMBER_ROLE_NAME}"
}

run_deploy() {
  deploy_irsa_role
  echo ""
  echo "Deploy-account IAM complete."
  echo "  IRSA role → arn:aws:iam::${DEPLOY_ACCOUNT_ID}:role/${IRSA_ROLE_NAME}"
  echo "  Next: someone with org-management access runs:"
  echo "    ./deploy/aws/onboard-iam.sh payer --profile MASTER --payer-account-id PAYER --deploy-account-id ${DEPLOY_ACCOUNT_ID} --ou-id OU"
  print_helm_hint
}

run_payer() {
  echo "Payer trusts IRSA arn:aws:iam::${DEPLOY_ACCOUNT_ID}:role/${IRSA_ROLE_NAME}" >&2
  echo "If StackSet fails with Invalid principal, run the deploy-account command first." >&2
  deploy_payer_role
  upsert_stack_set
  if [[ -n "$OU_ID" ]]; then
    deploy_member_roles ou
  else
    deploy_member_roles accounts
  fi
  echo ""
  echo "Payer / org IAM complete."
  echo "  Payer OrgCostReadOnly  → arn:aws:iam::${PAYER_ACCOUNT_ID}:role/${MEMBER_ROLE_NAME} (direct CFN)"
  echo "  Member OrgCostReadOnly → StackSet (trusts IRSA in ${DEPLOY_ACCOUNT_ID})"
  print_helm_hint
}

case "$SIDE" in
  deploy) run_deploy ;;
  payer) run_payer ;;
  all)
    deploy_irsa_role
    run_payer
    echo ""
    echo "IAM onboarding complete (deploy + payer)."
    ;;
esac
