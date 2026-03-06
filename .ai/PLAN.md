# Plan: v0.3.1 — Reliability Hardening, Browser Smoke Tests, Accessibility

## Scope

Based on ROADMAP v0.3.1 priorities and user choices:
- **Reliability hardening**: fix UX edge cases in dialogs and async actions
- **Browser smoke tests**: standalone Playwright e2e subproject (`e2e/`), fully independent from Go
- **Accessibility pass**: ARIA labels, focus management, keyboard nav, color contrast

No new features. No responsive design rework (existing mobile layout is retained as-is).

---

## 1. Reliability Hardening (bugs & edge cases)

### 1.1 Dialog state leaks on close/cancel

**Problem**: When a dialog is dismissed via Escape or backdrop click, Alpine store
state may not fully reset (e.g., `edit.forceOverlap` persists, `submit.running`
stays true if request was in-flight).

**Changes**:
- `app.js`: In the `close` event listener for `edit-dialog`, call `$store.edit.close()`
  (already wired). Verify `forceOverlap` resets. Add guard: if `submit.running` is
  true when `submit-dialog` closes, set `running = false`.
- `app.js`: In `cancelImportPreview()`, also reset the import form's file input
  so re-opening doesn't show stale filename.

**Files**: `web/static/js/app.js`

### 1.2 Edit dialog error not cleared on re-open

**Problem**: If a previous edit/create attempt produced a validation error
(`$store.edit.error`), re-opening the dialog for a different row still shows the
stale error until `htmx:before-request` fires.

**Changes**:
- `openEditDialog()`: already sets `state.error = ''`. Verify this is effective
  for both create and edit modes. Add a test to confirm.

**Files**: `web/static/js/app.js`

### 1.3 Submit dialog: missing endpoint guard on re-open

**Problem**: `syncSubmitFormEndpoint()` may fail silently when `$store.submit`
returns an empty `endpoint()` if `scope`/`value` were not set. The `hx-post`
attribute is removed, but HTMX may still hold the old cached value.

**Changes**:
- `openSubmitAction()`: after calling `store.openSubmit()` and
  `syncSubmitFormEndpoint()`, call `htmx.process(form)` to force HTMX to
  re-read attributes. Already done in `syncSubmitFormEndpoint` — verify it works
  when the endpoint changes between two consecutive opens.

**Files**: `web/static/js/app.js`

### 1.4 Degraded auth: refresh button error feedback

**Problem**: When auth token is expired, clicking "Refresh remote" on day/month
page triggers an HTMX request that returns 502. The `@htmx:response-error`
handler shows a toast, but the HTMX indicator spinner may remain visible
indefinitely because the indicator class is not removed on error.

**Changes**:
- `day.html` / `month.html`: Add `@htmx:after-request` (in addition to
  `@htmx:after-settle`) to ensure the indicator is hidden on both success and
  error. HTMX should handle this natively via `htmx-indicator` class, but
  verify by testing with a mock 502 response.
- `web/server.go`: In `renderDayPartial` and `handlePartialMonth`, when
  `failOnRemoteErr` is true and a remote error occurs, return HTTP 502 with
  an HTML error fragment so HTMX swaps it cleanly.

**Files**: `web/templates/day.html`, `web/templates/month.html`, `web/server.go`

### 1.5 Import: toast fires before preview dialog closes

**Problem**: In `confirmImportPreview()`, on success, `openStatusDialog()` is
called before `cancelImportPreview()`, which means two dialogs may briefly
overlap. The sequence should be: close preview first, then open status.

**Changes**:
- `app.js` `confirmImportPreview()`: reorder calls so `cancelImportPreview()`
  runs before `openStatusDialog()`.

**Files**: `web/static/js/app.js`

### 1.6 Month actions: deleteMonthEntries uses full-page reload

**Problem**: `deleteMonthEntries()` does `window.location.href = ...` after
success, causing a full reload. Other month actions (copy, delete remote) use
HTMX partial refresh. This is inconsistent.

**Changes**:
- `app.js` `deleteMonthEntries()`: after successful API call, use
  `refreshMonthPartial(month, false)` and `showToast(...)` instead of
  `window.location.href`.

