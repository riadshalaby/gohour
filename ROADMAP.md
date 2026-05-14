# ROADMAP

Cycle: **0.5.0** (minor release — storage schema change requires a `feat!` commit so release-please proposes the minor bump).

Goal: make local worklog classification correct and trustworthy by promoting OnePoint project / activity / skill IDs to first-class identity on local entries, and ship the queued UI fixes.

Operational constraints for the cycle:
- The release version is pinned via the `Release-As: 0.5.0` footer that `aide cycle end` emits.
- Rename the working branch `feature/0.4.2 → feature/0.5.0` at the start of the implementation cycle (housekeeping; release-please does not read the branch name).
- gohour does **not** support offline imports. Imports auto-refresh the OnePoint catalog before running.
- Documentation updates (`README.md`, `AGENTS.md`, Cobra help where applicable) are in-cycle scope.

Validation gate: `go fmt ./...`, `go vet ./...`, `go test ./...` pass before any commit.

---

## P1 — Strict catalog-backed worklog classification

Type: `feat!` (storage schema change; user-visible classification + import semantics change).

### Problem

`classifyLocalEntry` (`web/data.go:200`) and `localEntryIsSynced` (`web/server.go:2062`) currently compare local vs. remote entries by time range only. Local entries that share a start/end with an unrelated remote entry are flagged `synced`, and entries that genuinely match remote work cannot be distinguished from entries that merely fall in the same slot. The local schema stores project / activity / skill as **names** while OnePoint stores them as **IDs**, so the existing classifier cannot do anything sharper.

### Design summary

Promote project / activity / skill **IDs** to first-class fields on local worklogs and on import rules. Resolve names against the OnePoint catalog at import time and on every local CRUD save; refuse to persist unresolved names. Use the IDs in the classifier and in the post-import reconciliation gate.

### Components

1. **`internal/catalog` (new package).**
   In-memory store, persisted to SQLite cache. API: `Resolve(name) (ID, bool)`, `Lookup(ID) (name, bool)`, `Refresh(ctx)` (calls OnePoint). Assumes OnePoint catalog IDs are stable across refreshes (the submitter already depends on this).

2. **Storage migration.**
   Add nullable `project_id`, `activity_id`, `skill_id` columns to the worklog table and the rules table. Add a `catalog_backfill_complete` flag in a metadata table. "Unresolved" is derived, not stored: `name != ''` AND `id IS NULL`.

3. **Import path.**
   On file pick: build preview from cached rules and cached catalog (no network).
   On import submit, in order:
   1. Call `catalog.Refresh()`. If the call fails (network, auth), the import aborts with a clear error before touching local data.
   2. Resolve project / activity / skill on every entry (rule-matched values plus any user overrides on the preview) against the fresh catalog.
   3. If any entry has at least one unresolved name, the import aborts and the preview lists the offending rows and the unresolved values. No partial writes.
   4. On success, persist both names and resolved IDs.

4. **Rules CRUD (`/config`).**
   Project / activity / skill inputs become catalog-backed combobox controls (text-search, names shown to the user, IDs stored). A rule whose stored ID becomes NULL (catalog entry deleted in OnePoint) shows an inline "unresolved" marker. Such a rule is **skipped during import matching** until fixed. Other rules continue to apply.

5. **Day-view worklog editor.**
   Same combobox controls for project / activity / skill. Save is rejected if any of the three doesn't resolve. Symmetric with the strict import rule.

6. **Classifier (replaces `classifyLocalEntry` in `web/data.go`).**
   New badge truth table:
   - `unresolved` — entry has any non-empty name with NULL id. Preempts every other classification.
   - `synced` — equal `Start`, `End`, `Duration`, `ProjectID`, `ActivityID`, `SkillID`, `Billable` with some remote entry. `Comment` differences do **not** demote `synced` (consistent with the existing submit invariant where comment-only differences mark an update candidate).
   - `conflict` — overlaps a remote entry (exact or partial time overlap) and not `synced`.
   - `local` — no remote entry overlaps the time slot.

7. **`localEntryIsSynced` (`web/server.go:2062`).**
   Replaced by the same rule used in the classifier (used by post-import auto-reconcile to skip already-synced entries). Reconciliation eligibility expands — verify no regressions in existing reconcile tests.

8. **Submitter.**
   Reads stored IDs directly; no resolution at submit time. Refuses to submit `unresolved` entries with a clear error. Dry-run preview surfaces `unresolved` as a fourth outcome alongside `locked / duplicate / overlap`. Other entries on the same day submit normally.

9. **Legacy backfill.**
   Runs lazily on the first successful catalog refresh after upgrade (so an offline first-launch does not block). Walks worklog and rules tables, writes IDs where the catalog matches, leaves NULL where it does not, then sets `catalog_backfill_complete = true`. The month view also exposes an explicit "Refresh catalog" action so users who do not import a given month still trigger the backfill.

10. **Display side-fix.**
    Remote rows currently render project / activity / skill as raw numeric IDs (`web/data.go:144`). Replace with names via the catalog. When the stored local name differs from the current catalog name for the same ID, show the catalog name with the stored name as a tooltip.

### Acceptance criteria

