#!/usr/bin/env bash
# Trigger a dashboard fetch to warm cache and save a daily history snapshot.
# Add to cron, e.g.: 0 8 * * * /path/to/scripts/daily-snapshot.sh

set -euo pipefail

API_URL="${ORG_COST_API_URL:-${EC2_OTHER_API_URL:-http://localhost:8080}}"
AUTH_HEADER=()
if [[ -n "${ORG_COST_API_TOKEN:-}" ]]; then
  AUTH_HEADER=(-H "Authorization: Bearer ${ORG_COST_API_TOKEN}")
fi

curl -sf "${AUTH_HEADER[@]}" "${API_URL%/}/api/dashboard" >/dev/null
echo "Snapshot triggered at $(date -u +%Y-%m-%dT%H:%M:%SZ)"
