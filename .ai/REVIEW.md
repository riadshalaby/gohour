# Review Log

Shared review log for the current cycle. Append a new task section when review starts for a new task. Within a task, append a new review round instead of replacing prior history.

## Task: T-001 — Adopt release-please for version + changelog management

### Review Round 1

Status: **complete**

Reviewed: 2026-05-06

#### Findings
- (nit) `cmd/version.go:9-12` — comment was tightened from the plan's draft (no longer mentions a "dev sentinel"). The new wording is actually more accurate because `Version` is now a `const` and there is no runtime fallback. Not a fix; documenting as a deliberate, beneficial deviation.
- (minor, out of scope for T-001) `README.md:103`, `AGENTS.md:47`, and `scripts/build-all.sh:26` still document/use the now-defunct `-ldflags "-X cmd.Version=…"` override. Verified that the override silently no-ops because `Version` is a `const`. This is explicitly deferred to T-003 ("Remove obsolete release scripts and refresh release docs"), which is `ready_for_implement`. Not a required fix on T-001; flagged so it is not lost.

#### Required Fixes
- None.

#### Verification
##### Steps
- `go fmt ./...` — clean (no output).
- `go vet ./...` — clean (no output).
- `go test ./...` — all packages PASS (cmd, config, importer, internal/timeutil, onepoint, output, reconcile, storage, submitter, web).
- `go test ./cmd/ -run TestVersionLiteralFormat -v` — PASS.
- `go test ./cmd/... -v -run "TestRootCommandSurfaceOnlyExposesServeAndVersion|TestVersion"` — both PASS.
- `jq empty release-please-config.json .release-please-manifest.json` — valid JSON.
- `ruby -ryaml -e "YAML.load_file('.github/workflows/release-please.yml')"` — valid YAML.
- `go build -o /tmp/gohour-local . && /tmp/gohour-local version` — printed `gohour 0.4.1` (literal consumed without `-ldflags`).
- E2E sanity: `go build -ldflags "-X github.com/riadshalaby/gohour/cmd.Version=v9.9.9" -o /tmp/gohour-ldflag . && /tmp/gohour-ldflag version` — printed `gohour 0.4.1`, confirming `const` switch makes the old override a no-op (intended per plan Step 3 notes).
- File inspection:
  - `cmd/version.go` — `const Version = "0.4.1" // x-release-please-version` ✓
  - `cmd/version_test.go` — semver regex test with `dev` early return ✓
  - `release-please-config.json` — `release-type: simple`, `extra-files` -> `cmd/version.go`, changelog sections per plan ✓
  - `.release-please-manifest.json` — seeded at `0.4.0` per plan ✓
  - `.github/workflows/release-please.yml` — triggers on push to `main`, uses `googleapis/release-please-action@v4` ✓
- Plan-vs-implementation diff review: all eight specified files created/modified; no unexpected changes outside scope.

##### Findings
- All acceptance criteria for T-001 satisfied.

##### Risks
- The seeded manifest version is `0.4.0` while `cmd/version.go` carries `0.4.1`. release-please reconciles this on first release-PR run by computing the next version from commits since `0.4.0` and writing `0.4.1` (or higher) into both. If the user expects no manual reconciliation, this is correct behavior; flagging for awareness.
- Until T-003 lands, `scripts/build-all.sh` still attempts `-X cmd.Version=…` which silently no-ops. Anyone running that script before T-003 ships will get a binary stamped with the source-tree literal regardless of the requested version. Mitigation: T-003 deletes the script outright.

#### Open Questions
- None.

#### Verdict
`PASS_WITH_NOTES`

---

## Task: T-002 — Add goreleaser workflow producing release artifacts

### Review Round 1

Status: **complete**

Reviewed: 2026-05-06

#### Findings
- (nit) `.goreleaser.yaml:60` — the inline comment `# release-please owns the release notes` that appeared in the plan's spec for `changelog.disable: true` was omitted. The behavior is identical without it; purely a documentation nit.

#### Required Fixes
- None.

#### Verification
##### Steps
- `ruby -e 'require "yaml"; ARGV.each { |path| YAML.load_file(path) }; puts "yaml ok"' .goreleaser.yaml .github/workflows/goreleaser.yml` — valid YAML on both files.
- `go fmt ./...` — clean (no output).
- `go vet ./...` — clean (no output).
- `go test ./...` — all packages PASS (no Go code changed; run for hygiene).
- `goreleaser check` — skipped (not installed locally; plan explicitly permits this).
- File inspection:
  - `.goreleaser.yaml` — `version: 2`, builds: linux/darwin/windows × amd64/arm64, `CGO_ENABLED=0`, `-trimpath`, `-s -w` ldflags, archive template with `x86_64` alias for amd64, windows zip override, `checksum.name_template: SHA256SUMS`, `release.mode: replace`, `changelog.disable: true` ✓
  - `.github/workflows/goreleaser.yml` — triggers on `v*` tag pushes, `fetch-depth: 0`, `go-version: "1.25"` (matches `go.mod`), `goreleaser-action@v6` with `version: "~> v2"`, `GITHUB_TOKEN` env var, `contents: write` permission ✓
  - `LICENSE` file exists — goreleaser `files: LICENSE*` will bundle it correctly ✓
- Plan-vs-implementation diff review: both specified files created; content matches plan specification exactly except for the nit above.

##### Findings
- All acceptance criteria for T-002 satisfied.

##### Risks
- `goreleaser check` was not run locally. The first real execution will be on a `v*` tag push. The config mirrors the goreleaser v2 schema closely and YAML is syntactically valid, so the risk is low.

#### Open Questions
- None.

#### Verdict
`PASS`
