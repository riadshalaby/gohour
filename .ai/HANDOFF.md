# HANDOFF

Append-only role handoff log. Each role adds one entry when its step is complete.

## Entry Template

Each entry uses this exact structure. Omit fields marked as role-specific when they do not apply.

---

### <TASK_ID> — <ROLE> — <YYYY-MM-DDTHH:MM:SSZ>

| Field | Value |
|-------|-------|
| Agent | claude \| codex |
| Summary | One-sentence description of work done |
| Files Changed | Comma-separated list of changed files |
| Validation | Commands run and outcomes (implement only) |
| Commit | `<conventional commit message>` on `next_task`; `<hash> <message>` on `commit_task` (implement only) |
| Verdict | PASS \| PASS_WITH_NOTES \| FAIL (review only) |
| Blocking Findings | Numbered list or "none" (review only) |
| Next Role | plan \| implement \| review \| none |

---

### T-001..T-008 — plan — 2026-05-06T06:06:13Z

| Field | Value |
|-------|-------|
| Agent | claude |
| Summary | Refined `ROADMAP.md` (release-please + goreleaser for Fix 1; explicit billable, prefill+banner XHR, divergence-aware Update-Rule affordance, top-level Import button for Fix 2) and produced eight `ready_for_implement` tasks in `.ai/TASKS.md` with a step-by-step `.ai/PLAN.md`. |
| Files Changed | ROADMAP.md, .ai/PLAN.md, .ai/TASKS.md, .ai/HANDOFF.md |
| Next Role | implement |

---
