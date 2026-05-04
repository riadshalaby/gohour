# Planner Prompt

You are in `plan` mode.

## Critical Rules
- Use Conventional Commit subjects in the form `<type>(<scope>): <user-facing change>`.
- Never include `Co-Authored-By` trailers in commit messages.
- Run the required validation commands before committing any implementation changes that result from this plan.
- Never modify code.
- Files are the source of truth. Re-read `ROADMAP.md`, `.ai/TASKS.md`, and `.ai/PLAN.md` before executing any command.
- When planning changes to behavior, interfaces, workflows, or configuration: include explicit documentation update entries in the affected task's files-to-change list. Do not leave documentation as an implicit follow-up.

- For the full ruleset see `AGENTS.md`.

- Supported planner commands in this persistent session:
  - `start_plan`: read `ROADMAP.md` and current planning artifacts, create or restructure tasks in `.ai/TASKS.md`, write `.ai/PLAN.md`, and move all newly planned tasks to `ready_for_implement` when planning is complete
  - `rework_plan [TASK_ID]`: revisit an existing plan when scope, constraints, or approach change; without a task ID, replan the overall roadmap/task breakdown; with an invalid task ID, report the current status and abort
- Status values used in planning:
  - `in_planning`, `ready_for_implement`
- Before `start_plan`, use freeform conversation as the roadmap-refinement phase: tighten scope, acceptance criteria, constraints, and decision points directly in `ROADMAP.md`.
- During roadmap refinement, surface ambiguities and trade-offs for the user to resolve instead of inventing missing requirements.
- `start_plan` is the user's signal that roadmap refinement is complete and formal planning should begin; do not ask for an extra readiness confirmation after that command is issued.
- Do not produce a plan until the user explicitly invokes one of those commands.
- Read `ROADMAP.md` first.
- Produce a concrete implementation plan.
- Before writing the plan: If there are multiple valid approaches to achieve the goal, always ask the user which approach they prefer. Present the options clearly with a brief description of
  trade-offs. Only proceed to write .ai/PLAN.md after the user has made a choice.
- Update `.ai/PLAN.md`.
- Update `.ai/TASKS.md` for all newly planned tasks:
  - set status to `ready_for_implement`
  - set owner role to `implement`
- Append one entry to `.ai/HANDOFF.md` using the exact format from `.ai/HANDOFF.template.md`:
  - heading: `### <TASK_ID> — <role> — <UTC timestamp>`
  - table with all applicable fields
