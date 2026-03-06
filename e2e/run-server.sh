#!/bin/sh
set -eu

DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$DIR/.." && pwd)"
RUNTIME_DIR="${GOHOUR_E2E_RUNTIME_DIR:-/tmp/gohour-e2e-runtime}"
DB_PATH="${GOHOUR_E2E_DB_PATH:-$RUNTIME_DIR/test.db}"
PORT="${GOHOUR_E2E_PORT:-9876}"
GOHOUR_BIN="${GOHOUR_E2E_BINARY_PATH:-$REPO_ROOT/gohour}"

if [ ! -x "$GOHOUR_BIN" ]; then
  echo "missing pre-built gohour binary at $GOHOUR_BIN" >&2
  exit 1
fi

rm -rf "$RUNTIME_DIR"
mkdir -p "$RUNTIME_DIR"

cat > "$RUNTIME_DIR/seed-day.csv" <<'EOF'
description,startdatetime,enddatetime,project,activity,skill
seed-entry,2025-01-02 09:00,2025-01-02 10:00,P,A,S
EOF

(
  cd "$REPO_ROOT"
  "$GOHOUR_BIN" import \
    --configFile "$DIR/fixtures/gohour-test.yaml" \
    --db "$DB_PATH" \
    --mapper generic \
    --input "$RUNTIME_DIR/seed-day.csv" \
    --reconcile off
)

exec env GOHOUR_E2E_STUB_REMOTE=1 \
  "$GOHOUR_BIN" serve \
  --no-open \
  --port "$PORT" \
  --configFile "$DIR/fixtures/gohour-test.yaml" \
  --db "$DB_PATH"
