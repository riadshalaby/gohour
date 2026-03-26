# Review: v0.3.2 Module Path Migration

## Findings

1. Low: `e2e/tmp-import-probe.csv` is a checked-in scratch artifact with no repo references and no role in the actual browser smoke test. The Playwright import test already generates its own CSV at runtime via `testInfo.outputPath(...)`, so keeping this file in git adds noise and can mislead future maintainers into treating it as a required fixture. References: `e2e/tmp-import-probe.csv:1`, `e2e/tests/import.spec.ts:4`.

## Plan Compliance

- Phase 1 is implemented: `go.mod` now uses `github.com/riadshalaby/gohour`, and internal imports were rewritten to the canonical module path.
- Phase 2 is implemented: `scripts/build-all.sh`, `cmd/version.go`, and `CLAUDE.md` all use `github.com/riadshalaby/gohour/cmd.Version`.
- Phase 3 is implemented: `README.md` now documents `go install github.com/riadshalaby/gohour@latest`, PATH setup, and installed-binary quick start.
- Phase 4 verification passed locally:
  - `go vet ./...`
  - `go test ./...`
  - `go build -ldflags "-X github.com/riadshalaby/gohour/cmd.Version=v0.3.2-test" -o /tmp/gohour-review .`
  - `/tmp/gohour-review version` -> `gohour v0.3.2-test`
  - `GOBIN=/tmp/gobin-review go install -ldflags "-X github.com/riadshalaby/gohour/cmd.Version=v0.3.2-test" .`
  - `/tmp/gobin-review/gohour version` -> `gohour v0.3.2-test`
- `scripts/release.sh` still delegates to `scripts/build-all.sh`, so the ldflags migration propagates through the release helper as intended.
- Extra changes outside the stated plan landed in `.gitignore`, `ROADMAP.md`, and `e2e/tmp-import-probe.csv`; only the tracked CSV is review-worthy.

## CLAUDE.md Compliance

- Architecture boundaries remain intact. The patch is limited to module/import-path rewrites plus build metadata and documentation updates.
- The coding rule to keep documentation aligned with workflow changes is satisfied by the README and CLAUDE updates.
- No panic-oriented regressions, global mutable state additions, or architecture violations were found in the diff.

## Verification Notes

- Repo scan found no remaining code or script references to the old `gohour/...` module path.
- I did not run Playwright e2e during this review. For this change set, the Go-level verification required by the plan passed.
