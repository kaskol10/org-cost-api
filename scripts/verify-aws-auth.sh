#!/usr/bin/env bash
# Verify AWS credentials and Cost Explorer access for each account in config.yaml.
#
# Usage:
#   ./scripts/verify-aws-auth.sh [config.yaml]
#   ./scripts/verify-aws-auth.sh config.yaml --format markdown
#   ./scripts/verify-aws-auth.sh config.yaml --format json
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONFIG="$ROOT/config.yaml"
FORMAT="text"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --format|-f)
      FORMAT="${2:?--format requires text|json|markdown}"
      shift 2
      ;;
    --help|-h)
      sed -n '2,8p' "$0" | sed 's/^# \?//'
      exit 0
      ;;
    -*)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
    *)
      CONFIG="$1"
      shift
      ;;
  esac
done

if [[ ! -f "$CONFIG" ]]; then
  echo "Config not found: $CONFIG" >&2
  echo "Copy config.example.yaml or run ./scripts/bootstrap-config.sh first." >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "go not found; install Go 1.24+" >&2
  exit 1
fi

case "$FORMAT" in
  text|json|markdown) ;;
  *)
    echo "Invalid --format: $FORMAT (use text, json, or markdown)" >&2
    exit 1
    ;;
esac

cd "$ROOT/backend"
exec go run ./cmd/verify-auth --format "$FORMAT" "$CONFIG"
