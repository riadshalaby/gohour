# gohour

`gohour` is a local web UI for importing time-tracking files, reviewing local worklogs against OnePoint, editing local entries, and submitting day or month batches to OnePoint.

## Features

- Local browser UI for month/day review, import preview, local edit/delete, copy-from-remote, and submit workflows
- SQLite-backed worklog storage
- OnePoint lookup, comparison, and submit integration
- Import support for EPM-style Excel files, generic CSV, and atwork TSV exports
- Fixed data directory under `$HOME/.gohour`

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
go fmt ./...
go vet ./...
go test ./...
```

The Playwright suite in `e2e/` starts the pre-built `gohour` binary with an isolated `GOHOUR_DATA_DIR` and seeds data through the web import API.

## Mappers

- `epm`: EPM-like exports with date/time, hours, and description columns
- `generic`: CSV with explicit `description,startdatetime,enddatetime,project,activity,skill`
- `atwork`: UTF-16 tab-separated CSV exports from the atwork time-tracking app

## Version

Release builds embed version metadata with:

```bash
go build -ldflags "-X github.com/riadshalaby/gohour/cmd.Version=vX.Y.Z" .
```
