# ROADMAP for v0.2.3

## Status
Completed and shipped on branch `feature/v0.2.3` (pending release tag `v0.2.3`).

## Release Goal
Improve reliability and usability of month/day workflows in the web UI, with a focus on remote data consistency, faster operator feedback, and cleaner release handling.

## Scope (Planned)

### P0 - Must Have
- Web UI: add explicit remote refresh action
  - Add a `Refresh remote` control in month and day views.
  - Force reload of remote worklogs/totals for the visible range.
  - Show last refresh timestamp in UI.

- Web UI: unify preview and submit into one flow
  - Remove separate `Preview submit` button.
  - Add `Dry run` option directly in submit dialog/action.
  - Reuse one result view for dry-run and real submit.

- Web UI: post-import reconcile for unsynced local entries only
  - Trigger reconcile automatically after web import.
  - Reconcile only local entries that are not already synced to remote.
  - Keep synced entries untouched.

- Web UI: compact month totals and delta display
  - Show delta next to remote worked and remote billable totals.
  - Remove extra delta column.
  - Color semantics: green = no delta, orange = delta present.

### P1 - Should Have
- Web UI actions menu cleanup
  - Move destructive/secondary actions into one menu.
  - Keep only primary navigation/actions as direct buttons (`Previous`, `Next`, `Submit month`).

- Audit log for remote write operations
  - Log `submit` and `delete all remote` operations locally with timestamp, range/day, and result summary.
  - Keep implementation lightweight (file-based log).

### P2 - Nice to Have
- Release automation improvements
  - Build distribution binaries with version metadata injection by default.
  - Generate checksums for release artifacts.
  - Add one release command/script for tag + notes + asset upload.

## Non-Goals for v0.2.3
- No server-side workaround for OnePoint internal aggregation/cache behavior.
- No broad redesign of existing page layout beyond action/menu cleanup.

## Acceptance Criteria
- Users can refresh remote data without page reload and see updated timestamp.
- Dry-run is available in submit flow without separate preview action.
- Web import auto-reconcile does not mutate entries already synced to remote.
- Month view displays deltas inline with remote totals and expected colors.
- Remote write actions produce auditable local log entries.
- README and Cobra help text are updated when CLI/UI behavior changes.

## Delivered Notes
- Added month/day remote refresh APIs and UI timestamps.
- Unified submit and preview into one submit dialog with dry-run toggle.
- Added targeted reconcile and unsynced-only post-import reconcile behavior.
- Added inline month deltas and month actions menu cleanup.
- Added file-based audit logging for submit and delete-remote operations.
- Added release automation scripts: cross-platform build with checksums and release helper.
