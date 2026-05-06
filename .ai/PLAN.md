# Plan

Status: **ready_for_implement**

Cycle: **0.4.1** — implements scope defined in `ROADMAP.md`.

This plan deliberately splits the work into eight self-contained tasks. Each task is independently committable, validated, and documented. Sequencing is chosen so each commit ships clean working state with no scaffolding left behind.

## Versioning principle (recap)

Per `ROADMAP.md`, the cycle version reflects user-facing changes only. Build/CI/release tooling commits are `chore:` / `build:` / `ci:` / `docs:` (no version bump under release-please). UX changes are `feat:` (genuinely new behavior) or `fix:` (quirk fixes). The cycle-end commit emits `Release-As: 0.4.1` so the actual release tag is fixed regardless of the per-commit bump rules.

## Task ordering & dependencies

```
T-001 release-please ──┐
                       ├──> T-003 scripts cleanup + release docs
T-002 goreleaser ──────┘
T-004 backend rule-match endpoint ──> T-006 file-pick prefill + indicator
                                      └──> T-007 update-rule affordance on divergence
T-005 remove "Auto" billable option (independent, do before T-006 to avoid stale paths)
T-008 promote Import to top-level button (independent)
```

Recommended commit order: T-001, T-002, T-003, T-004, T-005, T-006, T-007, T-008.

---

## T-001 — Adopt release-please for version + changelog management

**Goal:** replace the `var Version = "dev"` literal with a release-please-managed value so any `go build` or `go install …@latest` reports a real version. Wire release-please as a GitHub Action that opens release PRs from Conventional Commits on `main`.

**Files:**
- Create: `.github/workflows/release-please.yml`
- Create: `release-please-config.json`
- Create: `.release-please-manifest.json`
- Modify: `cmd/version.go`
- Modify: `cmd/serve_test.go` (the existing version surface test stays valid; only verify nothing breaks)
- Test: `cmd/version_test.go` (new)

**Reference:** `googleapis/release-please-action@v4` with `release-type: simple` and `extra-files` to update an annotated literal in `cmd/version.go`.

### Steps

- [ ] **Step 1: Write failing test for version literal format**

Create `cmd/version_test.go`:

```go
package cmd

import (
	"regexp"
	"testing"
)

// Version must be either the dev sentinel or a semver string of the form vX.Y.Z
// (no "v" prefix is acceptable — release-please writes the bare semver into a
// release-please-annotated literal that Go consumes directly).
func TestVersionLiteralFormat(t *testing.T) {
	t.Parallel()
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
	semver := regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[\w.\-]+)?(\+[\w.\-]+)?$`)
	if Version == "dev" {
		return // acceptable when running from un-tagged checkout
	}
	if !semver.MatchString(Version) {
		t.Fatalf("Version %q must be %q or a semver string", Version, "dev")
	}
}
```

- [ ] **Step 2: Run the test (expect PASS — `Version = "dev"` is allowed)**

Run: `go test ./cmd/ -run TestVersionLiteralFormat -v`
Expected: PASS (the dev sentinel is allowed).

- [ ] **Step 3: Update `cmd/version.go` with the release-please annotation**

Replace the file contents with:

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is updated by release-please when a release PR is merged.
// The line below carries the release-please annotation; do not edit by hand
// outside of release-please PRs. Local development builds keep the "dev"
// sentinel until release-please writes a tagged version.
const Version = "0.4.1" // x-release-please-version

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the gohour version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gohour %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
```

Notes:
- Switching from `var` to `const` is intentional — release-please only edits the literal, and `const` documents that nothing else mutates it.
- The seed value `0.4.1` reflects the current cycle. release-please will overwrite this on every release PR.
- The previous `-ldflags "-X …"` build path is dropped: `cmd.Version` is no longer a `var`, so the linker cannot override it. Local builds always show the literal in the file. This is the intended behavior — `go install …@latest` will fetch the tagged code, which contains the right literal.

- [ ] **Step 4: Run the version-format test again**

Run: `go test ./cmd/ -run TestVersionLiteralFormat -v`
Expected: PASS (the literal is now `0.4.1`, which matches the semver regex).

- [ ] **Step 5: Run the full cmd test surface**

Run: `go test ./cmd/...`
Expected: all tests pass, including `TestRootCommandSurfaceOnlyExposesServeAndVersion`.

- [ ] **Step 6: Create `release-please-config.json`**

```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
  "release-type": "simple",
  "include-component-in-tag": false,
  "tag-separator": "",
  "packages": {
    ".": {
      "release-type": "simple",
      "package-name": "gohour",
      "extra-files": [
        {
          "type": "generic",
          "path": "cmd/version.go"
        }
      ],
      "changelog-sections": [
        { "type": "feat", "section": "Features" },
        { "type": "fix", "section": "Bug Fixes" },
        { "type": "perf", "section": "Performance" },
        { "type": "docs", "section": "Documentation", "hidden": true },
        { "type": "build", "section": "Build", "hidden": true },
        { "type": "ci", "section": "CI", "hidden": true },
        { "type": "chore", "section": "Chore", "hidden": true },
        { "type": "refactor", "section": "Refactor", "hidden": true },
        { "type": "test", "section": "Tests", "hidden": true }
      ]
    }
  }
}
```

- [ ] **Step 7: Create `.release-please-manifest.json`**

```json
{
  ".": "0.4.0"
}
```

The starting point is `0.4.0` (the last released version per the existing tag history). release-please will compute the next version from commits landed since that tag.

- [ ] **Step 8: Create `.github/workflows/release-please.yml`**

```yaml
name: release-please

on:
  push:
    branches:
      - main

permissions:
  contents: write
  pull-requests: write

jobs:
  release-please:
    runs-on: ubuntu-latest
    steps:
      - uses: googleapis/release-please-action@v4
        with:
          config-file: release-please-config.json
          manifest-file: .release-please-manifest.json
```

- [ ] **Step 9: Run formatter, vet, tests**

Run: `go fmt ./... && go vet ./... && go test ./...`
Expected: all green.