**Files**: `web/static/js/app.js`

---

## 2. Browser Smoke Tests (standalone Playwright e2e subproject)

### 2.1 Subproject structure

Create a new `e2e/` directory at project root as an independent Node.js subproject.
It has no dependency on the Go toolchain — tests run via `npx playwright test`.

```
e2e/
  package.json            # @playwright/test dependency
  playwright.config.ts    # Playwright config (baseURL from env, webServer launch)
  global-setup.ts         # Builds gohour binary if needed, starts `gohour serve`
  global-teardown.ts      # Stops the gohour serve process
  fixtures/
    import-smoke.csv      # Test CSV for import flow
    gohour-test.yaml      # Minimal gohour config for test (SQLite path, mock/stub settings)
  tests/
    month.spec.ts         # Month page smoke tests
    day.spec.ts           # Day page smoke tests
    import.spec.ts        # Import flow smoke test
    submit.spec.ts        # Submit dry-run smoke test
  .gitignore              # node_modules/, test-results/, playwright-report/
```

**Files**: all new under `e2e/`

### 2.2 package.json

Minimal dependencies:
- `@playwright/test` (latest stable)

Scripts:
- `test` → `playwright test`
- `test:headed` → `playwright test --headed`
- `test:report` → `playwright show-report`

**Files**: `e2e/package.json`

### 2.3 playwright.config.ts

Key settings:
- `baseURL`: read from `GOHOUR_BASE_URL` env var, default `http://localhost:9876`
- `webServer` block: launches `gohour serve --port 9876 --db <temp-test-db>`
  using the pre-built binary. The `webServer.command` handles starting the
  server; Playwright waits for the port to be ready.
  - `command`: `../gohour serve --port 9876 --config ./fixtures/gohour-test.yaml`
  - `port`: 9876
  - `reuseExistingServer`: true (allows running against a manually started server)
- `use.headless`: true (CI-friendly default)
- `retries`: 1 (flake tolerance)
- `timeout`: 15000ms per test
- `projects`: chromium only (keep it simple for smoke)

**Files**: `e2e/playwright.config.ts`

### 2.4 Test fixtures

**`e2e/fixtures/import-smoke.csv`**: Copy/move existing `web/testdata/import-smoke.csv`
content into the e2e subproject.

**`e2e/fixtures/gohour-test.yaml`**: Minimal config that points to a temporary
SQLite database and disables real OnePoint auth. The test server should use the
same mock/lookup data seeding approach. Two options:
- (a) Use a real `gohour serve` with a pre-seeded SQLite DB (fixture SQL script).
- (b) Use `gohour serve` with a test config that stubs the remote client.

**Recommendation**: Option (a) — seed a test SQLite DB via `gohour import` in
`global-setup.ts` before starting the server. This exercises the real import
path and keeps tests fully black-box.

**Seed steps in global-setup.ts**:
1. Create a temp directory for the test DB.
2. Run `gohour import --config ./fixtures/gohour-test.yaml --db <tmp>/test.db
   --mapper generic ./fixtures/import-smoke.csv` to populate seed data.
3. Start `gohour serve --port 9876 --config ./fixtures/gohour-test.yaml
   --db <tmp>/test.db`.
4. Store the child process handle for teardown.

**Note on remote/OnePoint stubs**: Since smoke tests focus on local UI flows,
remote operations (refresh remote, submit) will return errors if no auth is
configured. Tests that exercise remote-dependent flows should either:
- Be skipped when no auth token is available (env-gated).
- Or accept the error state as a valid test outcome (e.g., verify error toast
  appears on refresh failure).

**Files**: `e2e/fixtures/import-smoke.csv`, `e2e/fixtures/gohour-test.yaml`,
`e2e/global-setup.ts`, `e2e/global-teardown.ts`

### 2.5 Smoke test cases

Each test is a standard Playwright Test spec using `page` fixture and `expect`:

#### `tests/month.spec.ts`

