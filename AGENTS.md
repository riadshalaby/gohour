# AGENTS

## Project Overview
- `gohour` is a Go project with:
  - a CLI (Cobra + Viper),
  - a localhost web UI started via `gohour serve`.
- Core purpose: import time-tracking files, normalize/store worklogs in SQLite, reconcile overlaps, compare local vs. OnePoint, submit to OnePoint, and export reports.

## Current Status 
- Implemented commands: `serve`, `version`.
- `serve` exposes an interactive web UI at `localhost:<port>` with:
  - month/day compare views (local vs. remote),
  - local worklog create/update/delete,
  - import dialog pre-fills mapper/project/activity/skill/billable from matched rule on file pick, with per-field override on the preview dialog,
  - EPM imports automatically trigger reconciliation after a successful import,
  - day/month submit + dry-run preview,
  - month-level local delete, remote delete, remote-to-local copy/sync actions,
  - config page at `/config` for OnePoint URL and import rule CRUD.
- Import UI requires an explicit `billable`/`non-billable` choice (no implicit "auto"), with conflict-aware preview (clean/duplicate/overlap), and "update rule" persistence of overrides.
- Day/month views show worked and billable totals for both local and remote.

## Architecture Layers
- CLI flow: `cmd/serve` -> `web` -> `storage` + `onepoint` + `submitter`
- Web flow: `cmd/serve` -> `web` -> `storage` + `onepoint` + `submitter`
- Shared utilities: `internal/classify`, `internal/timeutil`

## Submit Command Invariants
- If a remote day contains any locked entry, skip the full day.
- Duplicate detection compares only: `StartTime`, `FinishTime`, `ProjectID`, `ActivityID`, `SkillID`.
- If duplicate key matches but billable/comment differ, treat it as an update candidate (not a duplicate skip).
- Overlaps should be resolved via reconciliation before submitting; the web UI dry-run preview surfaces overlap outcomes.
- Dry-run loads remote day worklogs, reports locked/duplicate/overlap outcomes, and performs no persist call.

## Coding Rules
- Return errors; never panic.
- Prefer small, test-backed changes.
- Avoid global mutable state.
- Keep documentation aligned with behavior:
  - Update `README.md` whenever commands, flags, workflows, or defaults change.
  - Update Cobra command help text (`Use`, `Short`, `Long`, `Example`, and flag descriptions) whenever related behavior changes.

## Release Rules
- Never release directly from a feature branch.
- A feature is releasable only after it is merged into `main` via PR and required checks/tests pass.
- Releases are produced by release-please:
  - Conventional Commits on `main` drive automated release PRs that update `cmd/version.go` and `CHANGELOG.md`.
  - Merging a release PR creates the `vX.Y.Z` tag.
- The `goreleaser` workflow runs on `vX.Y.Z` tag pushes and attaches `darwin/linux/windows` binaries on `amd64/arm64` plus `SHA256SUMS` to the GitHub release.
- To pin the next release version explicitly (for example when closing a planned cycle), include `Release-As: x.y.z` in a commit footer on `main`. The `aide cycle end` workflow already emits this footer.

## Session Workflow
- Keep entries concise and timestamped in UTC.
- Stage newly created files explicitly:
  - `git add <new-file>`

## Validation Commands
- Run formatting after every code change:
  - `go fmt ./...`
- Prefer targeted validation while iterating; run broader validation before finishing:
  - Format: `go fmt ./...`
  - Vet: `go vet ./...`
  - Test: `go test ./...`

## Language Rules
- Use English for code comments, log/output messages, `README.md`.

## PR Policy
- Feature PRs use `aide pr`.
- `aide pr` writes the Summary, Breaking Changes, Included Commits, and Test Plan sections for feature PRs.
- A PR to `main` remains mandatory for user-reviewed changes.

## Git Rules
- Work in the current branch.

<!-- agentinit:managed:start -->
## Documentation Rules
- Every change to behavior, interfaces, workflows, or configuration must include corresponding updates to affected documentation and code comments in the same commit.
- Documentation accuracy is part of implementation scope, not a follow-up task.
- The planner must include documentation update scope explicitly in the plan whenever behavior, interfaces, workflows, or configuration change. Documentation updates are implementation scope, not a follow-up task.

## Hard Rules
- Never include `Co-Authored-By` trailers in commit messages.

## AI Workflow Rules
- Plan Mode:
  - waits for explicit user start signal
  - writes `.ai/PLAN.md`
  - updates `.ai/TASKS.md` status to `ready_for_implement`
  - appends a handoff entry to `.ai/HANDOFF.md`
  - never edits code
- Review Mode:
  - waits for explicit user start signal
  - writes `.ai/REVIEW.md`
  - updates `.ai/TASKS.md` status to `ready_to_commit` or `changes_requested`
  - appends a handoff entry to `.ai/HANDOFF.md`
  - performs review plus verification, including E2E and exploratory checks when appropriate
  - never edits code
