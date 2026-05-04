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
