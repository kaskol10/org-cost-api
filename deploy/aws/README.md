# AWS IAM — org-wide onboarding (StackSet + IRSA)

One StackSet deploys **all IAM resources** for EKS-hosted org-cost-api:

| Resource | Accounts | Purpose |
|----------|----------|---------|
| `OrgCostReadOnly` | Every account in target OU | Member CE, EC2, CloudWatch |
| `org-cost-api-irsa` | Deploy account only (e.g. shared-services) | EKS pod identity |

Member accounts trust the **IRSA role ARN** in the deploy account (not account root). Create IRSA in the deploy account **first** so that principal exists before member stacks run (avoids StackSet `Invalid principal`).

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {
      "AWS": "arn:aws:iam::444455556666:role/org-cost-api-irsa"
    },
    "Action": "sts:AssumeRole"
  }]
}
```

## Split onboarding (recommended)

Use two commands when payer and EKS credentials live with different people (or different orgs). Run **deploy first**, then **payer**.

### 1. Deploy / EKS account — IRSA

Needs only shared-services credentials + the cluster OIDC ARN.

```bash
./deploy/aws/onboard-iam.sh deploy \
  --profile Shared-Services.AdministratorAccess \
  --deploy-account-id 444455556666 \
  --oidc-provider-arn "$OIDC_ARN" \
  --namespace monitoring
```

### 2. Payer / org management — OrgCostReadOnly + StackSet

Needs only management-account credentials. Pass the deploy account ID so member roles trust `arn:aws:iam::DEPLOY:role/org-cost-api-irsa`. No deploy-account login.

```bash
./deploy/aws/onboard-iam.sh payer \
  --profile Master.AdministratorAccess \
  --payer-account-id 111122223333 \
  --deploy-account-id 444455556666 \
  --ou-id ou-abcd-12345678
```

**Steps:**
1. Deploy `org-cost-api-irsa` in the EKS account via [`irsa-role.yaml`](./irsa-role.yaml)
2. Deploy `OrgCostReadOnly` in the management account (direct CFN — StackSets skip it)
3. StackSet deploys `OrgCostReadOnly` to the OU (trusts the IRSA ARN)

## Combined onboarding (both profiles on one machine)

```bash
./deploy/aws/onboard-iam.sh \
  --profile Master.AdministratorAccess \
  --deploy-account-profile Shared-Services.AdministratorAccess \
  --payer-account-id 111122223333 \
  --deploy-account-id 444455556666 \
  --oidc-provider-arn "$OIDC_ARN" \
  --ou-id ou-abcd-12345678
```

**Get OIDC provider ARN** (shared-services / EKS account):

```bash
ISSUER=$(aws eks describe-cluster --name YOUR_CLUSTER \
  --profile Shared-Services.AdministratorAccess \
  --query 'cluster.identity.oidc.issuer' --output text)
# https://oidc.eks.eu-west-1.amazonaws.com/id/XXXX
OIDC_ARN="arn:aws:iam::444455556666:oidc-provider/${ISSUER#https://}"
echo "$OIDC_ARN"
```

**Get OU ID** (not organization ID `o-...`):

```bash
ROOT=$(aws organizations list-roots --profile Master.AdministratorAccess --query 'Roots[0].Id' --output text)
aws organizations list-organizational-units-for-parent \
  --parent-id "$ROOT" --profile Master.AdministratorAccess \
  --query 'OrganizationalUnits[*].[Name,Id]' --output table
```

Pilot on specific accounts (uses org root OU + INTERSECTION filter):

```bash
./deploy/aws/onboard-iam.sh payer \
  --profile Master.AdministratorAccess \
  --payer-account-id 111122223333 \
  --deploy-account-id 444455556666 \
  --ou-id r-xxxx \
  --accounts 444455556666,777788889999,111122223333
