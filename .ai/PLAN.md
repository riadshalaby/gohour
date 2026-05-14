# Plan

Status: **ready_for_implement**

Goal: decompose `web/server.go` (2874 LOC) into focused components and helper files without changing observable behavior. `Server` becomes a thin coordinator; cache, config, and import logic each own their state.

## Scope

In-scope changes are confined to the `web` package. No new packages, no template/static asset changes, no behavior changes, no API/route changes.

Final on-disk layout for `web/`:

```
web/
  audit.go                 (unchanged)
  data.go                  (unchanged)
  data_test.go             (unchanged)
  server_test.go           (unchanged)
  routes.go                NEW: Server struct + NewServer + mux wiring + ServeHTTP
  cache.go                 NEW: dataCache type and methods
  config_store.go          NEW: configStore type, rule helpers, validateOnePointURL
  import_service.go        NEW: importService + import form helpers
  parsing.go               NEW: pure parse/format helpers
  conflict.go              NEW: local conflict detection helpers
  views.go                 NEW: view/API DTOs + view builders + lookup name helpers
  render.go                NEW: template + JSON render helpers + embedded FS
  upstream.go              NEW: upstreamErrorClient + wrapUpstreamError
  handlers_pages.go        NEW: page handlers
  handlers_partials.go     NEW: HTMX partial handlers + day partial builders
  handlers_api_month.go    NEW: month-scoped JSON handlers
  handlers_api_day.go      NEW: day + lookup JSON handlers
  handlers_api_worklog.go  NEW: worklog mutation handlers + writeMutationConflictIfAny
  handlers_api_config.go   NEW: config + rule JSON handlers
  handlers_api_import.go   NEW: import-related JSON handlers
  handlers_api_submit.go   NEW: submit JSON handlers + submitRange
```

`web/server.go` is deleted at the end of T-005.

## Acceptance Criteria

- `web/server.go` is removed; `routes.go` holds the `Server` struct and mux wiring and is <300 LOC.
- No file in `web/` exceeds ~700 LOC after the split.
- `Server` no longer contains: `dayCache`, `dayFetched`, `dayRefresh`, `localByDay`, `localLoaded`, `lookupSnap`, `lookupFetched`, `mu`, `remoteFetchMu`, `localLoadMu`, `lookupMu`, `configMu`, `cfg`. These move to `dataCache` and `configStore`.
- `Server` retains: `store`, `client`, `submitOptions`, `audit`, `cache *dataCache`, `config *configStore`, `imports *importService`, `mux`, `createMu`.
- Existing tests pass unchanged after every task commit.
- `go fmt ./...`, `go vet ./...`, `go test ./...` all green after every task commit.
- HTTP routes, response shapes, status codes, error messages, and template output are unchanged.
- The package doc comment currently at the top of `server.go` migrates to `routes.go`.

## Implementation Phases

Each phase is delivered as one task, one commit. Order is deliberate: pure-helper extractions come first (lowest risk), stateful component extractions next, handler split last.

### Phase 1 — T-001: Extract pure helpers

Move the following from `server.go` into new files. No method signatures change; nothing changes about `Server`.

- `parsing.go`: `parseMonth`, `parseISODate`, `parsePositiveInt64`, `parseMutationFromForm`, `buildEntryFromMutation`, `parseSkipIndicesSet`, `parseClockMinutes`, `parseBoolFormValue`, `parseInt64FormValue`, `firstNonEmptyString`, `firstNonZeroInt64`, `boolValuePtr`.
- `conflict.go`: `detectLocalConflict`, `sameLocalWorklogKey`, `containsSameLocalWorklogKey`, `timesOverlap`, `sortDayWorklogs`, `normalizeConflictName`.
- `render.go`: `renderTemplate`, `renderPartialTemplate`, `templateFuncMap`, `writePartialTableError`, `writeJSON`, `decodeJSON`, `formatRefreshTime`. Move the `//go:embed templates` and `//go:embed static` directives plus `templateFS` and `staticFS` here.
- `upstream.go`: `errOnePointUpstream`, `upstreamErrorClient` type + all its method receivers, `wrapUpstreamError`.
- `views.go`: all view/API DTOs currently inlined at the top of `server.go` (`monthRowView`, `monthPageView`, `dayPageView`, `dayAPIResponse`, `configPageView`, `configAPIResponse`, `configPatchRequest`, `rulePayload`, `monthAPIResponse`, `worklogMutationRequest`, `importResponse`, `importPreviewEntry`, `importPreviewResponse`, `importRuleMatchResponse`, `importFormResult`, `importOverlapItem`, `importConflictResponse`, `submitDayResult`, `submitResponse`, `worklogConflictResponse`, `submitPartialView`, `lookupProject`, `lookupActivity`, `lookupSkill`, `lookupResponse`), plus the helpers `buildMonthRows`, `fillMonthDays`, `endOfMonth`, `rangeDays`, `lookupProjectName`, `lookupActivityName`, `lookupSkillName`, `lookupResponseFromSnapshot`.

