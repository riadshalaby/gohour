# PLAN — Interactive Web UI + Bug Fixes

## Goal

1. **(High)** Fix atwork import producing 0 persisted rows.
2. **(Medium)** Show `billable` in `gohour config show`.
3. **(Current)** Upgrade `gohour serve` from read-only to interactive: edit/delete
   local entries, upload and import files, submit to OnePoint per day or month.

---

## Assumptions

- The read-only `gohour serve` pipeline is stable.
- The web server stays single-user / localhost-only.
- Submit via the UI skips overlaps non-interactively and returns JSON.
- Import via the UI uses the same pipeline as `gohour import`.
- `submitter` package extraction (T1) is a prerequisite for the submit
  endpoints (T4) so those two tasks must be sequenced.

---

## Tasks

---

### H1 — Fix atwork import: 0 rows persisted

**Root cause (confirmed by reading the example file and source):**

`storage/sqlite_store.go:53` has `CHECK(billable > 0)`. When a rule for the
atwork file sets `billable: false`, `importer/service.go` zeros `entry.Billable`
before calling `InsertWorklogs`. The `INSERT OR IGNORE` then silently drops every
row — no error is raised, `inserted = 0`, the import appears to succeed but
nothing is stored.

**Fix:**

In `storage/sqlite_store.go`, change:
```sql
-- before
billable INTEGER NOT NULL CHECK(billable > 0),
-- after
billable INTEGER NOT NULL CHECK(billable >= 0),
```

No migration needed. Delete `gohour.db` and re-import source files to pick up
the new constraint. No further change is needed in `service.go`; zeroing
`Billable` is the correct representation of non-billable time.

**Files:** `storage/sqlite_store.go`, `storage/sqlite_store_test.go`

**Acceptance criteria:**
- Importing the example atwork file with a rule that has `billable: false`
  persists all 6 rows with `billable = 0`.
- Re-importing the same file returns `Rows persisted: 0` (duplicate detection
  still works for 0-billable entries).

**Tests:**
- `TestInsertWorklogs_ZeroBillable`: insert entry with `Billable=0`; assert
  `inserted=1` and `ListWorklogs` returns it.

---

### M1 — Show `billable` in `gohour config show`

**Root cause:** `cmd/config_show.go` loops over rules but never prints
`rule.Billable`.

**Fix:** After the `skill` line, add:

```go
billableStr := "true (default)"
if rule.Billable != nil {
    billableStr = fmt.Sprintf("%t", *rule.Billable)
}
fmt.Printf("rules[%d].billable: %s\n", i, billableStr)
```

**Files:** `cmd/config_show.go`

**Acceptance criteria:** `gohour config show` prints `rules[N].billable: false`
for a rule with `billable: false` and `rules[N].billable: true (default)` for
a rule without the field set.

**Tests:** `cmd/config_create_test.go` — add a test that runs `configShowCmd`
with a rule that has `billable: false` and asserts the output contains the
expected line.

---

### T1 — Extract submit core to `submitter` package

The functions currently in `cmd/submit.go` that contain business logic must
move to `submitter/service.go` so both the CLI and the web server can call them.

Exported symbols to move:

```go
type DayBatch struct { Day time.Time; Worklogs []onepoint.PersistWorklog }
type NameTuple struct { Mapper, Project, Activity, Skill string }
type ResolvedIDs struct { ProjectID, ActivityID, SkillID int64 }

func CollectRequiredNameTuples(entries []worklog.Entry) ([]NameTuple, error)
func BuildRuleIDMap(rules []config.Rule) map[NameTuple]ResolvedIDs
func ResolveIDsForEntries(ctx, client, rules, entries, options) (map[NameTuple]ResolvedIDs, error)
func BuildDayBatches(entries []worklog.Entry, ids map[NameTuple]ResolvedIDs) ([]DayBatch, error)
func ClassifyWorklogs(local, existing []onepoint.PersistWorklog) (toAdd []onepoint.PersistWorklog, overlaps []onepoint.PersistWorklog, duplicates int)
```

Update `cmd/submit.go` to delegate to `submitter` (no behaviour change).

**Files:** `submitter/service.go` (new), `submitter/service_test.go` (new),
`cmd/submit.go` (updated).

---

### T2 — New storage methods

```go
// UpdateWorklog replaces all user-editable fields for the row with the given ID.
func (s *SQLiteStore) UpdateWorklog(entry worklog.Entry) error

// DeleteWorklog removes the row with the given ID.
// Returns false, nil when the ID does not exist.
func (s *SQLiteStore) DeleteWorklog(id int64) (bool, error)
```

`UpdateWorklog` updates:
`start_datetime, end_datetime, billable, description, project, activity, skill`
where `id = ?`.

