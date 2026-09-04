# Add one account

Use this when the dashboard already works and you need one more AWS account.

---

## Steps

1. **Discover the account ID** (if you don't have it):

   ```bash
   ./scripts/list-org-accounts.sh --profile YOUR_PAYER_PROFILE
   ```

2. **Choose billing model** ([decision tree](./onboarding-aws-auth.md#billing-model-payer-linked-vs-member-billed)):

   - On org payer bill → omit `billing_profile`
   - Separate payer → `billing_profile: account`

3. **Add a row** to `config.yaml`:

   ```yaml
   accounts:
     # ... existing rows ...
     - name: analytics
       id: "123456789012"
       profile: Analytics.AdministratorAccess
       region: eu-west-1
   ```

4. **Verify** only the new account (optional — full check also works):

   ```bash
   ./scripts/verify-aws-auth.sh
   ```

5. **Restart the backend** (config is read at startup):

   ```bash
   cd backend && go run ./cmd/server -config ../config.yaml
   ```

6. **Refresh the dashboard** — click **Refresh** in the UI or `GET /api/dashboard?refresh=true`.

---

## Common mistakes

| Mistake | Symptom |
|---------|---------|
| Wrong profile name | STS failure in verify script |
| Member-billed row missing `billing_profile: account` | Account shows $0 |
| Wrong `region` | EBS inventory empty; CE still works |
| Forgot to restart backend | New account not listed |

See [member-billed-account.md](./member-billed-account.md) if CE returns AccessDenied from the payer.
