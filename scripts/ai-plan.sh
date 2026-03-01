#!/usr/bin/env bash
set -euo pipefail

echo "plan" > .ai/MODE

echo "Claude started in PLAN mode"
echo "Expected output: .ai/PLAN.md"

exec claude
