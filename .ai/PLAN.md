# Plan: v0.3.2 — Go Module Path Migration for `go install` Support

## Goal

Make `gohour` installable via `go install github.com/riadshalaby/gohour@latest`
by migrating the Go module path from `gohour` to `github.com/riadshalaby/gohour`.

## Definition of Done

- `go install github.com/riadshalaby/gohour@v0.3.2` succeeds on a clean machine.
- Installed binary runs and reports correct version (`gohour version`).
- README documents install path and first-run flow.
- All existing tests pass (`go test ./...`).

---

## Phase 1: Module and Import Path Migration

### 1.1 Update `go.mod` module declaration

**File:** `go.mod`

**Change:** Line 1 from `module gohour` to `module github.com/riadshalaby/gohour`.

### 1.2 Rewrite all internal imports

Mechanically replace `"gohour/` with `"github.com/riadshalaby/gohour/` in all
44 Go source files. The 11 internal packages to rewrite:

| Old Import                  | New Import                                          |
|-----------------------------|-----------------------------------------------------|
| `gohour/cmd`                | `github.com/riadshalaby/gohour/cmd`                 |
| `gohour/config`             | `github.com/riadshalaby/gohour/config`              |
| `gohour/importer`           | `github.com/riadshalaby/gohour/importer`            |
| `gohour/internal/timeutil`  | `github.com/riadshalaby/gohour/internal/timeutil`   |
| `gohour/onepoint`           | `github.com/riadshalaby/gohour/onepoint`            |
| `gohour/output`             | `github.com/riadshalaby/gohour/output`              |
| `gohour/reconcile`          | `github.com/riadshalaby/gohour/reconcile`           |
| `gohour/storage`            | `github.com/riadshalaby/gohour/storage`             |
| `gohour/submitter`          | `github.com/riadshalaby/gohour/submitter`           |
| `gohour/web`                | `github.com/riadshalaby/gohour/web`                 |
| `gohour/worklog`            | `github.com/riadshalaby/gohour/worklog`             |

**Affected files by package (44 files):**

- `main.go` (1)
- `cmd/` (16): `auth_helpers.go`, `auth_helpers_test.go`, `auth_login.go`,
  `auth_show_cookies.go`, `config_create_test.go`, `config_edit.go`,
  `config_rule_add.go`, `config_rule_add_test.go`, `config_show.go`,
  `export.go`, `import.go`, `import_test.go`, `reconcile.go`, `root.go`,
  `serve.go`, `serve_e2e_stub.go`, `serve_test.go`, `submit.go`, `submit_test.go`
- `importer/` (9): `mapper.go`, `mapper_atwork.go`, `mapper_atwork_test.go`,
  `mapper_epm.go`, `mapper_epm_test.go`, `mapper_generic.go`,
  `mapper_generic_test.go`, `service.go`, `service_test.go`
- `output/` (5): `csv_writer.go`, `daily_summary.go`, `daily_summary_test.go`,
  `excel_writer.go`, `writer.go`
- `reconcile/` (2): `service.go`, `service_test.go`
- `storage/` (2): `sqlite_store.go`, `sqlite_store_test.go`
- `submitter/` (2): `service.go`, `service_test.go`
- `web/` (4): `data.go`, `data_test.go`, `server.go`, `server_test.go`

**Method:** Use `sed` or editor find-replace across all `*.go` files:
```
"gohour/  ->  "github.com/riadshalaby/gohour/
```

### 1.3 Run `go mod tidy`

After import rewrite, run `go mod tidy` to update `go.sum` and verify module
graph resolves cleanly.

### 1.4 Validate with `go vet ./...` and `go test ./...`

Confirm no import errors, no build failures, and all existing tests pass.

---

## Phase 2: Build Script and Metadata Updates

### 2.1 Update ldflags in `scripts/build-all.sh`

**File:** `scripts/build-all.sh` (line 26)

**Change:**
```
LDFLAGS="-X gohour/cmd.Version=${VERSION}"
```
to:
```
LDFLAGS="-X github.com/riadshalaby/gohour/cmd.Version=${VERSION}"
```