- Implement Mode (`next_task`):
  - waits for explicit user start signal
  - implements `.ai/PLAN.md`
  - writes or updates tests for each changed behaviour before writing implementation code
  - updates affected documentation and code comments whenever behavior, interfaces, or workflows change
  - writes the final Conventional Commit message into the HANDOFF entry `Commit` field
  - does not `git commit`
  - updates `.ai/TASKS.md` status to `ready_for_review`
  - appends a handoff entry to `.ai/HANDOFF.md`
  - must not invent requirements
- Implement Mode (`commit_task` after review):
  - only for tasks in `ready_to_commit`
  - reads the commit message from the task's `next_task` HANDOFF entry
  - updates `.ai/TASKS.md` status to `done`
  - appends a handoff entry to `.ai/HANDOFF.md`
  - runs `git add -A && git commit -m "<message>"`
- Implement Mode (rework after rejection):
  - reads `.ai/REVIEW.md` findings as a checklist
  - addresses every finding marked as required fix
  - re-runs validations
  - does not `git commit`
  - updates `.ai/TASKS.md` status from `changes_requested` to `ready_for_review`
  - appends a handoff entry to `.ai/HANDOFF.md`

## AI Operating Mode
- Mode is selected by the launcher prompt/context:
  - Cycle bootstrap:
    - `aide cycle start <branch-name>`
  - Convenience wrappers:
    - `aide plan [agent] [agent-options...]` (default agent from `.ai/config.json`, fallback: `claude`)
    - `aide implement [agent] [agent-options...]` (default agent from `.ai/config.json`, fallback: `codex`)
    - `aide review [agent] [agent-options...]` (default agent from `.ai/config.json`, fallback: `claude`)
    - `aide po [agent] [agent-options...]` (launcher for the PO orchestration session)
- No `.ai/MODE` file is used.

## Runtime Modes
- Manual mode:
  - you start planner, implementer, and reviewer sessions yourself in separate terminals
  - you drive task progress by sending the documented text commands directly to each session
- Auto mode:
  - you start the PO session with `aide po`
  - the PO session uses the `aide` MCP server to run the post-planning loop by coordinating implementer and reviewer sessions for the same task flow
- Both modes use the same `.ai/TASKS.md` board, `.ai/PLAN.md` plan, review artifacts, and status transitions.

## Persistent Session Workflow
- In manual mode, no role autostarts another role.
- In auto mode, the PO session may start or reconnect to the role sessions it coordinates.
- Start a new development cycle with `aide cycle start <branch-name>`.
- Start the planner, implementer, and reviewer once, then keep those sessions open for the rest of the cycle.
- When using auto mode, let the PO session manage those role sessions instead of driving them directly yourself.
- Every role waits in `WAIT_FOR_USER_START` state until you explicitly tell it to begin.
- After launch, steer the existing sessions with text commands instead of relaunching scripts for each step.
- Agent choice is manual when you launch each role (`claude` or `codex`) and can vary by session.
- Handoff log policy:
  - runtime log: `.ai/HANDOFF.md` (tracked cycle log)
  - tracked template: `.ai/HANDOFF.template.md`
- Handoffs are file-based:
  - planner -> implementer uses `.ai/PLAN.md` + `.ai/TASKS.md` + `.ai/HANDOFF.md`
  - implementer -> reviewer uses commit + `.ai/TASKS.md` + `.ai/HANDOFF.md`
- Recommended status flow in `.ai/TASKS.md`:
  - `in_planning` -> `ready_for_implement` -> `in_implementation` -> `ready_for_review` -> `in_review` -> `ready_to_commit` -> `done`
  - Rework loop: `changes_requested` -> `in_implementation` -> `ready_for_review` -> `in_review` -> `done`
- Every role must re-read `.ai/TASKS.md` before executing any command. Additional files depend on the role and command — see each role's prompt for specifics.
- Role-specific files to reload as needed:
  - planner: `ROADMAP.md`, `.ai/PLAN.md`
  - implementer: `.ai/PLAN.md`, `.ai/REVIEW.md` when reworking review findings
  - reviewer: `.ai/PLAN.md`, `.ai/REVIEW.md`
- Files are the source of truth. No role should rely on hidden session memory when file state disagrees.

## Session Commands
Use these text commands inside the already-running role sessions.
- PO session:
  - launched with `aide po [agent]` (default agent: `claude`)
  - uses MCP tools internally (`session_start`, `session_run`, `session_wait`, `session_get_output`, `session_get_result`, `session_status`, `session_list`, `session_stop`, `session_reset`, `session_delete`) to coordinate role sessions
  - `codex` PO runs use inline `-c mcp_servers.aide.*` overrides, so no global Codex MCP registration is required
  - never starts a planner session; if no tasks are in `ready_for_implement` or later, tells the user to run the planner first
  - `work_task [TASK_ID]`
    - no task ID: pick the first task that is not `done`, regardless of status (supports in-flight recovery)
    - with task ID: target that specific task
    - drive through full implement -> review -> commit cycle, then stop and report
    - if no eligible task exists, report that the board has no work remaining
  - `work_all`
    - run `work_task` repeatedly until all tasks are `done` or a blocker requires human intervention
    - stop at the first blocker and report
