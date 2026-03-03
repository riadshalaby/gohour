# Plan: v0.3.0 Web UI Redesign

## Strategy
**Foundation-first**, five phases, each independently shippable.
Dependencies (HTMX + Alpine.js) vendored as local static files.

---

## Phase 1 — Foundation: Extract, Tokenize, Shell

**Goal:** Break the monolithic `base.html` into maintainable pieces and establish the design system without changing any visible behavior.

### 1.1 Create static asset directory and serving
- Create `web/static/` directory.
- Add `web/static/css/tokens.css` — design tokens only (colors, typography, spacing, radii, shadows).
- Add `web/static/css/base.css` — reset + global element styles referencing tokens.
- Add `web/static/css/components.css` — buttons, inputs, selects, tables, badges, cards, dialogs, toasts.
- Add `web/static/css/layout.css` — header, content shell, page wrapper, responsive breakpoints.
- Add `web/static/js/app.js` — extract all current inline JS from `base.html` unchanged.
- Register `/static/` file handler in `server.go` using `http.FileServer` on embedded FS.
- Update `base.html` to `<link>` the CSS files and `<script src>` the JS file instead of inline blocks.
- **Files changed:** `web/server.go`, `web/templates/base.html`, new `web/static/` tree.
- **Verification:** UI looks and behaves identically to pre-change.

### 1.2 Vendor HTMX + Alpine.js
- Download `htmx.min.js` (v2.x) → `web/static/vendor/htmx.min.js`.
- Download `alpinejs` CDN build → `web/static/vendor/alpine.min.js`.
- Add `<script>` tags in `base.html` (defer for Alpine, before app.js for HTMX).
- No behavioral changes yet — just available for Phase 2.
- **Files changed:** `web/templates/base.html`, new vendor files.

### 1.3 New layout shell
- Redesign the `base.html` outer shell (header, nav, content area, footer/status bar).
- New header: brand left, month/day breadcrumb center, settings/status right.
- Content area: single centered column (`max-width: 1200px`) with padding that collapses on mobile.
- Sticky bottom bar placeholder (hidden until Phase 5 mobile work).
- **Files changed:** `web/templates/base.html`, `web/static/css/layout.css`.

### 1.4 Design token definitions
Define CSS custom properties for:
- **Colors:** background, surface, border, text (primary/secondary/muted), brand, status colors (local/synced/conflict/remote), semantic (success/warning/error/info).
- **Typography:** font-family (sans/mono), font-size scale (xs–2xl), font-weight, line-height.
- **Spacing:** 4px base scale (--sp-1 through --sp-12).
- **Borders:** radius (sm/md/lg/full), width.
- **Shadows:** sm/md/lg.
- **Transitions:** default duration and easing.
- **Files changed:** `web/static/css/tokens.css`.

---

## Phase 2 — Introduce HTMX + Alpine.js Incrementally

**Goal:** Replace vanilla JS fetch-and-DOM-manipulate patterns with HTMX for server-driven updates and Alpine.js for local UI state, one feature at a time.

### 2.1 HTMX: Month table refresh
- Add new handler `GET /partials/month/{month}` returning just the month table `<tbody>` HTML.
- Add `hx-get`, `hx-trigger`, `hx-target`, `hx-swap` attributes to the refresh button.
- Remove the `reloadMonthTable()` JS function and its DOM-building code.
- **Files changed:** `web/server.go`, `web/templates/month.html` (new partial template), `web/static/js/app.js`.

### 2.2 HTMX: Day table refresh
- Same pattern: `GET /partials/day/{date}` returns day entries `<tbody>`.
- Wire refresh button with `hx-get`.
- Remove `reloadDayTable()` JS.
- **Files changed:** `web/server.go`, `web/templates/day.html` (new partial), `web/static/js/app.js`.

### 2.3 HTMX: Worklog CRUD
- After create/update/delete, return updated day partial via `HX-Trigger` response header or `hx-swap` OOB.
- Remove `appendDayEntryRow()`, `renderDayRowCells()` JS.
- **Files changed:** `web/server.go`, `web/templates/day.html`, `web/static/js/app.js`.

### 2.4 Alpine.js: Dialog state management
- Convert edit/create dialog to Alpine component (`x-data`, `x-show`, `x-model`).
- Convert confirm dialog to Alpine component.
- Convert import dialog to Alpine component.
- Replace `_editState`, `_confirmCallback`, `_importPreviewState` globals.
- **Files changed:** `web/templates/base.html`, `web/static/js/app.js`.

### 2.5 Alpine.js: Action menu + toast
- Convert action dropdown to `x-data="{ open: false }"` with `@click.outside`.
- Convert toast system to Alpine store (`Alpine.store('toast', ...)`).
- **Files changed:** `web/templates/base.html`, `web/templates/month.html`, `web/static/js/app.js`.

### 2.6 HTMX: Submit flow (dry-run integrated)
- New partial `POST /partials/submit/day/{date}` returns submit result HTML fragment.
- Submit dialog uses `hx-post` with `hx-target` pointing at result container inside dialog.
- Dry-run checkbox becomes `hx-vals='{"dry_run": true}'` toggle.
- Removes need for separate `runSubmitAction()` / `renderDaySubmitResult()` JS.
- **Files changed:** `web/server.go`, `web/templates/base.html`, `web/static/js/app.js`.

---

## Phase 3 — Redesign Month Page

**Goal:** New visual design for the month view with improved information density.