```

When `--ou-id` is omitted with `--accounts`, the org root OU is resolved automatically.

## Customer name prefix

Use `--name-prefix` on **both** `deploy` and `payer` so roles, inline policies, and CloudFormation stack names do not collide with another tenant. A trailing hyphen is added if you omit it (`acme` → `acme-`).

```bash
./deploy/aws/onboard-iam.sh deploy ... --name-prefix acme
./deploy/aws/onboard-iam.sh payer  ... --name-prefix acme
```

| Resource | Default | With `--name-prefix acme` |
|----------|---------|---------------------------|
| IRSA role | `org-cost-api-irsa` | `acme-org-cost-api-irsa` |
| Member / payer role | `OrgCostReadOnly` | `acme-OrgCostReadOnly` |
| Inline policies | `OrgCostApiIrsa` / `OrgCostReadOnly` | `acme-OrgCostApiIrsa` / `acme-OrgCostReadOnly` |
| Stacks / StackSet | `org-cost-api-irsa`, `org-cost-api-iam` | `acme-org-cost-api-irsa`, `acme-org-cost-api-iam` |

Helm must match the prefixed member role:

```yaml
serviceAccount:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::444455556666:role/acme-org-cost-api-irsa
autoConfig:
  memberRoleName: acme-OrgCostReadOnly
```

**Note:** `SERVICE_MANAGED` StackSets require an OU in deployment targets. Single-account deploys use `AccountFilterType=INTERSECTION` (OU ∩ account list).

The script creates or updates the StackSet, deploys stack instances, and prints the Helm IRSA annotation.

## What the template creates

### Every account — `OrgCostReadOnly`

- **Trust:** `arn:aws:iam::DEPLOY_ACCOUNT:role/org-cost-api-irsa` (IRSA role in shared-services)
- **Permissions:** [iam-readonly-policy.json](../../docs/iam-readonly-policy.json) actions
- **Payer only:** `organizations:ListAccounts` (via `PayerAccountId` condition)

### Deploy account only — `org-cost-api-irsa`

- **Trust:** EKS OIDC provider + `system:serviceaccount:NAMESPACE:SERVICE_ACCOUNT`
- **Permissions:** Cost Explorer + `organizations:ListAccounts` + `sts:AssumeRole` to `arn:aws:iam::*:role/OrgCostReadOnly`

Helm values (match `--namespace` / `--service-account` if you changed defaults):

```yaml
serviceAccount:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::444455556666:role/org-cost-api-irsa
```

## Manual StackSet steps

If you prefer raw CLI instead of `onboard-iam.sh`:

```bash
aws cloudformation create-stack-set \
  --stack-set-name org-cost-api-iam \
  --template-body file://deploy/aws/stackset-iam.yaml \
  --permission-model SERVICE_MANAGED \
  --auto-deployment Enabled=true,RetainStacksOnAccountRemoval=false \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameters \
    ParameterKey=DeployAccountId,ParameterValue=444455556666 \
    ParameterKey=PayerAccountId,ParameterValue=111122223333 \
    ParameterKey=OidcProviderArn,ParameterValue=arn:aws:iam::444455556666:oidc-provider/oidc.eks.eu-west-1.amazonaws.com/id/XXXX

aws cloudformation create-stack-instances \
  --stack-set-name org-cost-api-iam \
  --deployment-targets OrganizationalUnitIds=ou-abcd-12345678 \
  --regions eu-west-1
```

## Verify

```bash
aws iam get-role --role-name OrgCostReadOnly --profile Shared-Services.AdministratorAccess
aws iam get-role --role-name org-cost-api-irsa --profile Shared-Services.AdministratorAccess
```

## Update or remove

```bash
aws cloudformation update-stack-set \
  --stack-set-name org-cost-api-iam \
  --template-body file://deploy/aws/stackset-iam.yaml \
  --capabilities CAPABILITY_NAMED_IAM
```

## Related

| Doc | Purpose |
|-----|---------|
| [docs/eks-operate.md](../../docs/eks-operate.md) | Helm + HTTPRoute after IAM |
| [deploy/terraform/README.md](../terraform/README.md) | Optional Terraform wrapper |
| [irsa-trust-policy.json](./irsa-trust-policy.json) | IRSA trust reference |
