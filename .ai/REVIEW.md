# Review Log

Shared review log for the current cycle. Append a new task section when review starts for a new task. Within a task, append a new review round instead of replacing prior history.

## Task: T-001 — Adopt release-please for version + changelog management

### Review Round 1

Status: **complete**

Reviewed: 2026-05-06

#### Findings
- (nit) `cmd/version.go:9-12` — comment was tightened from the plan's draft (no longer mentions a "dev sentinel"). The new wording is actually more accurate because `Version` is now a `const` and there is no runtime fallback. Not a fix; documenting as a deliberate, beneficial deviation.
- (minor, out of scope for T-001) `README.md:103`, `AGENTS.md:47`, and `scripts/build-all.sh:26` still document/use the now-defunct `-ldflags "-X cmd.Version=…"` override. Verified that the override silently no-ops because `Version` is a `const`. This is explicitly deferred to T-003 ("Remove obsolete release scripts and refresh release docs"), which is `ready_for_implement`. Not a required fix on T-001; flagged so it is not lost.

#### Required Fixes
- None.

#### Verification
##### Steps
- `go fmt ./...` — clean (no output).
- `go vet ./...` — clean (no output).
- `go test ./...` — all packages PASS (cmd, config, importer, internal/timeutil, onepoint, output, reconcile, storage, submitter, web).
- `go test ./cmd/ -run TestVersionLiteralFormat -v` — PASS.
- `go test ./cmd/... -v -run "TestRootCommandSurfaceOnlyExposesServeAndVersion|TestVersion"` — both PASS.
- `jq empty release-please-config.json .release-please-manifest.json` — valid JSON.
- `ruby -ryaml -e "YAML.load_file('.github/workflows/release-please.yml')"` — valid YAML.
- `go build -o /tmp/gohour-local . && /tmp/gohour-local version` — printed `gohour 0.4.1` (literal consumed without `-ldflags`).
- E2E sanity: `go build -ldflags "-X github.com/riadshalaby/gohour/cmd.Version=v9.9.9" -o /tmp/gohour-ldflag . && /tmp/gohour-ldflag version` — printed `gohour 0.4.1`, confirming `const` switch makes the old override a no-op (intended per plan Step 3 notes).
- File inspection:
  - `cmd/version.go` — `const Version = "0.4.1" // x-release-please-version` ✓
  - `cmd/version_test.go` — semver regex test with `dev` early return ✓
  - `release-please-config.json` — `release-type: simple`, `extra-files` -> `cmd/version.go`, changelog sections per plan ✓
  - `.release-please-manifest.json` — seeded at `0.4.0` per plan ✓
  - `.github/workflows/release-please.yml` — triggers on push to `main`, uses `googleapis/release-please-action@v4` ✓
- Plan-vs-implementation diff review: all eight specified files created/modified; no unexpected changes outside scope.

##### Findings
- All acceptance criteria for T-001 satisfied.

##### Risks
- The seeded manifest version is `0.4.0` while `cmd/version.go` carries `0.4.1`. release-please reconciles this on first release-PR run by computing the next version from commits since `0.4.0` and writing `0.4.1` (or higher) into both. If the user expects no manual reconciliation, this is correct behavior; flagging for awareness.
- Until T-003 lands, `scripts/build-all.sh` still attempts `-X cmd.Version=…` which silently no-ops. Anyone running that script before T-003 ships will get a binary stamped with the source-tree literal regardless of the requested version. Mitigation: T-003 deletes the script outright.

#### Open Questions
- None.

#### Verdict
`PASS_WITH_NOTES`

---

## Task: T-002 — Add goreleaser workflow producing release artifacts

### Review Round 1

Status: **complete**

Reviewed: 2026-05-06

#### Findings
- (nit) `.goreleaser.yaml:60` — the inline comment `# release-please owns the release notes` that appeared in the plan's spec for `changelog.disable: true` was omitted. The behavior is identical without it; purely a documentation nit.

#### Required Fixes
- None.

