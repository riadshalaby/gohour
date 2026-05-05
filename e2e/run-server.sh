#!/bin/sh
set -eu

DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$DIR/.." && pwd)"
RUNTIME_DIR="${GOHOUR_E2E_RUNTIME_DIR:-/tmp/gohour-e2e-runtime}"
PORT="${GOHOUR_E2E_PORT:-9876}"
GOHOUR_BIN="${GOHOUR_E2E_BINARY_PATH:-$REPO_ROOT/gohour}"

if [ ! -x "$GOHOUR_BIN" ]; then
  echo "missing pre-built gohour binary at $GOHOUR_BIN" >&2
  exit 1
fi

rm -rf "$RUNTIME_DIR"
mkdir -p "$RUNTIME_DIR"
cp "$DIR/fixtures/config.yaml" "$RUNTIME_DIR/config.yaml"

exec env GOHOUR_E2E_STUB_REMOTE=1 \
  GOHOUR_DATA_DIR="$RUNTIME_DIR" \
  "$GOHOUR_BIN" serve \
  --no-open \
  --port "$PORT"
