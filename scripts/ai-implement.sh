#!/usr/bin/env bash
set -euo pipefail

echo "idle" > .ai/MODE

echo "Running Codex implementation from PLAN.md"

exec codex "Implement .ai/PLAN.md following CLAUDE.md. Update tests. Do not invent requirements."
