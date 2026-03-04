#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

usage() {
  cat <<'EOF'
Usage:
  scripts/ai-launch.sh <role> <agent> [agent-options...]

Roles:
  plan | implement | review

Agents:
  claude | codex

Examples:
  scripts/ai-launch.sh plan claude
  scripts/ai-launch.sh review codex
  scripts/ai-launch.sh implement codex -m gpt-5
EOF
}

if [[ $# -lt 2 ]]; then
  usage
  exit 1
fi

role="$1"
agent="$2"
shift 2

case "$role" in
  plan)
    prompt_file=".ai/prompts/planner.md"
    expected_output=".ai/PLAN.md"
    ;;
  implement)
    prompt_file=".ai/prompts/implementer.md"
    expected_output="code + tests (per .ai/PLAN.md)"
    ;;
  review)
    prompt_file=".ai/prompts/reviewer.md"
    expected_output=".ai/REVIEW.md"
    ;;
  *)
    echo "Unsupported role: $role" >&2
    usage
    exit 1
    ;;
esac

if [[ ! -f "$prompt_file" ]]; then
  echo "Prompt file not found: $prompt_file" >&2
  exit 1
fi

echo "Agent: $agent"
echo "Role: $role"
echo "Prompt: $prompt_file"
echo "Expected output: $expected_output"

case "$agent" in
  claude)
    exec claude \
      --permission-mode acceptEdits \
      --add-dir "$REPO_ROOT" \
      "$@" --system-prompt-file "$prompt_file"
    ;;
  codex)
    prompt_text="$(cat "$prompt_file")"
    exec codex \
      --full-auto \
      --sandbox workspace-write \
      -c "sandbox_workspace_write.network_access=true" \
      "$@" "$prompt_text"
    ;;
  *)
    echo "Unsupported agent: $agent" >&2
    usage
    exit 1
    ;;
esac
