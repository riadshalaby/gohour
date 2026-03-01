#!/usr/bin/env bash
set -euo pipefail

echo "review" > .ai/MODE

echo "Claude started in REVIEW mode"
echo "Expected output: .ai/REVIEW.md"

exec claude