**Files:** `storage/sqlite_store.go`, `storage/sqlite_store_test.go`.

---

### T3 — New API endpoints in `web/server.go`

`web.Server` gains a `cfg config.Config` field and
`web.NewServer` signature changes to:

```go
func NewServer(store *storage.SQLiteStore, client onepoint.Client, cfg config.Config) http.Handler
```

`cmd/serve.go` loads config via `config.LoadAndValidate()` and passes it.

After any mutation (`PATCH`, `DELETE`, `POST /api/import`) the server
**invalidates its local cache** (`localLoaded = false`, `localByDay` reset).
After a submit the server **invalidates the remote cache** for the affected days.

| Method | Path | Purpose |
|--------|------|---------|
| `GET`  | `/api/lookup` | Projects, activities, skills from OnePoint |
| `POST` | `/api/worklog` | Create a new local entry |
| `PATCH` | `/api/worklog/{id}` | Edit a local entry |
| `DELETE` | `/api/worklog/{id}` | Delete a local entry |
| `POST` | `/api/import` | Upload + import a file |
| `POST` | `/api/submit/day/{date}` | Submit one day to OnePoint |
| `POST` | `/api/submit/month/{month}` | Submit a full month to OnePoint |

#### Request / response shapes

**`POST /api/worklog` and `PATCH /api/worklog/{id}`**

Request body:
```json
{ "start": "09:00", "end": "17:30", "project": "…", "activity": "…",
  "skill": "…", "billable": 480, "description": "…", "date": "2026-03-01" }
```
`POST` → `201 Created` + `{ "id": <new id> }`.
`PATCH` → `204 No Content`.
`400` on validation errors; `404` if ID not found.

**`DELETE /api/worklog/{id}`** → `204 No Content`; `404` if not found.

**`GET /api/lookup`**

Returns the OnePoint project/activity/skill catalogue. IDs are included so the
browser can cascade-filter dropdowns client-side (project → activities →
skills). The response is cached for the lifetime of the server process; a
`?refresh=1` query param forces a re-fetch.

`Server` gains three new fields for the cache:
```go
lookupMu      sync.Mutex
lookupSnap    *onepoint.LookupSnapshot
lookupFetched bool
```

Response:
```json
{
  "projects":   [{ "id": 1, "name": "Project A", "archived": false }],
  "activities": [{ "id": 2, "name": "Activity B", "projectId": 1, "locked": false }],
  "skills":     [{ "id": 3, "name": "Skill C",    "activityId": 2 }]
}
```

**`POST /api/import`**

Request: `multipart/form-data` with:
- `file` — the uploaded file
- `mapper` — `epm | generic | atwork` (default: `epm`)
- `project`, `activity`, `skill` — optional EPM overrides

Server writes upload to `os.CreateTemp`, calls `importer.Run`,
calls `store.InsertWorklogs`, optionally calls `reconcile.Run`, invalidates
local cache.

Response:
```json
{ "filesProcessed": 1, "rowsRead": 6, "rowsMapped": 6,
  "rowsSkipped": 0, "rowsPersisted": 6 }
```

**`POST /api/submit/day/{date}` and `POST /api/submit/month/{month}`**

Flow (non-interactive; overlaps are skipped not prompted):
1. Load local entries for date range.
2. `submitter.ResolveIDsForEntries` (uses `s.client` + `s.cfg.Rules`).
3. `submitter.BuildDayBatches`.
4. Per day: `GetDayWorklogs` → check locked → `ClassifyWorklogs` → skip overlaps → `PersistWorklogs`.
5. Invalidate remote cache for submitted days.

Response:
```json
{
  "submitted": 3, "duplicates": 1, "overlaps": 0,
  "lockedDays": [],
  "days": [
    { "date": "2026-03-01", "added": 2, "duplicates": 0, "overlaps": 0, "locked": false }
  ]
}
```

---

### T4 — UI changes in templates

#### Data model changes in `web/data.go`

`EntryRow` gains two new fields:

```go
type EntryRow struct {
    ID          int64  // needed to target PATCH/DELETE
    Source      string
    Start       string // always "HH:MM" (ISO 24h) — JS formats for display
    End         string // always "HH:MM" (ISO 24h)
    DurationMins int   // End - Start in minutes; always ≥ 0; read-only in UI
    Project     string
    Activity    string
    Skill       string
    BillableMins int   // stored billable minutes (may differ from DurationMins)
    Description string
}
```

`DurationMins` is computed in `BuildDailyView` as
`minutesFromMidnight(end) - minutesFromMidnight(start)`.
`BillableMins` replaces the existing `Billable int` field (rename only).

The JSON API response for `GET /api/day/{date}` exposes these raw integer
values; the browser formats them.

#### Locale-aware formatting (vanilla JS, `base.html`)