### 2.2 Update ldflags comment in `cmd/version.go`

**File:** `cmd/version.go` (lines 9-10)

**Change comment** from:
```go
// go build -ldflags "-X gohour/cmd.Version=v0.2.2"
```
to:
```go
// go build -ldflags "-X github.com/riadshalaby/gohour/cmd.Version=vX.Y.Z"
```

### 2.3 Verify `scripts/release.sh`

Confirm `release.sh` calls `build-all.sh` and inherits the corrected ldflags
path. No direct changes expected, but verify end-to-end.

### 2.4 Scan for any other references to old module path

Grep the entire repo for remaining `"gohour/` or `-X gohour/` strings to catch
any stragglers in comments, docs, or scripts. Also check CLAUDE.md and ROADMAP.md
for stale references (informational only — not code-breaking).

---

## Phase 3: Documentation Updates

### 3.1 Add Install section to `README.md`

Add a prominent section near the top of README.md:

```markdown
## Install

### Via `go install` (recommended)

Requires Go 1.25+:

```bash
go install github.com/riadshalaby/gohour@latest
```

The binary is placed in `$GOPATH/bin` (or `$GOBIN` if set). Ensure this
directory is in your `PATH`.

### Build from source

```bash
git clone https://github.com/riadshalaby/gohour.git
cd gohour
go build -o gohour .
```
```

### 3.2 Update existing build examples in README.md

Review the current Build section (lines 25-41) and ensure:
- `go build` examples use the new module path for ldflags if shown.
- Cross-compilation examples remain valid.

### 3.3 Update CLAUDE.md ldflags example

**File:** `CLAUDE.md` (Release Rules section)

**Change:**
```
go build -ldflags "-X gohour/cmd.Version=vX.Y.Z" ...
```
to:
```
go build -ldflags "-X github.com/riadshalaby/gohour/cmd.Version=vX.Y.Z" ...
```

---

## Phase 4: Verification

### 4.1 Full test suite

```bash
go test ./...
```

### 4.2 Build and version check

```bash
go build -ldflags "-X github.com/riadshalaby/gohour/cmd.Version=v0.3.2-test" -o gohour .
./gohour version
```

Confirm output shows `v0.3.2-test`.

### 4.3 Local `go install` simulation

```bash
go install -ldflags "-X github.com/riadshalaby/gohour/cmd.Version=v0.3.2-test" .
gohour version
```

### 4.4 Run e2e smoke tests (if applicable)

```bash
cd e2e && npx playwright test
```

Verify the web UI still works with the rebuilt binary.

---

## Implementation Order

| Step | Phase | Description                                   | Validates             |
|------|-------|-----------------------------------------------|-----------------------|
| 1    | 1.1   | Update `go.mod` module path                   | —                     |
| 2    | 1.2   | Rewrite all 44 Go files' imports              | —                     |
| 3    | 1.3   | Run `go mod tidy`                             | Module graph          |
| 4    | 1.4   | Run `go vet ./...` + `go test ./...`          | Build + tests         |
| 5    | 2.1   | Update ldflags in `build-all.sh`              | —                     |
| 6    | 2.2   | Update comment in `version.go`                | —                     |
| 7    | 2.3   | Verify `release.sh`                           | End-to-end build      |
| 8    | 2.4   | Grep for remaining old-path references        | Completeness          |
| 9    | 3.1   | Add Install section to README.md              | —                     |
| 10   | 3.2   | Update build examples in README.md            | —                     |
| 11   | 3.3   | Update CLAUDE.md ldflags reference            | —                     |
| 12   | 4.1-4 | Full verification pass                        | Everything            |

All changes are in a single logical commit (or two: code migration + docs).

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Missed import in a file | Build failure | Grep for `"gohour/` after migration; `go vet` catches it |
| ldflags path not updated | `gohour version` shows empty | Verify with explicit build before release |
| `go.sum` drift | Build failure on clean checkout | `go mod tidy` + verify `go build` from scratch |
| `@latest` resolves wrong tag | Users get old version | Tag only on `main` merge commit per release rules |
| Go version constraint (1.25) | Users on older Go can't install | Document minimum version in README |
