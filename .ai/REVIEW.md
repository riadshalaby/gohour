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

## Task: T-002

### Review Round 1

Status: **passed**

Reviewed: 2026-05-20

#### Findings
- None.

#### Verification
##### Steps
- Reviewed the T-002 plan scope against the diff: `dataCache` owns cache state/mutexes in `web/cache.go`; `Server` keeps only `cache *dataCache` plus non-cache state; call sites delegate through `s.cache`.
- Confirmed the moved `EnsureLocalCache` two-phase `localLoadMu` + recheck pattern is preserved.
- Confirmed remote miss handling keeps the serialized `remoteFetchMu` path and cache write/read lock sequence.
- Ran `go fmt ./...`: PASS.
- Ran `go vet ./...`: PASS.
- Ran `go test ./...`: PASS.
- Ran `npm run test --prefix e2e`: PASS, 14 tests passed.
##### Findings
- None.
##### Risks
- Low residual risk: this is a mechanical receiver/state move with no route or response-shape changes.

#### Open Questions
- None.

#### Verdict
`PASS`

## Task: T-001

### Review Round 1

Status: **passed**

Reviewed: 2026-05-20

#### Findings
- None.

#### Verification
##### Steps
- Reviewed the Phase 1 extraction list against the current files: `web/parsing.go`, `web/conflict.go`, `web/render.go`, `web/upstream.go`, and `web/views.go` contain the planned helpers and DTOs.
- Confirmed the extracted helpers keep package-local names/signatures and compile from the current combined worktree; T-002 cache changes are present but outside this T-001 review scope.
- Ran `go fmt ./...`: PASS.
- Ran `go vet ./...`: PASS.
- Ran `go test ./...`: PASS.
- Ran `npm run test --prefix e2e`: PASS, 14 tests passed.
##### Findings
- None.
##### Risks
- Low residual risk: review ran on the current combined T-001/T-002 worktree because T-002 was already implemented.

#### Open Questions
- None.

#### Verdict
`PASS`
## Task: T-003

### Review Round 1
- Status: **passed**
- Reviewed: 2026-05-20
- Findings: None.
- Verification: confirmed `web/config_store.go` owns config locking and rule helpers, `Server` now holds `config *configStore`, call sites use `Snapshot`/`Update`, and `Server` no longer holds `cfg`/`configMu`.
- Validation:
  - `go fmt ./...`: PASS
  - `go vet ./...`: PASS
  - `go test ./...`: PASS
  - `npm run test --prefix e2e`: PASS
- Risks: Low residual risk; the change is a mechanical state/helper extraction with unchanged handler behavior.
- Verdict: PASS
## Task: T-004

### Review Round 1
- Status: **passed**
- Reviewed: 2026-05-20
- Findings: None.
- Verification: confirmed `web/import_service.go` owns multipart import parsing, import selection application, mapper/temp helpers, and rule-update persistence; import handlers delegate through `s.imports`; `Server.parseAndRunImportForm` and `Server.persistImportRuleUpdate` are removed.
- Validation:
  - `go fmt ./...`: PASS
  - `go vet ./...`: PASS
  - `go test ./...`: PASS
  - `npm run test --prefix e2e`: PASS
- Risks: Low residual risk; the change is a mechanical extraction with handler response behavior left in place.
- Verdict: PASS
## Task: T-005

### Review Round 1
- Status: **passed**
- Reviewed: 2026-05-20
- Findings: None.
- Mechanical retries:
  - `go fmt ./...` retry 1: fixed an incomplete mechanical split range in `handlers_api_month.go`.
  - `go vet ./...` retry 1: fixed split-file import drift in config/worklog handler files.
- Verification: confirmed `web/server.go` is removed, `routes.go` owns `Server` and route registration, handlers are grouped into route files, `autoReconcileImportedRange` remains in `routes.go`, and `AGENTS.md` documents the new web layout.
- Size check: `routes.go` is 152 lines; production `web/*.go` files are all below 700 lines. `web/server_test.go` remains unchanged and intentionally exceeds that size.
- Smoke: ran `GOHOUR_DATA_DIR=/tmp/gohour-smoke-data GOHOUR_E2E_STUB_REMOTE=1 go run . serve --port 18080 --no-open` with `fresh` migration response; `/month/2026-05` returned 200 and `/config` returned 200.
- Validation:
  - `go fmt ./...`: PASS
  - `go vet ./...`: PASS
  - `go test ./...`: PASS
  - `npm run test --prefix e2e`: PASS
- Risks: Low residual risk; this is a large mechanical file split, and route behavior is covered by the existing web and e2e tests.
- Verdict: PASS
