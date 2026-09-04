# Onboarding — AWS credentials

This guide is split so generic auth stays separate from optional multi-org SSO examples.

| Doc | Audience |
|-----|----------|
| **[getting-started.md](./getting-started.md)** | Everyone — first-hour checklist and prod auth |
| **[onboarding-aws-auth.md](./onboarding-aws-auth.md)** | Generic: SSO/profiles/API keys, billing model, IAM, verification |
| **[examples/onboarding-multi-org-sso.md](./examples/onboarding-multi-org-sso.md)** | Example: multi-org SSO with Granted profile fixes |

**Incremental guides**

- [add-account.md](./add-account.md) — add one account to an existing config
- [member-billed-account.md](./member-billed-account.md) — separate payer / member billing
- [browser-users.md](./browser-users.md) — static frontend + Bearer token

**Templates**

- [config.example.yaml](../config.example.yaml) — minimal 2-account starter
- [config.example.org.yaml](../config.example.org.yaml) — multi-pattern org template

**Hermes / agents**

- [mcp-setup.md](./mcp-setup.md) — **Hermes + Cursor** MCP setup (start here)
- [hermes-setup.md](./hermes-setup.md) — Hermes-specific troubleshooting
- [hermes-nous-deployment.md](./hermes-nous-deployment.md) — central API deployment
