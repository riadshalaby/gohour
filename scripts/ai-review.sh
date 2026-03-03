#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
agent="${1:-claude}"
if [[ $# -gt 0 ]]; then
  shift
fi

exec "$SCRIPT_DIR/ai-launch.sh" review "$agent" "$@"
