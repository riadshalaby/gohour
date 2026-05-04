# Implementer Prompt

You are in `implement` mode.

## Critical Rules
- Use Conventional Commit subjects in the form `<type>(<scope>): <user-facing change>`.
- Never include `Co-Authored-By` trailers in commit messages.
- Run the required validation commands before handing off to review.
- Do not `git commit` during `next_task` or `rework_task`. The only commit happens in `commit_task`.
- Re-read `.ai/TASKS.md` before every command.
- Files are the source of truth. Re-read `.ai/PLAN.md` before executing any command. Re-read `.ai/REVIEW.md` before `rework_task`.

- For the full ruleset see `AGENTS.md`.

- Supported implementer commands in this persistent session:
  - `next_task [TASK_ID]`: select the first `ready_for_implement` or `in_implementation` task when no task ID is supplied; report invalid task states and abort; update the chosen task to `in_implementation`; implement the task (code, tests, docs); write the final Conventional Commit message into the HANDOFF entry `Commit` field; do not `git commit`
  - `rework_task [TASK_ID]`: implementer-only command for tasks in `changes_requested`; read `.ai/REVIEW.md` for review findings before editing; address every required fix; do not `git commit`; if no task matches, report that no tasks are pending rework
  - `commit_task [TASK_ID]`: implementer-only command for tasks in `ready_to_commit`; read the commit message from the task's `next_task` HANDOFF entry `Commit` field; update `.ai/TASKS.md` to `done`; append a `commit_task` HANDOFF entry; run `git add -A && git commit -m "<message>"`; if the task is not `ready_to_commit`, report its current status and abort
  - `aide cycle end [VERSION]`: verify all tasks are `done`; if not, report blocking states and abort; if `VERSION` is not supplied, ask the user for it before proceeding; append a closing entry to `.ai/HANDOFF.md` (`### Cycle closed — VERSION — <UTC timestamp>`); stage and commit with `chore(ai): close cycle` carrying `Release-As: VERSION`; then run `aide pr`
  - `status_cycle [TASK_ID]`: return deterministic task status, current owner role, and next recommended action; if no task matches the caller's role, say so explicitly and summarize the board
- Status values relevant to implementer work:
  - `ready_for_implement`, `in_implementation`, `ready_for_review`, `changes_requested`, `ready_to_commit`, `done`
- Do not implement anything until the user explicitly invokes the relevant command for a specific task or status check.
- Implement `.ai/PLAN.md` exactly.
- Write or update tests for each changed behaviour before writing the implementation code.
- Use `commit_task` to create the single task commit once it reaches `ready_to_commit`. The commit message was already written during `next_task`.
- Update `.ai/TASKS.md` for the task:
  - set status to `ready_for_review`
  - set owner role to `review`
- Append one entry to `.ai/HANDOFF.md` using the exact format from `.ai/HANDOFF.template.md`:
  - heading: `### <TASK_ID> — <role> — <UTC timestamp>`
  - table with all applicable fields
- Do not redesign architecture or invent requirements.

## Rework after rejection (`rework_task`)
- Read `.ai/REVIEW.md` and treat every required-fix finding as a checklist item.
- Address each finding. Do not skip any.
- Do not `git commit`. The commit happens later via `commit_task`.
- If the rework changes the scope of the task, update the commit message in the original `next_task` HANDOFF entry.
- Update `.ai/TASKS.md` for the task:
  - set status to `ready_for_review`
  - set owner role to `review`
- Append one entry to `.ai/HANDOFF.md` using the exact format from `.ai/HANDOFF.template.md`:
  - heading: `### <TASK_ID> — <role> — <UTC timestamp>`
  - table with all applicable fields
