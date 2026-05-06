# ROADMAP

Cycle: **0.4.1** — fix quirks in 0.4.0.

Versioning principle: the cycle version reflects user-facing changes only. Build, CI, and release tooling refactors do not drive version bumps; release-please will derive bumps from Conventional Commit types (`feat:`/`fix:` bump; `chore:`/`build:`/`ci:`/`docs:` do not).

## Fix 1 — Version + release tooling refactor (no user-facing version bump)

Problem: `gohour version` prints `dev` whenever the binary is built without `-ldflags` (i.e., any local `go build`, any `go install`). Release artifacts are built by hand-maintained shell scripts under `scripts/`. The cycle workflow already emits a `Release-As: x.y.z` footer, which is a release-please convention that we have not yet adopted.

Decision:
- Adopt **Google release-please** to manage releases:
  - release-please reads Conventional Commits on `main`, opens release PRs, manages the changelog, and writes the version into a tracked Go file so `go build` and `go install github.com/riadshalaby/gohour@latest` both report the correct version without `-ldflags`.
  - The existing `Release-As: x.y.z` footer in `aide cycle end` is honored by release-please as an explicit version override.
- Add a **goreleaser** GitHub Actions workflow triggered by release-please tags to produce the same matrix of binaries (`darwin/linux/windows` × `amd64/arm64`) plus `SHA256SUMS` that the local scripts produce today.
- Once release-please + goreleaser are in place, **delete `scripts/build-all.sh` and `scripts/release.sh`**. Removal is conditional on the CI replacement actually existing in the same cycle — not before.
- Replace the `cmd/version.go` `var Version = "dev"` mechanism with a value sourced from a release-please-managed file. The `-ldflags` override path may stay as a fallback or be removed; planner decides based on what release-please writes.
- Update `README.md`:
  - Add an **Install** section with `go install github.com/riadshalaby/gohour@latest` as the primary path.
  - Mention prebuilt binaries as the alternative; do not document the removed `scripts/` paths.
  - Update the **Build and Test** / release-tooling sections to describe the release-please + goreleaser flow instead of the deleted shell scripts.
- Update `AGENTS.md` **Release Rules** section to describe the release-please + goreleaser workflow and remove references to the deleted scripts.

Acceptance criteria:
- `go build ./... && ./gohour version` on a clean checkout of a tagged commit prints the tag (e.g., `gohour v0.4.1`), not `dev`.
- `go install github.com/riadshalaby/gohour@latest` followed by `gohour version` prints the installed module version.
- A release-please workflow exists in `.github/workflows/` and runs on pushes to `main`.
- A goreleaser workflow (or equivalent matrix workflow) exists in `.github/workflows/` and is triggered by release-please tags; it produces the same artifact set as the current scripts.
- `scripts/build-all.sh` and `scripts/release.sh` are removed.
- `README.md` documents `go install` as the primary install path and no longer references the deleted scripts.
- `AGENTS.md` Release Rules section reflects the new workflow.

Out of scope:
- Auto-generating release notes beyond what release-please writes (no manual changelog formatting).
- Signing release artifacts (cosign, SLSA) — separate cycle.

## Fix 2 — Import flow UX

Problem statements grouped by sub-fix.

### 2a — Pre-fill import form on file selection

Today the matched-rule data is wired through `importSelectionFromForm` server-side, but the file-pick import dialog (`#month-import-dialog` in `web/templates/month.html`) does not pre-fill the mapper / project / activity / skill / billable fields when the user picks a file. The user must submit before seeing what the rule chose.

Decision:
- When the user picks a file in the month-import dialog, trigger an XHR (or equivalent fetch) to a new endpoint that returns the matched rule for the chosen filename plus the resolved field values.
- The dialog updates **mapper**, project, activity, skill, and billable selectors immediately to reflect the matched rule. If no rule matches, fields keep whatever the user last set (no destructive reset) and billable defaults to `billable`.

Acceptance criteria:
- Picking a file whose name matches a configured rule pre-fills mapper, project, activity, skill, and billable in the import dialog before the user submits.
- Picking a file with no matching rule leaves the form untouched and shows the "no rule matched" indicator (see 2c).
- Re-picking a different file updates the form to the new match.

### 2b — Remove the "Auto" billable option from import UI

Today both `web/templates/month.html` (line 232) and `web/templates/base.html` (line 168) offer a billable selector with `Auto / Billable / Non-billable`. The "Auto" option is conceptually wrong for the import flow: the user (or the matched rule) must commit to billable or non-billable.

