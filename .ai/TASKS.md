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
| T-001 | Extract pure helpers from `web/server.go` into `parsing.go`, `conflict.go`, `render.go`, `upstream.go`, `views.go`. No `Server` changes. | done | All five new files compile. `go fmt ./...`, `go vet ./...`, `go test ./...` green. No signature changes. No behavior changes. | bundled in T-002 commit per user instruction; `go fmt ./...`; `go vet ./...`; `go test ./...`; `npm run test --prefix e2e` | none |
| T-002 | Introduce `dataCache` in `web/cache.go`; move cache state and methods off `Server`. Update call sites. | done | `Server` no longer holds cache fields/mutexes. `EnsureLocalCache` two-phase locking preserved verbatim. Tests green. | `go fmt ./...`; `go vet ./...`; `go test ./...`; `npm run test --prefix e2e` | none |
| T-003 | Introduce `configStore` in `web/config_store.go`; move config state and rule helpers. Update call sites. | ready_for_implement | `Server` no longer holds `cfg`/`configMu`. `Snapshot`/`Update` cover existing callers. Tests green. | n/a | implement |
| T-004 | Introduce `importService` in `web/import_service.go`; move import form parsing and rule-update persistence. Import handlers delegate. | ready_for_implement | `Server.parseAndRunImportForm` and `Server.persistImportRuleUpdate` removed. Tests green. | n/a | implement |
| T-005 | Split remaining handlers into route-group files; delete `web/server.go`; create `routes.go`. Update `AGENTS.md` Architecture Layers. | ready_for_implement | `web/server.go` gone; `routes.go` <300 LOC; no `web/*.go` file exceeds ~700 LOC; `AGENTS.md` reflects new layout; smoke check (`go run . serve` + `/month` + `/config`) passes; tests green. | n/a | implement |
