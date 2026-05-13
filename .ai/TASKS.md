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
| T-001 | Adopt release-please for version + changelog management | done | `gohour version` from a plain `go build` prints the release-please-managed literal (e.g., `gohour 0.4.1`); `release-please-config.json`, `.release-please-manifest.json`, and `.github/workflows/release-please.yml` exist; `cmd/version.go` carries the `// x-release-please-version` annotation; all Go tests pass. | `go fmt ./...`; `go vet ./...`; `go test ./...`; `go build -o /tmp/gohour-local .`; `/tmp/gohour-local version`; `jq empty release-please-config.json .release-please-manifest.json` | none |
| T-002 | Add goreleaser workflow producing release artifacts | done | `.goreleaser.yaml` and `.github/workflows/goreleaser.yml` exist; workflow triggers on `v*` tag pushes; build matrix covers `darwin/linux/windows` × `amd64/arm64` and emits `SHA256SUMS`; `goreleaser check` (when available) passes. | `ruby -e 'require "yaml"; ARGV.each { \|path\| YAML.load_file(path) }; puts "yaml ok"' .goreleaser.yaml .github/workflows/goreleaser.yml`; `go fmt ./...`; `go vet ./...`; `go test ./...`; `command -v goreleaser` not available, so local `goreleaser check` skipped | none |
| T-003 | Remove obsolete release scripts and refresh release docs | done | `scripts/build-all.sh` and `scripts/release.sh` are deleted; `README.md` has an `Install` section with `go install github.com/riadshalaby/gohour@latest`, and `Build and Test` / `Releasing` sections describe release-please + goreleaser; `AGENTS.md` `Release Rules` reflects the new workflow; no tracked file references the deleted scripts or `-X cmd.Version` ldflag. | `rg -n "scripts/build-all\|scripts/release\|build-all\\.sh\|release\\.sh\|-X github.com/riadshalaby/gohour/cmd.Version\|ldflags" .` no hits; `go fmt ./...`; `go vet ./...`; `go test ./...` | none |
| T-004 | Add `GET /api/import/rule-match` endpoint | done | New endpoint returns 200 with the matched rule + resolved selection + lookup snapshot for a known filename; returns 400 for missing `filename`; existing import-preview tests still pass; new `TestServer_ImportRuleMatch_*` tests pass. | `go test ./web/ -run TestServer_ImportRuleMatch -v`; `go fmt ./...`; `go test ./web/...`; `go vet ./...`; `go test ./...` | none |
| T-005 | Remove the "Auto" billable option from the import flow | done | Both `#month-import-billable` and `#preview-billable` selectors expose exactly `billable` / `non-billable`; server rejects `billable=auto` with 400; default selection when no rule matches is `billable=true`; e2e import test asserts the option count; `AGENTS.md` `Current Status` reflects the explicit-only choice. | `go test ./web/ -run 'TestServer_ImportPreview_DefaultsBillableTrueWhenNoRule\|TestServer_ImportPreview_RejectsAutoBillable' -v`; `go fmt ./...`; `go test ./web/...`; `npm run test` in `e2e/` PASS when rerun outside sandbox after Chromium launch was blocked by sandbox; `go vet ./...`; `go test ./...` | none |
| T-006 | Pre-fill import dialog from matched rule on file selection | done | Picking a matching file in `#month-import-dialog` pre-fills mapper, project, activity, skill, and billable, and shows a `Matched rule: <name>` banner; picking an unmatched file shows `No rule matched — using defaults`; preview dialog continues to display matched rule on the existing `#preview-rule-match` line; e2e suite asserts the banner; `AGENTS.md` `Current Status` updated. | `go build -o gohour .`; `npm run test` in `e2e/`; `go fmt ./...`; `go vet ./...`; `go test ./...`; review PASS | none |
| T-007 | Surface "Update matched rule" affordance on field divergence | done | `#preview-update-rule-wrapper` is hidden with no rule matched or no overrides; surfaces and auto-checks the moment any pre-filled field changes; user-triggered untick is respected; e2e test covers the divergence behavior; `AGENTS.md` `Current Status` updated. | `go build -o gohour .`; `npm run test` in `e2e/`; `go fmt ./...`; `go vet ./...`; `go test ./...` | implement |
| T-008 | Promote Import to a top-level button on the month view | done | Month view shows an `Import file` button alongside `Submit month` in the header; Actions dropdown no longer contains an `Import file` entry; sticky-bar Import button unchanged; e2e import test clicks the new top-level button and completes end-to-end; `AGENTS.md` `Current Status` updated. | `go fmt ./...`; `go vet ./...`; `go test ./...`; `npm run test` in `e2e/` 14/14 PASS | implement |
