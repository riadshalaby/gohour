package reconcile

import (
	"gohour/storage"
	"gohour/worklog"
	"path/filepath"
	"testing"
	"time"
)

func TestReconcileDay_ShiftsEPMEntriesAfterNonEPMIntervals(t *testing.T) {
	entries := []worklog.Entry{
		{
			ID:            1,
			StartDateTime: mustParse(t, "2026-03-10T09:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-03-10T11:00:00+01:00"),
			SourceMapper:  "generic",
		},
		{
			ID:            2,
			StartDateTime: mustParse(t, "2026-03-10T08:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-03-10T10:00:00+01:00"),
			SourceMapper:  "epm",
			Billable:      120,
		},
		{
			ID:            3,
			StartDateTime: mustParse(t, "2026-03-10T10:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-03-10T12:00:00+01:00"),
			SourceMapper:  "epm",
			Billable:      120,
		},
	}

	updates, adjusted := reconcileDay(entries)
	if adjusted != 2 {
		t.Fatalf("expected 2 adjusted entries, got %d", adjusted)
	}
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	updatedByID := make(map[int64]worklog.Entry, len(updates))
	for _, update := range updates {
		updatedByID[update.ID] = update
	}

	assertTime(t, mustParse(t, "2026-03-10T11:00:00+01:00"), updatedByID[2].StartDateTime, "entry 2 start")
	assertTime(t, mustParse(t, "2026-03-10T13:00:00+01:00"), updatedByID[2].EndDateTime, "entry 2 end")
	assertTime(t, mustParse(t, "2026-03-10T13:00:00+01:00"), updatedByID[3].StartDateTime, "entry 3 start")
	assertTime(t, mustParse(t, "2026-03-10T15:00:00+01:00"), updatedByID[3].EndDateTime, "entry 3 end")
}

func TestRun_PersistsAdjustedEPMRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "reconcile.db")
	store, err := storage.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	entries := []worklog.Entry{
		{
			StartDateTime: mustParse(t, "2026-03-11T09:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-03-11T10:00:00+01:00"),
			Billable:      60,
			Description:   "Generic fixed",
			Project:       "p",
			Activity:      "a",
			Skill:         "s",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "generic.csv",
		},
		{
			StartDateTime: mustParse(t, "2026-03-11T08:30:00+01:00"),
			EndDateTime:   mustParse(t, "2026-03-11T09:30:00+01:00"),
			Billable:      60,
			Description:   "EPM simulated",
			Project:       "p",
			Activity:      "a",
			Skill:         "s",
			SourceFormat:  "excel",
			SourceMapper:  "epm",
			SourceFile:    "EPMExportRZ202601.xlsx",
		},
	}

	inserted, err := store.InsertWorklogs(entries)
	if err != nil {
		t.Fatalf("insert worklogs: %v", err)
	}
	if inserted != 2 {
		t.Fatalf("expected 2 inserted rows, got %d", inserted)
	}

	result, err := Run(store)
	if err != nil {
		t.Fatalf("run reconcile: %v", err)
	}

	if result.EPMEntriesAdjusted != 1 {
		t.Fatalf("expected 1 adjusted epm entry, got %d", result.EPMEntriesAdjusted)
	}
	if result.RowsUpdated != 1 {
		t.Fatalf("expected 1 updated row, got %d", result.RowsUpdated)
	}
	if result.OverlapsBefore != 1 || result.OverlapsAfter != 0 {
		t.Fatalf("unexpected conflict stats: before=%d after=%d", result.OverlapsBefore, result.OverlapsAfter)
	}

	listed, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}

	var epmEntry *worklog.Entry
	for i := range listed {
		if listed[i].SourceMapper == "epm" {
			epmEntry = &listed[i]
			break
		}
	}
	if epmEntry == nil {
		t.Fatalf("expected epm entry in database")
	}

	assertTime(t, mustParse(t, "2026-03-11T10:00:00+01:00"), epmEntry.StartDateTime, "persisted epm start")
	assertTime(t, mustParse(t, "2026-03-11T11:00:00+01:00"), epmEntry.EndDateTime, "persisted epm end")
}

func mustParse(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed
}

func assertTime(t *testing.T, expected, actual time.Time, field string) {
	t.Helper()
	if !expected.Equal(actual) {
		t.Fatalf("unexpected %s: expected %s, got %s", field, expected.Format(time.RFC3339), actual.Format(time.RFC3339))
	}
}