#### Verification
##### Steps
- `ruby -e 'require "yaml"; ARGV.each { |path| YAML.load_file(path) }; puts "yaml ok"' .goreleaser.yaml .github/workflows/goreleaser.yml` — valid YAML on both files.
- `go fmt ./...` — clean (no output).
- `go vet ./...` — clean (no output).
- `go test ./...` — all packages PASS (no Go code changed; run for hygiene).
- `goreleaser check` — skipped (not installed locally; plan explicitly permits this).
- File inspection:
  - `.goreleaser.yaml` — `version: 2`, builds: linux/darwin/windows × amd64/arm64, `CGO_ENABLED=0`, `-trimpath`, `-s -w` ldflags, archive template with `x86_64` alias for amd64, windows zip override, `checksum.name_template: SHA256SUMS`, `release.mode: replace`, `changelog.disable: true` ✓
  - `.github/workflows/goreleaser.yml` — triggers on `v*` tag pushes, `fetch-depth: 0`, `go-version: "1.25"` (matches `go.mod`), `goreleaser-action@v6` with `version: "~> v2"`, `GITHUB_TOKEN` env var, `contents: write` permission ✓
  - `LICENSE` file exists — goreleaser `files: LICENSE*` will bundle it correctly ✓
- Plan-vs-implementation diff review: both specified files created; content matches plan specification exactly except for the nit above.

##### Findings
- All acceptance criteria for T-002 satisfied.

##### Risks
- `goreleaser check` was not run locally. The first real execution will be on a `v*` tag push. The config mirrors the goreleaser v2 schema closely and YAML is syntactically valid, so the risk is low.

#### Open Questions
- None.

#### Verdict
`PASS`

---

## Task: T-003 — Remove obsolete release scripts and refresh release docs

### Review Round 1

Status: **complete**

Reviewed: 2026-05-06

#### Findings
- (nit) `ROADMAP.md` was modified (references to `scripts/` and `-ldflags` replaced with generic language) but is not listed as a target file in the T-003 plan. The changes are correct and consistent with the task goal; flagging only because the plan did not call for it. No functional or accuracy concern.

#### Required Fixes
- None.

#### Verification
##### Steps
- `rg -n "scripts/build-all|scripts/release|build-all\.sh|release\.sh|-X github.com/riadshalaby/gohour/cmd.Version|ldflags" .` (excluding `.ai/PLAN.md`, `.ai/HANDOFF.md`, `.ai/REVIEW.md`) — no hits.
- `go fmt ./...` — clean.
- `go vet ./...` — clean.
- `go test ./...` — all packages PASS.
- `git status` — `scripts/build-all.sh` and `scripts/release.sh` both deleted (`D`); `README.md`, `AGENTS.md`, `ROADMAP.md` modified.
- File inspection:
  - `README.md` — `## Install` section present with `go install github.com/riadshalaby/gohour@latest` and prebuilt-binary alternative ✓; `## Build and Test` simplified to `go build ./... && go test ./...` with note about version literal ✓; `## Releasing` section added with release-please + goreleaser flow ✓; no references to `scripts/` or `-ldflags` ✓
  - `AGENTS.md` `Release Rules` — updated to describe release-please + goreleaser; manual tag + script references removed ✓
  - `ROADMAP.md` — references to `scripts/build-all.sh`, `scripts/release.sh`, and `-ldflags` replaced with generic language; no new scope introduced ✓
- Plan-vs-implementation diff review: all five plan-specified actions (delete scripts, add Install section, replace Build/Release sections, update AGENTS.md Release Rules) completed. One unspecified file (`ROADMAP.md`) modified with appropriate cleanup.

##### Findings
- All acceptance criteria for T-003 satisfied.

##### Risks
- None.

#### Open Questions
- None.

#### Verdict
`PASS`

---

## Task: T-004 — Add `GET /api/import/rule-match` endpoint

### Review Round 1

Status: **complete**

Reviewed: 2026-05-13

#### Findings

- (nit) `web/server.go:1466,1473` — `rulePayloadFromRule(matched)` is called twice when a match is found: once to build `selection` and again to build `matchedPayload`. Both produce identical struct values. Not a correctness issue; a local variable could eliminate the duplicate call. No fix required.
- (nit) Plan Step 4 referenced non-existent helpers `s.loadConfig()` and `s.lookupSnapshotResponse()`. The implementation correctly used `s.configSnapshot()` and the `s.loadLookupSnapshot()` + `lookupResponseFromSnapshot()` pair, which is the established pattern in `handleAPIImportPreview`. Beneficial deviation; documenting for traceability.

