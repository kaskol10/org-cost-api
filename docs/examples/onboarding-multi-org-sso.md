# Multi-org SSO example (Granted)

Pattern for AWS Organizations that share one SSO portal but span multiple billing models.
Replace profile names and SSO URLs with your own.

For generic auth (profiles, API keys, billing models), start with [onboarding-aws-auth.md](../onboarding-aws-auth.md).

---

## SSO portal (example)

| Org | SSO start URL | Region |
|-----|---------------|--------|
| Org A (payer) | `https://your-org.awsapps.com/start` | `eu-west-1` |
| Org B (member-billed) | Same portal after profile setup | `eu-west-1` |

Each session:

```bash
granted sso login
# SSO URL: https://your-org.awsapps.com/start
# Region: eu-west-1
```

---

## Profile naming conventions

Payer-linked accounts typically use Granted-style dot notation:

```yaml
billing_profile: org-payer.AdministratorAccess

accounts:
  - name: production
    profile: Production.AdministratorAccess
    region: eu-west-1
```

Acquired / partner accounts often use a prefix after you normalize SSO profiles:

```yaml
accounts:
  - name: partner-production
    profile: Partner-Production.AdministratorAccess
    billing_profile: account
    region: eu-west-1
```

---

## One-time Granted profile fixes

If SSO profiles are missing `credential_process`, or use the wrong SSO start URL / role naming:

1. Open `~/.aws/config`
2. Ensure each profile has `granted_sso_*` keys (or run `granted sso populate`)
3. Align `granted_sso_start_url` to your org portal
4. Prefer dot-notation profile names (`Account.Role`) for `config.yaml`

Verify:

```bash
aws sts get-caller-identity --profile Production.AdministratorAccess
aws sts get-caller-identity --profile Partner-Production.AdministratorAccess
```

---

## Member-billed accounts

Accounts that are **not** on the org payer bill need Cost Explorer run from the account itself:

```yaml
accounts:
  - name: partner-production
    profile: Partner-Production.AdministratorAccess
    billing_profile: account
    region: eu-west-1
```

See [member-billed-account.md](../member-billed-account.md). A mixed payer-linked + member-billed file is normal — use [config.example.org.yaml](../../config.example.org.yaml) as a structural reference.

---

## Discover accounts

List org account IDs from the payer profile:

```bash
./scripts/list-org-accounts.sh --profile org-payer.AdministratorAccess
```

Paste the YAML snippets into `config.yaml` and assign profiles per account.

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `Unable to locate credentials` | `granted sso login` |
| Profile uses wrong SSO start URL | Fix `granted_sso_start_url` in `~/.aws/config` |
| Member-billed costs $0, payer-linked OK | Add `billing_profile: account` on those rows |
| Slash profile name works but dot does not | Add a Granted-style alias profile |