- Planner session:
  - before `start_plan`, conversation with the planner is the roadmap-refinement phase:
    - tighten scope, acceptance criteria, constraints, and decision points directly in `ROADMAP.md`
    - surface ambiguities and trade-offs for the user to resolve instead of inventing requirements
  - `start_plan` is the gate to formal planning; once invoked, write the plan without asking for another readiness confirmation
  - `start_plan`
    - read `ROADMAP.md` and current planning artifacts
    - create or restructure tasks in `.ai/TASKS.md` as needed
    - write or rewrite `.ai/PLAN.md`
    - when planning is complete, move all newly planned tasks to `ready_for_implement`
  - `rework_plan [TASK_ID]`
    - revisit an existing plan when scope, constraints, or approach change
    - update `.ai/PLAN.md`, `.ai/TASKS.md`, and `.ai/HANDOFF.md` as needed without modifying code
    - when no task ID is supplied, replan the overall roadmap/task breakdown
    - when a task ID is supplied and it does not exist or is not appropriate for replanning, report the current status and abort
- Implementer session:
  - `next_task [TASK_ID]`
    - select the first task in `ready_for_implement` or `in_implementation` when no task ID is supplied
    - if the supplied task is not valid for implementer work, report its current status and abort
    - when work begins, update the task to `in_implementation`
  - `rework_task [TASK_ID]`
    - implementer only
    - target a task in `changes_requested`
    - load `.ai/REVIEW.md` as the required-fix checklist for review rework
    - if no task matches, report that no tasks are pending rework
  - `commit_task [TASK_ID]`
    - implementer only
    - target a task in `ready_to_commit`
    - read the commit message from the task's `next_task` HANDOFF entry `Commit` field
    - update `.ai/TASKS.md` to `done`
    - append a `commit_task` HANDOFF entry
    - run `git add -A && git commit -m "<message>"`
    - if the supplied task is not ready to commit, report its current status and abort
  - `aide cycle end [VERSION]`
    - verify all tasks are `done`
    - if the completion condition is not met, report the blocking task states and abort
    - if no version is supplied, ask the user for it before proceeding
    - append a closing entry to `.ai/HANDOFF.md` (`### Cycle closed — <version> — <UTC timestamp>`)
    - stage and commit with `chore(ai): close cycle` and a `Release-As: x.y.z` footer
    - then run `aide pr` to update the PR
  - `status_cycle [TASK_ID]`
    - return deterministic task status, current owner role, and next recommended action
    - when no task ID is supplied, summarize tasks relevant to the caller and the overall board state
    - if no task matches the caller's role, say so explicitly and summarize the board
- Reviewer session:
  - `next_task [TASK_ID]`
    - select the first task in `ready_for_review` or `in_review` when no task ID is supplied
    - if the supplied task is not valid for reviewer work, report its current status and abort
    - when review begins, update the task to `in_review`
    - when review and verification pass, move the task to `ready_to_commit`
  - `status_cycle [TASK_ID]`
    - return deterministic task status, current owner role, and next recommended action
    - when no task ID is supplied, summarize tasks relevant to the caller and the overall board state
    - if no task matches the caller's role, say so explicitly and summarize the board

## Commit Conventions
- Commit behavior by role:
  - `plan` role never commits.
  - `review` role never commits.
  - `implement` role does not commit during `next_task` or `rework_task`. The single task commit is created by `commit_task` after review approval.
  - `aide cycle end` commits the cycle-close artifacts with a `Release-As: x.y.z` footer and can be followed by `aide pr`.
- Conventional Commit subjects must be release-note ready: describe the user-visible change or outcome, not just the implementation mechanism.
- Prefer subjects in the form `<type>(<scope>): <user-facing change>`; if the subject alone would be too vague in release notes, add a short body summarizing the key changes.

## Tool Preferences
- For shell-based JSON parsing or filtering, prefer `jq`.
- For shell-based repository search, prefer `rg` over `grep`.
- For shell-based file discovery, prefer `fd` over `find`.
- For shell-based file previews, prefer `bat` over `cat`.
- When available, use `ast-grep` (`sg`) for structural code search using AST patterns (for example, matching function signatures or type definitions).
- When available, use `fzf` for interactive fuzzy file and symbol selection in the shell.
- Respect `.gitignore` in all search operations.
- Exclude build artifacts (`dist`, `build`, `node_modules`, `vendor`, `target`) by default.

### Tool Selection

| Task | Preferred | Instead of |
|------|-----------|------------|
| Code search | `rg` (ripgrep) | `grep`, `grep -r` |
| File discovery | `fd` | `find` |
| File preview | `bat` | `cat`, `head`, `tail` |
| JSON processing | `jq` | manual parsing, `python -c` |

### Search Rules

- Always respect `.gitignore` (rg and fd do this by default).
- Exclude build artifacts: `dist`, `build`, `node_modules`, `vendor`, `target`.
- Use glob filters to narrow scope before broad scans.
- Prefer exact match (`-w`) or fixed-string (`-F`) when searching for identifiers.

### Example Commands

- Search for an identifier: `rg -n -w "Task ID" .`
- List tracked files in a subtree: `fd . .ai/prompts`
- Preview a file with line numbers: `bat -n AGENTS.md`
<!-- agentinit:managed:end -->