Out of scope for this task: any change to `Server`, any new constructors, any signature edits.

Risk: low. No state moves. The compiler enforces correctness; the existing test suite confirms behavior.

Validation: `go fmt ./...`, `go vet ./...`, `go test ./...`.

### Phase 2 — T-002: Extract `dataCache`

Introduce `web/cache.go`:

```go
type dataCache struct {
    store  *storage.SQLiteStore
    client onepoint.Client

    mu          sync.RWMutex
    dayCache    map[string][]onepoint.DayWorklog
    dayFetched  map[string]bool
    dayRefresh  map[string]time.Time
    localByDay  map[string][]worklog.Entry
    localLoaded bool

    remoteFetchMu sync.Mutex
    localLoadMu   sync.Mutex

    lookupMu      sync.Mutex
    lookupSnap    *onepoint.LookupSnapshot
    lookupFetched bool
}

func newDataCache(store *storage.SQLiteStore, client onepoint.Client) *dataCache
```

Move these from `*Server` to `*dataCache` (rename to exported-style names within the package):
- `loadLocalRange` → `LoadLocalRange`
- `loadRemoteRange` → `LoadRemoteRange`
- `ensureLocalCache` → `EnsureLocalCache`
- `hasRemoteCacheMiss` → `HasRemoteCacheMiss`
- `invalidateLocalCache` → `InvalidateLocal`
- `invalidateRemoteDays` → `InvalidateRemoteDays`
- `remoteRangeRefreshTime` → `RemoteRangeRefreshTime`
- `loadLookupSnapshot` → `LookupSnapshot`

Co-locate `localEntryIsSynced` in `cache.go` (used only by cache callers).

In `server.go`:
- Remove fields: `dayCache`, `dayFetched`, `dayRefresh`, `localByDay`, `localLoaded`, `mu`, `remoteFetchMu`, `localLoadMu`, `lookupSnap`, `lookupFetched`, `lookupMu`.
- Add field `cache *dataCache`.
- `NewServer` constructs `cache: newDataCache(store, client)`.
- Update every call site (`s.loadLocalRange(...)` → `s.cache.LoadLocalRange(...)`, etc.).

`autoReconcileImportedRange` reads `s.store` and only invalidates caches; it stays on `*Server` for now, but its invalidation calls go through `s.cache`.

Risk: medium. Mutex ownership moves. The two-phase locking in `EnsureLocalCache` must be preserved exactly (the `localLoadMu` + recheck pattern). The `loadRemoteRange` body relies on holding `remoteFetchMu` while reading/writing both `dayCache` and `dayRefresh` — preserve the original lock interleaving verbatim. Do not rewrite for "clarity".

Validation: `go fmt ./...`, `go vet ./...`, `go test ./...`. If any test exercises `-race`, run with it too.

### Phase 3 — T-003: Extract `configStore`

Introduce `web/config_store.go`:

```go
type configStore struct {
    mu  sync.RWMutex
    cfg config.Config
}

func newConfigStore(cfg config.Config) *configStore
func (c *configStore) Snapshot() config.Config
func (c *configStore) Update(mutator func(*config.Config) error) (config.Config, error)
```

Co-locate the pure helpers it depends on:
- `cloneConfig`
- `configResponseFromConfig`
- `rulePayloadsFromRules`
- `rulePayloadFromRule`
- `ruleFromPayload`
- `findRuleIndex`
- `sameRuleName`
- `validateOnePointURL`

In `server.go`:
- Remove fields `cfg` and `configMu`.
- Add field `config *configStore`.
- `NewServer` constructs `config: newConfigStore(cfg)`.
- `configSnapshot()` and `updateConfig(...)` methods on `Server` are removed; callers use `s.config.Snapshot()` and `s.config.Update(...)`.

`handleAPIConfig*` and `handleAPIRule*` handler bodies stay where they are for this task — only call sites change. The rule CRUD code paths still live in their existing handler bodies; they will be relocated unchanged in T-005.

Risk: medium-low. Mutex semantics are simpler than the cache.

Validation: `go fmt ./...`, `go vet ./...`, `go test ./...`.

### Phase 4 — T-004: Extract `importService`

Introduce `web/import_service.go`:

```go
type importService struct {
    store  *storage.SQLiteStore
    config *configStore
}

func newImportService(store *storage.SQLiteStore, cfg *configStore) *importService
func (i *importService) ParseAndRunForm(r *http.Request) (importFormResult, error)
func (i *importService) PersistRuleUpdate(result importFormResult) error
```

