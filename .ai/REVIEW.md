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
