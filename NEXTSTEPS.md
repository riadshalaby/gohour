# NEXTSTEPS

All 9 validation bugs from the initial code review have been resolved.

## Recent Additions

- **`atwork` mapper**: New mapper for UTF-16 tab-separated CSV exports from the atwork time-tracking app. Includes a dedicated reader (`ATWorkReader`) that handles encoding conversion, section parsing, and column mapping. Project/Activity/Skill are resolved from rule config (like EPM).
- **`billable` rule flag**: Rules now support an optional `billable` field (default: `true`). When set to `false`, all entries imported via that rule get `Billable=0`. Works with all mappers.

## No Open Items
