# IAM minimal setup for org-cost-api

Step-by-step read-only IAM for a new adopter connecting a standard AWS Organization. No org-specific scripts required.

## What you need

| Identity | Purpose |
|----------|---------|
| **Payer (management) profile** | Organization-wide Cost Explorer via `LINKED_ACCOUNT` filter |
| **Per-account profile or role** | EC2 inventory, CloudWatch metrics, member-billed CE |

Attach the policy in [iam-readonly-policy.json](./iam-readonly-policy.json) (or a scoped variant below).

## 1. Enable Cost Explorer org view

In the **management (payer) account**:

1. AWS Console → **Billing** → **Cost Explorer** → enable if prompted.
2. **Settings** → enable **Linked account access** / organization view (wording varies by console version).

Without this, payer-linked queries return empty or AccessDenied for member accounts.

## 2. Activate cost allocation tag (recommended)

1. **Billing** → **Cost allocation tags** → activate tag key **`Name`** (or your standard resource tag).
2. Wait up to 24 hours for tag data in CE.

Used by service tag delta/totals endpoints.

## 3. Create a read-only IAM user or role

### Option A — IAM user + access keys (CI / single host)

1. IAM → **Users** → **Create user** (e.g. `org-cost-readonly`).
2. Attach inline policy from [iam-readonly-policy.json](./iam-readonly-policy.json).
3. Create access keys; store secret in env (e.g. `ORG_COST_BILLING_SECRET`).

In `config.yaml`:

```yaml
billing_access_key_id: AKIA...
billing_secret_access_key_env: ORG_COST_BILLING_SECRET
```

### Option B — SSO profile (Granted / IAM Identity Center)

1. Assign a permission set with the readonly policy to your FinOps group.
2. Use Granted or `aws configure sso` to create profiles.
3. Set `billing_profile: YourPayer.AdministratorAccess` (or a dedicated read-only profile).

See [onboarding-aws-auth.md](./onboarding-aws-auth.md).

### Option C — Cross-account role

Create role `OrgCostReadOnly` in each member account:

**Trust policy** (payer account principal):

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": { "AWS": "arn:aws:iam::PAYER_ACCOUNT_ID:root" },
    "Action": "sts:AssumeRole"
  }]
}
```

**Permissions:** attach [iam-readonly-policy.json](./iam-readonly-policy.json).

In `config.yaml`:

```yaml
accounts:
  - name: sandbox
    profile: org-payer.AdministratorAccess
    role_arn: arn:aws:iam::MEMBER_ID:role/OrgCostReadOnly
    region: eu-west-1
```

## 4. Minimal policy (Cost Explorer + EC2 only)

If you want to skip CUR/Athena and Organizations list permissions, use this subset:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ce:GetCostAndUsage",
        "ce:GetCostAndUsageWithResources",
        "ce:GetDimensionValues",
        "ce:GetTags",
        "ec2:DescribeVolumes",
        "ec2:DescribeSnapshots",
        "ec2:DescribeInstances",
        "ec2:DescribeRegions",
        "cloudwatch:GetMetricData",
        "cloudwatch:ListMetrics",
        "sts:GetCallerIdentity"
      ],
      "Resource": "*"
    }
  ]
}
```

Add `organizations:ListAccounts` on the payer if you use `./scripts/bootstrap-config.sh`.

## 5. CloudFormation snippet (payer read-only role)

Save as `org-cost-readonly.yaml` and deploy in the **management account**:

```yaml
AWSTemplateFormatVersion: "2010-09-09"
Description: Read-only role for org-cost-api Cost Explorer

Resources:
  OrgCostReadOnlyRole:
    Type: AWS::IAM::Role
    Properties:
      RoleName: OrgCostReadOnly
      AssumeRolePolicyDocument:
        Version: "2012-10-17"
        Statement:
          - Effect: Allow
            Principal:
              AWS: !Sub "arn:aws:iam::${AWS::AccountId}:root"
            Action: sts:AssumeRole
      ManagedPolicyArns: []
      Policies:
        - PolicyName: OrgCostReadOnly
          PolicyDocument:
            Version: "2012-10-17"
            Statement:
              - Effect: Allow
                Action:
                  - ce:GetCostAndUsage
                  - ce:GetCostAndUsageWithResources
                  - ce:GetDimensionValues
                  - ce:GetTags
                  - ec2:Describe*
                  - cloudwatch:GetMetricData
                  - cloudwatch:ListMetrics
                  - sts:GetCallerIdentity
                  - organizations:ListAccounts
                  - organizations:DescribeOrganization
                Resource: "*"

Outputs:
  RoleArn:
    Value: !GetAtt OrgCostReadOnlyRole.Arn
```

Deploy:

```bash
aws cloudformation deploy \
  --template-file org-cost-readonly.yaml \
  --stack-name org-cost-readonly \
  --capabilities CAPABILITY_NAMED_IAM \
  --profile YOUR_PAYER_PROFILE
```

## 6. Verify

```bash
./scripts/bootstrap-config.sh --profile YOUR_PAYER_PROFILE --output config.yaml
./scripts/verify-aws-auth.sh config.yaml --format markdown
```

Fix any ❌ rows before starting the server.

## Next steps

| Task | Doc |
|------|-----|
| Member-billed account | [member-billed-account.md](./member-billed-account.md) |
| Docker production | [../docker-compose.prod.yml](../docker-compose.prod.yml) |
| API token for production | [getting-started.md](./getting-started.md#production-auth--one-page) |
