# CLAUDE

## Project Overview
- `gohour` is a Go CLI built with Cobra + Viper.
- Purpose: import time-tracking files, normalize worklogs, store in SQLite, reconcile overlaps, submit to OnePoint, and export reports.

## Architecture Layers
- `cmd` -> `importer` -> `reconcile` -> `storage` -> `onepoint` -> `output`

## Submit Command Invariants
- If a remote day contains any locked entry, skip the full day.
- Duplicate detection compares only: `StartTime`, `FinishTime`, `ProjectID`, `ActivityID`, `SkillID`.
- Overlaps are handled interactively in normal mode (`w`/`s`/`W`/`S`/`a`).
- `--dry-run` reads remote day worklogs, reports locked days/overlaps, and performs no persist call.

## Coding Rules
- Return errors; never panic.
- Prefer small, test-backed changes.
- Avoid global mutable state.

## AI Workflow Rules
- Claude:
  - writes `.ai/PLAN.md`
  - writes `.ai/REVIEW.md`
  - never edits code
- Codex:
  - implements `.ai/PLAN.md`
  - updates tests
  - must not invent requirements

## AI Operating Mode
- No `.ai/MODE` file is required or used.
- Mode is selected by the launcher prompt/context:
  - Planner (`.ai/prompts/planner.md`): update ONLY `.ai/PLAN.md`, never modify source code.
  - Reviewer (`.ai/prompts/reviewer.md`): update ONLY `.ai/REVIEW.md`, never modify source code.
  - Implementer/Codex: implement `.ai/PLAN.md` and update tests under the rules above.

## Git Rules
- Work in the current branch.
- Never auto commit.
- Human reviews diffs before commit.
