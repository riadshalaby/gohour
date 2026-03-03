# ROADMAP

## v0.2.2 (Completed)
- Import billable override and two-step import preview with per-row conflict selection.
- Submit CLI two-phase classification, detailed dry-run output, and pre-flight confirmation.
- Month/day reporting updates for worked vs billable visibility.
- Local overlap save warning flow (`X-Force-Overlap`) and month bulk actions.
- Remote month actions: copy from remote (skip already-local) and delete remote (skip locked days).
- Web submit dry-run preview actions on day/month pages.
- Day status badge model (`local`, `synced`, `conflict`, `remote`) with legend.
- Version command: `gohour version`.
- Submit update propagation for billable/comment edits on already-synced entries.

## Next Candidates
- Add server-side pagination/filtering for large month/day tables.
- Add audit logging for remote destructive operations (delete all remote, submit).
- Add e2e browser tests for import preview and submit preview flows.
- Add release pipeline that injects semantic version/build metadata automatically.
