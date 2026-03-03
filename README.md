# gohour

`gohour` is a Go CLI for importing time-tracking source files, normalizing records into a local SQLite database, and exporting normalized worklogs.

## Features

- CLI built with Cobra and Viper
- Config file support (`onepoint.url`, `import.auto_reconcile_after_import`, `rules`)
- Input formats: Excel (`.xlsx`, `.xlsm`, `.xls`) and CSV (`.csv`)
- Mapper-based normalization pipeline (`epm`, `generic`, `atwork`)
- SQLite persistence with duplicate protection
- Export normalized worklogs to CSV or Excel
- Submit local SQLite worklogs to OnePoint REST
- Local web UI for month/day review, import preview, edit, copy-from-remote, and submit
- Submit safety checks: duplicate detection, overlap warnings/prompts, locked-day skip
- Submit update propagation: billable/comment edits on synced entries are written back to remote
- `gohour version` command for release/build identification

> **Recommended workflow:** `gohour import` loads files locally, then `gohour serve` opens a browser UI to review local vs. remote hours and submit. Login happens automatically when needed - a browser window will open.

## Requirements

- Go 1.24+

## Build

```bash
go build -o gohour .
```

## Quick Start (3 Steps)

If you just want to get started, do this:

1. Create config and add one import rule:

```bash
./gohour config create
./gohour config rule add
```

2. Import your local worklog file(s) into SQLite:

```bash
./gohour import -i <your-file.xlsx>
```

3. Review and submit from the local browser UI:

```bash
./gohour serve
```

`auth login` is not a required manual step; login is triggered automatically when needed.

## Configuration

Create a default config file:

```bash
./gohour config create
```

Or with `go run`:

```bash
go run . config create
```

Default config file location:

- `$HOME/.gohour.yaml`

Show active config:

```bash
./gohour config show
```

Edit config in your terminal editor (`$VISUAL`, then `$EDITOR`, fallback `vi`):

```bash
./gohour config edit
```
If no config exists yet, `config edit` creates one with an example template first, then opens it.
After closing the editor, the file is validated as gohour YAML config.

Add one rule interactively from OnePoint (project/activity/skill selection):

```bash
./gohour config rule add
```

Optional flags:
- `--url`: override OnePoint URL from config for this run (full home URL)
- `--state-file`: custom auth state file (default `$HOME/.gohour/onepoint-auth-state.json`)
- `--include-archived-projects`: include archived projects in selection
- `--include-locked-activities`: include locked activities in selection

During `config rule add`, mapper is selected interactively from available mappers.

Delete active config:

```bash
./gohour config delete
```

Example config:

```yaml
onepoint:
  url: "https://onepoint.virtual7.io/onepoint/faces/home"

import:
  auto_reconcile_after_import: true

rules:
  - name: "rz"
    mapper: "epm"
    file_template: "EPMExportRZ*.xlsx"
    project_id: 432904811
    project: "MySpecial RZ Project"
    activity_id: 436142369
    activity: "Delivery"
    skill_id: 44498948
    skill: "Go"
  - name: "atwork-travel"
    mapper: "atwork"
    file_template: "excel-export-atwork*.csv"
    billable: false
    project_id: 432904811
    project: "MySpecial RZ Project"
    activity_id: 436142369
    activity: "Delivery"
    skill_id: 44498948
    skill: "Go"
```

Each rule supports an optional `billable` field (default: `true`). When set to `false`, all entries
imported via that rule get `Billable=0` (entry is imported but not counted as billable time).

`gohour config create` creates a standard config with `rules: []` (no demo rule).

## Import

Import one or more files into SQLite:

```bash
./gohour import -i examples/EPMExportRZ202601.xlsx
./gohour import -i examples/EPMExportRZ202601.xlsx -i examples/EPMExportSZ202601.xlsx
```

Flags:

- `-i, --input` (required, repeatable): input file path
- `-f, --format` (optional): `csv` or `excel` (auto-detected from file extension if omitted)
- `-m, --mapper` (optional): fallback mapper when no rule matches (`epm` default, `generic`, or `atwork`)
- `--project` (optional): explicit project for EPM import (overrides rule)
- `--activity` (optional): explicit activity for EPM import (overrides rule)
- `--skill` (optional): explicit skill for EPM import (overrides rule)
- `--reconcile` (optional): `auto` (default, uses config), `on`, or `off`
- `--db` (optional): SQLite file path (default `./gohour.db`)

By default (`import.auto_reconcile_after_import: true`), import automatically runs reconciliation after every import, independent of source format/mapper.
If a file matches a `rules` entry by `file_template`, that rule's `mapper` is used for importing that file.
For EPM-mapped files, `project/activity/skill` must come from a matching `rules` entry or explicit `--project/--activity/--skill`.
If no rule matches and no explicit values are provided, import fails.
Use optional flags like `--mapper`, `--format`, `--project`, `--activity`, `--skill`, or `--reconcile` only when needed.

## Export

Export normalized records from SQLite:

```bash
./gohour export --output ./worklogs.csv
./gohour export --output ./worklogs.xlsx
```

Export daily summaries:
- `StartTime`: start time of the first worklog entry of the day
- `EndTime`: end time of the last worklog entry of the day
- `WorkedHours`: sum of `(EndDateTime - StartDateTime)` per worklog of the day
- `BillableHours`: sum of billable values of the day
- `BreakHours`: gaps without worklog coverage between `StartTime` and `EndTime`

For daily summary export, use the optional `--mode daily` flag.

Flags:

- `-o, --output` (required): output file path
- `-f, --format` (optional): `csv` or `excel` (auto-detected from output extension if omitted)
- `--mode` (optional): `raw` (default) or `daily`
- `--db` (optional): SQLite file path (default `./gohour.db`)

## Serve (Recommended Review + Submit Workflow)

Run the local web UI for month/day review, edits, import, and submit actions:

```bash
./gohour serve
```

If no valid OnePoint session is available, `serve` opens a browser login flow automatically before starting.

Month view includes:
- `Preview submit` (dry-run classification before submit)
- `Submit month`
- `Copy from remote` (imports only remote rows that do not already exist locally)
- `Delete all local` and `Delete all remote` (with confirmation)

Day view includes:
- `Preview submit` and `Submit day`
- local add/edit/delete with overlap warning + "save anyway" flow
- status badges: `local`, `synced`, `conflict`, `remote`

Main flags:

- `--port` (optional): HTTP port (default `8080`)
- `--db` (optional): SQLite path (default `./gohour.db`)
- `--from` / `--to` (optional): month range for initial view, format `YYYY-MM`
- `--state-file` (optional): auth state JSON path
- `--url` (optional): override OnePoint home URL for this run
- `--no-open` (optional): do not auto-open browser tab

## Submit To OnePoint

Submit normalized worklogs from SQLite to OnePoint:

```bash
./gohour submit
```

Use optional flags like `--dry-run`, `--from`, `--to`, `--timeout`, `--url`, and `--state-file` only when needed.

Required prerequisites:

- Session cookies are managed automatically; a browser window opens if login is needed
- Reachable OnePoint endpoint (`onepoint.url` in config or `--url`)

What submit does:

- Reads local rows from SQLite.
- Resolves `project/activity/skill` names to OnePoint IDs:
  - first from `rules` IDs in config,
  - fallback via OnePoint lookup APIs.
- Groups local rows by day.
- For each day:
  - loads existing remote day worklogs (`getFilteredWorklogs` day range),
  - skips the full day when any existing entry is locked (`Locked != 0`),
  - skips local duplicates (same `StartTime`, `FinishTime`, `ProjectID`, `ActivityID`, `SkillID`),
  - treats equivalent entries with changed billable/comment as updates (writes local value to remote),
  - detects local-vs-existing overlaps and handles them:
    - `--dry-run`: warning only, no prompt,
    - normal mode: interactive choice per day (`w/s/W/S/a`),
  - persists the merged payload via `persistWorklogs` (only when entries remain to add).

Dry-run output includes:
- detailed per-entry output (`ready`, `duplicate`, `overlap`) and per-day summary
- summary with skipped locked days and overlap warnings

