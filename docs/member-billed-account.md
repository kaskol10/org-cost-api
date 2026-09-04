# Member-billed account

Some accounts are **not** on your organization payer's Cost Explorer bill. You must query Cost Explorer **from credentials inside that account**.

---

## When to use `billing_profile: account`

```
Does Cost Explorer for this account require credentials IN that account?
  YES → billing_profile: account
  NO  → omit billing_profile; use global billing_profile
```

Typical cases:

- Acquired company with its own AWS payer
- Partner org that bills independently
- Multi-org SSO appendix — see [examples/onboarding-multi-org-sso.md](./examples/onboarding-multi-org-sso.md)

---

## Config

**With SSO profile:**

```yaml
billing_profile: org-payer.AdministratorAccess   # still needed for other linked accounts

accounts:
  - name: partner-production
    id: "444444444444"
    profile: Partner-Production.AdministratorAccess
    billing_profile: account
    region: eu-west-1
```

**With API keys:**

```yaml
accounts:
  - name: partner-production
    id: "444444444444"
    access_key_id: AKIA...
    secret_access_key_env: ORG_COST_PARTNER_SECRET
    billing_profile: account
    region: eu-west-1
```

The account's profile or keys must have `ce:GetCostAndUsage` **in that account**. Cost Explorer must be enabled on that account (standard for any account with billing).

---

## Verify

```bash
export ORG_COST_PARTNER_SECRET='...'   # if using API keys
./scripts/verify-aws-auth.sh
```

The script runs CE from the account's own credentials when `billing_profile: account` is set.

---

## Mixed org

One `config.yaml` can mix payer-linked and member-billed rows. See [config.example.org.yaml](../config.example.org.yaml).

**Note:** The README example `partner-org` with `billing_profile: account` illustrates member billing — it does not mean every partner account must use it. Decide per account using the decision tree above.
