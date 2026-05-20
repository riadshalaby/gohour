# Dev Prompt

You are in `dev` mode.

## Critical Rules
- Use Conventional Commit subjects in the form `<type>(<scope>): <user-facing change>`.
- Never include `Co-Authored-By` trailers in commit messages.
- Re-read `.ai/TASKS.md` before every command.
- Files are the source of truth. Re-read `.ai/PLAN.md` before executing any command. Re-read `.ai/REVIEW.md` before `rework_task`.
- During the review hat, if a test fails you may either change implementation code to make it pass, or add new assertions. You must not weaken or delete existing assertions to make a test pass. If you believe a test is genuinely wrong for the new behavior, halt to `changes_requested`, do not retry, and write your proposed test change to `REVIEW.md` for human approval — even if you have retries remaining.

- For the full ruleset see `AGENTS.md`.

- Review hat validation commands:
  - Run `go fmt ./...`.
  - Run `go vet ./...`.
  - Run `go test ./...`.
  - Run the project's e2e command too if the project documentation declares one.

- Mechanical failures vs semantic concerns:
  - Mechanical failures are formatter diffs, vet warnings, lint issues, or test failures with a clear local code fix.
  - Retry only mechanical failures, up to 3 attempts per failing validation, and log each retry in `.ai/REVIEW.md`.
  - Semantic concerns are acceptance-criteria gaps, incorrect behavior, wrong tests, or design-level issues. Halt immediately to `changes_requested` without retries.

- Dev session loop per task:
  - Implement hat: write or update tests for the changed behavior first, implement the change, and update docs or code comments touched by the behavior change.
  - Review hat: switch status to `in_review`, run the required validations, re-read the diff against `.ai/PLAN.md`, and write findings plus validation results to `.ai/REVIEW.md`.
  - If all validations pass and there is no semantic concern, halt at `ready_for_review` for the human gate.

- Supported dev commands in this persistent session:
  - `next_task [TASK_ID]`: pick one `ready_for_implement` task (or the supplied task), move it to `in_implementation`, run the implement hat, run the review hat, and halt at `ready_for_review` for the human.
  - `all_task`: process every remaining non-`done` task without per-task human review, committing each completed task before moving to the next one. Halt only if a semantic concern appears or the 3-attempt mechanical retry cap is exhausted.
  - `rework_task [TASK_ID] [feedback]`: resume a task from `ready_for_review` or `changes_requested`, apply the feedback when provided, move it to `in_implementation`, and run the implement/review loop again.
  - `commit_task [TASK_ID]`: valid only when the task is `ready_for_review`; this is the human approval step. Move the task through `ready_to_commit` to `done`, then run `git add -A && git commit -m "<message>"` using the commit message already written in `.ai/HANDOFF.md`.
  - `status_cycle [TASK_ID]`: report deterministic task status, current owner, and the next recommended action.

- `all_task` commit policy:
  - Each completed task in `all_task` ends with a real `git add -A && git commit -m "<message>"`.
  - The commit message must already be drafted in the task's HANDOFF entry before the commit runs.
  - No batching, no squashing — one commit per task.

- At the `ready_for_review` halt, print this exact summary block:

```text
READY_FOR_REVIEW
Task: <TASK_ID>
Summary: <one-line summary>
Validation:
- go fmt ./...: PASS|FAIL
- go vet ./...: PASS|FAIL
- go test ./...: PASS|FAIL
- e2e: PASS|FAIL|NOT_CONFIGURED
Valid next commands:
- commit_task [TASK_ID]
- rework_task [TASK_ID] [feedback]
```
