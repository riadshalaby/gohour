package storage

import (
	"gohour/worklog"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStore_AllowsDifferentSourcesOnSameDay(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "gohour_test.db")
	store, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	entries := []worklog.Entry{
		{
			StartDateTime: mustParseRFC3339(t, "2026-01-23T08:00:00+01:00"),
			EndDateTime:   mustParseRFC3339(t, "2026-01-23T09:00:00+01:00"),
			Billable:      60,
			Description:   "same logical work",
			Project:       "p",
			Activity:      "a",
			Skill:         "s",
			SourceFormat:  "excel",
			SourceFile:    "EPMExportRZ202601.xlsx",
		},
		{
			StartDateTime: mustParseRFC3339(t, "2026-01-23T08:00:00+01:00"),
			EndDateTime:   mustParseRFC3339(t, "2026-01-23T09:00:00+01:00"),
			Billable:      60,
			Description:   "same logical work",
			Project:       "p",
			Activity:      "a",
			Skill:         "s",
			SourceFormat:  "csv",
			SourceFile:    "generic_same_day.csv",
		},
	}

	inserted, err := store.InsertWorklogs(entries)
	if err != nil {
		t.Fatalf("insert worklogs: %v", err)
	}
	if inserted != 2 {
		t.Fatalf("expected 2 inserted rows, got %d", inserted)
	}

	listed, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 stored rows, got %d", len(listed))
	}
}

func mustParseRFC3339(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed
}
