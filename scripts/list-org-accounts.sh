#!/usr/bin/env bash
# List AWS Organization accounts and emit YAML snippets for config.yaml.
#
# Usage:
#   ./scripts/list-org-accounts.sh --profile org-payer.AdministratorAccess
#   ./scripts/list-org-accounts.sh --profile org-payer.AdministratorAccess --prefix MyOrg
#   ./scripts/list-org-accounts.sh --org-id o-abc123 --profile org-payer.AdministratorAccess
#
# Output is stdout only — paste into config.yaml and assign real profile names.
set -euo pipefail

PROFILE=""
ORG_ID=""
PREFIX=""
REGION="eu-west-1"

usage() {
  sed -n '2,8p' "$0" | sed 's/^# \?//'
  exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profile|-p)
      PROFILE="${2:?--profile requires a value}"
      shift 2
      ;;
    --org-id|-o)
      ORG_ID="${2:?--org-id requires a value}"
      shift 2
      ;;
    --prefix)
      PREFIX="${2:?--prefix requires a value}"
      shift 2
      ;;
    --region|-r)
      REGION="${2:?--region requires a value}"
      shift 2
      ;;
    -h|--help)
      usage 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage 1
      ;;
  esac
done

if [[ -z "$PROFILE" ]]; then
  echo "ERROR: --profile is required (payer or org management account SSO profile)" >&2
  usage 1
fi

if ! command -v aws >/dev/null 2>&1; then
  echo "aws CLI not found; install AWS CLI v2" >&2
  exit 1
fi

AWS=(aws --profile "$PROFILE" --output json)

echo "# Paste under accounts: in config.yaml — assign profile names for each row" >&2
echo "# Generated $(date -u +%Y-%m-%dT%H:%M:%SZ) from profile: $PROFILE" >&2
echo "" >&2

if [[ -z "$ORG_ID" ]]; then
  ORG_ID="$("${AWS[@]}" organizations describe-organization 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('Organization',{}).get('Id',''))" || true)"
fi

if [[ -z "$ORG_ID" ]]; then
  echo "WARNING: Could not detect organization ID. Pass --org-id or ensure organizations:DescribeOrganization is allowed." >&2
fi

# Paginate ListAccounts
NEXT=""
while true; do
  if [[ -n "$NEXT" && "$NEXT" != "null" ]]; then
    PAGE="$("${AWS[@]}" organizations list-accounts --starting-token "$NEXT")"
  else
    PAGE="$("${AWS[@]}" organizations list-accounts)"
  fi

  echo "$PAGE" | PREFIX="$PREFIX" REGION="$REGION" python3 -c '
import json, os, re, sys

prefix = os.environ.get("PREFIX", "")
region = os.environ.get("REGION", "eu-west-1")
data = json.load(sys.stdin)

def slug(name: str) -> str:
    s = name.lower()
    s = re.sub(r"[^a-z0-9]+", "-", s).strip("-")
    return s or "account"

for acct in sorted(data.get("Accounts", []), key=lambda a: a.get("Name", "")):
    if acct.get("Status") != "ACTIVE":
        continue
    aid = acct["Id"]
    name = acct.get("Name", aid)
    slug_name = slug(name)
    if prefix:
        display = f"{prefix}-{slug_name}"
        profile_hint = f"{prefix}-{name}.AdministratorAccess".replace(" ", "-")
    else:
        display = slug_name
        profile_hint = f"{name}.AdministratorAccess".replace(" ", "-")
    print(f"  - name: {display}")
    print(f"    id: \"{aid}\"")
    print(f"    profile: {profile_hint}  # TODO: match your ~/.aws/config profile")
    print(f"    region: {region}")
    print()
'

  NEXT="$(echo "$PAGE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('NextToken') or '')")"
  if [[ -z "$NEXT" ]]; then
    break
  fi
done

echo "# Member-billed accounts need billing_profile: account — see docs/member-billed-account.md" >&2
