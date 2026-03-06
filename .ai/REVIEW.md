# Review: v0.3.1 Reliability/Smoke/A11y

## Findings

1. Medium: The e2e harness now expects a pre-built `gohour` binary, but the documented flow still does not ensure that binary exists. `e2e/run-server.sh:11-13` exits immediately if `../gohour` is missing, and `e2e/global-setup.ts:1-7` only validates env vars instead of building the binary "if needed" as called for in `.ai/PLAN.md` section 2.1/2.4. At the same time, the README still advertises `cd e2e && npm install && npx playwright test` as the run sequence (`README.md:270-279`). On a clean checkout without an existing `./gohour` binary, the smoke suite will fail before the app starts.

## Plan Compliance

- Previously reported issues are addressed:
  - Day refresh now has an Alpine root, so the `@htmx:*` handlers on the day page can execute.
  - Month actions menu now supports opening/focusing items from the trigger with arrow keys.
  - Playwright no longer reuses an arbitrary already-running server, so fixture determinism is improved.
- Reliability hardening items 1.1 through 1.6 are implemented.
- Accessibility items are implemented, including ARIA labels, focus management, keyboard support, and contrast token updates.
- Handler tests listed in section 4.1 are present in `web/server_test.go`.
- Browser smoke coverage exists in `e2e/`, but the setup still diverges from the plan’s “build binary if needed” workflow.

## CLAUDE.md Compliance

- Architecture boundaries are respected.
- Error handling remains return-based; no panic-oriented regressions found.
- README was updated, but it is still not fully aligned with the actual e2e prerequisite noted above.

## Verification

- `go test ./...` passed.
- `cd e2e && npm test` still could not exercise the app in this environment because Playwright Chromium is not installed locally; the run fails before browser startup and asks for `npx playwright install`.