All number and date rendering is done client-side using the browser's locale:

```js
// Decimal hours from integer minutes
function fmtHours(mins) {
  return new Intl.NumberFormat(navigator.language, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(mins / 60);
}

// Short time ("09:30" or "9:30 AM" depending on locale)
function fmtTime(hhmm) {          // hhmm = "HH:MM"
  const [h, m] = hhmm.split(':');
  const d = new Date(2000, 0, 1, +h, +m);
  return new Intl.DateTimeFormat(navigator.language, { timeStyle: 'short' }).format(d);
}

// Medium date ("1 Mar 2026" or "Mar 1, 2026" etc.)
function fmtDate(iso) {            // iso = "YYYY-MM-DD"
  const [y, mo, d] = iso.split('-');
  return new Intl.DateTimeFormat(navigator.language, { dateStyle: 'medium' })
    .format(new Date(+y, +mo - 1, +d));
}
```

The Go templates emit raw values (`{{ .Start }}`, `{{ .DurationMins }}`);
a `DOMContentLoaded` listener walks the table and replaces each cell's text
with the locale-formatted string. This keeps the Go templates simple and
avoids server-side locale coupling.

#### Billable auto-calculation on time change

In the inline edit form, whenever the `start` or `end` `<input type="time">`
changes:

```js
function recalcBillable(form) {
  const start = form.querySelector('[name=start]').value; // "HH:MM"
  const end   = form.querySelector('[name=end]').value;
  if (!start || !end) return;
  const startMins = toMins(start);
  const endMins   = toMins(end);
  const diff = endMins - startMins;
  if (diff > 0) {
    form.querySelector('[name=billableMins]').value = diff;
  }
}
```

The `billableMins` input is pre-populated by `recalcBillable` but remains an
editable `<input type="number">` so the user can override it after the
auto-fill. The displayed unit label next to the field reads "hours" and the
input value shown to the user is in decimal hours (converted on blur;
internally the field stores minutes as an integer for the PATCH body).

#### Project / Activity / Skill dropdowns

In edit mode, the three text fields are replaced by `<select>` elements
populated from `GET /api/lookup` (fetched once and cached in a JS module-level
variable on first edit open):

```js
let _lookup = null;
async function getLookup() {
  if (!_lookup) _lookup = await apiFetch('GET', '/api/lookup');
  return _lookup;
}
```

Cascade behaviour:
- **Project** `<select>` shows all non-archived projects.
- When project changes → **Activity** `<select>` is rebuilt to show only
  activities whose `projectId` matches and that are not locked.
- When activity changes → **Skill** `<select>` is rebuilt to show only
  skills whose `activityId` matches.
- The current `EntryRow.Project / .Activity / .Skill` string values are used
  to pre-select the matching option on edit open (case-insensitive match on
  `name`).
- If the current name is not found in the dropdown (e.g. the project was
  archived since import), it is added as a disabled option so the user sees
  the existing value and is prompted to pick a valid one.

The selected option's `data-name` attribute (the display name string) is sent
in the PATCH body — not the numeric ID. IDs are for client-side filtering only.

#### `web/templates/day.html`

- Each row gets a **Duration** column (read-only, decimal hours, locale-formatted).
- **Billable** column shows decimal hours (locale-formatted); editable in edit
  mode, auto-populated from start/end change.
- **Start** and **End** columns display locale-formatted times; edit mode shows
  `<input type="time">`.
- **Date** column (if shown) displays locale-formatted date.
- Local `new` / `overlap` rows get **Edit** and **Delete** buttons.
- Edit switches the row to an inline form; Save sends `PATCH /api/worklog/{id}`.
- Delete sends `DELETE /api/worklog/{id}`; removes the row from the DOM.
- An **"Add entry"** button appends a blank inline form; Save sends
  `POST /api/worklog` and inserts the returned row with a `new` badge.
- A **"Submit day"** button calls `POST /api/submit/day/{date}` and shows the
  result (added / duplicates / overlaps / locked) as an inline notification.
- An **"Import file"** button reveals a file-upload form that posts to
  `POST /api/import`; on success reloads the entry table via `GET /api/day/{date}`.

#### `web/templates/month.html`

- Hour totals in the monthly grid are locale-formatted decimal hours.
- Dates in the Date column are locale-formatted.
- A **"Submit month"** button calls `POST /api/submit/month/{month}` and shows
  a per-day result table below the monthly grid.
- An **"Import file"** button (same upload form as day page).

#### `web/templates/base.html`

- Add a `<script>` block with shared vanilla JS:
  `apiFetch`, `showToast`, `fmtHours`, `fmtTime`, `fmtDate`,
  `toMins`, `recalcBillable`, `getLookup`.
- No external JS dependencies; no build step.

---

## Files Expected To Change