#### Required Fixes

None.

#### Verification

##### Steps

- `go test ./web/ -run TestServer_ImportRuleMatch -v` — all three new tests PASS (`TestServer_ImportRuleMatch_ReturnsMatchedRule`, `TestServer_ImportRuleMatch_NoMatchReturnsEmptySelection`, `TestServer_ImportRuleMatch_RequiresFilename`).
- `go fmt ./...` — clean (no output).
- `go vet ./...` — clean (no output).
- `go test ./...` — all packages PASS (cmd, config, importer, internal/timeutil, onepoint, output, reconcile, storage, submitter, web).
- Code inspection:
  - `importRuleMatchResponse` struct added near `importPreviewResponse` ✓
  - Route `GET /api/import/rule-match` registered in `NewServer` mux before `POST /api/import-preview` ✓
  - Handler uses `s.configSnapshot()` (thread-safe config read) ✓
  - `importer.MatchRuleByTemplate` called with `cfg.Rules`; returns empty `config.Rule{}` on no-match; `FileTemplate != ""` guard correctly distinguishes no-match from match ✓
  - Lookup failure handled silently: `lookup` initialized to `&lookupResponse{}`, populated only on success — matches plan intent ✓
  - Mapper list uses `importMapperNames()` helper (consistent with `handleAPIImportPreview`) ✓
  - Handler doc comment present at line 1456 ✓
  - No changes outside `web/server.go` and `web/server_test.go` ✓
- Plan-vs-implementation diff review: all acceptance criteria met; no scope creep.

##### Findings

- All acceptance criteria for T-004 satisfied.

##### Risks

- Lookup failures produce an empty `lookup` field rather than surfacing an error to the caller. This is intentional per the plan. Callers (future T-006 JS) should treat an empty lookup gracefully.

#### Verdict

`PASS`

---

## Task: T-005 — Remove the "Auto" billable option from import flow

### Review Round 1

Reviewed: 2026-05-13

#### Findings

- (nit) `web/server.go` — `importSelectionFromForm` now applies `strings.ToLower` before the switch, making the guard case-insensitive. The plan only mentioned `strings.TrimSpace`; the extra `ToLower` is a beneficial defensive addition. Not a required fix; documenting for traceability.
- (nit) `web/templates/base.html` — the original template had `<option value="auto">Auto</option>` as the *first* option (implicit default). Removing it and adding `selected` to the `billable` option is correct and matches the plan exactly.
- (nit) `TestServer_ImportPreview_NoRuleReturnsEmptySelection` — the plan noted this test might need a billable-assertion update. It does not assert on `Billable` at all, so no update was needed. The test continues to pass. Documenting for traceability.

#### Required Fixes

None.

#### Verification

##### Steps

- `go test ./web/ -run "TestServer_ImportPreview_DefaultsBillableTrueWhenNoRule|TestServer_ImportPreview_RejectsAutoBillable" -v` — both PASS.
- `go test ./web/ -run TestServer_ImportPreview -v` — all five ImportPreview tests PASS (including pre-existing ones; no regressions).
- `go fmt ./...` — clean (no output).
- `go vet ./...` — clean (no output).
- `go test ./...` — all 10 packages PASS.
- `(cd e2e && npm run test)` — 12/12 PASS, including `Import file flow` which now asserts `#month-import-billable option` count = 2.
- Template inspection:
  - `web/templates/month.html` — `auto` option removed; `billable` is the only `selected` default; selector has exactly 2 options ✓
  - `web/templates/base.html` — `auto` option removed; `billable` is the only `selected` default; selector has exactly 2 options ✓
- JS inspection:
  - `renderImportPreviewSelection`: 3-case if/else replaced with ternary `selection.billable === false ? 'non-billable' : 'billable'` ✓
  - `appendImportPreviewSelection`: fallback changed from `|| 'auto'` to `|| 'billable'` ✓
