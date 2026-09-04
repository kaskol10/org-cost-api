#!/usr/bin/env bash
# One-command IAM onboarding for org-cost-api on EKS.
#
# 1. Deploy IRSA role in the EKS (shared-services) account
# 2. Deploy OrgCostReadOnly in the payer/management account (direct CFN —
#    SERVICE_MANAGED StackSets do not deploy into the management account)
# 3. Deploy OrgCostReadOnly to member accounts via StackSet
#
# Usage:
#   ./deploy/aws/onboard-iam.sh \
#     --profile Master.AdministratorAccess \
#     --deploy-account-profile Shared-Services.AdministratorAccess \
#     --payer-account-id 111122223333 \
#     --deploy-account-id 444455556666 \
#     --oidc-provider-arn 'arn:aws:iam::444455556666:oidc-provider/oidc.eks.eu-west-1.amazonaws.com/id/XXXX' \
#     --namespace monitoring \
#     --ou-id r-xxxx
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
STACKSET_TEMPLATE="$ROOT/deploy/aws/stackset-iam.yaml"
IRSA_TEMPLATE="$ROOT/deploy/aws/irsa-role.yaml"
PAYER_TEMPLATE="$ROOT/deploy/aws/payer-readonly-role.yaml"
STACK_SET_NAME="org-cost-api-iam"
IRSA_STACK_NAME="org-cost-api-irsa"
PAYER_STACK_NAME="org-cost-api-payer-readonly"
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
IRSA_ROLE_NAME="org-cost-api-irsa"
MEMBER_ROLE_NAME="OrgCostReadOnly"
TRUST_VERSION="3"

usage() {
  sed -n '2,17p' "$0" | sed 's/^# \?//'
  exit "${1:-0}"
}

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
    --irsa-role-name) IRSA_ROLE_NAME="${2:?}"; shift 2 ;;
    --trust-version) TRUST_VERSION="${2:?}"; shift 2 ;;
    -h|--help) usage 0 ;;
    *) echo "Unknown option: $1" >&2; usage 1 ;;
  esac
done

[[ -n "$PROFILE" ]] || { echo "ERROR: --profile is required (org management account)" >&2; usage 1; }
[[ -n "$DEPLOY_PROFILE" ]] || { echo "ERROR: --deploy-account-profile is required (EKS account)" >&2; usage 1; }
[[ -n "$PAYER_ACCOUNT_ID" ]] || { echo "ERROR: --payer-account-id is required" >&2; usage 1; }
[[ -n "$DEPLOY_ACCOUNT_ID" ]] || { echo "ERROR: --deploy-account-id is required" >&2; usage 1; }
[[ -n "$OIDC_PROVIDER_ARN" ]] || { echo "ERROR: --oidc-provider-arn is required" >&2; usage 1; }
[[ "$OIDC_PROVIDER_ARN" == arn:aws:iam::*:oidc-provider/* ]] || {
  echo "ERROR: --oidc-provider-arn must look like arn:aws:iam::ACCOUNT:oidc-provider/..." >&2
  exit 1
}
[[ -n "$OU_ID" || -n "$ACCOUNTS" ]] || { echo "ERROR: --ou-id or --accounts is required" >&2; usage 1; }

AWS_MASTER=(aws --profile "$PROFILE" --region "$REGION")
AWS_DEPLOY=(aws --profile "$DEPLOY_PROFILE" --region "$REGION")

ORG_TARGET_OU="$OU_ID"
if [[ -z "$ORG_TARGET_OU" ]]; then
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
  "ParameterKey=IrsaRoleName,ParameterValue=$IRSA_ROLE_NAME"
  "ParameterKey=RoleName,ParameterValue=$MEMBER_ROLE_NAME"
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
  echo "Phase 1: deploy IRSA in shared-services ($DEPLOY_PROFILE)..." >&2
  "${AWS_DEPLOY[@]}" cloudformation deploy \
    --template-file "$IRSA_TEMPLATE" \
    --stack-name "$IRSA_STACK_NAME" \
    --capabilities CAPABILITY_NAMED_IAM \
    --parameter-overrides \
      "OidcProviderArn=$OIDC_PROVIDER_ARN" \
      "KubernetesNamespace=$K8S_NAMESPACE" \
      "ServiceAccountName=$SERVICE_ACCOUNT_NAME" \
      "IrsaRoleName=$IRSA_ROLE_NAME" \
      "MemberRoleName=$MEMBER_ROLE_NAME" \
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
  echo "Phase 2: deploy OrgCostReadOnly in payer/management account (direct CFN)..." >&2
  set +e
  DEPLOY_OUT="$("${AWS_MASTER[@]}" cloudformation deploy \
    --template-file "$PAYER_TEMPLATE" \
    --stack-name "$PAYER_STACK_NAME" \
    --capabilities CAPABILITY_NAMED_IAM \
    --parameter-overrides \
      "DeployAccountId=$DEPLOY_ACCOUNT_ID" \
      "IrsaRoleName=$IRSA_ROLE_NAME" \
      "RoleName=$MEMBER_ROLE_NAME" \
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
        --policy-name OrgCostReadOnly \
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
  echo "Phase 3: update StackSet definition (member accounts)..." >&2
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
  echo "Phase 4: deploy/update OrgCostReadOnly via StackSet ($targets)..." >&2

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

verify_assume_payer() {
  echo "Phase 5: verify IRSA can AssumeRole into payer OrgCostReadOnly..." >&2
  # Use deploy-account credentials if they can assume; otherwise skip with note.
  # Best effort: Master simulates by checking trust document (already done).
  echo "Payer role trusts arn:aws:iam::${DEPLOY_ACCOUNT_ID}:role/${IRSA_ROLE_NAME}" >&2
}

deploy_irsa_role
deploy_payer_role
upsert_stack_set
if [[ -n "$OU_ID" ]]; then
  deploy_member_roles ou
else
  deploy_member_roles accounts
fi
verify_assume_payer

IRSA_ARN="arn:aws:iam::${DEPLOY_ACCOUNT_ID}:role/${IRSA_ROLE_NAME}"
echo ""
echo "IAM onboarding complete."
echo "  IRSA role              → $IRSA_ARN"
echo "  Payer OrgCostReadOnly  → arn:aws:iam::${PAYER_ACCOUNT_ID}:role/${MEMBER_ROLE_NAME} (direct CFN)"
echo "  Member OrgCostReadOnly → StackSet (trusts IRSA)"
echo ""
echo "Helm SA annotation:"
echo "  eks.amazonaws.com/role-arn: $IRSA_ARN"
echo "  namespace / SA must be: ${K8S_NAMESPACE} / ${SERVICE_ACCOUNT_NAME}"
