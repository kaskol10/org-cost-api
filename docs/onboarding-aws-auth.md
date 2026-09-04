# AWS credentials for Cost Explorer

The backend needs read-only AWS access for each account in `config.yaml`, plus a **payer** identity for organization-wide Cost Explorer (unless every account uses `billing_profile: account`).

You can authenticate in three ways. Pick what matches how your team already works.

| Method | Best for | Cost Explorer payer |
|--------|----------|---------------------|
| **Granted + SSO** | Teams using [Granted](https://granted.dev) with AWS IAM Identity Center | Global `billing_profile` pointing at the org payer |
| **`~/.aws/credentials` profiles** | Long-lived IAM users in the standard AWS config files | Same `profile` fields — no Granted required |
| **API keys in config / env** | One-off keys per account, CI, or hosts without SSO | `billing_access_key_id` + env secret, or payer profile |

All methods use the same `config.yaml` shape; only the auth fields change.

For multi-org Granted SSO setup examples, see [examples/onboarding-multi-org-sso.md](./examples/onboarding-multi-org-sso.md).

---

## Billing model: payer-linked vs member-billed

Most accounts in an AWS Organization appear on the **management (payer) account's** Cost Explorer. Query them from the payer with a `LINKED_ACCOUNT` filter — set a global `billing_profile` and omit per-account `billing_profile`.

Some accounts bill **independently** (separate payer, acquired org, or partner with their own billing). Cost Explorer for those accounts must run **from credentials inside that account**. Set `billing_profile: account` on that row.

```
Does Cost Explorer for this account require credentials IN that account?
  YES → billing_profile: account on that row
  NO  → omit billing_profile; use global billing_profile / payer keys
```

**How to tell:** From your org payer profile, run Cost Explorer for the account ID. If you get data (or $0 with no AccessDenied), payer-linked is correct. If you get AccessDenied or the account never appears on the payer bill, use `billing_profile: account`.

| Pattern | Config | CE client |
|---------|--------|-----------|
| Standard linked account | Global `billing_profile` only | Payer profile, filtered by account ID |
| Member-billed / separate payer | `billing_profile: account` on that row | Account's own profile or keys |
| Mixed org | Both patterns in one `config.yaml` | Per-row as above |

See [config.example.org.yaml](../config.example.org.yaml) for a multi-account template and [member-billed-account.md](./member-billed-account.md) for step-by-step member billing setup.

**README vs live config:** The README shows `partner-org` with `billing_profile: account` as an example of member-billed accounts — not as a rule that every partner account must use it. Use the decision tree above per account.

---

## Option A — Granted + SSO

If your team uses [Granted](https://granted.dev) with IAM Identity Center, reference SSO profiles in config:

```yaml
billing_profile: org-payer.AdministratorAccess

accounts:
  - name: production
    profile: Production.AdministratorAccess
    region: eu-west-1
```

**Each session:**

```bash
granted sso login
aws sts get-caller-identity --profile Production.AdministratorAccess
```

Multi-org SSO example: [examples/onboarding-multi-org-sso.md](./examples/onboarding-multi-org-sso.md).

---

## Option B — `~/.aws/credentials` (no Granted)

Define a **named profile** in the standard AWS files and reference it in config.

`~/.aws/credentials`:

```ini
[production-costs]
aws_access_key_id = AKIA...
aws_secret_access_key = ...
```

`~/.aws/config` (optional region):

```ini
[profile production-costs]
region = eu-west-1
```

`config.yaml`:

```yaml
billing_profile: master-payer   # payer IAM user profile

accounts:
  - name: production
    profile: production-costs
    region: eu-west-1
```

This is the same code path as Granted; only the credential source differs.

---

## Option C — API keys in config (per account)

Use when someone has **dedicated access keys per account** and does not use Granted or shared profiles.

**Prefer environment variables for secrets** — do not commit keys to git.

```yaml
billing_profile: org-payer.AdministratorAccess   # or billing keys below

accounts:
  - name: production
    id: "123456789012"          # optional; validated against STS if set
    access_key_id: AKIA...
    secret_access_key_env: ORG_COST_PROD_SECRET
    region: eu-west-1

  - name: staging
    access_key_id: AKIA...
    secret_access_key_env: ORG_COST_STAGING_SECRET
    region: eu-west-1
```

Export secrets before starting the server:

```bash
export ORG_COST_PROD_SECRET='...'
export ORG_COST_STAGING_SECRET='...'
cd backend && go run ./cmd/server -config ../config.yaml
```

**Temporary credentials** (e.g. assumed-role session) — add `session_token` or `session_token_env`.

**Payer without a profile** — set billing keys at the top level:

```yaml
billing_access_key_id: AKIA...
billing_secret_access_key_env: ORG_COST_BILLING_SECRET

accounts:
  - name: production
    access_key_id: AKIA...
    secret_access_key_env: ORG_COST_PROD_SECRET
    region: eu-west-1
```

---

## Assume role (`role_arn`)

When your base profile can **assume a read-only role** in the target account (common with SSO or a central IAM user), add `role_arn`:

```yaml
accounts:
  - name: production
    profile: org-payer.AdministratorAccess   # base credentials
    role_arn: arn:aws:iam::123456789012:role/OrgCostReadOnly
    region: eu-west-1
```

The server calls STS `AssumeRole` before EC2/CloudWatch calls. Cost Explorer still uses the billing model above (payer profile or `billing_profile: account`).

---

## Regions: Cost Explorer vs EC2/CloudWatch

- **Cost Explorer** always runs in **`us-east-1`** (AWS API requirement). The backend sets this automatically.
- **EC2, CloudWatch, EBS inventory** use each account's `region` field (e.g. `eu-west-1`).

Set `region` to where your workloads live; do not set it to `us-east-1` unless that is your primary region.

---

## Optional: CUR / Athena (historical EBS)

Daily disk/snapshot counts including deleted resources require a Cost and Usage Report exported to S3 and queryable via Athena. Most teams do not need this on day one.

When you need it, uncomment and fill the `cur:` block in config (see [config.example.org.yaml](../config.example.org.yaml)). Requires Athena + S3 read on the payer (or CUR bucket account).

---

## Server-side credentials (no laptop profiles)

For Kubernetes (IRSA), EC2 instance profiles, or CI roles, omit `profile` on accounts and rely on the **default credential chain** on that host, or use env-based API keys. Documented in [hermes-nous-deployment.md](./hermes-nous-deployment.md).

---

## Required IAM permissions

Each account role/user needs at minimum:

- `ce:GetCostAndUsage`, `ce:GetCostAndUsageWithResources` (if using tag drill-downs)
- `ec2:DescribeVolumes`, `ec2:DescribeSnapshots`
- `cloudwatch:GetMetricData`, `cloudwatch:ListMetrics` (EBS utilization)
- `sts:GetCallerIdentity`

Payer / org-wide CE also needs permission to query linked accounts (standard billing view access).

Attachable policy: [iam-readonly-policy.json](./iam-readonly-policy.json).

Cost allocation tag **`Name`** must be activated in the billing console for “why did S3 change?” breakdowns.

---

## Verify setup

```bash
chmod +x scripts/verify-aws-auth.sh
./scripts/verify-aws-auth.sh
```

This checks STS identity and Cost Explorer access (`GetCostAndUsage` for the last 7 days) for each configured account.

Discover account IDs from your org:

```bash
./scripts/list-org-accounts.sh --profile org-payer.AdministratorAccess
```

Start the app — see [getting-started.md](./getting-started.md) for the full first-hour checklist.

If costs show **$0**, see [README troubleshooting](../README.md#costs-show-0).

---

## Security notes

- Never commit `config.yaml` with inline `secret_access_key` or `billing_secret_access_key`.
- Prefer `*_env` fields and shell exports, or a secrets manager that injects env vars at runtime.
- Rotate keys if they were ever committed; use read-only IAM policies scoped to CE + inventory APIs.
- Granted SSO sessions expire — run `granted sso login` when the UI shows auth errors.
