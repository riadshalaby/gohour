# gohour

`gohour` is a Go CLI for importing time-tracking source files, normalizing records into a local SQLite database, and exporting normalized worklogs.

## Features

- CLI built with Cobra and Viper
- Config file support (`onepoint.url`, `import.auto_reconcile_after_import`, `epm.rules`)
- Input formats: Excel (`.xlsx`, `.xlsm`, `.xls`) and CSV (`.csv`)
- Mapper-based normalization pipeline (`epm`, `generic`)
- SQLite persistence with duplicate protection
- Export normalized worklogs to CSV or Excel

## Requirements

- Go 1.24+

## Build

```bash
go build -o gohour .
```

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

Add one EPM rule interactively from OnePoint (project/activity/skill selection):

```bash
./gohour config rule add
```

Optional flags:
- `--url`: override OnePoint URL from config for this run (full home URL)
- `--state-file`: custom auth state file (default `$HOME/.gohour/onepoint-auth-state.json`)
- `--include-archived-projects`: include archived projects in selection
- `--include-locked-activities`: include locked activities in selection

If no config exists yet, `config edit` creates one with an example template first, then opens it.
After closing the editor, the file is validated as gohour YAML config.

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

epm:
  rules:
    - name: "rz"
      file_template: "EPMExportRZ*.xlsx"
      project_id: 432904811
      project: "MySpecial RZ Project"
      activity_id: 436142369
      activity: "Delivery"
      skill_id: 44498948
      skill: "Go"
```

## Import

Import one or more files into SQLite:

```bash
./gohour import -i examples/EPMExportRZ202601.xlsx -i examples/EPMExportSZ202601.xlsx --mapper epm --db ./gohour.db
```

```bash
./gohour import -i examples/generic_import_example.csv --format csv --mapper generic --db ./gohour.db
```

Flags:

- `-i, --input` (required, repeatable): input file path
- `-f, --format` (optional): `csv` or `excel` (auto-detected from file extension if omitted)
- `-m, --mapper` (optional): `epm` (default) or `generic`
- `--project` (optional): explicit project for EPM import (overrides rule)
- `--activity` (optional): explicit activity for EPM import (overrides rule)
- `--skill` (optional): explicit skill for EPM import (overrides rule)
- `--reconcile` (optional): `auto` (default, uses config), `on`, or `off`
- `--db` (optional): SQLite file path (default `./gohour.db`)

By default (`import.auto_reconcile_after_import: true`), import automatically runs reconciliation after every import, independent of source format/mapper.
For EPM imports, `project/activity/skill` must come from a matching `epm.rules` entry or explicit `--project/--activity/--skill`.
If no rule matches and no explicit values are provided, import fails.

## Export

Export normalized records from SQLite:

```bash
./gohour export --db ./gohour.db --output ./worklogs.csv
./gohour export --db ./gohour.db --output ./worklogs.xlsx
```

Export daily summaries:
- `StartTime`: start time of the first worklog entry of the day
- `EndTime`: end time of the last worklog entry of the day
- `WorkedHours`: sum of `(EndDateTime - StartDateTime)` per worklog of the day
- `BillableHours`: sum of billable values of the day
- `BreakHours`: gaps without worklog coverage between `StartTime` and `EndTime`

```bash
./gohour export --mode daily --db ./gohour.db --output ./daily-summary.csv
./gohour export --mode daily --db ./gohour.db --output ./daily-summary.xlsx
```

Flags:

- `-o, --output` (required): output file path
- `-f, --format` (optional): `csv` or `excel` (auto-detected from output extension if omitted)
- `--mode` (optional): `raw` (default) or `daily`
- `--db` (optional): SQLite file path (default `./gohour.db`)

## Reconcile (Verify + Correct)

After importing mixed sources (for example `epm` plus `generic`) on the same day, you can run an explicit reconciliation step:

```bash
./gohour reconcile --db ./gohour.db
```

What it does:

- Verifies overlaps that involve EPM entries.
- Repositions only EPM entries so they no longer overlap with other worklogs on the same day.
- Persists corrected start/end times back to SQLite.

This is useful because EPM task times are simulated during import and may collide with precise times from other sources.

## OnePoint Authentication (Microsoft SSO)

For direct OnePoint REST calls, `gohour` provides an embedded interactive browser login flow.
The URL is taken from `onepoint.url` in your config (`~/.gohour.yaml`) and defaults to:
`https://onepoint.virtual7.io/onepoint/faces/home`.
You can override it via `--url`.

1) Start login and save auth state:

```bash
gohour auth login
```

Optional override:

```bash
gohour auth login --url https://onepoint.virtual7.io/onepoint/faces/home
```

Debug cookie detection during login wait:

```bash
gohour auth login --debug-cookies
```

This opens a headed browser. Complete Microsoft login manually; the command auto-detects
successful login by waiting for OnePoint session cookies.
The auth state is saved by default to:

- `$HOME/.gohour/onepoint-auth-state.json`

2) Print Cookie header for direct API usage:

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
- Session cookies can rotate; run `auth login` again when REST calls fail with auth/session errors.

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

## Notes

- REST submission to the company website is planned for a later implementation phase.
