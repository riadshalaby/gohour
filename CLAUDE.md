# CLAUDE

## Project Overview
- `gohour` is a Go project with:
  - a CLI (Cobra + Viper),
  - a localhost web UI started via `gohour serve`.
- Core purpose: import time-tracking files, normalize/store worklogs in SQLite, reconcile overlaps, compare local vs. OnePoint, submit to OnePoint, and export reports.

## Current Status (v0.2.x)
- Implemented commands include: `config`, `import`, `reconcile`, `submit`, `serve`, `export`, `delete`, `auth`, `version`.
- `serve` now exposes an interactive UI (not read-only in practice):
  - month/day compare views (local vs. remote),
  - local worklog create/update/delete,
  - import preview + import execution,
  - day/month submit + dry-run preview,
  - month-level local delete, remote delete, remote-to-local copy/sync actions.
- Import UI supports `billable` mode selection and conflict-aware preview (clean/duplicate/overlap).
- Day/month views show worked and billable totals for both local and remote.

## Architecture Layers
- CLI flow: `cmd` -> `importer` / `reconcile` / `storage` / `submitter` / `onepoint` / `output`
- Web flow: `cmd/serve` -> `web` -> `storage` + `onepoint` + `submitter`
- Shared utilities: `internal/classify`, `internal/timeutil`

## Submit Command Invariants
- If a remote day contains any locked entry, skip the full day.
- Duplicate detection compares only: `StartTime`, `FinishTime`, `ProjectID`, `ActivityID`, `SkillID`.
- If duplicate key matches but billable/comment differ, treat it as an update candidate (not a duplicate skip).
- Overlaps are handled interactively in normal CLI mode (`w`/`s`/`W`/`S`/`a`).
- `--dry-run` still loads remote day worklogs, reports locked/duplicate/overlap outcomes, and performs no persist call.

## Coding Rules
- Return errors; never panic.
- Prefer small, test-backed changes.
- Avoid global mutable state.
- Keep documentation aligned with behavior:
  - Update `README.md` whenever commands, flags, workflows, or defaults change.
  - Update Cobra command help text (`Use`, `Short`, `Long`, `Example`, and flag descriptions) whenever related behavior changes.

## AI Workflow Rules
- Plan Mode:
  - writes `.ai/PLAN.md`
  - never edits code
- Review Mode:
  - writes `.ai/REVIEW.md`
  - never edits code
- Implement Mode:
  - implements `.ai/PLAN.md`
  - updates tests
  - must not invent requirements

## AI Operating Mode
- Mode is selected by the launcher prompt/context:
  - Generic launcher: `scripts/ai-launch.sh <role> <agent> [agent-options...]`
    - roles: `plan`, `implement`, `review`
    - agents: `claude`, `codex`
  - Convenience wrappers:
    - `scripts/ai-plan.sh [agent] [agent-options...]` (default agent: `claude`)
    - `scripts/ai-implement.sh [agent] [agent-options...]` (default agent: `codex`)
    - `scripts/ai-review.sh [agent] [agent-options...]` (default agent: `codex`)
- No `.ai/MODE` file is used.

## Git Rules
- Work in the current branch.
- Never auto commit.
- Human reviews diffs before commit.

## Release Rules
- Never release directly from a feature branch.
- A feature is releasable only after it is merged into `main` via PR and required checks/tests pass.
- Create tag `vX.Y.Z` on the corresponding merge commit in `main` (no unrelated extra commit between merge and tag).
- Build release artifacts from the tagged commit with embedded version metadata:
  - `go build -ldflags "-X gohour/cmd.Version=vX.Y.Z" ...`
- Publish GitHub release with:
  - consistent notes format: "Changes since `<previous-tag>`",
  - distribution binaries for `darwin/linux/windows` on `amd64/arm64`,
  - `SHA256SUMS`.
- After the release is done:
  - reset `.ai/PLAN.md` for the next cycle,
  - reset `.ai/REVIEW.md` for the next cycle,
  - rework `ROADMAP.md` to prepare scope and priorities for the next version.
