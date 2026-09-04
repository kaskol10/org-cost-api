#!/usr/bin/env bash
# Verify all layers for org-cost-api + MCP (+ optional Hermes).
#
# Usage:
#   ./scripts/verify-hermes-costs.sh
#   ORG_COST_API_URL=https://org-cost-api-demo.fly.dev ./scripts/verify-hermes-costs.sh
#   ./scripts/verify-hermes-costs.sh --skip-hermes
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
API_URL="${ORG_COST_API_URL:-${EC2_OTHER_API_URL:-http://localhost:8080}}"
HERMES="${HERMES_BIN:-$HOME/.local/bin/hermes}"
SKIP_HERMES=0
MCP_BIN=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-hermes) SKIP_HERMES=1; shift ;;
    -h|--help)
      sed -n '2,7p' "$0" | sed 's/^# \?//'
      exit 0
      ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

# Resolve MCP: venv, pipx, or PATH
if [[ -x "$REPO_ROOT/mcp-server/.venv/bin/org-cost-mcp" ]]; then
  MCP_BIN="$REPO_ROOT/mcp-server/.venv/bin/org-cost-mcp"
elif command -v org-cost-mcp >/dev/null 2>&1; then
  MCP_BIN="$(command -v org-cost-mcp)"
fi

FAIL=0
pass() { echo "OK   $*"; }
fail() { echo "FAIL $*"; FAIL=1; }
warn() { echo "WARN $*"; }

echo "== org-cost-api integration check =="
echo "API: $API_URL"
echo ""

# --- Layer 1: API health ---
if curl -sf "$API_URL/api/health" | grep -q '"status"'; then
  pass "GET /api/health"
else
  fail "GET /api/health — start API: docker compose -f docker-compose.demo.yml up"
fi

READY_JSON="$(curl -sf "$API_URL/api/ready" 2>/dev/null || true)"
if [[ -n "$READY_JSON" ]]; then
  pass "GET /api/ready"
  echo "     $(echo "$READY_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); print('demo='+str(d.get('demo','?'))+' billing='+str(d.get('billing_check','?')))" 2>/dev/null || echo "$READY_JSON")"
else
  fail "GET /api/ready — API not ready or unreachable"
fi

# --- Layer 2: MCP Python client ---
if [[ -n "$MCP_BIN" ]]; then
  if ORG_COST_API_URL="$API_URL" ORG_COST_API_TOKEN="${ORG_COST_API_TOKEN:-}" \
    python3 -c "
from org_cost_mcp.client import health_status
s = health_status()
print('     ready=%s demo=%s auth_ok=%s' % (s.get('ready'), s.get('demo'), s.get('auth_ok')))
assert s.get('healthy'), s
" 2>/dev/null; then
    pass "org_cost_mcp.client.health_status ($MCP_BIN)"
  else
    fail "org_cost_mcp.client.health_status — pip install org-cost-mcp or cd mcp-server && pip install -e ."
  fi
else
  fail "org-cost-mcp not found — pip install org-cost-mcp"
fi

# --- Layer 3: MCP client report smoke ---
if [[ -n "$MCP_BIN" && "$FAIL" -eq 0 ]]; then
  if ORG_COST_API_URL="$API_URL" ORG_COST_API_TOKEN="${ORG_COST_API_TOKEN:-}" \
    python3 -c "
from org_cost_mcp.client import fetch_report
r = fetch_report(refresh=False)
dash = r.get('dashboard', {})
total = dash.get('totals', {}).get('org_total')
accounts = dash.get('accounts', [])
print('     org_total=%s accounts=%s' % (total, len(accounts)))
" 2>/dev/null; then
    pass "fetch_report(refresh=false)"
  else
    warn "fetch_report failed — auth token or API data issue"
  fi
fi

# --- Layer 4: Hermes (optional) ---
if [[ "$SKIP_HERMES" -eq 0 ]]; then
  echo ""
  echo "== Hermes (optional) =="
  if [[ -f "$HOME/.hermes/config.yaml" ]] && grep -q 'org-cost-api' "$HOME/.hermes/config.yaml"; then
    pass "org-cost-api in ~/.hermes/config.yaml"
    grep -A5 'org-cost-api:' "$HOME/.hermes/config.yaml" | head -6 | sed 's/^/     /'
  else
    warn "Add mcp_servers.org-cost-api — see docs/integrate-quickstart.md#4-hermes"
  fi

  if [[ -e "$HOME/.hermes/skills/org-cost-api/SKILL.md" ]]; then
    pass "~/.hermes/skills/org-cost-api"
  else
    warn "Optional skill: ln -sf $REPO_ROOT/skills/org-cost-api ~/.hermes/skills/org-cost-api"
  fi

  if command -v "$HERMES" >/dev/null 2>&1; then
    echo ""
    echo "== hermes mcp list =="
    "$HERMES" mcp list 2>&1 | grep -E 'org-cost|Name' | sed 's/^/     /' || true
  else
    warn "Hermes CLI not found at $HERMES (set HERMES_BIN if elsewhere)"
  fi
fi

echo ""
if [[ "$FAIL" -eq 0 ]]; then
  echo "All required checks passed."
  echo ""
  echo "Try in your agent:"
  echo '  "Call check_api_health, then get_org_summary with refresh=false."'
  echo ""
  echo "Docs: docs/integrate-quickstart.md"
  exit 0
fi

echo "One or more checks failed — see docs/integrate-quickstart.md#troubleshooting"
exit 1
