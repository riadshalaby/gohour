# TASKS

Use this board to coordinate manual handoff between planner, implementer, and reviewer.

Status values:
- `in_planning`
- `ready_for_implement`
- `in_implementation`
- `ready_for_review`
- `in_review`
- `ready_to_commit`
- `changes_requested`
- `done`

Command expectations:
- planner moves tasks into `in_planning` and `ready_for_implement`
- implementer moves tasks into `in_implementation`, `ready_for_review`, and `done`, and resumes work from `changes_requested` and `ready_to_commit`
- reviewer moves tasks into `in_review`, `ready_to_commit`, or `changes_requested`
- `status_cycle` should report deterministic task status, current owner role, and next recommended action based on this board

| Task ID | Scope | Status | Acceptance Criteria | Evidence | Next Role |
| --- | --- | --- | --- | --- | --- |
| T-001 | Consolidate file paths under `~/.gohour/` with migration | done | Config at `~/.gohour/config.yaml`, DB at `~/.gohour/gohour.db`, auth at `~/.gohour/onepoint-auth-state.json`; migration prompt works for old files; `--db`, `--configFile`, `--url`, `--state-file`, `--from`, `--to` flags removed; `WriteConfig` exists; `auto_reconcile_after_import` config field removed; tests pass | `go fmt ./...`; `go vet ./...`; `go test ./...` | none |
| T-002 | Remove CLI commands (auth, config, delete, export, import, reconcile, submit) | done | Only `serve` and `version` commands remain; 16 cmd files + 4 test files deleted; `serve_e2e_stub.go` and `serve_test.go` updated for new paths; Playwright E2E suite (`e2e/`) updated for new CLI surface (no import cmd, no removed flags, seeding via API or pre-placed DB); `go build`, `go vet`, `go test ./cmd/...` clean; `npx playwright test` passes; Cobra help text updated for serve | `go fmt ./...`; `go test ./cmd/...`; `go build ./...`; `go vet ./...`; `go test ./...`; `PLAYWRIGHT_BROWSERS_PATH=/tmp/gohour-playwright-browsers npx playwright test` | none |
| T-003 | Web UI config management page with rule CRUD | done | `/config` page accessible from nav; OnePoint URL editable; rules listable, creatable, editable, deletable; mutations persist to YAML; API endpoints functional; handler tests pass | `go fmt ./...`; `go test ./web`; `go vet ./...`; `go test ./...` | none |
| T-004 | Web UI import with rule auto-matching and override | done | Import preview auto-matches rules and shows pre-filled values; user can override all fields; "update rule" option persists overrides; no-match shows empty form; all three mappers selectable; handler tests pass | `go fmt ./...`; `go test ./web`; `go vet ./...`; `go test ./...` | none |
| T-005 | Reconcile auto-run after EPM import (unconditional) | done | EPM imports trigger reconcile automatically; generic/atwork imports do not; no config toggle; tests verify mapper-conditional behavior | `go fmt ./...`; `go test ./web`; `go vet ./...`; `go test ./...` | none |
| T-006 | Rewrite README and update AGENTS.md | ready_for_implement | README reflects web-UI-first workflow, two CLI commands, `~/.gohour/` layout, migration, removed commands; AGENTS.md updated; no broken references | n/a | implement |