| # | Test Name | Steps | Assertions |
|---|-----------|-------|------------|
| 1 | Month page loads | Navigate to `/month/2025-01` | Title contains "2025-01", stat cards visible, table has day rows |
| 2 | Month navigation | Click next-month arrow | URL changes to `/month/2025-02` |
| 3 | Refresh remote (error state) | Click actions > "Refresh remote" | Error toast appears, spinner clears |

#### `tests/day.spec.ts`

| # | Test Name | Steps | Assertions |
|---|-----------|-------|------------|
| 4 | Day page loads | Navigate to `/day/2025-01-02` | Stat cards visible, entry table visible, "Add entry" button present |
| 5 | Day navigation | Press ArrowRight on day page | URL changes to next day |
| 6 | Add entry dialog opens | Click "Add entry" | Edit dialog opens with "Add entry" title, selects populated |
| 7 | Create entry | Fill and submit add-entry form | Toast "Entry created.", new row in table |
| 8 | Edit entry | Click edit on existing row | Dialog opens with pre-filled values |
| 9 | Delete entry | Click delete, confirm | Toast "Entry deleted.", row removed |
| 10 | Edit dialog clears stale error | Trigger validation error, close, reopen | Error text is cleared |

#### `tests/submit.spec.ts`

| # | Test Name | Steps | Assertions |
|---|-----------|-------|------------|
| 11 | Submit day dry-run | Open submit dialog, check dry-run, run | Result box shows dry-run output |

#### `tests/import.spec.ts`

| # | Test Name | Steps | Assertions |
|---|-----------|-------|------------|
| 12 | Import file flow | Open import, upload CSV, preview, confirm | Toast "Imported N row(s)." |

**Files**: `e2e/tests/month.spec.ts`, `e2e/tests/day.spec.ts`,
`e2e/tests/submit.spec.ts`, `e2e/tests/import.spec.ts`

### 2.6 CI / run instructions

- Add `e2e/.gitignore`: `node_modules/`, `test-results/`, `playwright-report/`, `*.db`
- Update project `README.md` with a section on running e2e tests:
  ```
  cd e2e
  npm install
  npx playwright install chromium
  npx playwright test
  ```
- The `webServer` block in `playwright.config.ts` auto-starts `gohour serve`,
  so no manual server management is needed.

### 2.7 Cleanup of old browser_test.go

- Remove `web/browser_test.go` (the Go-based Playwright shim is replaced).
- Remove `web/testdata/` directory (fixtures move to `e2e/fixtures/`).
- Optionally remove `.playwright-cli/` logs if no longer needed.

**Files**: delete `web/browser_test.go`, delete `web/testdata/`

---

## 3. Accessibility Pass

### 3.1 ARIA roles and labels

**Changes across templates**:

- `base.html`:
  - Add `role="banner"` to `<header class="top">`.
  - Add `role="main"` to `<main class="content">` (already `<main>`, but verify).
  - Add `aria-label="Navigation"` to the month-picker form.
  - Toast: add `role="status"` and `aria-live="polite"`.
  - Confirm dialog: add `aria-labelledby="confirm-title"` and
    `aria-describedby="confirm-body"`.
  - Submit dialog: add `aria-labelledby="submit-dialog-title"`.
  - Edit dialog: add `aria-labelledby="edit-dialog-title"`.
  - Import preview dialog: add `aria-labelledby` pointing to its header.

- `month.html`:
  - Actions dropdown: add `role="menu"` on `.actions-menu-items`, `role="menuitem"`
    on each button inside.
  - Month table: add `aria-label="Monthly worklogs"` on `<table>`.
  - Navigation arrows: add `aria-label="Previous month"` / `aria-label="Next month"`.

- `day.html`:
  - Day table: add `aria-label="Day entries"` on `<table>`.
  - Navigation arrows: already have `title`; add matching `aria-label`.
  - "Add entry" button: add `aria-label="Add new worklog entry"`.

**Files**: `web/templates/base.html`, `web/templates/month.html`,
`web/templates/day.html`, `web/templates/partials/day_tbody.html`

### 3.2 Focus management in dialogs

**Problem**: When a dialog opens, focus is not explicitly moved to the first
interactive element. Native `<dialog>.showModal()` moves focus to the dialog
itself, but not to the first input.