Decision:
- Remove the `Auto` option from both import dialogs. Allowed values: `billable`, `non-billable`.
- Server-side: remove the `auto`/empty-string code path in `importSelectionFromForm` and any companion logic. The matched rule's `Billable *bool` continues to set the dialog default; if the rule does not specify, default to `billable` (matches `Rule.IsBillable()` today).
- Worklog entry forms (`day.html`, `base.html` edit dialog) are unrelated and stay as-is.

Acceptance criteria:
- The import dialog billable selector contains only "Billable" and "Non-billable" — no third "Auto" option.
- Submitting an import always persists an explicit billable choice; no `auto` value reaches storage.
- The default selection on dialog open reflects the matched rule (or `billable` when no rule applies).

### 2c — Show which rule was matched on the import dialog

Today the file-pick import dialog (`#month-import-dialog` in `web/templates/month.html`) does not tell the user which rule (if any) was matched against the chosen file. The downstream preview dialog (`#import-preview-dialog` in `web/templates/base.html`) already has a `<p id="preview-rule-match">` placeholder — the planner should verify whether it is currently populated and ensure it is, but the primary user-visible change is on the file-pick dialog.

Decision:
- Add an inline indicator inside `#month-import-dialog` (top of the form body) that displays:
  - `Matched rule: <rule name>` when a rule matches the selected file, or
  - `No rule matched — using defaults` when no rule matches.
- The indicator updates as part of the same XHR triggered by file pick in 2a (no extra request).
- Verify the existing `#preview-rule-match` line in the preview dialog is wired and shows the same information; fix it if not. No new placeholder needed there.

Acceptance criteria:
- Picking a file in the import dialog displays the matched-rule indicator before the user clicks Upload.
- Picking a different file updates the indicator.
- Picking a file with no rule shows the "No rule matched" message.
- The preview dialog continues to display the same matched-rule information on its existing line.

### 2d — Make "update rule" affordance discoverable

Today `base.html` line 174-176 has an "Update matched rule" checkbox in the preview dialog. It is subtle: the user has to find and tick it. The roadmap calls out that the user should be able to update the rule when overriding properties — meaning the affordance should be obvious.

Decision:
- When a rule is matched and the user changes any field (project / activity / skill / billable) away from the matched rule's value, surface the "Update matched rule" affordance prominently (e.g., highlight the existing checkbox, or auto-check it with a clear inline note that the user can untick). Planner picks the exact UX, but the principle is: the override-vs-rule divergence must be visible and the persist option must be one click away.
- If no rule is matched, the affordance stays hidden (there is nothing to update).
- Add the same affordance to the month-import dialog if it supports overrides; if not, planner notes that explicitly.

Acceptance criteria:
- With a matched rule and no overrides, the rule-update affordance is hidden or disabled.
- The moment the user overrides any pre-filled field, the affordance becomes visible/active without further clicks.
- Submitting with the affordance enabled persists the override into the matching rule (existing `persistImportRuleUpdate` path).

### 2e — Promote Import to a top-level button on the month view

Today the "Import file" action lives inside the **Actions** dropdown at the top of the month view (`web/templates/month.html` line 60), grouped after the danger-zone separator. The sticky bar at the bottom already exposes it as a top-level button, but the top-of-page entry point is hidden behind a dropdown.

Decision:
- Move "Import file" out of the Actions dropdown and render it as a top-level button in the month-view header, alongside "Submit month".
- Remove the dropdown entry and its preceding separator if the separator is no longer needed.
- Keep the existing sticky-bar Import button as-is (already top-level).
- No change to the day view (no day-import flow exists today and we are not adding one in this cycle).

Acceptance criteria:
- The month view shows an "Import file" button at the top, visible without opening the Actions dropdown.
- The Actions dropdown no longer contains an "Import file" entry.
- Clicking the new top-level button opens the same `#month-import-dialog`.

## Documentation scope (cross-cutting)

Per `AGENTS.md` Documentation Rules, every behavior change in this cycle ships with the matching docs/comments update in the same commit:
- `README.md`: install section, build/release section, import workflow description.
- `AGENTS.md`: Release Rules; Current Status (import flow behavior).
- Cobra help text: `version` and `serve` commands if their behavior or flags change. (No flag changes expected, but planner verifies.)
- Inline code comments in `cmd/version.go`, `web/server.go` (import handlers), and the affected templates.

## Out of scope for 0.4.1
- Adding a third "automatic" billable mode (explicitly rejected — see 2b).
- Per-entry billable inference from comment/project content.
- Release artifact signing (cosign / SLSA).
- Any reconciliation, submit, or remote-API changes.
- A day-level import dialog (only the month-import dialog exists and stays the only one).
