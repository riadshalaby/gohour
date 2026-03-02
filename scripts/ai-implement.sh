#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

echo "Running Codex implementation from PLAN.md"

exec codex "Implement .ai/PLAN.md following CLAUDE.md. Update tests. Do not invent requirements."