Co-locate the package-level helpers in the same file:
- `importSelectionFromForm`
- `applyImportSelection`
- `shouldAutoReconcileImport`
- `worklogRange`
- `tempUploadPattern`
- `importMapperNames`

In `server.go`:
- Add field `imports *importService`.
- `NewServer` constructs `imports: newImportService(store, server.config)`.
- `handleAPIImport`, `handleAPIImportPreview`, `handleAPIImportRuleMatch` keep their HTTP wiring on `*Server` but delegate to `s.imports.ParseAndRunForm(r)` and `s.imports.PersistRuleUpdate(...)`.
- `s.parseAndRunImportForm` and `s.persistImportRuleUpdate` are removed.

`autoReconcileImportedRange` stays on `*Server` (it spans store + cache + reconcile package; making it part of a service muddles boundaries without value).

Risk: low. The only state involved is the temp file path, which lives inside `importFormResult`.

Validation: `go fmt ./...`, `go vet ./...`, `go test ./...`.

### Phase 5 — T-005: Split handlers and finish

Move handler methods on `*Server` into route-group files. Purely physical relocation — no signature or behavior changes.

- `handlers_pages.go`: `ServeHTTP`, `handleMonthPicker`, `handleMonth`, `handleDay`, `handleConfig`.
- `handlers_partials.go`: `handlePartialMonth`, `handlePartialDay`, `handlePartialWorklogCreate`, `handlePartialWorklogUpdate`, `handlePartialWorklogDelete`, `handlePartialSubmitDay`, `handlePartialSubmitMonth`, `handlePartialSubmit`, `renderDayPartial`, `buildDayPartialView`.
- `handlers_api_month.go`: `handleAPIMonth`, `handleAPIDeleteMonthWorklogs`, `handleAPIDeleteMonthRemoteWorklogs`, `handleAPICopyMonthRemote`, `handleAPISyncMonthRemote`.
- `handlers_api_day.go`: `handleAPIDay`, `handleAPILookup`.
- `handlers_api_worklog.go`: `handleAPIWorklogCreate`, `handleAPIWorklogPatch`, `handleAPIWorklogDelete`, `writeMutationConflictIfAny`.
- `handlers_api_config.go`: `handleAPIConfig`, `handleAPIConfigPatch`, `handleAPIRules`, `handleAPIRuleCreate`, `handleAPIRulePatch`, `handleAPIRuleDelete`.
- `handlers_api_import.go`: `handleAPIImport`, `handleAPIImportPreview`, `handleAPIImportRuleMatch`.
- `handlers_api_submit.go`: `handleAPISubmitDay`, `handleAPISubmitMonth`, `submitRange`, `submitErrorStatus`.
- `routes.go`: package doc comment, `Server` struct, `NewServer`, mux wiring, the `errRuleDuplicate` / `errRuleNotFound` package-level errors. `autoReconcileImportedRange` stays here (cross-component coordinator owned by `Server`).
- `web/server.go` is deleted.

Documentation updates required as part of this same task (cannot be deferred):
- Move the package doc comment (`// Package web serves a localhost-only single-user UI ...`) from `server.go` to the top of `routes.go`.
- Update `AGENTS.md` "Architecture Layers" section to reflect the new file layout in `web/` (a short bullet list of file roles is sufficient; do not invent new architectural concepts).
- If during T-005 the implementer finds any user-facing doc that names `web/server.go` specifically, update it. `README.md` is not expected to need changes since there is no user-facing behavior change.

Risk: low. Largest diff of the cycle but mostly file moves. The compiler enforces correctness.

Validation: `go fmt ./...`, `go vet ./...`, `go test ./...`. Plus a manual smoke check: run `go run . serve` against a dev DB, load `/month/<current>` and `/config`, confirm pages render. Record the smoke check outcome in the HANDOFF entry's Validation field.

## Cross-cutting Rules

- Each task results in exactly one commit, created by `commit_task` after review approval.
- No task may introduce new behavior or new exported package API.
- No task is allowed to "improve" the code it moves. Mechanical extraction only. Tempting cleanup is deferred to a follow-up cycle.
- If a task encounters a needed change that crosses task boundaries (e.g., a call site that won't compile until two phases are done), pause and surface it — do not silently widen scope.

## Validation

Per-task:
- `go fmt ./...`
- `go vet ./...`
- `go test ./...`

End-of-cycle (after T-005):
- All of the above, plus a manual smoke run of `go run . serve`.
- `wc -l web/*.go` to confirm the size ceiling and that `server.go` is gone.

## Out of Scope

- Moving code out of package `web` into `internal/app/`, `service/`, or any new package.
- Adding new tests beyond what existing tests already cover. Targeted new tests are allowed only where an extraction creates a previously untestable seam.
- Any change to template files, static assets, or the audit log format.
- Any behavioral, route, response-shape, or status-code change.
- Renaming exported identifiers visible outside the `web` package.