### 3.1 Month summary header
- Redesign top summary: local hours, remote hours, delta — as large stat cards.
- Inline delta indicators (green/red with arrow icons).
- Show worked vs. billable breakdown.
- **Files changed:** `web/templates/month.html`, `web/static/css/components.css`.

### 3.2 Month table redesign
- Cleaner table with alternating row shading.
- Weekend rows visually distinct (subtle background, not bold color).
- Today row highlighted with left border accent.
- Delta column: colored pill badges instead of plain text.
- Locked-day indicator icon.
- **Files changed:** `web/templates/month.html`, `web/static/css/components.css`.

### 3.3 Consolidated action menu
- Group secondary/destructive actions (delete local, delete remote, copy from remote) into a dropdown menu with clear labels, descriptions, and danger styling.
- Primary actions (submit month, import) stay as top-level buttons.
- **Files changed:** `web/templates/month.html`, `web/static/css/components.css`.

### 3.4 Remote visibility
- "Refresh remote" button with last-refresh timestamp display (e.g., "Remote data from 2 min ago").
- Timestamp stored in Alpine reactive data, updated on each refresh.
- **Files changed:** `web/templates/month.html`, `web/static/js/app.js`.

---

## Phase 4 — Redesign Day Page

**Goal:** New visual design for the day detail view.

### 4.1 Day summary header
- Stat cards: local worked, remote worked, local billable, remote billable.
- Visual delta between local and remote.
- **Files changed:** `web/templates/day.html`, `web/static/css/components.css`.

### 4.2 Entry table/cards redesign
- Desktop: clean table with source badge, time range, duration, project/activity/skill, billable, description, actions.
- Improved badge styles (local = green, synced = blue, conflict = amber, remote = purple).
- Edit/delete buttons as icon buttons in last column.
- **Files changed:** `web/templates/day.html`, `web/static/css/components.css`.

### 4.3 Edit/create dialog redesign
- Form layout: two-column grid on desktop (time fields side-by-side), single column on mobile.
- Cascading selects styled consistently.
- Duration auto-calculated and displayed inline.
- Validation feedback inline (not just toast).
- **Files changed:** `web/templates/base.html`, `web/static/css/components.css`.

### 4.4 Day navigation
- Prev/next day arrows.
- "Back to month" breadcrumb link.
- Keyboard shortcuts (← → for prev/next day).
- **Files changed:** `web/templates/day.html`, `web/static/js/app.js`.

### 4.5 Remote visibility (day level)
- Same "Refresh remote" + timestamp pattern as month.
- **Files changed:** `web/templates/day.html`.

---

## Phase 5 — Mobile Responsive + Polish

**Goal:** Full mobile support and P1/P2 polish items.

### 5.1 Responsive month view
- Table → stacked card layout at `< 768px`.
- Each day becomes a card showing date, hours, delta.
- Tap card → navigate to day.
- Summary stats stack vertically.
- **Files changed:** `web/static/css/layout.css`, `web/templates/month.html`.

### 5.2 Responsive day view
- Entry table → card layout at `< 768px`.
- Each entry card: time range header, project/activity below, action buttons bottom.
- **Files changed:** `web/static/css/layout.css`, `web/templates/day.html`.

### 5.3 Sticky mobile action bar
- Fixed bottom bar on mobile with primary actions (submit, add entry).
- Collapses into page flow on desktop.
- **Files changed:** `web/static/css/layout.css`, `web/templates/base.html`.

### 5.4 Dialog responsive
- Dialogs go full-width on mobile (max-width: 100vw, no side padding).
- Form fields stack single-column.
- **Files changed:** `web/static/css/components.css`.

### 5.5 Status feedback improvements (P1)
- Errors shown inside dialogs (above action buttons).
- Loading states: button spinners during async operations.
- HTMX `hx-indicator` class for loading spinners on tables.
- **Files changed:** `web/static/css/components.css`, `web/templates/base.html`.

### 5.6 Loading skeletons (P2, optional)
- CSS-only skeleton placeholder for month table and day table during initial load.
- Activated via `hx-indicator` + CSS `.htmx-request` class.
- **Files changed:** `web/static/css/components.css`, templates.

---

## File Impact Summary

| File | Phases |
|------|--------|
| `web/server.go` | 1, 2 (new partial routes) |
| `web/templates/base.html` | 1, 2, 4, 5 |
| `web/templates/month.html` | 2, 3, 5 |
| `web/templates/day.html` | 2, 4, 5 |
| `web/static/css/tokens.css` | 1 (new) |
| `web/static/css/base.css` | 1 (new) |
| `web/static/css/components.css` | 1, 3, 4, 5 (new) |
| `web/static/css/layout.css` | 1, 5 (new) |
| `web/static/js/app.js` | 1, 2, 3, 4 (new, then iterated) |
| `web/static/vendor/htmx.min.js` | 1 (new) |
| `web/static/vendor/alpine.min.js` | 1 (new) |

## Testing Strategy

- **Each phase:** Manual smoke test of all workflows (import, submit, delete, refresh, CRUD).
- **Phase 1:** Pixel-diff before/after extraction to confirm no visual change.
- **Phase 2:** Test each HTMX migration individually; old JS removed only after HTMX equivalent verified.
- **Phase 5:** Test at 375px (mobile) and 1440px (desktop) widths.
- **Acceptance:** All criteria from ROADMAP.md verified before v0.3.0 tag.

## Out of Scope
- No SPA/React migration.
- No Node.js build tooling.
- No server-side auth/CSRF (stays localhost-only).
- No new API endpoints beyond HTMX partials.
- README and Cobra help updates deferred to final phase wrap-up.
