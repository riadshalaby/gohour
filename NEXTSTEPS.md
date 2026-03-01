# NEXTSTEPS

Status as of March 2026: all planned submit fixes S1-S4 are completed.

## Completed

- `S1` Billable handling in submit:
  - `Billable=0` is preserved (supports `billable: false` rules).
  - negative billable values are rejected.

- `S2` Locked-day behavior:
  - if any existing remote entry for a day is locked, the full day is skipped.
  - no persist call is made for that day.

- `S3` Duplicate equivalence:
  - duplicate detection uses only:
    - `StartTime`
    - `FinishTime`
    - `ProjectID`
    - `ActivityID`
    - `SkillID`
  - `Comment`, `Billable`, and `Duration` are intentionally ignored.

- `S4` Submit overlap handling and dry-run refactor:
  - submit loop now does explicit per-day steps (`GetDayWorklogs` -> classify -> `PersistWorklogs`).
  - overlaps are detected and handled interactively in normal mode:
    - `w` write overlaps
    - `s` skip overlaps
    - `W` write all remaining overlaps
    - `S` skip all remaining overlaps
    - `a` abort
  - `--dry-run` now reads remote day worklogs, reports locked days and overlaps, and prints a summary.
  - `MergeAndPersistWorklogs` was removed from the OnePoint client API.

## Test Coverage Added

- submit tests for:
  - zero/negative billable behavior
  - duplicate/overlap classification
  - overlap prompt flows (including invalid input + abort)
  - locked-entry filtering for existing payload

- onepoint tests for:
  - duplicate equivalence semantics
  - overlap boundary/missing-time edge cases

## Open Items

No critical open items tracked here at the moment.

## Potential Follow-ups

- Add higher-level integration tests for the full submit command flow with a deterministic fake OnePoint server.
- Consider non-interactive overlap policy flags for CI automation (`--overlap-policy=write|skip|abort`).
