# Review Log

Shared review log for the current cycle. Append a new task section when review starts for a new task. Within a task, append a new review round instead of replacing prior history.

## Task: T-XXX

### Review Round 1

Status: **pending**

Reviewed: YYYY-MM-DD

#### Findings
- Pending review.

#### Verification
##### Steps
- Pending verification.
##### Findings
- None.
##### Risks
- None.

#### Open Questions
- None.

#### Verdict
`PENDING`

---

## Task: T-004

### Review Round 1

Status: **complete**

Reviewed: 2026-05-05

#### Findings

- severity: `nit`
  - file: `web/server.go` — `importSelectionFromForm`
  - description: `firstNonEmptyString` is called with the form `mapper` value as first arg but the matched rule's mapper is already pre-populated in `selection` via `rulePayloadFromRule`. Then it's immediately overridden with the form value again as first arg, which means an explicitly submitted form value overrides the rule. This is the correct priority, but the logic could be simplified. No functional issue.
  - required fix: no

- severity: `nit`
  - file: `web/server.go` — `persistImportRuleUpdate`
  - description: If `updateRule=true` is sent but no rule matched (empty `FileTemplate`), the handler returns a 500 error. A 400 Bad Request would be more appropriate for a client-driven mismatch.
  - required fix: no

- severity: `nit`
  - file: `web/templates/month.html`
  - description: The new "Billable (force full duration)" option in the billable select was added. Previously `applyImportSelection` only handled `non-billable`; now it handles `billable` too via `boolValuePtr(true)`. The JS and handler are consistent. Minor note: the "auto" option still falls through to whatever the file provides, which is correct.
  - required fix: no

#### Verification
##### Steps
1. Re-read `.ai/PLAN.md` T-004 section and acceptance criteria.
2. Reviewed all diffs: `web/server.go`, `web/server_test.go`, `web/static/js/app.js`, `web/templates/month.html`.
3. Verified acceptance criteria:
   - Import preview auto-matches rules via `importer.MatchRuleByTemplate` — ✅
   - Preview returns `MatchedRule`, `Selection`, `Mappers`, `Lookup` — ✅
   - Pre-filled values from matched rule: mapper, project/ID, activity/ID, skill/ID, billable — ✅
   - User can override all fields via form values (priority: form > rule > default) — ✅
   - "Update rule" option (`updateRule=true`) persists overrides to YAML via `persistImportRuleUpdate` → `updateConfig` — ✅
   - No-match returns empty selection and no `MatchedRule` — ✅
   - All three mappers selectable (epm/generic/atwork) returned in `Mappers` field — ✅
   - Handler tests pass — ✅
4. Verified `lookupResponseFromSnapshot` was correctly extracted into a standalone function (no duplicate logic).
5. Verified `applyImportSelection` correctly handles billable/non-billable/auto with the new `boolValuePtr` helper.
6. Verified `persistImportRuleUpdate` safely guards against no-match (`FileTemplate == ""`) and uses the same `updateConfig` lock path as T-003.
7. Ran `go fmt ./...` → PASS
8. Ran `go vet ./...` → PASS
9. Ran `go test ./...` → all 11 packages PASS
10. Ran `npx playwright test` → 12/12 PASS (no regressions)

##### Findings
- All four required handler tests are present and pass: preview-with-match, preview-no-match, import-with-overrides, import-update-rule-persist.
- Round-trip test (`TestServer_Import_UpdateRulePersistsOverrides`) reads from disk after import to confirm YAML was updated — correct.
- JavaScript functions (`renderImportPreviewSelection`, `fillImportOverrideSelects`, `appendImportPreviewSelection`) wired to pre-fill and submit form overrides.

##### Risks
- None. All tests pass.

#### Open Questions
- None.

#### Verdict
`PASS`

---

## Task: T-003

### Review Round 1

Status: **complete**

Reviewed: 2026-05-05

#### Findings

- severity: `nit`
  - file: `web/server.go` — `configSnapshot()` / `cloneConfig()`
  - description: `cloneConfig` copies the `Rules` slice but `config.Rule` contains only value types and a `*bool` pointer for `Billable`. Callers who mutate `Billable` through the returned pointer would see a shared mutation. In practice this never happens in the current code paths, but a deep clone of `Billable` would be strictly safer. Not a practical risk today.
  - required fix: no