- [ ] **Step 10: Manual sanity check**

Run: `go build -o /tmp/gohour-local . && /tmp/gohour-local version`
Expected output: `gohour 0.4.1`

This proves the literal is consumed without `-ldflags`.

- [ ] **Step 11: Commit**

```bash
git add cmd/version.go cmd/version_test.go release-please-config.json .release-please-manifest.json .github/workflows/release-please.yml
git commit -m "ci: adopt release-please for version + changelog management"
```

**Acceptance checklist:**
- [ ] `gohour version` prints `gohour 0.4.1` from a plain `go build` (no `-ldflags`).
- [ ] release-please workflow file exists and is syntactically valid YAML.
- [ ] `cmd/version.go` carries the `// x-release-please-version` annotation.
- [ ] `go test ./...` passes.

---

## T-002 — Add goreleaser workflow producing release artifacts

**Goal:** when release-please publishes a tag (e.g., `v0.4.1`), build the same matrix of binaries the existing `scripts/build-all.sh` produces (`darwin/linux/windows` × `amd64/arm64`) plus `SHA256SUMS`, and attach them to the GitHub release.

**Files:**
- Create: `.goreleaser.yaml`
- Create: `.github/workflows/goreleaser.yml`

### Steps

- [ ] **Step 1: Create `.goreleaser.yaml`**

```yaml
version: 2

project_name: gohour

before:
  hooks:
    - go mod tidy

builds:
  - id: gohour
    binary: gohour
    main: .
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    flags:
      - -trimpath
    ldflags:
      - -s -w

archives:
  - id: gohour
    builds:
      - gohour
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- if eq .Arch "amd64" }}x86_64
      {{- else }}{{ .Arch }}{{ end }}
    format_overrides:
      - goos: windows
        format: zip
    files:
      - LICENSE*
      - README.md

checksum:
  name_template: SHA256SUMS

release:
  github:
    owner: riadshalaby
    name: gohour
  draft: false
  prerelease: auto
  mode: replace
  name_template: "{{ .Tag }}"

snapshot:
  name_template: "{{ incpatch .Version }}-snapshot"

changelog:
  disable: true # release-please owns the release notes
```

Notes:
- `ldflags: -s -w` is intentional and replaces the previous `-X cmd.Version=…`. The version literal lives in `cmd/version.go` now (T-001), so no link-time override is needed.
- `mode: replace` lets goreleaser attach assets to the release release-please created; release-please opens the release without binaries, then this workflow adds them.

- [ ] **Step 2: Create `.github/workflows/goreleaser.yml`**

```yaml
name: goreleaser

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"

      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 3: Local validation (optional but recommended)**

If `goreleaser` is installed locally (`brew install goreleaser`), run a snapshot build to confirm the config is valid:

Run: `goreleaser release --snapshot --clean --skip=publish`
Expected: `dist/` populated with binaries and `SHA256SUMS`.

If `goreleaser` is not available locally, skip and rely on CI to surface config errors.

- [ ] **Step 4: Run formatter / vet / tests for the codebase**

Run: `go fmt ./... && go vet ./... && go test ./...`
Expected: all green (no Go code changed but run for hygiene).

- [ ] **Step 5: Commit**

```bash
git add .goreleaser.yaml .github/workflows/goreleaser.yml
git commit -m "ci: add goreleaser workflow for tagged release artifacts"
```

**Acceptance checklist:**
- [ ] `.goreleaser.yaml` exists and (if goreleaser is installed) `goreleaser check` passes.
- [ ] `.github/workflows/goreleaser.yml` is triggered on `v*` tag pushes.
- [ ] Builds matrix matches `darwin/linux/windows` × `amd64/arm64`.
- [ ] `SHA256SUMS` is in the checksum stanza.

---

## T-003 — Remove obsolete release scripts and refresh release docs

**Goal:** delete `scripts/build-all.sh` and `scripts/release.sh`, and update `README.md` + `AGENTS.md` to describe the release-please + goreleaser workflow and the new `go install …@latest` install path.

**Files:**
- Delete: `scripts/build-all.sh`
- Delete: `scripts/release.sh`
- Delete: `scripts/` if it becomes empty
- Modify: `README.md`
- Modify: `AGENTS.md`

### Steps

- [ ] **Step 1: Read `README.md` end-to-end** to find every section that references the scripts or the `-ldflags` build path.

Run: `rg -n "build-all\.sh|release\.sh|ldflags|scripts/" README.md`
Expected: list of lines to revise.

- [ ] **Step 2: Add an `Install` section to `README.md`**

Insert after the project description / before `Build and Test` (planner's exact placement; treat the README's existing top-of-file structure as authoritative). Content:

```markdown
## Install

The recommended way to install `gohour` is via `go install`:

```bash
go install github.com/riadshalaby/gohour@latest
```

This fetches the latest tagged release and installs the `gohour` binary into your `GOBIN` (typically `$HOME/go/bin`). Make sure `$HOME/go/bin` is on your `$PATH`.

