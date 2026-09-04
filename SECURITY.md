# Security

## Reporting a vulnerability

Please report security issues **privately** via [GitHub Security Advisories](https://github.com/kaskol10/org-cost-api/security/advisories/new) (or email the maintainers if the repo is not yet public).

Do not open public issues for exploitable vulnerabilities.

## Demo vs production

- The **public demo** (`ORG_COST_DEMO=1` or `demo: true` in config) serves **synthetic fixture data only**. It does not connect to AWS and should not hold real credentials.
- **Self-hosted production** must set `api_token_env` and restrict network access. See [docs/browser-users.md](docs/browser-users.md) and [docs/KNOWN-ISSUES.md](docs/KNOWN-ISSUES.md).

## Scope

In scope: authentication bypass, data exposure between tenants, SSRF from the API, MCP tool injection.

Out of scope for the demo deployment: denial-of-service on the public fixture endpoint (rate limits are best-effort).
