# ROADMAP for v0.2.2

## High Priority
- Web Import Completeness: add missing `billable` selection for file-based imports.
- Web Import Preview: show parsed and mapped rows before the import is confirmed.
- Web Import Conflict Handling: if an imported row is a duplicate or overlaps, ask the user to import or skip that row.
- Submit Dry-Run: add a non-persisting mode that shows exactly what would be submitted, including duplicates and overlaps.
- Submit Confirmation: for real submit, show a warning and require explicit confirmation when duplicates or overlaps are detected.
- Monthly Reporting: show total duration and billable hours for both local and remote entries.

## Medium Priority
- Entry UX: allow overlapping local entries, but always show a clear warning before save.
- Bulk Month Actions: add delete-all for a selected month (local and/or remote) with confirmation.
- Month Sync Action: add a one-click action to import all remote entries of the selected month into the local database.
- Day View Layout: display duration and billable side by side.
- Add a Version to GoHour including a version command to display the version.

## Future Ideas
