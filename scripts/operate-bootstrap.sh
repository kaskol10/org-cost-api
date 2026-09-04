#!/usr/bin/env bash
# Operate path helper: bootstrap config.yaml and verify AWS auth.
#
# Usage:
#   ./scripts/operate-bootstrap.sh --profile org-payer.AdministratorAccess
#   ./scripts/operate-bootstrap.sh --profile org-payer.AdministratorAccess --output config.yaml --profile-map profiles.csv
#   ./scripts/operate-bootstrap.sh --profile org-payer.AdministratorAccess --skip-verify
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SKIP_VERIFY=false
BOOTSTRAP_ARGS=()

usage() {
  sed -n '2,8p' "$0" | sed 's/^# \?//'
  exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-verify)
      SKIP_VERIFY=true
      shift
      ;;
    -h|--help)
      usage 0
      ;;
    *)
      BOOTSTRAP_ARGS+=("$1")
      shift
      ;;
  esac
done

if [[ ${#BOOTSTRAP_ARGS[@]} -eq 0 ]]; then
  echo "ERROR: pass bootstrap-config.sh options (at minimum --profile)" >&2
  usage 1
fi

chmod +x "$ROOT/scripts/bootstrap-config.sh" "$ROOT/scripts/verify-aws-auth.sh"

"$ROOT/scripts/bootstrap-config.sh" "${BOOTSTRAP_ARGS[@]}"

OUTPUT="$ROOT/config.yaml"
for i in "${!BOOTSTRAP_ARGS[@]}"; do
  if [[ "${BOOTSTRAP_ARGS[$i]}" == "--output" || "${BOOTSTRAP_ARGS[$i]}" == "-o" ]]; then
    OUTPUT="${BOOTSTRAP_ARGS[$((i + 1))]}"
  fi
done

if [[ "$SKIP_VERIFY" == true ]]; then
  echo "Skipped verify-aws-auth (--skip-verify)" >&2
else
  echo "" >&2
  echo "Verifying AWS auth..." >&2
  "$ROOT/scripts/verify-aws-auth.sh" "$OUTPUT" --format markdown
fi

echo "" >&2
echo "Operate next steps:" >&2
echo "  aws sso login   # refresh SSO if needed" >&2
echo "  docker compose -f docker-compose.prod.yml up --build" >&2
echo "  # optional chat: docker compose -f docker-compose.full.yml up --build" >&2
echo "  open http://localhost:8080" >&2