Main flags:

- `--db` (optional): SQLite path (default `./gohour.db`)
- `--from` / `--to` (optional): day range filter, format `YYYY-MM-DD`
- `--state-file` (optional): auth state JSON path
- `--url` (optional): override OnePoint home URL for this run
- `--timeout` (optional): timeout per API operation (default `60s`)
- `--dry-run` (optional): no API writes
- `--include-archived-projects` (optional): allow archived project fallback resolution
- `--include-locked-activities` (optional): allow locked activity fallback resolution

## Reconcile (Verify + Correct)

After importing mixed sources (for example `epm` plus `generic`) on the same day, you can run an explicit reconciliation step:

```bash
./gohour reconcile
```

What it does:

- Verifies overlaps that involve EPM entries.
- Repositions only EPM entries so they no longer overlap with other worklogs on the same day.
- Persists corrected start/end times back to SQLite.

This is useful because EPM task times are simulated during import and may collide with precise times from other sources.

## Delete Data / DB

Destructive cleanup command (always deletes the complete SQLite database file):

```bash
./gohour delete
```

Notes:
- The command asks for interactive confirmation.
- Type exactly `Y` to confirm deletion.

## OnePoint Authentication (Microsoft SSO)

`gohour` can trigger browser login automatically when needed.
Automatic login is used by:

- `gohour submit`
- `gohour serve`
- `gohour config rule add`

If no valid session cookie exists, a headed browser opens, you complete Microsoft login, and auth state is saved automatically.
The URL comes from `onepoint.url` in config (`~/.gohour.yaml`) and defaults to:
`https://onepoint.virtual7.io/onepoint/faces/home`.
You can override it with `--url` on the corresponding command.

Manual override login command:

```bash
gohour auth login
```

This command explicitly opens login and saves auth state to:

- `$HOME/.gohour/onepoint-auth-state.json` (default)

Show cookie header for direct API/debug usage:

```bash
gohour auth show-cookies
```

Expected output format:

```text
JSESSIONID=<...>; _WL_AUTHCOOKIE_JSESSIONID=<...>
```

Notes:
- Login opens a visible Chrome/Chromium browser window from inside `gohour`.
- By default, each login run uses a fresh temporary browser profile to avoid profile-lock issues.
- Use `--profile-dir` only if you explicitly want a reusable browser profile.
- Use `--browser-bin` if your browser executable is not auto-detected.
- Use `--timeout` to increase waiting time for MFA/conditional-access flows.
- Use `--debug-cookies` to print detected cookie names/domains while waiting.
- Session cookies expire periodically; the next `submit`, `serve`, or `config rule add` run re-triggers login automatically.

## Normalized SQLite Schema

Table: `worklogs`

- `start_datetime` (`TEXT`)
- `end_datetime` (`TEXT`)
- `billable` (`INTEGER`) -> billable minutes
- `description` (`TEXT`)
- `project` (`TEXT`)
- `activity` (`TEXT`)
- `skill` (`TEXT`)
- `source_format` (`TEXT`)
- `source_mapper` (`TEXT`)
- `source_file` (`TEXT`)

A unique constraint prevents duplicate imports of the same normalized row.

## Mappers

- `epm`: for EPM-like exports with columns such as date/time, hours, and description.
  - Uses source-day `Von`/`Bis` as the original day window.
  - Builds sequential worklogs for the day.
  - If `Tagessumme` is present, computes a single break (`(Bis - Von) - Tagessumme`) and inserts it near the middle of the billable work progression.
- `generic`: for already structured files with explicit start/end and optional billable value.
- `atwork`: for UTF-16 tab-separated CSV exports from the atwork time-tracking app.
  - Reads only the "Einträge" section (stops at "Gesamt" summary row).
  - Parses `Beginn`/`Ende` as datetimes, `Dauer` as German decimal hours.
  - Description is built from `Notiz` (with `Projekt`/`Aufgabe` as context prefix).
  - `Project`/`Activity`/`Skill` come from the matching rule config (like EPM).

## Notes

- REST submission is available via `gohour submit`.

## Version

Print current build version:

```bash
./gohour version
```
