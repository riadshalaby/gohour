# Review: v0.3.0 Web UI Redesign (Latest Re-Review)

## Findings
- No new blocking defects found in the currently staged implementation based on static review and backend tests.

## Previously Reported Issues — Current Status

### ✅ Submit `405 Method Not Allowed` (day/month, dry-run and normal)
- Frontend submit flow now explicitly syncs endpoint attributes onto `#submit-form` when opening/closing submit:
  - sets `hx-post`, `action`, and `method="post"` when endpoint exists
  - removes them when endpoint is empty
  (`web/static/js/app.js:1103-1121`).
- A guard now blocks submission if endpoint is missing and surfaces an inline error instead of sending an invalid request (`web/static/js/app.js:1123-1138`).
- Backend POST handlers remain correctly registered (`web/server.go:286-287`, `web/server.go:647-664`).
- This code change addresses the prior likely cause of posting to the current page URL and getting `405`.

### ✅ Day/month refresh formatting issues
- HTMX settle now reapplies locale formatting to the full document, covering OOB swaps (`web/static/js/app.js:300-305`).

### ✅ Day import removal
- Day template no longer includes import controls/dialog (`web/templates/day.html`).

### ✅ Day partial remote error behavior
- Day partial now fails closed only on explicit refresh (`web/server.go:472-475`).

## Validation
- `go test ./web` passed.

## Residual Risk / Gaps
- No browser E2E test in this review pass, so runtime confirmation of the submit dialog interaction path is still recommended.

## Verdict
- **Ready for targeted manual UI verification**; no remaining code-level blockers identified in this pass.
