#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

echo "Claude started in REVIEW mode"
echo "Expected output: .ai/REVIEW.md"

exec claude --system-prompt-file .ai/prompts/reviewer.md