- severity: `nit`
  - file: `web/server.go` — `handleAPIRulePatch`
  - description: After a successful patch, the response looks up the rule by `rule.Name` (which may differ from the path param `name` if the client renamed the rule). This is correct, but it silently allows renaming via PATCH by sending a different `name` in the body. The plan doesn't prohibit this, but it isn't explicitly specified either. Acceptable as-is.
  - required fix: no

#### Verification
##### Steps
1. Re-read `.ai/PLAN.md` T-003 section and all acceptance criteria.
2. Reviewed all diffs: `web/server.go`, `web/server_test.go`, `web/templates/config.html`, `web/templates/base.html`, `web/static/css/components.css`, `web/static/js/app.js`.
3. Verified acceptance criteria:
   - `/config` page renders via `handleConfig` with `config.html` template — ✅
   - "Config" nav link added to `base.html` — ✅
   - OnePoint URL editable via `PATCH /api/config` with `validateOnePointURL` — ✅
   - Rules listable via `GET /api/rules` — ✅
   - Rules creatable via `POST /api/rules` → 201 Created — ✅
   - Rules editable via `PATCH /api/rules/{name}` — ✅
   - Rules deletable via `DELETE /api/rules/{name}` → 204 No Content — ✅
   - Mutations persist to YAML via `updateConfig()` → `config.WriteConfig()` — ✅
   - Thread safety via `configMu sync.RWMutex` — ✅
   - `configSnapshot()` used in import handler instead of `s.cfg` directly — ✅
4. Verified error handling: duplicate name → 409, not found → 404, bad request → 400, invalid URL → 400.
5. Verified round-trip test: `TestServer_ConfigAPIsPersistOnePointURLAndRuleCRUD` exercises PATCH config, create/update/list/delete rule, and reads from disk to confirm YAML persistence — matches plan requirement exactly.
6. Verified `TestServer_ConfigPageRendersCurrentConfigAndNav` confirms `/config` route returns 200 with the "Config" heading.
7. Ran `go fmt ./...` → PASS
8. Ran `go vet ./...` → PASS
9. Ran `go test ./...` → all 11 packages PASS
10. Ran `npx playwright test` → 12/12 PASS (no regressions from nav change)

##### Findings
- All acceptance criteria met.
- Implementation adds a clean `updateConfig(mutator)` abstraction that safely serializes mutations, persists to disk, and updates in-memory state under one lock.
- JavaScript functions (`openRuleDialog`, `editRuleRow`, `deleteRuleRow`, `handleConfigOnePointSubmit`, `handleRuleSubmit`) all present for full UI interaction.

##### Risks
- None. All tests pass including full Playwright regression suite.

#### Open Questions
- None.

#### Verdict
`PASS`

---

## Task: T-002

### Review Round 1

Status: **complete**

Reviewed: 2026-05-05

#### Findings

- severity: `nit`
  - file: `cmd/root.go`
  - description: `cobra.OnInitialize(initConfig)` was removed; `initConfig()` is now called directly inside `serveCmd.RunE`. This is a clean design improvement (config only loads when needed), but it's a quiet behavior change from the T-001 implementation. Not a problem since `version` correctly doesn't load config, and serve is the only command that uses it.
  - required fix: no

- severity: `nit`
  - file: `cmd/serve.go` Example block
  - description: The tab character inside the Example literal (`\t# Start on a custom port`) is inconsistent with the leading spaces in the first example line. Minor formatting nit.
  - required fix: no

#### Verification
##### Steps
1. Re-read `.ai/PLAN.md` T-002 section and acceptance criteria.
2. Inspected `git status --short` — confirmed 22 files deleted, 4 modified, 3 new (serve_auth_helpers.go, serve_browser_login.go, e2e/fixtures/config.yaml).
3. Verified new files `cmd/serve_auth_helpers.go` and `cmd/serve_browser_login.go` — correctly extract the auth/browser-login logic that was previously in the deleted `cmd/auth_helpers.go` / `cmd/auth_login.go`, now scoped to the serve command only.
4. Verified `cmd/` contains exactly: `root.go`, `serve.go`, `serve_auth_helpers.go`, `serve_browser_login.go`, `serve_e2e_stub.go`, `serve_test.go`, `version.go` — no leftover CLI command files.
5. Verified `e2e/run-server.sh` — no `--configFile`/`--db` flags, uses `GOHOUR_DATA_DIR`, copies `config.yaml`, no import CLI call.
6. Verified `e2e/global-setup.ts` — seeds via `POST /api/import` (server already started in Playwright 1.58 before globalSetup runs).
7. Verified `e2e/playwright.config.ts` — `GOHOUR_DATA_DIR` set, `gohour.db` naming consistent.
8. Verified `AGENTS.md` — implemented commands updated to `serve`, `version`.
9. Ran `go fmt ./...` → PASS
10. Ran `go vet ./...` → PASS
11. Ran `go build ./...` → PASS
12. Ran `go test ./...` → all 11 packages PASS
13. Ran `go test ./cmd/... -v` — `TestRootCommandSurfaceOnlyExposesServeAndVersion` PASS
14. Ran `PLAYWRIGHT_BROWSERS_PATH=/tmp/gohour-playwright-browsers npx playwright test` → 12/12 PASS in 6.1s

