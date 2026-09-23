#!/usr/bin/env bash
# Trigger report fetches to warm PVC dashboard cache and save daily history snapshots.
# Add to cron, e.g.: 0 8 * * * /path/to/scripts/daily-snapshot.sh
# Helm CronJob calls this for period=30d and period=mtd once per day.

set -euo pipefail

API_URL="${ORG_COST_API_URL:-${EC2_OTHER_API_URL:-http://localhost:8080}}"
AUTH_HEADER=()
if [[ -n "${ORG_COST_API_TOKEN:-}" ]]; then
  AUTH_HEADER=(-H "Authorization: Bearer ${ORG_COST_API_TOKEN}")
fi

PERIODS=("30d" "mtd")
if [[ $# -gt 0 ]]; then
  PERIODS=("$@")
fi

for period in "${PERIODS[@]}"; do
  curl -sf "${AUTH_HEADER[@]}" "${API_URL%/}/api/report?period=${period}" >/dev/null
  echo "Warmed period=${period} at $(date -u +%Y-%m-%dT%H:%M:%SZ)"
done
