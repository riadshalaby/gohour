# Plan

Status: **active**

Goal: Simplify gohour to a web-UI-first tool with minimal CLI surface (`serve` + `version`), consolidated file paths under `~/.gohour/`, and rule-aware import in the web UI.

## Scope
- Consolidate all data files (config, DB, auth state) under `~/.gohour/`
- Implement migration logic for existing users
- Remove CLI commands: `auth`, `config`, `delete`, `export`, `import`, `reconcile`, `submit`
- Add web UI config management page with rule CRUD
- Enhance web UI import to auto-match rules with manual override
- Make reconcile unconditional after EPM imports
- Rewrite README and Cobra help text

## Acceptance Criteria
- `gohour serve` and `gohour version` are the only CLI commands
- Config at `~/.gohour/config.yaml`, DB at `~/.gohour/gohour.db`, auth at `~/.gohour/onepoint-auth-state.json`
- No `--db`, `--configFile`, `--url`, `--state-file`, `--from`, `--to` flags remain
- `--port` and `--no-open` flags remain on `serve`
- Migration prompt works: detects old files in CWD then `$HOME`, offers move-or-fresh
- Web UI has a config page for OnePoint URL and rule CRUD
- Web UI import auto-matches rules, shows editable preview, supports rule update
- Reconcile runs automatically after every EPM import (no config toggle)
- README reflects the new workflow
- `go fmt`, `go vet`, `go test` all pass

## Implementation Phases

### Phase 1: Foundation — file consolidation, migration, CLI cleanup (T-001, T-002)

#### T-001: Consolidate file paths under `~/.gohour/` with migration

**What changes:**
- `config/config.go`: New exported function `DataDir() string` returning `~/.gohour/`, `ConfigPath() string`, `DBPath() string`, `AuthStatePath() string`. Remove config search path logic.
- `config/config.go`: Remove `auto_reconcile_after_import` field from `ImportConfig` struct.
- `config/config.go`: Update `SetDefaults()` to drop the reconcile flag default.
- `config/config.go`: Add `WriteConfig(cfg *Config) error` to write config back to `~/.gohour/config.yaml` (needed for web UI config edits and rule updates).
- `config/migration.go` (new): `RunMigration()` — implements the detection + prompt logic:
  1. If `~/.gohour/config.yaml` exists → return (no-op).
  2. Check CWD for `.gohour.yaml` / `gohour.db`, then `$HOME/.gohour.yaml`.
  3. If found → prompt user: move or fresh.
  4. Move: copy to `~/.gohour/`, rename originals to `.bak`.
  5. Fresh / nothing found: create dir, write default config, empty DB created on first open.
- `cmd/root.go`: Replace `initConfig()` — call `config.RunMigration()`, then load config from fixed path `~/.gohour/config.yaml`. Remove `--configFile` persistent flag.
- `cmd/serve.go`: Remove `--db`, `--url`, `--state-file`, `--from`, `--to` flags. Use `config.DBPath()` and `config.AuthStatePath()` directly. Keep `--port` and `--no-open`.
- `web/server.go`: Update `NewServer` and any handler that reads `--url` or state-file to use config/fixed paths instead.

**Files to change:**
- `config/config.go` — add path functions, `WriteConfig`, remove reconcile flag
- `config/migration.go` — new file, migration logic
- `config/migration_test.go` — new file, test migration scenarios
- `cmd/root.go` — rewrite `initConfig`, remove `--configFile` flag
- `cmd/serve.go` — remove flags, use fixed paths
- `web/server.go` — use config paths instead of flag-passed values

**Tests:**
- Unit tests for `DataDir`, `ConfigPath`, `DBPath`, `AuthStatePath`
- Unit tests for `RunMigration` covering: existing new config (no-op), old files found (move), old files found (fresh), nothing found (fresh default)
- Unit test for `WriteConfig` round-trip

#### T-002: Remove CLI commands