##### Findings
- All acceptance criteria met.
- Auth logic correctly extracted to serve-scoped files rather than deleted entirely — the serve command still needs browser login for OnePoint session management.
- E2E seeding approach (API-based) works correctly with Playwright 1.58 webServer-before-globalSetup ordering.

##### Risks
- None. All tests pass, including full E2E suite.

#### Open Questions
- None.

#### Verdict
`PASS`

---

## Task: T-001

### Review Round 1

Status: **complete**

Reviewed: 2026-05-04

#### Findings

- severity: `nit`
  - file: `cmd/root.go:26`
  - description: `var cfgFile string` is declared but no longer bound to a `--configFile` flag (flag was correctly removed). The variable is still referenced by `cmd/config_create.go`, `cmd/config_edit.go`, and `cmd/config_rule_add.go`, which are T-002 deletions. Not a bug, but the variable becomes fully dead code after T-002.
  - required fix: no (T-002 will eliminate it)

- severity: `nit`
  - file: `cmd/root.go:29-59`
  - description: `rootCmd.Long` and `rootCmd.Example` still reference removed commands (`import`, `reconcile`, `submit`, `export`). Out of T-001 scope; T-002 cleanup or T-006 docs rewrite will address.
  - required fix: no

- severity: `nit`
  - file: `web/server.go` (`handleAPIImport`)
  - description: `reconcileWarning := ""` is always empty now that the auto-reconcile block was removed. Variable is used downstream in the JSON response so this is fine, and T-005 will repopulate it for EPM imports.
  - required fix: no

#### Verification
##### Steps
1. Read plan (`PLAN.md`) T-001 section and acceptance criteria.
2. Read all diff hunks for changed files via `git diff HEAD`.
3. Confirmed all acceptance criteria met:
   - `DataDir()`, `ConfigPath()`, `DBPath()`, `AuthStatePath()` functions present and using `GOHOUR_DATA_DIR` env override
   - `WriteConfig()` exported, marshals to YAML with `yaml:` tags, respects `DataDir()`
   - `ImportConfig` struct and `auto_reconcile_after_import` fully removed from `Config`, `SetDefaults`, `ExampleYAML`, `setDefaults`
   - `config/migration.go` implements the 5 required scenarios (no-op, CWD move, CWD fresh, home config, nothing found)
   - `cmd/root.go` calls `config.RunMigration()`, reads from `config.ConfigPath()`
   - `cmd/serve.go` removes all 5 flags (`--db`, `--url`, `--state-file`, `--from`, `--to`), uses `config.DBPath()`
   - `cmd/serve_e2e_stub.go` uses `cfg.OnePoint.URL` and `config.AuthStatePath()` directly
   - `web/audit.go` moves audit log to `config.DataDir()`
   - `web/server.go` auto-reconcile block correctly removed
   - `cmd/auth_helpers.go` uses `config.AuthStatePath()` and `config.DataDir()` instead of constructing paths manually
4. Ran `go fmt ./...` → PASS
5. Ran `go vet ./...` → PASS
6. Ran `go test ./...` → all packages PASS (11 packages, no failures)
7. Grepped for any remaining references to `AutoReconcileAfterImport`, `ImportConfig`, `auto_reconcile_after_import` → only in test assertion string (correct)
8. Verified removed flags not present in any non-test Go source files

##### Findings
- All acceptance criteria met.
- Test coverage is complete: `TestFixedPathsUseConfiguredDataDir`, `TestWriteConfigRoundTrip`, and 5 migration scenario tests covering the full plan requirements.
- The `GOHOUR_DATA_DIR` env override enables clean testing without touching real home directories.

##### Risks
- `initConfig` now calls `cobra.CheckErr` on read failure (previously printed a message and continued). Since `RunMigration` always creates a default config, this is safe in practice. Edge case: if home dir is read-only and migration fails, CLI exits with a hard error — which is the correct behavior.

#### Open Questions
- None.

#### Verdict
`PASS`
