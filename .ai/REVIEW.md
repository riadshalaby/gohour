# REVIEW — Interactive Web UI, revision 4 (working-tree changes on feature/web-ui)

## Reviewed Scope

Changes relative to last committed state (`ebd8d32`) against the latest PLAN.md.

The only substantive delta from revision 3 is a **simplification of task H1**: the
`migrateBillableConstraint()` migration path was removed from both the plan and the
implementation.

Test run: `go test ./...` — **all packages pass**.

---

## Plan Changes Since Revision 3

| Area | Revision 3 plan | Current plan |
|---|---|---|
| H1 fix | Schema change + `migrateBillableConstraint()` in a tx | Schema change only; "No migration needed — delete DB and re-import" |
| H1 tests | `TestInsertWorklogs_ZeroBillable` + `TestMigrateBillableConstraint_Idempotent` + `TestMigrateBillableConstraint_MigratesLegacyDB` | `TestInsertWorklogs_ZeroBillable` only |
| All other tasks | unchanged | unchanged |

---

## Plan Compliance

### H1 — Schema fix (simplified)

- Constraint changed from `CHECK(billable > 0)` to `CHECK(billable >= 0)`. ✅
- No `migrateBillableConstraint()` function — consistent with the updated plan. ✅
- `TestInsertWorklogs_ZeroBillable` present and correct. ✅
- No migration tests — consistent with the updated plan. ✅

### M1, T1, T2, T3, T4

Unchanged from revision 3; all verified as compliant then and still present
without regression.

---

## Findings

### 1. Plan inconsistency — "Files Expected To Change" table is stale (plan artifact)

The H1 task body now reads "No migration needed" but the **Files Expected To
Change** table was not updated:

```
| H1 | storage/sqlite_store.go | `migrateBillableConstraint`, schema fix |
| H1 | storage/sqlite_store_test.go | Tests for 0-billable insert and migration |
```

The entry should read:

```
| H1 | storage/sqlite_store.go | schema fix (billable >= 0) |
| H1 | storage/sqlite_store_test.go | TestInsertWorklogs_ZeroBillable |
```

This does not affect the implementation, but leaves the plan document
internally inconsistent.

### 2. H1 — No warning about data loss for existing databases (advisory)

The simplified approach requires users to delete their `gohour.db` and
re-import from source files. Any worklogs that were added manually (not from
an imported file) will be permanently lost on upgrade.

Neither the plan nor any code comment warns about this. A comment in
`ensureSchema()` noting the constraint change and the manual migration
requirement would help operators.

---

## Previous Findings — Carry-Forward Status

| Finding | Status |
|---|---|
| All revision 3 approved items | ✅ still resolved |
| Rev 3 Finding 1 (lookup cold-start double fetch) | ⚪ still open (advisory, not blocking) |
| Rev 3 Finding 2 (frozen DDL comment in migration test helper) | ✅ resolved by removal of migration code |

---

## Test Coverage

All tests from revision 3 remain present and pass, minus the three migration tests
which were intentionally removed alongside the migration code.

| Test | Status |
|------|--------|
| `TestInsertWorklogs_ZeroBillable` | ✅ |
| All other revision-3 tests | ✅ |
| `TestMigrateBillableConstraint_*` (3 tests) | removed per plan ✅ |

---

## Decision

**Approve.**

The implementation correctly tracks the simplified plan. Finding 1 is a plan
document cleanup (stale table row) that should be fixed before finalising the
plan. Finding 2 is advisory operational documentation and does not block merge.
