# gohour

`gohour` is a local web UI for importing time-tracking files, reviewing local worklogs against OnePoint, editing local entries, and submitting day or month batches to OnePoint.

## Features

- Local browser UI for month/day review, import preview, local edit/delete, copy-from-remote, and submit workflows
- SQLite-backed worklog storage
- OnePoint lookup, comparison, and submit integration
- Import support for EPM-style Excel files, generic CSV, and atwork TSV exports
- Fixed data directory under `$HOME/.gohour`

## Install

The recommended way to install `gohour` is via `go install`:

```bash
go install github.com/riadshalaby/gohour@latest
```

This fetches the latest tagged release and installs the `gohour` binary into your `GOBIN` (typically `$HOME/go/bin`). Make sure `$HOME/go/bin` is on your `$PATH`.

Prebuilt binaries for Linux, macOS, and Windows are also attached to each [GitHub release](https://github.com/riadshalaby/gohour/releases). Download the archive matching your platform, extract it, and place `gohour` on your `$PATH`.

## Commands

The CLI surface is intentionally small:

```bash
gohour serve
gohour version
```

`gohour serve` starts the local web UI. `gohour version` prints the build version.

## Quick Start

Create or migrate the app data, then open the UI:

```bash
gohour serve
```

The first run uses `$HOME/.gohour/config.yaml`. If older files are found in the current directory or home directory, gohour prompts to migrate them into `$HOME/.gohour`.

## Data Files

Default paths:

- Config: `$HOME/.gohour/config.yaml`
- SQLite database: `$HOME/.gohour/gohour.db`
- OnePoint auth state: `$HOME/.gohour/onepoint-auth-state.json`

For automated tests or isolated runs, set `GOHOUR_DATA_DIR` to use a different directory.

## Configuration

The config file contains the OnePoint home URL and import rules:

```yaml
onepoint:
  url: "https://onepoint.virtual7.io/onepoint/faces/home"

rules:
  - name: "generic-local"
    mapper: "generic"
    file_template: "*.csv"
    project_id: 100
    project: "P"
    activity_id: 200
    activity: "A"
    skill_id: 300
    skill: "S"
```

Rules map imported files to project, activity, and skill values. Add and manage rules from the `/config` page in the web UI.

## Web UI

The web UI supports:

- month and day views with local vs. remote totals
- import preview and import execution, with automatic rule matching and per-field override
- local worklog create, update, and delete
- day and month submit with dry-run preview
- remote refresh, remote delete, and remote-to-local copy/sync actions
- config page for OnePoint URL and import rule CRUD at `/config`

EPM imports automatically trigger reconciliation after a successful import.

OnePoint login opens a browser automatically when a valid auth state is missing or expired.

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

## Mappers

- `epm`: EPM-like exports with date/time, hours, and description columns
- `generic`: CSV with explicit `description,startdatetime,enddatetime,project,activity,skill`
- `atwork`: UTF-16 tab-separated CSV exports from the atwork time-tracking app