| Task | File | Change |
|------|------|--------|
| H1 | `storage/sqlite_store.go` | schema fix (`billable >= 0`) |
| H1 | `storage/sqlite_store_test.go` | `TestInsertWorklogs_ZeroBillable` |
| M1 | `cmd/config_show.go` | Print `billable` per rule |
| T1 | `submitter/service.go` | New package |
| T1 | `submitter/service_test.go` | New unit tests |
| T1 | `cmd/submit.go` | Delegate to `submitter` |
| T2 | `storage/sqlite_store.go` | `UpdateWorklog`, `DeleteWorklog` |
| T2 | `storage/sqlite_store_test.go` | Tests for new methods |
| T3 | `web/server.go` | New routes, `cfg` field, lookup cache, cache invalidation |
| T3 | `cmd/serve.go` | Pass `cfg` to `web.NewServer` |
| T4 | `web/data.go` | Add `ID`, `DurationMins`; rename `Billable` → `BillableMins` |
| T4 | `web/templates/day.html` | Duration col, locale formatting, dropdowns, edit/delete/add/submit/import |
| T4 | `web/templates/month.html` | Locale formatting, submit month, import |
| T4 | `web/templates/base.html` | Shared JS: formatting helpers, lookup cache, billable auto-calc |

---

## Acceptance Criteria

1. **H1**: Importing `examples/excel-export-atwork-2026-03-fake.csv` with a rule
   having `billable: false` persists all 6 rows. Re-import persists 0 (dedup works).
2. **M1**: `gohour config show` prints `rules[N].billable: false` for a rule with
   `billable: false`, and `rules[N].billable: true (default)` otherwise.
3. **T2/Edit**: Changing start/end time inline and saving updates the SQLite row;
   reloading reflects the change.
4. **T2/Delete**: Clicking Delete removes the row from SQLite and from the DOM.
5. **T4/Add**: Filling the "Add entry" form creates a new SQLite row with a `new` badge.
6. **T3/Import**: Uploading the atwork example via the UI imports 6 rows.
7. **T3/Submit day**: Submitting a day with `new` entries calls OnePoint persist;
   those entries then show as `duplicate`.
8. **T3/Submit month**: Submitting a month returns per-day outcomes; locked days
   show `locked: true`.
9. **Duration**: Every local entry row shows a read-only Duration column in
   decimal hours formatted with the browser locale (e.g. `1,50` or `1.50`).
10. **Billable auto-calc**: Changing Start or End in edit mode auto-fills the
    Billable field with the new duration; the user can then type a different value.
11. **Locale times/dates**: Start, End, and date columns display using the
    browser locale (e.g. `09:30` vs `9:30 AM`; `1 Mar 2026` vs `Mar 1, 2026`).
12. **Dropdowns**: In edit mode, Project is a `<select>` from OnePoint; changing
    Project rebuilds the Activity list; changing Activity rebuilds the Skill list.
    The current entry's names are pre-selected; archived/unknown names appear as
    a disabled option.
13. No JavaScript framework or build step is introduced.

---

## Test Plan

**H1** — `storage/sqlite_store_test.go`:
- `TestInsertWorklogs_ZeroBillable`: insert with `Billable=0`, assert success.

**M1** — existing test file, new case asserting billable line appears in output.

**T1** — `submitter/service_test.go`:
- `TestClassifyWorklogs_Duplicate`, `_Overlap`, `_New`.
- `TestBuildDayBatches_CrossDay`: entry spanning midnight → error.

**T2** — `storage/sqlite_store_test.go`:
- `TestUpdateWorklog_ChangesFields`, `TestUpdateWorklog_NotFound`.
- `TestDeleteWorklog_Removes`, `TestDeleteWorklog_NotFound`.

**T3** — `web/server_test.go` (using `httptest` + stub store + stub client):
- `TestPatchWorklog_ValidBody`: 204 returned.
- `TestPatchWorklog_InvalidTime`: 400 returned.
- `TestDeleteWorklog_Exists`: 204 returned.
- `TestSubmitDay_LockedDay`: stub returns locked entry, response has `locked: true`.
- `TestSubmitDay_NewEntry`: stub empty day, persist called once, `added=1`.
- `TestImport_ValidFile`: multipart upload, stub store insert called.
- `TestGetLookup_ReturnsJSON`: stub client returns fixture snapshot; assert
  response contains expected project/activity/skill names and IDs.
- `TestGetLookup_CachedOnSecondCall`: stub client called only once across two
  requests (lookup is cached).

**T4** — `web/data_test.go`:
- `TestBuildDailyView_DurationMins`: entry with 90-minute window → `DurationMins=90`.
- `TestBuildDailyView_DurationIndependentOfBillable`: entry with `Billable=0`
  still has correct `DurationMins`.
