# ROADMAP for v0.3.0

## Goal
Deliver a complete web UI redesign with a modern look, clearer workflows, and full mobile responsiveness, without breaking existing submit/import behavior.

## Framework Decision
Recommended stack: **Server-rendered Go templates + HTMX + Alpine.js + CSS variables**.

Why this stack:
- Fits current Go server/template architecture.
- Enables incremental migration instead of a risky full SPA rewrite.
- Keeps runtime and deployment simple (no Node build requirement on server).
- Adds enough client-side interactivity for dialogs, async refresh, and progressive updates.

Alternative (only if product scope grows strongly): React + TypeScript SPA.

## Scope for v0.3.0

### P0 - Must Have
1. New design system foundation
- Typography, spacing, color tokens, button/input/table styles.
- Shared layout shell for month/day pages.
- Accessible contrast and focus states.

2. Responsive month/day experience
- Works cleanly on desktop and mobile (navigation, tables/cards, dialogs).
- Primary actions remain reachable on small screens.
- Sticky summary/action bar on mobile.

3. Workflow clarity improvements
- Consolidated action menu for secondary/destructive actions.
- Unified submit flow (`Dry run` integrated, no duplicate preview path).
- Clear status feedback after import/submit/delete actions.

4. Remote visibility controls
- Explicit `Refresh remote` action.
- Visible timestamp of last remote refresh.

### P1 - Should Have
1. Improved information density
- Better month totals layout with inline delta indicators.
- Readability improvements for day timeline and badges.

2. Dialog and error UX cleanup
- Errors shown inside/above active dialogs.
- No hidden notifications behind modals.

### P2 - Nice to Have
1. UI performance and quality
- Basic client-side loading states/skeletons.
- Lightweight visual regression screenshots for core pages.

## Acceptance Criteria
- Month and day pages are fully usable at mobile width (375px) and desktop width (1440px).
- No regression in import, submit, delete, and refresh workflows.
- `Dry run` and real submit use one consistent UX path.
- Dialog errors are always readable and actionable.
- README and Cobra help text are updated where behavior changes.
