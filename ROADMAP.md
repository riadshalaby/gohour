# ROADMAP v0.4.0

## Goal
Simplify gohour by removing most CLI commands and concentrating on the web interface. All data files live under `~/.gohour/`.

## CLI surface after v0.4.0
- `gohour serve` — start the web UI (the primary entry point)
- `gohour version` — print version info
- All other CLI commands are removed: `auth`, `config`, `delete`, `export`, `import`, `reconcile`, `submit`

## File consolidation
- Config: `~/.gohour/config.yaml`
- Database: `~/.gohour/gohour.db`
- Auth state: `~/.gohour/onepoint-auth-state.json` (already here today)
- The `--db`, `--configFile` CLI flags are removed. No path overrides.
- The `serve` command keeps `--port` and `--no-open` flags. The `--url`, `--state-file`, `--from`, `--to` flags are removed (URL comes from config, state file is fixed path, month range is handled in the web UI).

## Migration logic (runs before server starts)
1. If `~/.gohour/config.yaml` already exists → use it, no prompt.
2. Else search for old files:
   - CWD: `.gohour.yaml`, `gohour.db`
   - `$HOME`: `~/.gohour.yaml`
3. If old files found → CLI prompt: "Move existing config/db to ~/.gohour/ or start fresh?"
   - "Move": copy files to `~/.gohour/`, rename originals to `.bak`.
   - "Fresh": create default config + empty DB in `~/.gohour/`.
4. If no old files found → create default config + empty DB in `~/.gohour/`.

## Web UI: config management
- New config page accessible from the web UI navigation.
- Manage OnePoint URL and other settings.
- Full rule CRUD: list, create, edit, delete import rules.
- Rules stay in `~/.gohour/config.yaml` (not in SQLite).

## Web UI: import with rule matching
- Upload files via the existing import UI.
- Auto-match uploaded filenames against config rules (same glob logic as CLI import today).
- Show matched rule values (mapper, project, activity, skill, billable) in the preview.
- User can override any matched value before importing.
- Option to update the matched rule with the user's overrides.
- If no rule matches, user fills in all parameters manually.
- All three mappers supported: epm, generic, atwork.

## Reconcile
- No standalone CLI command.
- Reconcile always runs automatically after EPM imports (both CLI-initiated and web-initiated).
- The `auto_reconcile_after_import` config flag is removed; reconcile-after-EPM-import is unconditional.

## Auth
- Web UI already handles login when not authenticated.
- No CLI auth command needed.

## Documentation
- `README.md` must be rewritten to reflect the new CLI surface, removed commands, `~/.gohour/` file layout, migration behavior, and web-UI-first workflow.
- Cobra command help text (`Short`, `Long`, `Example`) updated for `serve` and `version`; removed commands' help text deleted with their code.