- New schema migration runs cleanly on (a) a fresh database and (b) a database upgraded from 0.4.1 with existing worklogs and rules. No data loss.
- Backfill, after a successful catalog refresh, populates IDs for every worklog and rule whose name matches a catalog entry, and leaves the rest with NULL IDs and an unresolved marker in the UI.
- Importing a file whose entries all resolve against the catalog persists IDs alongside names and succeeds.
- Importing a file containing at least one unresolved name aborts with no DB writes and surfaces the unresolved names on the preview.
- A network failure during the pre-import catalog refresh aborts the import with a clear error and no DB writes.
- Day-view worklog create / update with an unresolved project / activity / skill name returns a validation error.
- A rule with an unresolved stored ID is skipped during import matching; resolved rules continue to match.
- Day-view badges follow the new truth table for all four states, with `unresolved` preempting the others.
- `localEntryIsSynced` uses the new rule; the post-import auto-reconcile no longer skips entries that look time-equal but differ in project / activity / skill / billable / duration.
- Submit refuses entries in the `unresolved` state; other entries on the same day submit normally. Dry-run preview reports `unresolved` outcomes.
- Remote rows in the day view show project / activity / skill names instead of raw numeric IDs; name-drift between stored and catalog names is shown via tooltip.

### Out of scope (for 0.5.0)

- Auto-renaming local entries when an OnePoint project is renamed (only the soft tooltip signal).
- CLI-side catalog management commands. The `serve` UI is the only catalog surface.
- Bulk "fix all unresolved" wizard. Users fix unresolved entries / rules one at a time via the existing edit forms.

### Documentation deltas (in-cycle)

- `README.md` — import behaviour (auto catalog refresh, strict resolution failure), unresolved state and how to clear it.
- `AGENTS.md` "Submit Command Invariants" — add: submit refuses unresolved entries.
- `AGENTS.md` — add a "Classification Rules" section codifying the new badge truth table.
- Cobra command help — audit `serve` `Long` / `Example` / flag descriptions for any references that change; no new CLI flags expected.
- Release notes — driven by release-please; the migration commit must be `feat!` so 0.5.0 lands as a minor bump.

---

## P2 — Config page close button

Type: `feat`.

### Scope

Add a close control to `/config` that returns the user to the compare view they came from. Capture the previous compare-view URL (`/month/...` or `/day/...`) in `sessionStorage` when navigating to `/config`. The close button reads it. Fallback when no entry exists (deep link, refresh, fresh tab): month root for the current month.

### Acceptance criteria

- Navigating from a month view to `/config` and pressing close returns to that month view.
- Navigating from a day view to `/config` and pressing close returns to that day view.
- Opening `/config` directly (no prior compare view in the session) and pressing close lands on the current month's month view.

---

## P3 — Inline month picker on month view

Type: `feat`.

### Scope

Remove the standalone header row containing the month input and Go button. Re-layout the month view's existing nav row to `← [YYYY-MM input] [Go] →`. Left / right arrows pre-fill the inline input with the navigated month; pressing Enter in the input is equivalent to clicking Go. Day view navigation is **unchanged**.

### Acceptance criteria

- The header-level month entry field and Go button are removed.
- The month view's nav row contains, in order: left arrow, month input, Go button, right arrow.
- Clicking left / right navigates to the previous / next month and updates the input value to match the displayed month.
- Submitting the input via Go button or Enter key navigates to the entered month.
- Day view navigation continues to work as it does today.

---

## P4 — Billable dropdown text cleanup

Type: `chore`.

### Scope

Drop the parenthetical labels `(force full duration)` and `(force 0)` from the billable dropdown options in `web/templates/month.html:232-233`. Underlying behaviour of `billable` and `non-billable` (full-duration vs. zero forcing) is unchanged — this change is text only. Audit other templates for the same labels and remove them consistently.

### Acceptance criteria

- The billable dropdown in the import preview shows `Billable` and `Non-billable` without the parenthetical labels.
- Selecting `Billable` continues to force the full entry duration; selecting `Non-billable` continues to force 0 minutes.
- No other template surfaces the removed text.

---

## P5a — "Update matched rule" notice spans full popup width

Type: `fix`.

### Scope

The notice text "Update matched rule with these overrides — You changed values from the matched rule. Untick to import without saving." is currently constrained to the width of the Mapper field on the preview popup. Make it span the full popup content width.

### Acceptance criteria

- The notice renders on one or more lines that span the full preview popup content width at all viewport widths supported by the existing UI.
- No regression in the popup's other field widths or alignment.

---

## P5b — "Update matched rule" defaults to off

Type: `feat`.

### Scope

The "Update matched rule" checkbox on the preview popup defaults to unchecked. No memory of the user's previous choice — the checkbox is always off on every open.

### Acceptance criteria

- Opening the preview popup after overriding any rule-derived field shows the "Update matched rule" checkbox unchecked.
- Importing without ticking the checkbox does not modify the matched rule.
- Importing with the checkbox ticked updates the matched rule with the overridden values (existing behaviour).

---

## Task partition (final shape decided at `start_plan`)

Rough cut, ~13 tasks:

- T-01 — `internal/catalog` package + OnePoint catalog fetch wiring (`feat`).
- T-02 — Schema migration: nullable IDs on worklogs + rules, backfill flag (`feat!`).
- T-03 — Lazy legacy backfill on first successful catalog refresh (`feat`).
- T-04 — Strict import resolution with pre-flight catalog refresh (`feat!`).
- T-05 — Rules CRUD catalog-backed combobox controls (`feat`).
- T-06 — Day-view worklog editor catalog-backed combobox controls (`feat`).
- T-07 — Classifier rewrite + `unresolved` badge + `localEntryIsSynced` parity (`fix`/`feat`).
- T-08 — Submitter refuses unresolved entries; dry-run reports `unresolved` outcome (`feat`).
- T-09 — Remote rows display names instead of raw IDs; name-drift tooltip (`fix`).
- T-10 — `/config` close button with sessionStorage return (`feat`).
- T-11 — Inline month picker layout (`feat`).
- T-12 — Billable dropdown text cleanup (`chore`).
- T-13 — P5a "Update matched rule" notice spans full popup width (`fix`).
- T-14 — P5b "Update matched rule" checkbox defaults to off (`feat`).
- T-15 — Documentation pass (`docs`).