**Changes**:
- `app.js` `openEditDialog()`: after `state.open = true`, use
  `requestAnimationFrame` to focus the start-time input.
- `app.js` `openImportPreviewDialog()`: after `dialog.showModal()`, focus the
  first checkbox or the import button.
- `app.js` `openSubmitAction()`: after dialog opens, focus the dry-run checkbox.

**Files**: `web/static/js/app.js`

### 3.3 Keyboard navigation improvements

**Changes**:
- `app.js`: Actions dropdown (month page) — add keyboard support:
  `ArrowDown`/`ArrowUp` to move between items, `Enter`/`Space` to activate,
  `Escape` to close (already handled by Alpine `@keydown.escape`).
- `app.js`: Add `Escape` key handler for import preview dialog (call
  `cancelImportPreview()`). The native `<dialog>` Escape closes the element,
  but our state (`importPreviewStore`) needs cleanup too — wire the dialog's
  `close` event.

**Files**: `web/static/js/app.js`, `web/templates/month.html`

### 3.4 Color contrast audit

**Review these token values against WCAG 2.1 AA (4.5:1 for normal text)**:
- `--muted: #6b7280` on `--surface: #ffffff` = 4.6:1 — passes (barely).
- `--muted-light: #9ca3af` on `--surface: #ffffff` = 2.9:1 — **fails**.
  Used in `.stat-sublabel`. Fix: darken to `#6b7280` (reuse `--muted`), or
  accept since sublabels are supplementary.
- `--hdr-muted: #5c6070` on `--hdr-bg: #0f1117` = 3.6:1 — **fails**.
  Used in header breadcrumb. Fix: lighten to `#8890a4` (~4.7:1).
- `--txt-conflict: #92400e` on `--bg-conflict: #fef3c7` = 4.2:1 — borderline.
  Fix: darken to `#78350f` (~5.1:1).

**Changes**:
- `tokens.css`: Adjust `--muted-light`, `--hdr-muted`, `--txt-conflict` values.

**Files**: `web/static/css/tokens.css`

### 3.5 Semantic table markup

**Changes**:
- Month table `<tfoot>`: first cell uses `<th>` ("Total") — correct. Verify
  `scope="row"` is set.
- Day table: add `<caption class="sr-only">` for screen readers.
- Import preview table: add `<caption class="sr-only">`.

**Files**: `web/templates/month.html`, `web/templates/day.html`,
`web/templates/base.html`

---

## 4. Test Coverage (httptest handler tests)

### 4.1 New/expanded handler tests

Add tests in `web/server_test.go` for edge cases not yet covered:

| # | Test | What it covers |
|---|------|----------------|
| 1 | `TestServer_PartialMonth_AuthError_GracefulWithoutRefresh` | Month partial returns 200 with stale cache when refresh=0 and remote fails |
| 2 | `TestServer_WorklogUpdate_OverlapConflict` | PATCH worklog returns 409 with overlap JSON when update overlaps existing |
| 3 | `TestServer_WorklogUpdate_DuplicateConflict` | PATCH worklog returns 409 with duplicate JSON |
| 4 | `TestServer_SubmitMonth_LockedDaysSkipped` | Month submit skips days with locked remote entries |
| 5 | `TestServer_SyncMonthRemote_Redirects` | Sync endpoint delegates to copy-from-remote |
| 6 | `TestServer_Import_EmptyFile` | Import with empty CSV returns 400 |

**Files**: `web/server_test.go`

---

## 5. Documentation alignment

- `README.md`: Add section on running e2e tests (`cd e2e && npm install && npx playwright test`).
- Cobra help text: no changes needed (no CLI changes in scope).

---

## Implementation Order

1. **Phase A** — Reliability fixes (1.1-1.6): `app.js`, `server.go`, templates
2. **Phase B** — Accessibility (3.1-3.5): templates, `tokens.css`, `app.js`
3. **Phase C** — Handler test expansion (4.1): `server_test.go`
4. **Phase D** — Playwright e2e subproject (2.1-2.7): `e2e/` directory, cleanup old `web/browser_test.go`

Each phase is independently committable and testable.