- Server inspection:
  - `importSelectionFromForm`: signature changed to `(rulePayload, error)`; `billable=auto` triggers `default` branch returning `fmt.Errorf`; empty billable with `nil` matched-rule defaults to `boolValuePtr(true)` ✓
  - `parseAndRunImportForm`: error is propagated; callers (`handleAPIImport`, `handleAPIImportPreview`) both surface it as HTTP 400 ✓
- `AGENTS.md` `Current Status` updated to reflect explicit-only billable choice ✓
- Residual `auto` scan: `rg -n "auto" web/templates/month.html web/templates/base.html web/static/js/app.js` — only match is a CSS `overflow:auto` style attribute; no billable-related `auto` references remain ✓
- Plan-vs-implementation diff: all seven plan-specified files modified; no scope creep.

##### Findings

- All acceptance criteria for T-005 satisfied.

##### Risks

- None.

#### Verdict

`PASS`

---

## Task: T-006 — Pre-fill import dialog from matched rule on file selection

### Review Round 1

Reviewed: 2026-05-13

#### Findings

- (nit) `web/static/js/app.js` — `applyImportFormSelection` falls back to the `_lookup` global when `payload.lookup` is absent. This follows the exact same pattern in `renderImportPreviewSelection`, so it is idiomatic for this codebase. Not a required fix.
- (nit) `populateImportSelects` refactored from inline cascade logic to `fillImportSelectionSelects`. The old code used `addEventListener('change', …)`; the new code uses `projectSelect.onchange = …` (assignment). Because `applyImportFormSelection` later calls `fillImportSelectionSelects` again on file-pick, the second call safely overwrites via `onchange =`. No correctness issue.
- (nit) The e2e test's unmatched-file path uses a `.txt` extension — clean, explicit way to exercise the no-match path. Deliberate design choice.

#### Required Fixes

None.

#### Verification

##### Steps

- `go fmt ./...` — clean (no output).
- `go vet ./...` — clean (no output).
- `go test ./...` — all 10 packages PASS.
- `(cd e2e && npm run test)` — **13/13 PASS**, including the new `Import dialog prefills fields and shows matched rule on file pick` test.
- Template inspection:
  - `web/templates/month.html` — `<p id="month-import-rule-match" class="muted import-rule-match" aria-live="polite">` inserted immediately after `<div class="dialog-body import-fields">` ✓; `name=` attributes match what `applyImportFormSelection` queries ✓
  - `web/templates/base.html` — no changes; `#preview-rule-match` element unchanged and still populated by `renderImportPreviewSelection` ✓
- JS inspection:
  - `openImportDialog`: clears banner before `showModal()` ✓; `change` listener on file input guarded by `data-prefillBound` to prevent duplicates ✓
  - `prefillImportFormFromFile`: XHR to `/api/import/rule-match?filename=…`, handles fetch errors gracefully, sets banner to `Matched rule: <name>` or `No rule matched — using defaults` ✓
  - `applyImportFormSelection`: applies mapper, project/activity/skill cascade via `fillImportSelectionSelects`, billable ✓
  - `fillImportOverrideSelects` delegates to `fillImportSelectionSelects` with `allowEmpty: true` — preview dialog retains empty option ✓
  - `populateImportSelects` delegates with `allowEmpty: false` — month dialog auto-selects first item, matches original behavior ✓
  - `fillImportSelectionSelect`: `allowEmpty` gate correctly handles placeholder and `selectedIndex = 0` fallback ✓
- `apiFetch` confirmed at line 162 — established helper ✓; `_lookup` global at line 8 — module-level cache, consistent with existing usage ✓
- e2e fixture (`e2e/fixtures/config.yaml`) — `generic-local` rule: `mapper: generic`, `file_template: "*.csv"`, `billable: false`, `project: P`, `activity: A`, `skill: S` — matches all new e2e assertions ✓
- `AGENTS.md` `Current Status` updated to mention prefill behavior ✓
- Plan-vs-implementation diff: all five plan-specified file groups touched; no scope creep. `fillImportSelectionSelects` / `fillImportSelectionSelect` are a clean generalization — exactly what the plan called for.

##### Findings

- All acceptance criteria for T-006 satisfied.

##### Risks

- `data-prefillBound` guard is per-element (survives dialog open/close cycles). On full page reload the DOM resets naturally. No risk for the current SPA pattern.

#### Verdict

`PASS`
