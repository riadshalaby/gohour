# ROADMAP

Goal: decompose `web/server.go` (2874 LOC, god-object `Server`) into focused, independently testable components without changing any externally observable behavior.

## Priority 1

Objective: extract cohesive components out of `web/server.go` so that `Server` becomes a thin coordinator and each subsystem owns its own state and mutex.

### Target end state

`Server` struct shrinks to roughly:

```go
type Server struct {
    store         *storage.SQLiteStore
    client        onepoint.Client
    submitOptions onepoint.ResolveOptions
    audit         auditLogger
    cache         *dataCache
    config        *configStore
    imports       *importService
    mux           *http.ServeMux
    createMu      sync.Mutex
}
```

### Components to extract

- `dataCache` (new file `web/cache.go`)
  - Owns: `dayCache`, `dayFetched`, `dayRefresh`, `localByDay`, `localLoaded`, `lookupSnap`, `lookupFetched`, and the mutexes that protect them (`mu`, `remoteFetchMu`, `localLoadMu`, `lookupMu`).
  - Methods: `LoadLocalRange`, `LoadRemoteRange`, `EnsureLocalCache`, `InvalidateLocal`, `InvalidateRemoteDays`, `HasRemoteCacheMiss`, `RemoteRangeRefreshTime`, `LookupSnapshot`.
  - Plus helper `localEntryIsSynced`.

- `configStore` (new file `web/config_store.go`)
  - Owns: `configMu`, `cfg`.
  - Methods: `Snapshot`, `Update(mutator)`, `AddRule`, `PatchRule`, `DeleteRule`.
  - Co-located pure helpers: `cloneConfig`, `configResponseFromConfig`, `rulePayloadsFromRules`, `rulePayloadFromRule`, `ruleFromPayload`, `findRuleIndex`, `sameRuleName`, `validateOnePointURL`.

- `importService` (new file `web/import_service.go`)
  - Methods/functions: `parseAndRunImportForm`, `persistImportRuleUpdate`, `importSelectionFromForm`, `applyImportSelection`, `shouldAutoReconcileImport`, `worklogRange`, `tempUploadPattern`, `importMapperNames`.
  - Takes explicit deps: `*storage.SQLiteStore`, `*configStore`.

### Pure utility files (no methods on `Server`)

- `web/parsing.go`: `parseMonth`, `parseISODate`, `parsePositiveInt64`, `parseMutationFromForm`, `buildEntryFromMutation`, `parseSkipIndicesSet`, `parseClockMinutes`, `parseBoolFormValue`, `parseInt64FormValue`, `firstNonEmptyString`, `firstNonZeroInt64`, `boolValuePtr`.
- `web/conflict.go`: `detectLocalConflict`, `sameLocalWorklogKey`, `containsSameLocalWorklogKey`, `timesOverlap`, `sortDayWorklogs`, `normalizeConflictName`.
- `web/views.go`: all view/API DTOs currently inlined in `server.go`, plus `buildMonthRows`, `fillMonthDays`, `endOfMonth`, `rangeDays`, `lookupProjectName`, `lookupActivityName`, `lookupSkillName`, `lookupResponseFromSnapshot`.
- `web/render.go`: `renderTemplate`, `renderPartialTemplate`, `templateFuncMap`, `writePartialTableError`, `writeJSON`, `decodeJSON`, `formatRefreshTime`, and the `templateFS` / `staticFS` embeds.
- `web/upstream.go`: `upstreamErrorClient` and its methods, `wrapUpstreamError`, `errOnePointUpstream`.

### Handler split (by route group)

- `web/handlers_pages.go`: `handleMonthPicker`, `handleMonth`, `handleDay`, `handleConfig`, plus `ServeHTTP`.
- `web/handlers_partials.go`: all `handlePartial*`, `renderDayPartial`, `buildDayPartialView`.
- `web/handlers_api_month.go`: `handleAPIMonth`, `handleAPIDeleteMonthWorklogs`, `handleAPIDeleteMonthRemoteWorklogs`, `handleAPICopyMonthRemote`, `handleAPISyncMonthRemote`.
- `web/handlers_api_day.go`: `handleAPIDay`, `handleAPILookup`.
- `web/handlers_api_worklog.go`: `handleAPIWorklogCreate`, `handleAPIWorklogPatch`, `handleAPIWorklogDelete`, `writeMutationConflictIfAny`.
- `web/handlers_api_config.go`: `handleAPIConfig`, `handleAPIConfigPatch`, `handleAPIRules`, `handleAPIRuleCreate`, `handleAPIRulePatch`, `handleAPIRuleDelete`.
- `web/handlers_api_import.go`: `handleAPIImport`, `handleAPIImportPreview`, `handleAPIImportRuleMatch`.
- `web/handlers_api_submit.go`: `handleAPISubmitDay`, `handleAPISubmitMonth`, `submitRange`, `submitErrorStatus`.
- `web/routes.go`: `NewServer`, `Server` struct, mux wiring.

### Phased execution (ordered for low-risk increments)

Each phase must leave `go fmt ./...`, `go vet ./...`, and `go test ./...` green and is delivered as one task / one commit.

1. Extract pure helpers — `parsing.go`, `conflict.go`, `render.go`, `upstream.go`, `views.go`. No struct changes.
2. Extract `dataCache` — move cache fields and methods; `Server` keeps a `*dataCache`.
3. Extract `configStore` — move config field + rule CRUD primitives; rule handlers delegate.
4. Extract `importService` — move import orchestration; import handlers delegate.
5. Split handlers into route-group files. `server.go` becomes `routes.go` (or is deleted in favor of it).

### Acceptance criteria

- `web/server.go` either no longer exists or is reduced to a small `routes.go` (<300 LOC).
- No single file in `web/` exceeds ~700 LOC after the split.
- `Server` struct contains no cache maps, no cache mutexes, and no config mutex.
- All existing tests in `web/server_test.go` and `web/data_test.go` pass unchanged.
- `go fmt ./...`, `go vet ./...`, `go test ./...` all green at every commit on the cycle branch.
- No HTTP route, response shape, status code, or template output changes.

### Out of scope

- Moving logic out of package `web` into `internal/app/` or a new `service/` package.
- Adding new tests for the extracted components (existing tests must continue to cover behavior; targeted new tests are allowed only where an extraction creates a previously unreachable seam).
- Any change to template files, static assets, or audit log format.
- Behavioral changes of any kind.
