package storage

import (
	"errors"
	"gohour/worklog"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStore_AllowsDifferentSourcesOnSameDay(t *testing.T) {
	t.Parallel()

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

func TestSQLiteStore_DeleteAllWorklogs(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gohour_test.db")
	store, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	entries := []worklog.Entry{
		{
			StartDateTime: mustParseRFC3339(t, "2026-03-05T08:00:00+01:00"),
			EndDateTime:   mustParseRFC3339(t, "2026-03-05T09:00:00+01:00"),
			Billable:      60,
			Description:   "task 1",
			Project:       "p",
			Activity:      "a",
			Skill:         "s",
			SourceFormat:  "excel",
			SourceFile:    "a.xlsx",
		},
		{
			StartDateTime: mustParseRFC3339(t, "2026-03-06T10:00:00+01:00"),
			EndDateTime:   mustParseRFC3339(t, "2026-03-06T11:00:00+01:00"),
			Billable:      60,
			Description:   "task 2",
			Project:       "p",
			Activity:      "a",
			Skill:         "s",
			SourceFormat:  "csv",
			SourceFile:    "b.csv",
		},
	}

	inserted, err := store.InsertWorklogs(entries)
	if err != nil {
		t.Fatalf("insert worklogs: %v", err)
	}
	if inserted != 2 {
		t.Fatalf("expected 2 inserted rows, got %d", inserted)
	}

	deleted, err := store.DeleteAllWorklogs()
	if err != nil {
		t.Fatalf("delete all worklogs: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deleted rows, got %d", deleted)
	}

	listed, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected 0 stored rows after delete, got %d", len(listed))
	}
}

func TestUpdateWorklog_ChangesFields(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gohour_test.db")
	store, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	entries := []worklog.Entry{
		{
			StartDateTime: mustParseRFC3339(t, "2026-03-05T08:00:00+01:00"),
			EndDateTime:   mustParseRFC3339(t, "2026-03-05T09:00:00+01:00"),
			Billable:      60,
			Description:   "before",
			Project:       "p1",
			Activity:      "a1",
			Skill:         "s1",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "a.csv",
		},
	}
	inserted, err := store.InsertWorklogs(entries)
	if err != nil {
		t.Fatalf("insert worklogs: %v", err)
	}
	if inserted != 1 {
		t.Fatalf("expected 1 inserted row, got %d", inserted)
	}

	listed, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 listed row, got %d", len(listed))
	}
	entry := listed[0]
	entry.StartDateTime = mustParseRFC3339(t, "2026-03-05T10:00:00+01:00")
	entry.EndDateTime = mustParseRFC3339(t, "2026-03-05T12:00:00+01:00")
	entry.Billable = 120
	entry.Description = "after"
	entry.Project = "p2"
	entry.Activity = "a2"
	entry.Skill = "s2"

	if err := store.UpdateWorklog(entry); err != nil {
		t.Fatalf("update worklog: %v", err)
	}

	listed, err = store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 listed row, got %d", len(listed))
	}
	updated := listed[0]
	if !updated.StartDateTime.Equal(entry.StartDateTime) || !updated.EndDateTime.Equal(entry.EndDateTime) {
		t.Fatalf("unexpected updated times: %+v", updated)
	}
	if updated.Billable != 120 || updated.Description != "after" || updated.Project != "p2" || updated.Activity != "a2" || updated.Skill != "s2" {
		t.Fatalf("unexpected updated fields: %+v", updated)
	}
}

func TestUpdateWorklog_NotFound(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gohour_test.db")
	store, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	err = store.UpdateWorklog(worklog.Entry{
		ID:            999,
		StartDateTime: mustParseRFC3339(t, "2026-03-05T08:00:00+01:00"),
		EndDateTime:   mustParseRFC3339(t, "2026-03-05T09:00:00+01:00"),
		Billable:      60,
		Description:   "x",
		Project:       "p",
		Activity:      "a",
		Skill:         "s",
	})
	if !errors.Is(err, ErrWorklogNotFound) {
		t.Fatalf("expected ErrWorklogNotFound, got %v", err)
	}
}

func TestDeleteWorklog_Removes(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gohour_test.db")
	store, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	entries := []worklog.Entry{
		{
			StartDateTime: mustParseRFC3339(t, "2026-03-05T08:00:00+01:00"),
			EndDateTime:   mustParseRFC3339(t, "2026-03-05T09:00:00+01:00"),
			Billable:      60,
			Description:   "before",
			Project:       "p1",
			Activity:      "a1",
			Skill:         "s1",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "a.csv",
		},
	}
	inserted, err := store.InsertWorklogs(entries)
	if err != nil {
		t.Fatalf("insert worklogs: %v", err)
	}
	if inserted != 1 {
		t.Fatalf("expected 1 inserted row, got %d", inserted)
	}

	listed, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	removed, err := store.DeleteWorklog(listed[0].ID)
	if err != nil {
		t.Fatalf("delete worklog: %v", err)
	}
	if !removed {
		t.Fatalf("expected removed=true")
	}

	listed, err = store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected empty store after delete, got %d", len(listed))
	}
}

func TestDeleteWorklog_NotFound(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gohour_test.db")
	store, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	removed, err := store.DeleteWorklog(999)
	if err != nil {
		t.Fatalf("delete worklog: %v", err)
	}
	if removed {
		t.Fatalf("expected removed=false for missing id")
	}
}

func TestInsertWorklog_ReturnsID(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gohour_test.db")
	store, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	id, inserted, err := store.InsertWorklog(worklog.Entry{
		StartDateTime: mustParseRFC3339(t, "2026-03-05T08:00:00+01:00"),
		EndDateTime:   mustParseRFC3339(t, "2026-03-05T09:00:00+01:00"),
		Billable:      60,
		Description:   "task",
		Project:       "p",
		Activity:      "a",
		Skill:         "s",
		SourceFormat:  "csv",
		SourceMapper:  "generic",
		SourceFile:    "a.csv",
	})
	if err != nil {
		t.Fatalf("insert worklog: %v", err)
	}
	if !inserted {
		t.Fatalf("expected inserted=true")
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	entry, found, err := store.GetWorklogByID(id)
	if err != nil {
		t.Fatalf("get worklog by id: %v", err)
	}
	if !found {
		t.Fatalf("expected found=true")
	}
	if entry.Description != "task" {
		t.Fatalf("unexpected stored entry: %+v", entry)
	}
}

func TestInsertWorklogs_ZeroBillable(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gohour_zero_billable.db")
	store, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	inserted, err := store.InsertWorklogs([]worklog.Entry{
		{
			StartDateTime: mustParseRFC3339(t, "2026-03-05T08:00:00+01:00"),
			EndDateTime:   mustParseRFC3339(t, "2026-03-05T09:00:00+01:00"),
			Billable:      0,
			Description:   "non-billable",
			Project:       "p",
			Activity:      "a",
			Skill:         "s",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "zero.csv",
		},
	})
	if err != nil {
		t.Fatalf("insert worklogs: %v", err)
	}
	if inserted != 1 {
		t.Fatalf("expected 1 inserted row, got %d", inserted)
	}

	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 listed row, got %d", len(entries))
	}
	if entries[0].Billable != 0 {
		t.Fatalf("expected billable=0, got %d", entries[0].Billable)
	}
}