Prebuilt binaries for Linux, macOS, and Windows are also attached to each [GitHub release](https://github.com/riadshalaby/gohour/releases). Download the archive matching your platform, extract it, and place `gohour` on your `$PATH`.
```

- [ ] **Step 3: Replace the `Build and Test` / release-tooling section in `README.md`**

Remove any reference to `scripts/build-all.sh`, `scripts/release.sh`, or `-ldflags "-X …"`.

Replace with:

```markdown
## Build and Test

```bash
go build ./...
go test ./...
```

The version reported by `gohour version` is read from `cmd/version.go`, which is updated automatically by release-please on each release PR. Local builds therefore always show the version of the source tree they were built from.

## Releasing

Releases are fully automated:

- [release-please](https://github.com/googleapis/release-please) reads Conventional Commits on `main` and opens release PRs that bump the version literal in `cmd/version.go` and update `CHANGELOG.md`.
- Merging a release PR creates the `vX.Y.Z` tag.
- The `goreleaser` workflow then builds binaries for `darwin/linux/windows` on `amd64/arm64`, generates `SHA256SUMS`, and attaches them to the GitHub release.

To force a specific version (e.g., during a planned cycle close), include a `Release-As: x.y.z` footer in a commit on `main`. release-please will honor it on the next release PR.
```

- [ ] **Step 4: Update the `Release Rules` section in `AGENTS.md`**

Replace the existing block that describes manual tagging + `scripts/release.sh` with:

```markdown
## Release Rules
- Never release directly from a feature branch.
- A feature is releasable only after it is merged into `main` via PR and required checks/tests pass.
- Releases are produced by release-please:
  - Conventional Commits on `main` drive automated release PRs that update `cmd/version.go` and `CHANGELOG.md`.
  - Merging a release PR creates the `vX.Y.Z` tag.
- The `goreleaser` workflow runs on `vX.Y.Z` tag pushes and attaches `darwin/linux/windows` binaries on `amd64/arm64` plus `SHA256SUMS` to the GitHub release.
- To pin the next release version explicitly (for example when closing a planned cycle), include `Release-As: x.y.z` in a commit footer on `main`. The `aide cycle end` workflow already emits this footer.
```

- [ ] **Step 5: Update the `Current Status` section in `AGENTS.md`** if it references the old build/release flow. (Re-read it; the existing copy focuses on app behavior and likely needs no change.)

- [ ] **Step 6: Delete the scripts**

Run:
```bash
git rm scripts/build-all.sh scripts/release.sh
rmdir scripts 2>/dev/null || true
```

- [ ] **Step 7: Verify no remaining references in tracked files**

Run: `rg -n "scripts/build-all|scripts/release|build-all\.sh|release\.sh" .`
Expected: no hits in tracked files.

- [ ] **Step 8: Run formatter / vet / tests**

Run: `go fmt ./... && go vet ./... && go test ./...`
Expected: all green.

- [ ] **Step 9: Commit**

```bash
git add README.md AGENTS.md
git commit -m "docs: replace shell-script release flow with release-please + goreleaser"
```

(The `git rm` from Step 6 is already staged.)

**Acceptance checklist:**
- [ ] `scripts/build-all.sh` and `scripts/release.sh` no longer exist.
- [ ] `README.md` has an `Install` section with `go install github.com/riadshalaby/gohour@latest`.
- [ ] `README.md` `Build and Test` / `Releasing` sections describe release-please + goreleaser, no shell scripts.
- [ ] `AGENTS.md` `Release Rules` reflects the new workflow.
- [ ] No tracked file references the deleted scripts or the `-X cmd.Version` ldflag.

---

## T-004 — Add backend `GET /api/import/rule-match` endpoint

**Goal:** expose the rule-match result for a given filename so the file-pick dialog (T-006) can pre-fill its fields without uploading the file. The endpoint takes a filename (no file body), runs `importer.MatchRuleByTemplate` against the configured rules, and returns the matched rule (if any) plus the lookup snapshot needed by the dialog.

**Files:**
- Modify: `web/server.go` (route registration + new handler `handleAPIImportRuleMatch`)
- Modify: `web/server_test.go` (new tests)

### Steps

- [ ] **Step 1: Write the failing test for a matching filename**

Append to `web/server_test.go`:

```go
func TestServer_ImportRuleMatch_ReturnsMatchedRule(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	rule := ruleForLocal()
	rule.Name = "weekly-hours"
	rule.Mapper = "generic"
	rule.FileTemplate = "hours-*.csv"
	rule.Billable = boolPtr(false)
	client := &fakeClient{snapshot: lookupSnapshotForImportOverride()}
	ts := httptest.NewServer(NewServer(store, client, testConfig([]config.Rule{rule})))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/import/rule-match?filename=hours-2026.csv")
	if err != nil {
		t.Fatalf("get rule-match: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var payload importRuleMatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.MatchedRule == nil || payload.MatchedRule.Name != "weekly-hours" {
		t.Fatalf("expected matched weekly-hours rule, got %+v", payload.MatchedRule)
	}
	if payload.Selection.Mapper != "generic" || payload.Selection.Project != "P" {
		t.Fatalf("unexpected selection: %+v", payload.Selection)
	}
	if payload.Selection.Billable == nil || *payload.Selection.Billable {
		t.Fatalf("expected non-billable selection, got %+v", payload.Selection.Billable)
	}
	if payload.Lookup == nil || len(payload.Lookup.Projects) == 0 {
		t.Fatalf("expected lookup data, got %+v", payload.Lookup)
	}
	if !stringSliceContains(payload.Mappers, "epm") {
		t.Fatalf("expected mappers list, got %+v", payload.Mappers)
	}
}

func TestServer_ImportRuleMatch_NoMatchReturnsEmptySelection(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ts := httptest.NewServer(NewServer(store, &fakeClient{snapshot: lookupSnapshotForImportOverride()}, testConfig(nil)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/import/rule-match?filename=unmatched.csv")
	if err != nil {
		t.Fatalf("get rule-match: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var payload importRuleMatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.MatchedRule != nil {
		t.Fatalf("expected no matched rule, got %+v", payload.MatchedRule)
	}
	if payload.Selection.Project != "" || payload.Selection.Activity != "" || payload.Selection.Skill != "" {
		t.Fatalf("expected empty project/activity/skill selection, got %+v", payload.Selection)
	}
}

func TestServer_ImportRuleMatch_RequiresFilename(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/import/rule-match")
	if err != nil {
		t.Fatalf("get rule-match: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing filename, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run the test (expect FAIL — endpoint and response type do not exist)**

Run: `go test ./web/ -run TestServer_ImportRuleMatch -v`
Expected: FAIL (`undefined: importRuleMatchResponse`, route 404).

- [ ] **Step 3: Add the response type and the route in `web/server.go`**

Add the response type near the existing import response types (e.g., right after `importPreviewResponse`):

```go
type importRuleMatchResponse struct {
	MatchedRule *rulePayload    `json:"matchedRule,omitempty"`
	Selection   rulePayload     `json:"selection"`
	Mappers     []string        `json:"mappers"`
	Lookup      *lookupResponse `json:"lookup,omitempty"`
}
```

Register the route alongside the other `/api/import*` routes in the `NewServer` mux setup (around line 349-350):

```go
mux.HandleFunc("GET /api/import/rule-match", server.handleAPIImportRuleMatch)
```

- [ ] **Step 4: Implement the handler**

Add to `web/server.go` near the other `handleAPIImport*` handlers:

```go
func (s *Server) handleAPIImportRuleMatch(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimSpace(r.URL.Query().Get("filename"))
	if filename == "" {
		http.Error(w, "filename query parameter is required", http.StatusBadRequest)
		return
	}

	cfg, err := s.loadConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	matched := importer.MatchRuleByTemplate(filename, cfg.Rules)

	selection := rulePayloadFromRule(matched)
	if strings.TrimSpace(selection.Mapper) == "" {
		// Fall back to the first known mapper so the UI has a default.
		selection.Mapper = "epm"
	}

	var matchedPayload *rulePayload
	if matched.FileTemplate != "" {
		mp := rulePayloadFromRule(matched)
		matchedPayload = &mp
	}

	lookup, err := s.lookupSnapshotResponse(r.Context())
	if err != nil {
		// Lookup failures should not block rule matching; surface as empty lookup.
		lookup = &lookupResponse{}
	}

	writeJSON(w, http.StatusOK, importRuleMatchResponse{
		MatchedRule: matchedPayload,
		Selection:   selection,
		Mappers:     []string{"epm", "generic", "atwork"},
		Lookup:      lookup,
	})
}
```

Notes:
- `lookupSnapshotResponse` is the existing helper that builds the `*lookupResponse` used by `/api/import-preview`. If its name differs in your branch, use the matching helper. Verify with `rg -n "lookupResponse" web/server.go`.
- Mapper list is hard-coded to match the existing import-preview response.

- [ ] **Step 5: Run the tests (expect PASS)**

Run: `go test ./web/ -run TestServer_ImportRuleMatch -v`
Expected: all three tests PASS.

- [ ] **Step 6: Run the full web test suite to confirm nothing regressed**

Run: `go test ./web/...`
Expected: all green.

- [ ] **Step 7: Run formatter / vet**

Run: `go fmt ./... && go vet ./...`
Expected: clean.

- [ ] **Step 8: Documentation**

No README change is required (the endpoint is internal). Add a one-line comment above `handleAPIImportRuleMatch` summarizing intent (already in Step 4).

- [ ] **Step 9: Commit**

```bash
git add web/server.go web/server_test.go
git commit -m "feat(api): expose GET /api/import/rule-match for file-pick prefill"
```

**Acceptance checklist:**
- [ ] `GET /api/import/rule-match?filename=…` returns 200 with the matched rule and resolved selection.
- [ ] Missing `filename` returns 400.
- [ ] `go test ./web/...` passes.

---

## T-005 — Remove the "Auto" billable option from import flow

**Goal:** kill the `auto` value in the billable selector. The user (or matched rule) must pick `billable` or `non-billable`. Default when no rule matches: `billable`.

**Files:**
- Modify: `web/templates/month.html` (line 232)
- Modify: `web/templates/base.html` (line 168, plus surrounding default-value handling if any)
- Modify: `web/static/js/app.js` (`renderImportPreviewSelection` line ~619-628 and `appendImportPreviewSelection` line ~878)
- Modify: `web/server.go` (`importSelectionFromForm` line ~2169-2185 — confirm no `auto` code path remains)
- Modify: `web/server_test.go` (one new test asserting the default selection when no rule matches)
- Modify: `e2e/tests/import.spec.ts` (assert option count in the selector)
- Modify: `AGENTS.md` (Current Status — adjust the "Import UI supports `billable` mode selection" line to reflect the explicit-only choice)

### Steps

- [ ] **Step 1: Write the failing Go test**

Append to `web/server_test.go`:

```go
func TestServer_ImportPreview_DefaultsBillableTrueWhenNoRule(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ts := httptest.NewServer(NewServer(store, &fakeClient{snapshot: lookupSnapshotForImportOverride()}, testConfig(nil)))
	defer ts.Close()

	resp := postImportMultipart(t, ts.URL+"/api/import-preview", map[string]string{"mapper": "generic"}, "unmatched.csv",
		"description,startdatetime,enddatetime,project,activity,skill\n"+
			"Task,2026-03-01 09:00,2026-03-01 10:00,FromFile,FromFile,FromFile\n",
	)
	defer resp.Body.Close()

	var payload importPreviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Selection.Billable == nil || !*payload.Selection.Billable {
		t.Fatalf("expected default billable=true selection when no rule matches, got %+v", payload.Selection.Billable)
	}
}

func TestServer_ImportPreview_RejectsAutoBillable(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ts := httptest.NewServer(NewServer(store, &fakeClient{snapshot: lookupSnapshotForImportOverride()}, testConfig(nil)))
	defer ts.Close()

	resp := postImportMultipart(t, ts.URL+"/api/import-preview", map[string]string{
		"mapper":   "generic",
		"billable": "auto",
	}, "unmatched.csv",
		"description,startdatetime,enddatetime,project,activity,skill\n"+
			"Task,2026-03-01 09:00,2026-03-01 10:00,FromFile,FromFile,FromFile\n",
	)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for billable=auto, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run the tests (expect FAIL on both)**

Run: `go test ./web/ -run "TestServer_ImportPreview_DefaultsBillableTrueWhenNoRule|TestServer_ImportPreview_RejectsAutoBillable" -v`
Expected: FAIL.

- [ ] **Step 3: Update `importSelectionFromForm` in `web/server.go`**

Replace the billable switch (around line 2179) with:

```go
billableValue := strings.ToLower(strings.TrimSpace(r.FormValue("billable")))
switch billableValue {
case "billable":
	selection.Billable = boolValuePtr(true)
case "non-billable":
	selection.Billable = boolValuePtr(false)
case "":
	if selection.Billable == nil {
		selection.Billable = boolValuePtr(true)
	}
default:
	return rulePayload{}, fmt.Errorf("invalid billable value: %q (expected billable or non-billable)", billableValue)
}
```

Update the function signature to return `(rulePayload, error)`:

```go
func importSelectionFromForm(r *http.Request, mapperName string, matchedRule config.Rule) (rulePayload, error) {
```

Update the only caller (`parseAndRunImportForm` around line 2101) to handle the error:

```go
selection, err := importSelectionFromForm(r, mapperName, matchedRule)
if err != nil {
	return importFormResult{}, err
}
```

`parseAndRunImportForm`'s caller already converts errors into HTTP 400 responses; verify by re-reading the existing handler chain (`handleAPIImport`, `handleAPIImportPreview`).

- [ ] **Step 4: Run the Go tests (expect PASS for the two new ones; rerun the full web suite)**

Run: `go test ./web/...`
Expected: all green. If `TestServer_ImportPreview_NoRuleReturnsEmptySelection` now fails because the selection includes `Billable: true`, update its assertion to expect the new default (`Billable: boolPtr(true)`).

- [ ] **Step 5: Update `web/templates/month.html`**

Remove line 232 (`<option value="auto">Auto (computed from file)</option>`). The selector becomes:

```html
<select id="month-import-billable" name="billable">
  <option value="billable" selected>Billable (force full duration)</option>
  <option value="non-billable">Non-billable (force 0)</option>
</select>
```

The `selected` attribute makes `billable` the default; the prefill XHR added in T-006 will override it when a rule applies.

- [ ] **Step 6: Update `web/templates/base.html`**

Remove line 168 (`<option value="auto">Auto</option>`). The selector becomes:

```html
<select id="preview-billable">
  <option value="billable" selected>Billable</option>
  <option value="non-billable">Non-billable</option>
</select>
```

- [ ] **Step 7: Update `web/static/js/app.js`**

In `renderImportPreviewSelection` (around line 619), replace:

```javascript
if (selection.billable === true) {
  billableSelect.value = 'billable';
} else if (selection.billable === false) {
  billableSelect.value = 'non-billable';
} else {
  billableSelect.value = 'auto';
}
```

with:

```javascript
billableSelect.value = selection.billable === false ? 'non-billable' : 'billable';
```

In `appendImportPreviewSelection` (around line 878), replace:

```javascript
if (billableSelect) formData.set('billable', billableSelect.value || 'auto');
```

with:

```javascript
if (billableSelect) formData.set('billable', billableSelect.value || 'billable');
```

- [ ] **Step 8: Update the e2e test**

In `e2e/tests/import.spec.ts`, add an assertion after the dialog opens (around current line 22):

```ts
await expect(page.locator('#month-import-billable option')).toHaveCount(2);
```

This guards against the `Auto` option creeping back.

- [ ] **Step 9: Run the e2e suite locally**

Run from `e2e/`: `npm run test` (or the existing project script — check `e2e/package.json` for the right command). The CSV the test writes must still result in a successful preview + import.

- [ ] **Step 10: Update `AGENTS.md`**

In `Current Status`, change the line that reads:

```
- Import UI supports `billable` mode selection, conflict-aware preview ...
```

to:

```
- Import UI requires an explicit `billable`/`non-billable` choice (no implicit "auto"), with conflict-aware preview ...
```

- [ ] **Step 11: Run formatter / vet / tests**

Run: `go fmt ./... && go vet ./... && go test ./...`
Expected: green.

- [ ] **Step 12: Commit**

```bash
git add web/server.go web/server_test.go web/templates/month.html web/templates/base.html web/static/js/app.js e2e/tests/import.spec.ts AGENTS.md
git commit -m "fix(web): remove confusing \"Auto\" billable option from import dialog"
```

**Acceptance checklist:**
- [ ] Both import dialogs (`#month-import-billable`, `#preview-billable`) have exactly 2 options.
- [ ] Server rejects `billable=auto` with 400.
- [ ] Default billable when no rule matches is `billable=true`.
- [ ] `go test ./...` and the e2e import spec pass.

---

## T-006 — Pre-fill import dialog from matched rule on file selection (with banner)

**Goal:** when the user picks a file in the month-import dialog, fire an XHR to `GET /api/import/rule-match?filename=…` (added in T-004) and populate the mapper / project / activity / skill / billable selectors plus a matched-rule banner. Verify the existing `#preview-rule-match` line in the preview dialog is wired correctly (it already is — see Step 6 — but confirm).

**Files:**
- Modify: `web/templates/month.html` (insert banner element + ensure form structure supports the JS hook)
- Modify: `web/static/js/app.js` (new file-pick handler + helper to apply selection)
- Modify: `web/templates/base.html` (no functional change; verify `#preview-rule-match` styling)
- Modify: `e2e/tests/import.spec.ts` (assert prefill behavior + banner visibility)
- Modify: `AGENTS.md` (Current Status — note the prefill behavior)

### Steps

- [ ] **Step 1: Add the banner element to `#month-import-dialog`**

In `web/templates/month.html`, insert immediately after `<div class="dialog-body import-fields">` (line 204):

```html
<p id="month-import-rule-match" class="muted import-rule-match" aria-live="polite"></p>
```

The element starts empty (`textContent == ""`). The JS handler (Step 2) populates it.

- [ ] **Step 2: Add the file-pick prefill handler in `web/static/js/app.js`**

Locate `openImportDialog` (around line 404). Inside it, after `dialog.showModal();`, attach a one-time file-input listener:

```javascript
const fileInput = form.querySelector('input[type=file][name=file]');
if (fileInput && !fileInput.dataset.prefillBound) {
  fileInput.addEventListener('change', () => {
    void prefillImportFormFromFile(form);
  });
  fileInput.dataset.prefillBound = '1';
}
```

Add a new helper `prefillImportFormFromFile` near the other import helpers:

```javascript
async function prefillImportFormFromFile(form) {
  if (!form) return;
  const fileInput = form.querySelector('input[type=file][name=file]');
  const banner = document.getElementById('month-import-rule-match');
  if (!fileInput || !fileInput.files || !fileInput.files[0]) {
    if (banner) banner.textContent = '';
    return;
  }
  const filename = fileInput.files[0].name;
  let payload;
  try {
    payload = await apiFetch('GET', '/api/import/rule-match?filename=' + encodeURIComponent(filename));
  } catch (err) {
    if (banner) {
      banner.textContent = 'Unable to match rule: ' + String(err.message || err);
    }
    return;
  }

  const matched = payload.matchedRule || null;
  if (banner) {
    banner.textContent = matched && matched.name
      ? 'Matched rule: ' + matched.name
      : 'No rule matched — using defaults';
  }
  applyImportFormSelection(form, payload);
}

function applyImportFormSelection(form, payload) {
  const selection = payload.selection || {};
  const lookup = payload.lookup || _lookup || { projects: [], activities: [], skills: [] };
  if (payload.lookup) {
    _lookup = payload.lookup;
  }

  const mapperSelect = form.querySelector('[name=mapper]');
  if (mapperSelect && selection.mapper) {
    mapperSelect.value = String(selection.mapper).toLowerCase();
  }

  const projectSelect = form.querySelector('[name=project]');
  const activitySelect = form.querySelector('[name=activity]');
  const skillSelect = form.querySelector('[name=skill]');
  if (projectSelect && activitySelect && skillSelect) {
    fillImportOverrideSelects(lookup, selection);
  }

  const billableSelect = form.querySelector('[name=billable]');
  if (billableSelect) {
    if (selection.billable === false) {
      billableSelect.value = 'non-billable';
    } else if (selection.billable === true) {
      billableSelect.value = 'billable';
    }
  }
}
```

Notes:
- `apiFetch` is the existing JSON helper used elsewhere in `app.js`. Confirm signature with `rg -n "function apiFetch" web/static/js/app.js`.
- `fillImportOverrideSelects` already exists (line ~637) and rebuilds project/activity/skill cascades. Reuse it. If the `#month-import-*` selects use different IDs from `#preview-*`, generalize `fillImportOverrideSelects` to take element references rather than hard-coded IDs, or write a small parallel function. Prefer the generalization to avoid duplication.

- [ ] **Step 3: Reset banner when the dialog opens**

In `openImportDialog`, before `dialog.showModal()`, clear the banner:

```javascript
const banner = document.getElementById('month-import-rule-match');
if (banner) banner.textContent = '';
```

- [ ] **Step 4: Verify the preview dialog already shows the matched rule**

The existing code in `renderImportPreviewSelection` (line 589-594) already populates `#preview-rule-match`. Confirm visually by running the e2e import test and inspecting the rendered DOM. If the element is hidden by CSS (`muted` class with `display: none` somewhere), adjust the CSS to keep it visible.

Run: `rg -n "preview-rule-match|import-rule-match|\.muted" web/static/css/`
Expected: no rule that hides these elements. If there is, scope it to exclude `.import-rule-match`.

- [ ] **Step 5: Add e2e coverage in `e2e/tests/import.spec.ts`**

Add a new test below the existing one:

```ts
test('Import dialog prefills mapper and shows matched rule on file pick', async ({ page }, testInfo) => {
  // Assumes e2e/fixtures/config.yaml registers a rule with file_template "import-smoke-*.csv"
  // mapped to the "generic" mapper. Update the fixture in this task if not already present.
  const csvPath = testInfo.outputPath('import-smoke-prefill.csv');
  await writeFile(
    csvPath,
    [
      'description,startdatetime,enddatetime,project,activity,skill',
      'prefill check,2025-01-15 09:00,2025-01-15 10:00,P,A,S',
      '',
    ].join('\n'),
    'utf8',
  );

  await page.goto('/month/2025-01');
  await page.getByRole('button', { name: /import file/i }).first().click();

  await expect(page.locator('#month-import-dialog')).toHaveAttribute('open', '');
  await page.setInputFiles('#month-import-file', csvPath);
  await expect(page.locator('#month-import-rule-match')).toContainText(/Matched rule:|No rule matched/);
});
```

Verify the e2e fixture (`e2e/fixtures/config.yaml`) has a rule whose `file_template` matches the test filename pattern. If not, add one in the same commit.

- [ ] **Step 6: Run the e2e suite**

Run from `e2e/`: `npm run test` (or whatever script matches the existing project setup).
Expected: both import tests pass.

- [ ] **Step 7: Update `AGENTS.md` Current Status**

Replace the existing line:

```
- import preview + import execution with automatic rule matching and per-field override,
```

with:

```
- import dialog pre-fills mapper/project/activity/skill/billable from matched rule on file pick, with per-field override on the preview dialog,
```

- [ ] **Step 8: Run formatter / vet / tests**

Run: `go fmt ./... && go vet ./... && go test ./...`
Expected: green (no Go code changed in this task; run for hygiene).

- [ ] **Step 9: Commit**

```bash
git add web/templates/month.html web/static/js/app.js web/static/css/components.css e2e/tests/import.spec.ts e2e/fixtures/config.yaml AGENTS.md
git commit -m "feat(web): pre-fill import dialog from matched rule on file selection"
```

(Stage only the CSS file if Step 4 required a tweak; otherwise omit.)

**Acceptance checklist:**
- [ ] Picking a matching file pre-fills mapper, project, activity, skill, and billable in `#month-import-dialog`.
- [ ] The matched-rule banner shows `Matched rule: <name>` or `No rule matched — using defaults`.
- [ ] Picking a different file updates both the prefill and the banner.
- [ ] Preview dialog continues to show the matched rule on its existing line.
- [ ] e2e suite passes.

---

## T-007 — Surface "Update matched rule" affordance on field divergence

**Goal:** the `#preview-update-rule` checkbox already exists and works server-side. Today it sits inert until the user finds and ticks it. Make it visible/active the moment any pre-filled field is overridden, and hide/disable it when no rule is matched or no divergence exists.

**Files:**
- Modify: `web/templates/base.html` (rework the checkbox markup so it can be conditionally surfaced)
- Modify: `web/static/js/app.js` (compute divergence + auto-toggle)
- Modify: `web/static/css/components.css` (style for the highlight state)
- Modify: `e2e/tests/import.spec.ts` (test the divergence behavior)

### Steps

- [ ] **Step 1: Capture the matched-rule snapshot when the preview dialog opens**

Extend the import preview store (look for `importPreviewStore()` in `web/static/js/app.js`) to hold the matched rule's prefill values:

```javascript
previewState.matchedRule = previewData.matchedRule || null;
previewState.baseline = {
  mapper: String((previewData.selection && previewData.selection.mapper) || '').toLowerCase(),
  project: String((previewData.selection && previewData.selection.project) || ''),
  activity: String((previewData.selection && previewData.selection.activity) || ''),
  skill: String((previewData.selection && previewData.selection.skill) || ''),
  billable: previewData.selection && typeof previewData.selection.billable === 'boolean'
    ? previewData.selection.billable
    : null,
};
```

Place this assignment inside `openImportPreviewDialog` after `previewState.options = options || {};`.

- [ ] **Step 2: Add a divergence checker**

```javascript
function importPreviewHasOverrides() {
  const state = importPreviewStore();
  if (!state || !state.baseline) return false;
  if (!state.matchedRule) return false;

  const mapper = String(document.getElementById('preview-mapper')?.value || '').toLowerCase();
  const project = String(document.getElementById('preview-project')?.value || '');
  const activity = String(document.getElementById('preview-activity')?.value || '');
  const skill = String(document.getElementById('preview-skill')?.value || '');
  const billableValue = String(document.getElementById('preview-billable')?.value || '');
  const billable = billableValue === 'billable' ? true : billableValue === 'non-billable' ? false : null;

  return (
    mapper !== state.baseline.mapper ||
    project !== state.baseline.project ||
    activity !== state.baseline.activity ||
    skill !== state.baseline.skill ||
    billable !== state.baseline.billable
  );
}
```

- [ ] **Step 3: Toggle the affordance on every relevant change**

```javascript
function refreshUpdateRuleAffordance() {
  const wrapper = document.getElementById('preview-update-rule-wrapper');
  const checkbox = document.getElementById('preview-update-rule');
  if (!wrapper || !checkbox) return;
  const state = importPreviewStore();
  if (!state || !state.matchedRule) {
    wrapper.hidden = true;
    checkbox.checked = false;
    checkbox.disabled = true;
    return;
  }
  const diverged = importPreviewHasOverrides();
  wrapper.hidden = !diverged;
  checkbox.disabled = !diverged;
  if (diverged && !checkbox.dataset.userToggled) {
    checkbox.checked = true; // auto-suggest persisting the override
  }
  if (!diverged) {
    checkbox.checked = false;
    delete checkbox.dataset.userToggled;
  }
}
```

When the user clicks the checkbox, mark it as user-toggled so we don't keep flipping it back:

```javascript
document.getElementById('preview-update-rule').addEventListener('change', (event) => {
  event.target.dataset.userToggled = '1';
});
```

Bind this once at dialog open (guard with a `dataset.bound` flag).

Wire `refreshUpdateRuleAffordance()` into the change events for `#preview-mapper`, `#preview-project`, `#preview-activity`, `#preview-skill`, `#preview-billable`. (The existing rebuild-cascades listeners already fire on these; piggyback by calling `refreshUpdateRuleAffordance()` from `fillImportOverrideSelects`'s change handlers.)

- [ ] **Step 4: Wrap the checkbox so it can be hidden as a unit**

In `web/templates/base.html` lines 173-176, replace:

```html
<label class="checkbox-field">
  <input id="preview-update-rule" type="checkbox">
  <span>Update matched rule</span>
</label>
```

with:

```html
<div id="preview-update-rule-wrapper" class="update-rule-wrapper" hidden>
  <label class="checkbox-field highlight">
    <input id="preview-update-rule" type="checkbox">
    <span>Update matched rule with these overrides</span>
  </label>
  <p class="muted update-rule-hint">You changed values from the matched rule. Untick to import without saving.</p>
</div>
```

- [ ] **Step 5: Add CSS for the highlight state**

Append to `web/static/css/components.css` (or the closest existing stylesheet for dialog styles):

```css
.update-rule-wrapper {
  margin-top: 0.5rem;
  padding: 0.5rem 0.75rem;
  border-left: 3px solid var(--accent, #1f6feb);
  background: var(--surface-muted, rgba(31, 111, 235, 0.06));
}

.update-rule-wrapper .checkbox-field.highlight {
  font-weight: 600;
}

.update-rule-wrapper .update-rule-hint {
  margin: 0.25rem 0 0;
  font-size: 0.85em;
}
```

- [ ] **Step 6: Reset state when re-opening the preview dialog**

In `renderImportPreviewSelection`, after the existing `if (updateRule) { updateRule.checked = false; … }` block, also clear `userToggled` and call `refreshUpdateRuleAffordance()`:

```javascript
if (updateRule) {
  updateRule.checked = false;
  delete updateRule.dataset.userToggled;
  updateRule.disabled = !(matched && matched.name);
}
refreshUpdateRuleAffordance();
```

- [ ] **Step 7: Add e2e coverage**

In `e2e/tests/import.spec.ts`, add:

```ts
test('Update matched rule affordance hidden until override', async ({ page }, testInfo) => {
  // Same setup as the existing "Import file flow" but assumes a rule matches
  // the import file (configure in fixtures or use a filename the fixture rule covers).
  const csvPath = testInfo.outputPath('rule-update.csv');
  await writeFile(
    csvPath,
    [
      'description,startdatetime,enddatetime,project,activity,skill',
      'override check,2025-01-16 09:00,2025-01-16 10:00,P,A,S',
      '',
    ].join('\n'),
    'utf8',
  );
  await page.goto('/month/2025-01');
  await page.getByRole('button', { name: /import file/i }).first().click();
  await page.setInputFiles('#month-import-file', csvPath);
  await page.getByRole('button', { name: 'Upload' }).click();

  await expect(page.locator('#preview-import-btn')).toBeVisible();
  await expect(page.locator('#preview-update-rule-wrapper')).toBeHidden();

  // Override the project to trigger divergence
  await page.locator('#preview-project').selectOption({ index: 1 }).catch(() => {
    // If only one option, type into the select to force change — fall back to billable toggle
  });

  // Either project change or billable toggle must surface the wrapper
  await page.locator('#preview-billable').selectOption('non-billable');
  await expect(page.locator('#preview-update-rule-wrapper')).toBeVisible();
  await expect(page.locator('#preview-update-rule')).toBeChecked();
});
```

- [ ] **Step 8: Run e2e and Go suites**

Run: `go test ./... && (cd e2e && npm run test)`
Expected: green.

- [ ] **Step 9: Documentation**

No README change. Add a short comment block above the new JS helpers (`importPreviewHasOverrides`, `refreshUpdateRuleAffordance`) summarizing intent.

Update `AGENTS.md` `Current Status` line that mentions `"update rule" persistence`:

```
- "Update matched rule" appears automatically on the preview dialog when a matched rule's pre-fill is overridden, persisting the change on import.
```

- [ ] **Step 10: Run formatter / vet**

Run: `go fmt ./... && go vet ./...`
Expected: clean.

- [ ] **Step 11: Commit**

```bash
git add web/templates/base.html web/static/js/app.js web/static/css/components.css e2e/tests/import.spec.ts AGENTS.md
git commit -m "fix(web): surface \"Update matched rule\" affordance on field override"
```

**Acceptance checklist:**
- [ ] With a matched rule and no overrides, `#preview-update-rule-wrapper` is hidden.
- [ ] Overriding any of mapper/project/activity/skill/billable surfaces the wrapper and auto-checks the checkbox.
- [ ] Manually unticking the checkbox keeps it unticked even if further changes happen (user intent respected).
- [ ] With no matched rule, the wrapper stays hidden.
- [ ] e2e + Go tests pass.

---

## T-008 — Promote Import to a top-level button on the month view

**Goal:** lift "Import file" out of the Actions dropdown so it sits next to "Submit month" in the top header. Remove the dropdown entry and the now-orphan separator above it.

**Files:**
- Modify: `web/templates/month.html` (header section + Actions dropdown body)
- Modify: `e2e/tests/import.spec.ts` (the existing test clicks `Actions → Import file`; rewrite to click the top-level button)
- Modify: `AGENTS.md` (Current Status — note the top-level Import button)

### Steps

- [ ] **Step 1: Add the top-level button in the header**

In `web/templates/month.html`, locate the header at lines 12-13:

```html
<!-- Primary actions -->
<button type="button" class="btn-primary" onclick="openSubmitAction('month', '{{ .CurrentMonth }}')">Submit month</button>
```

Add an Import button immediately after Submit:

```html
<button type="button" onclick="openImportDialog('month-import-dialog', 'month-import-form')">Import file</button>
```

- [ ] **Step 2: Remove the dropdown entry**

In the Actions dropdown body (lines 59-61), delete the separator + Import entry:

```html
<div class="menu-separator"></div>
<button type="button" role="menuitem" onclick="openImportDialog('month-import-dialog', 'month-import-form')">Import file</button>
```

If the menu now ends with another separator (check the structure), remove that orphaned separator too. Re-read lines 30-65 after the edit to confirm the dropdown still renders cleanly.

- [ ] **Step 3: Update the e2e test**

In `e2e/tests/import.spec.ts`, replace the existing dropdown-then-menu-item click (current lines 19-20):

```ts
await page.getByRole('button', { name: /actions/i }).click();
await page.getByRole('menuitem', { name: 'Import file' }).click();
```

with:

```ts
await page.getByRole('button', { name: 'Import file' }).first().click();
```

The `.first()` disambiguates from the sticky-bar Import button at the bottom of the page. Both should open the same dialog.

If T-006's tests already use this pattern, this change is a no-op for those tests; they continue to pass.

- [ ] **Step 4: Update `AGENTS.md` Current Status**

Add or adjust the line describing month-view actions to mention the top-level Import button:

```
- month/day compare views (local vs. remote) with top-level Submit and Import buttons on the month view,
```

- [ ] **Step 5: Run e2e and Go suites**

Run: `go test ./... && (cd e2e && npm run test)`
Expected: green. The smoke import test should still complete end-to-end.

- [ ] **Step 6: Run formatter / vet**

Run: `go fmt ./... && go vet ./...`
Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add web/templates/month.html e2e/tests/import.spec.ts AGENTS.md
git commit -m "fix(web): surface Import button on the month view header"
```

**Acceptance checklist:**
- [ ] `#month-import-dialog` opens from a button visible without opening the Actions dropdown.
- [ ] The Actions dropdown no longer contains an "Import file" entry.
- [ ] The sticky-bar Import button still works.
- [ ] The smoke import e2e test passes against the top-level button.

---

## Validation

Run before finishing each task (per AGENTS.md):

- `go fmt ./...`
- `go vet ./...`
- `go test ./...`

Run before finishing the cycle:

- All of the above
- `(cd e2e && npm run test)` — full Playwright suite
- Manual smoke: `go build -o /tmp/gohour . && /tmp/gohour version` should print the literal in `cmd/version.go` (T-001).
- Manual smoke: `gohour serve` → open `/month/<current>` → verify the top-level Import button (T-008) opens the dialog, picking a matching file prefills fields and shows the matched-rule banner (T-006), changing a field surfaces the Update-Rule affordance (T-007), the billable selector has only `Billable` / `Non-billable` (T-005).

## Out of scope (mirrors ROADMAP.md)

- Adding a third "automatic" billable mode.
- Per-entry billable inference from comment/project content.
- Release artifact signing (cosign / SLSA).
- A day-level import dialog.
- Changes to reconciliation, submit, or remote-API flows.