**What changes:**
- Delete command files: `cmd/auth.go`, `cmd/auth_login.go`, `cmd/auth_show_cookies.go`, `cmd/auth_helpers.go`, `cmd/config.go`, `cmd/config_create.go`, `cmd/config_show.go`, `cmd/config_edit.go`, `cmd/config_delete_command.go`, `cmd/config_rule.go`, `cmd/config_rule_add.go`, `cmd/delete.go`, `cmd/export.go`, `cmd/import.go`, `cmd/reconcile.go`, `cmd/submit.go`
- Delete test files for removed commands: `cmd/auth_helpers_test.go`, `cmd/auth_login_test.go`, `cmd/import_test.go`, `cmd/submit_test.go`
- `cmd/root.go`: Remove all `AddCommand` calls for deleted commands. Clean up any imports only used by those commands.
- Update `cmd/serve.go` Cobra metadata: `Short`, `Long`, `Example` to reflect it being the primary command.
- Update `cmd/version.go` if needed.
- Update `cmd/serve_e2e_stub.go`: adapt to new fixed paths (config, DB, auth state from `config.*Path()` functions instead of CLI flags).
- Update `cmd/serve_test.go`: remove tests for deleted flag parsing (e.g. month bounds from `--from`/`--to`), update E2E stub tests to use new config path resolution, ensure remaining tests pass with the new path setup.
- Update Playwright E2E infrastructure:
  - `e2e/run-server.sh`: rewrite to stop using `--configFile`, `--db`, and the `gohour import` CLI command. Instead: set `GOHOUR_DATA_DIR` (or symlink `~/.gohour/`) to the runtime temp dir so `gohour serve` picks up the test config/DB from there. Seed data via the web API (`/api/import`) or by pre-placing the SQLite DB file instead of running the CLI import command.
  - `e2e/fixtures/gohour-test.yaml`: remove `auto_reconcile_after_import` field (no longer exists). Rename to `config.yaml` to match the new expected filename.
  - `e2e/playwright.config.ts`: update env vars if the seeding approach changes.
  - `e2e/global-setup.ts`: update if seeding logic moves here.
  - `e2e/tests/import.spec.ts`: review and update for any changed import UI behavior (rule matching UI changes come in T-004, but the basic flow must still work).
  - `e2e/tests/day.spec.ts`, `month.spec.ts`, `submit.spec.ts`: verify they still pass with the new server startup; update selectors/flows if the nav or page structure changes.

**Files to change:**
- Delete 16 `cmd/*.go` command files listed above
- Delete 4 `cmd/*_test.go` files: `auth_helpers_test.go`, `auth_login_test.go`, `import_test.go`, `submit_test.go`
- `cmd/root.go` — remove AddCommand registrations, clean imports
- `cmd/serve.go` — update Cobra help text
- `cmd/serve_e2e_stub.go` — adapt to fixed config paths
- `cmd/serve_test.go` — update for removed flags and new path resolution
- `cmd/version.go` — review, update if needed
- `e2e/run-server.sh` — rewrite for new CLI surface (no import cmd, no --configFile/--db flags)
- `e2e/fixtures/gohour-test.yaml` — remove `auto_reconcile_after_import`, rename to `config.yaml`
- `e2e/playwright.config.ts` — update env vars if needed
- `e2e/global-setup.ts` — update if seeding logic changes
- `e2e/tests/*.spec.ts` — verify/update all 4 spec files

**Tests:**
- `go build ./...` succeeds (no dangling references)
- `go vet ./...` clean
- `go test ./cmd/...` passes with updated serve tests
- `npx playwright test` passes in `e2e/` directory

### Phase 2: Web UI features (T-003, T-004, T-005)

#### T-003: Web UI config management page

**What changes:**
- New config page at `/config` with sections for:
  - OnePoint URL (text input, save button)
  - Import rules table (list all rules with name, mapper, file template, project, activity, skill, billable)
  - Add rule form (with OnePoint lookup for project/activity/skill selection)
  - Edit rule (inline or modal)
  - Delete rule (with confirmation)
- API endpoints:
  - `GET /api/config` — return current config as JSON
  - `PATCH /api/config` — update config fields (OnePoint URL, etc.)
  - `GET /api/rules` — list all rules
  - `POST /api/rules` — create a rule
  - `PATCH /api/rules/{name}` — update a rule by name
  - `DELETE /api/rules/{name}` — delete a rule by name
