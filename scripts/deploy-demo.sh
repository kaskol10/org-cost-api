#!/usr/bin/env bash
# Deploy the public synthetic demo to Fly.io.
#
# Prerequisites:
#   flyctl auth login   # or export FLY_API_TOKEN
#
# Usage:
#   ./scripts/deploy-demo.sh
#   ./scripts/deploy-demo.sh --dry-run
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEMO_URL_FILE="$ROOT/deploy/DEMO_URL"
FLY_CONFIG="$ROOT/deploy/fly.toml"
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    -h|--help)
      sed -n '2,10p' "$0" | sed 's/^# \?//'
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

if ! command -v flyctl >/dev/null 2>&1; then
  echo "flyctl not found — install: https://fly.io/docs/hands-on/install-flyctl/" >&2
  exit 1
fi

if ! flyctl auth whoami >/dev/null 2>&1; then
  echo "Not logged in to Fly.io — run: flyctl auth login" >&2
  exit 1
fi

APP="$(grep "^app = " "$FLY_CONFIG" | sed "s/app = '//;s/'//")"
if [[ "$DRY_RUN" == true ]]; then
  echo "Would deploy $APP using $FLY_CONFIG"
  flyctl deploy --config "$FLY_CONFIG" --dry-run
  exit 0
fi

echo "Deploying demo to Fly.io app: $APP" >&2
flyctl deploy --config "$FLY_CONFIG" --remote-only

URL="https://${APP}.fly.dev"
echo "$URL" > "$DEMO_URL_FILE"
echo "" >&2
echo "Demo deployed: $URL" >&2
echo "Update README live-demo link if this URL changed." >&2
echo "Smoke test:" >&2
echo "  curl -s $URL/api/health | jq ." >&2
echo "  curl -s $URL/api/meta | jq ." >&2
