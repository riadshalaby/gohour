# NEXTSTEPS — Validation Bugs to Fix

This document lists all validation bugs and logic errors found during code review.
Each item includes the file, line range, description of the problem, and a concrete fix instruction.

---

## V1. `parseMinutesOrHours` — Minutes vs. Hours Ambiguity

**File:** `importer/parse.go:35-49`
**Severity:** Medium

**Problem:**
The function uses a heuristic: integers without a decimal separator are treated as **minutes**, values with a decimal separator are treated as **hours** (converted to minutes). This creates a silent semantic mismatch:

- `"8"` → 8 minutes
- `"8,0"` → 480 minutes (8 hours)
- `"8.0"` → 480 minutes (8 hours)

The function is called from `mapper_generic.go:38` where the CSV column can be named `"billable"`, `"minutes"`, `"arbeitszeit"`, or `"duration"`. A user entering `"8"` meaning 8 hours gets 8 minutes instead.

**Fix:**
Remove the ambiguity. Since the generic mapper already computes `billable := int(end.Sub(start).Minutes())` from start/end times, the override field should consistently expect **minutes**. Rename the function to `parseMinutes` and always treat the value as minutes (integer). If the value contains a decimal separator, treat it as decimal minutes (e.g., `"7.5"` → 8 minutes after rounding), NOT as hours. Update the caller in `mapper_generic.go` accordingly. Add a comment documenting the unit.

---

## V2. EPM Mapper — Entries Can Exceed Day Boundary

**File:** `importer/mapper_epm.go` (inside `Map()`, around lines 63-75)
**Severity:** Medium

**Problem:**
The EPM mapper sequences entries starting from `dayStart`. If the total billable hours plus break exceeds the remaining time in the day, entries can cross midnight. Example:

```
Von=17:00, Bis=18:00, Total billable=8h
Entry 1: 3h → 17:00-20:00
Entry 2: 5h → 20:00-01:00 (next day!)
```

No validation catches this during import. The error surfaces much later in `buildSubmitDayBatches` (`cmd/submit.go:401`) with the misleading message "crosses day boundaries".

**Fix:**
After computing `end` for each entry in the EPM mapper's `Map()` function, validate that `end` is still on the same calendar day as `start`. If not, return an error like: `"row %d: computed end time %s exceeds day boundary (started at %s)"`. This gives the user an actionable error at import time rather than at submit time.

---

## V3. EPM Mapper — `Von > Bis` (Start > End) Not Validated

**File:** `importer/mapper_epm.go:167-176` (inside `computeBreakMinutes`)
**Severity:** Low-Medium

**Problem:**
If the Excel file has "Von" (start time) after "Bis" (end time), e.g., Von=17:00, Bis=08:00, then `computeBreakMinutes` computes a negative `spanMins` and silently returns 0:

```go
spanMins := int(dayEnd.Sub(dayStart).Minutes())
if spanMins <= 0 {
    return 0  // no error, no warning
}
```

The user gets no indication that their source data is inconsistent.

**Fix:**
In `ensureDayState` (or wherever `dayStart` and `dayEnd` are first set), validate that `dayEnd.After(dayStart)`. If not, return an error: `"day start time %s is after end time %s — check the Von/Bis columns in the source file"`.

---

## V4. `MergeAndPersistWorklogs` Loses `Locked` Status

**File:** `onepoint/client.go:302-324`
**Severity:** Medium

**Problem:**
When merging new worklogs with existing ones for a day, `MergeAndPersistWorklogs` fetches all existing `DayWorklog` entries and converts them via `ToPersistWorklog()`. The `Locked` field from `DayWorklog` is not carried over to `PersistWorklog` (which has no `Locked` field). If the persist endpoint replaces all entries for the day, locked entries may be modified or the API may reject the request.

**Fix:**
Filter out locked entries from the existing worklogs before including them in the persist payload. Add this check after fetching existing entries:

```go
for _, item := range existing {
    if item.Locked != 0 {
        continue // don't re-send locked entries
    }
    payload = append(payload, item.ToPersistWorklog())
}
```

Alternatively, if locked entries MUST be included for the API to work correctly, add a `Locked` field to `PersistWorklog` and preserve the value in `ToPersistWorklog()`.

---

## V5. `collectRequiredNameTuples` vs. `buildSubmitDayBatches` — Inconsistent Validation

**File:** `cmd/submit.go:295-327` and `cmd/submit.go:356-459`
**Severity:** Low-Medium

**Problem:**
For entries with empty project/activity/skill:
- `collectRequiredNameTuples` (line 304-306): **silently skips** them
- `buildSubmitDayBatches` (line 376-378): **returns an error**