- All rule mutations call `config.WriteConfig()` to persist to YAML.
- Navigation: add "Config" link to `base.html` template header/nav.

**Files to change:**
- `web/server.go` — register new routes, implement handlers
- `web/templates/config.html` — new template for config page
- `web/templates/base.html` — add nav link
- `web/static/css/components.css` — styles for config page (if needed)
- `web/static/js/app.js` — config page interactions (if needed)

**Tests:**
- Handler tests for each API endpoint (GET/POST/PATCH/DELETE)
- Round-trip test: create rule via API → verify in config file → delete → verify gone

#### T-004: Web UI import with rule auto-matching

**What changes:**
- Enhance `/api/import-preview` to:
  - Accept uploaded files
  - For each file, call `importer.MatchRuleByTemplate()` against config rules
  - Return matched rule values (or empty if no match) alongside parsed entries
  - Include available mappers list and lookup data for manual selection
- Enhance `/api/import` to:
  - Accept per-file overrides for mapper, project, activity, skill, billable
  - Accept `update_rule: true` flag per file to persist overrides back to the matched rule via `config.WriteConfig()`
- Update import UI template/JS to:
  - Show matched rule name and pre-filled values per uploaded file
  - Allow user to change any field (mapper, project, activity, skill, billable)
  - Checkbox: "Update rule with these values"
  - If no rule matched, show empty form for all fields
- All three mappers (epm, generic, atwork) selectable in the UI.

**Files to change:**
- `web/server.go` — enhance import-preview and import handlers
- `web/templates/` — update import-related partials/templates (likely in `day.html` or dedicated import template)
- `web/static/js/app.js` — import preview/override UI logic
- `importer/service.go` — ensure `MatchRuleByTemplate` and `Run` are usable from web context (may need minor refactoring if tightly coupled to CLI flags)

**Tests:**
- Handler test: upload file that matches a rule → preview returns matched values
- Handler test: upload file with no rule match → preview returns empty fields
- Handler test: import with overrides → entries use override values
- Handler test: import with `update_rule: true` → config file updated

#### T-005: Reconcile auto-run after EPM import

**What changes:**
- Remove the `auto_reconcile_after_import` config check from wherever it's evaluated (already removed from config struct in T-001).
- In the web import handler (`/api/import`), after a successful EPM import, unconditionally run reconciliation on the imported entries.
- In `importer/service.go` or a new orchestration point, wire reconcile to run after EPM `Run()` completes.
- Ensure reconcile is NOT run for generic/atwork imports.

**Files to change:**
- `web/server.go` — call reconcile after EPM import in the import handler
- `reconcile/` package — verify it can be called programmatically (not just as CLI command)
- `importer/service.go` — possibly add a post-import hook or return mapper type so caller knows to reconcile

**Tests:**
- Test: EPM import triggers reconcile
- Test: generic import does NOT trigger reconcile
- Test: atwork import does NOT trigger reconcile

### Phase 3: Documentation (T-006)

#### T-006: Rewrite README and finalize docs

**What changes:**
- Rewrite `README.md` to reflect:
  - Web-UI-first workflow (install → `gohour serve` → done)
  - Only two CLI commands: `serve` and `version`
  - `~/.gohour/` directory layout
  - Migration from older versions
  - Config management via web UI
  - Import via web UI (with rule matching)
  - Remove all sections about deleted CLI commands (import, export, submit, reconcile, delete, auth, config)
  - Keep: Features, Requirements, Install, Build, Normalized SQLite Schema, Mappers, Notes, Version
  - Update: Quick Start, Configuration, Serve sections
- Update `AGENTS.md` "Implemented commands" line to reflect `serve` and `version` only.

**Files to change:**
- `README.md` — full rewrite
- `AGENTS.md` — update implemented commands list and any stale references

**Tests:**
- No code tests; documentation-only task.
- Verify no broken internal references.

## Task Dependencies
```
T-001 (file consolidation) → T-002 (remove CLI commands) → T-003 (config UI)
                                                          → T-004 (import UI) — depends on T-003
                                                          → T-005 (reconcile) — independent of T-003/T-004
T-006 (docs) — after all other tasks
```

## Validation
- `go fmt ./...`
- `go vet ./...`
- `go test ./...`
