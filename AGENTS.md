# AGENTS.md

## Purpose
Guidance for coding agents working in this repository.

## Project Overview
- Project: `gohour`
- Type: Go CLI (Cobra + Viper)
- Goal: Import time-tracking files, normalize worklogs, store in SQLite, export reports.

## Core Features
- Import from Excel/CSV into normalized worklogs.
- Mapper pipeline:
  - `epm`
  - `generic`
  - `atwork`
- SQLite persistence with duplicate protection.
- Reconciliation command to resolve overlaps by moving only EPM entries.
- Submit command with day-level remote validation:
  - skip full day when remote day contains locked entries
  - duplicate detection by time + project/activity/skill
  - overlap detection with interactive handling (`w/s/W/S/a`)
  - dry-run performs remote read checks and emits warnings/summary without writes
- Export:
  - Raw mode (`csv`, `excel`)
  - Daily summary mode

## Build & Test
- Build:
  - `go build -o gohour .`
- Tests:
  - `GOCACHE=/tmp/gocache go test ./...`
- Note:
  - In constrained environments, always set `GOCACHE=/tmp/gocache`.

## Configuration
- Config file: `$HOME/.gohour.yaml`
- Existing config commands:
  - `gohour config create`
  - `gohour config show`
  - `gohour config edit`
  - `gohour config delete`
- Important config keys:
  - `onepoint.url`
  - `import.auto_reconcile_after_import`
  - `rules[]` with `name`, `mapper`, `file_template`, `billable` (optional, default true), `project_id`, `project`, `activity_id`, `activity`, `skill_id`, `skill`

## Architecture Pointers
- CLI commands: `cmd/`
- Import logic: `importer/`
- Reconciliation: `reconcile/`
- Storage (SQLite): `storage/`
- Export writers: `output/`
- OnePoint API client: `onepoint/`

## OnePoint Integration Status
Implemented package:
- `onepoint/client.go`
- `onepoint/client_test.go`

Client capabilities:
- `ListProjects`
- `ListActivities`
- `ListSkills`
- `GetFilteredWorklogs`
- `GetDayWorklogs`
- `PersistWorklogs`
- `FetchLookupSnapshot`
- `ResolveIDs`
- submit helper utilities:
  - `PersistWorklogsEquivalent`
  - `WorklogTimeOverlaps`
  - `OverlapInfo`

Discovered endpoints:
- `POST /OPServices/resources/OpProjects/getAllUserProjects?mode=all`
- `POST /OPServices/resources/OpProjects/getAllUserActivities?mode=all`
- `POST /OPServices/resources/OpProjects/getAllUserSkills?mode=all`
- `GET /OPServices/resources/OpWorklogs/{DD-MM-YYYY}:{DD-MM-YYYY}/getFilteredWorklogs`
- `POST /OPServices/resources/OpWorklogs/{DD-MM-YYYY}/persistWorklogs`

Required request behavior:
- Cookie auth via `JSESSIONID` and `_WL_AUTHCOOKIE_JSESSIONID`
- Header `X-Requested-With: XMLHttpRequest`
- `Referer: /onepoint/faces/home`
- Persist uses JSON body
- Date format in path: `DD-MM-YYYY`
- Time values are minutes from midnight

Artifacts:
- Endpoint notes and checks: `artifacts/playwright/`

## Known Issues & Next Steps
See [NEXTSTEPS.md](NEXTSTEPS.md) for open items.

## Working Rules For Agents
- Keep all user-facing text, logs, docs, and comments in English.
- Do not commit secrets, cookies, auth state, or traces with session data.
- Prefer small, test-backed changes.