This means `resolveIDsForEntries` runs successfully (including potentially expensive API calls), and only afterward does `buildSubmitDayBatches` fail on the same entries that were silently skipped earlier.

**Fix:**
Move the validation to `collectRequiredNameTuples`. Instead of silently skipping entries with empty values, return an error immediately:

```go
if tuple.Project == "" || tuple.Activity == "" || tuple.Skill == "" {
    return nil, fmt.Errorf("worklog id=%d has empty project/activity/skill — cannot resolve IDs", entry.ID)
}
```

Change the return signature from `[]submitNameTuple` to `([]submitNameTuple, error)` and update the caller in `resolveIDsForEntries`.

---

## V6. Mapper Name Not Validated in Config

**File:** `config/config.go` (inside `validateRules`, around line 119)
**Severity:** Low-Medium

**Problem:**
`validateRules` only checks that `mapper` is not empty. A typo like `mapper: "expm"` passes config validation but fails at import time with `"unsupported mapper: expm"`. The valid values ("epm", "generic") are known at validation time.

**Fix:**
Add a check against the known mapper names. Use `importer.SupportedMapperNames()` or a hardcoded set:

```go
validMappers := map[string]bool{"epm": true, "generic": true}
if !validMappers[strings.ToLower(strings.TrimSpace(rule.Mapper))] {
    return fmt.Errorf("validation failed: rules[%d].mapper %q is not supported (valid: epm, generic)", i, rule.Mapper)
}
```

Note: To avoid a circular import (config → importer), either hardcode the valid names or move the mapper name list to a shared package.

---

## V7. `findNextAvailableStart` Can Push Entries Past Midnight

**File:** `reconcile/service.go:132-145`
**Severity:** Low

**Problem:**
The reconcile function shifts EPM entries to avoid overlaps, but has no day boundary check. If all busy slots are occupied, entries are pushed further and further — potentially past midnight:

```go
func findNextAvailableStart(busy []interval, desiredStart time.Time, duration time.Duration) time.Time {
    candidate := desiredStart
    for _, slot := range busy {
        // ...
        candidate = slot.end  // could be 23:00 or later
    }
    return candidate  // no check if still same day
}
```

Combined with V2, this creates entries that cross midnight, which fail at submit time.

**Fix:**
After computing `newStart` and `newEnd` in `reconcileDay`, check that `newEnd` is still on the same calendar day. If not, log a warning or skip the adjustment:

```go
newEnd := newStart.Add(duration)
if newStart.Day() != entry.StartDateTime.Day() || newEnd.Day() != entry.StartDateTime.Day() {
    // skip adjustment — would cross day boundary
    continue
}
```

---

## V8. Config Files Written with 0o644 Permissions

**Files:** `cmd/config_edit.go:100` and `cmd/config_rule_add.go:218`
**Severity:** Low-Medium

**Problem:**
Config files are written with world-readable permissions (0o644). While the config may not contain secrets directly, it contains OnePoint project/activity IDs and organizational structure that should not be world-readable.

**Fix:**
Change both `os.WriteFile` calls to use `0o600`:

```go
// cmd/config_edit.go:100
if err := os.WriteFile(configPath, updated, 0o600); err != nil {

// cmd/config_rule_add.go:218
if err := os.WriteFile(configPath, updated, 0o600); err != nil {
```

---

## V9. `countConflicts` Only Counts EPM-Involved Overlaps

**File:** `reconcile/service.go:174-200`
**Severity:** Low

**Problem:**
`countConflicts` counts an overlap only if at least one of the two overlapping entries is an EPM entry (line 193). Non-EPM overlaps (e.g., two generic entries overlapping) are silently ignored. The field names `OverlapsBefore` / `OverlapsAfter` suggest counting ALL overlaps.

**Fix:**
Either remove the EPM filter so all overlaps are counted:

```go
if !sorted[j].StartDateTime.Before(sorted[i].EndDateTime) {
    break
}
conflicts++  // count all overlaps, not just EPM ones
```

Or rename the result fields to `EPMOverlapsBefore` / `EPMOverlapsAfter` to make the semantics clear.

---

## Priority Order for Fixes

1. **V1** — `parseMinutesOrHours` ambiguity (silent data corruption)
2. **V2** — EPM day boundary overflow (confusing late error)
3. **V4** — Locked entries lost during merge (potential API failure)
4. **V5** — Early validation for empty values (wasted API calls)
5. **V3** — Von > Bis validation (silent data issue)
6. **V6** — Mapper name validation (fail-fast config check)
7. **V7** — Reconcile day boundary check (consistency with V2 fix)
8. **V8** — File permissions (security hardening)
9. **V9** — countConflicts semantics (cosmetic/clarity)
