package output

import (
	"gohour/worklog"
	"testing"
	"time"
)

func TestBuildDailySummaries_CalculatesWorkedBillableAndBreakHours(t *testing.T) {
	entries := []worklog.Entry{
		{
			StartDateTime: mustParse(t, "2026-01-05T08:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-01-05T09:00:00+01:00"),
			Billable:      60,
		},
		{
			StartDateTime: mustParse(t, "2026-01-05T09:30:00+01:00"),
			EndDateTime:   mustParse(t, "2026-01-05T10:30:00+01:00"),
			Billable:      60,
		},
		{
			StartDateTime: mustParse(t, "2026-01-05T11:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-01-05T12:00:00+01:00"),
			Billable:      60,
		},
	}

	summaries := BuildDailySummaries(entries)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	summary := summaries[0]
	assertTimeEqual(t, mustParse(t, "2026-01-05T08:00:00+01:00"), summary.StartDateTime, "start time")
	assertTimeEqual(t, mustParse(t, "2026-01-05T12:00:00+01:00"), summary.EndDateTime, "end time")
	assertFloatEqual(t, 3.00, summary.WorkedHours, "worked hours")
	assertFloatEqual(t, 3.00, summary.BillableHours, "billable hours")
	assertFloatEqual(t, 1.00, summary.BreakHours, "break hours")
	if summary.WorklogCount != 3 {
		t.Fatalf("expected 3 worklogs, got %d", summary.WorklogCount)
	}
}

func TestBuildDailySummaries_UsesFirstAndLastEntryOfDay(t *testing.T) {
	entries := []worklog.Entry{
		{
			StartDateTime: mustParse(t, "2026-01-06T08:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-01-06T17:00:00+01:00"),
			Billable:      120,
		},
		{
			StartDateTime: mustParse(t, "2026-01-06T09:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-01-06T10:00:00+01:00"),
			Billable:      60,
		},
		{
			StartDateTime: mustParse(t, "2026-01-06T10:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-01-06T11:00:00+01:00"),
			Billable:      60,
		},
	}

	summaries := BuildDailySummaries(entries)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	summary := summaries[0]
	assertTimeEqual(t, mustParse(t, "2026-01-06T08:00:00+01:00"), summary.StartDateTime, "start time")
	assertTimeEqual(t, mustParse(t, "2026-01-06T11:00:00+01:00"), summary.EndDateTime, "end time")
	assertFloatEqual(t, 11.00, summary.WorkedHours, "worked hours")
	assertFloatEqual(t, 4.00, summary.BillableHours, "billable hours")
	assertFloatEqual(t, 0.00, summary.BreakHours, "break hours")
}

func TestBuildDailySummaries_GroupsByDay(t *testing.T) {
	entries := []worklog.Entry{
		{
			StartDateTime: mustParse(t, "2026-01-07T08:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-01-07T09:00:00+01:00"),
			Billable:      60,
		},
		{
			StartDateTime: mustParse(t, "2026-01-08T10:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-01-08T12:00:00+01:00"),
			Billable:      120,
		},
	}

	summaries := BuildDailySummaries(entries)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	if summaries[0].Date != "2026-01-07" {
		t.Fatalf("expected first summary date 2026-01-07, got %s", summaries[0].Date)
	}
	if summaries[1].Date != "2026-01-08" {
		t.Fatalf("expected second summary date 2026-01-08, got %s", summaries[1].Date)
	}
}

func TestBuildDailySummaries_MergesDifferentSourcesOnSameDay(t *testing.T) {
	entries := []worklog.Entry{
		{
			StartDateTime: mustParse(t, "2026-03-03T14:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-03-03T16:00:00+01:00"),
			Billable:      120,
			SourceFormat:  "csv",
			SourceFile:    "generic.csv",
		},
		{
			StartDateTime: mustParse(t, "2026-03-03T08:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-03-03T10:00:00+01:00"),
			Billable:      120,
			SourceFormat:  "excel",
			SourceFile:    "epm.xlsx",
		},
		{
			StartDateTime: mustParse(t, "2026-03-03T10:30:00+01:00"),
			EndDateTime:   mustParse(t, "2026-03-03T12:00:00+01:00"),
			Billable:      90,
			SourceFormat:  "excel",
			SourceFile:    "epm.xlsx",
		},
	}

	summaries := BuildDailySummaries(entries)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	summary := summaries[0]
	assertTimeEqual(t, mustParse(t, "2026-03-03T08:00:00+01:00"), summary.StartDateTime, "start time")
	assertTimeEqual(t, mustParse(t, "2026-03-03T16:00:00+01:00"), summary.EndDateTime, "end time")
	assertFloatEqual(t, 5.50, summary.WorkedHours, "worked hours")
	assertFloatEqual(t, 5.50, summary.BillableHours, "billable hours")
	assertFloatEqual(t, 2.50, summary.BreakHours, "break hours")
	if summary.WorklogCount != 3 {
		t.Fatalf("expected 3 worklogs, got %d", summary.WorklogCount)
	}
}

func TestBuildDailySummaries_UsesCoverageUnionForBreakWithMultipleSources(t *testing.T) {
	entries := []worklog.Entry{
		{
			StartDateTime: mustParse(t, "2026-03-04T08:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-03-04T12:00:00+01:00"),
			Billable:      240,
			SourceFormat:  "excel",
			SourceFile:    "epm.xlsx",
		},
		{
			StartDateTime: mustParse(t, "2026-03-04T10:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-03-04T11:00:00+01:00"),
			Billable:      60,
			SourceFormat:  "csv",
			SourceFile:    "generic.csv",
		},
		{
			StartDateTime: mustParse(t, "2026-03-04T13:00:00+01:00"),
			EndDateTime:   mustParse(t, "2026-03-04T14:00:00+01:00"),
			Billable:      60,
			SourceFormat:  "csv",
			SourceFile:    "generic.csv",
		},
	}

	summaries := BuildDailySummaries(entries)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	summary := summaries[0]
	assertTimeEqual(t, mustParse(t, "2026-03-04T08:00:00+01:00"), summary.StartDateTime, "start time")
	assertTimeEqual(t, mustParse(t, "2026-03-04T14:00:00+01:00"), summary.EndDateTime, "end time")
	assertFloatEqual(t, 6.00, summary.WorkedHours, "worked hours")
	assertFloatEqual(t, 6.00, summary.BillableHours, "billable hours")
	assertFloatEqual(t, 1.00, summary.BreakHours, "break hours")
	if summary.WorklogCount != 3 {
		t.Fatalf("expected 3 worklogs, got %d", summary.WorklogCount)
	}
}

func mustParse(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed
}

func assertFloatEqual(t *testing.T, expected, actual float64, field string) {
	t.Helper()
	if expected != actual {
		t.Fatalf("unexpected %s: expected %.2f, got %.2f", field, expected, actual)
	}
}

func assertTimeEqual(t *testing.T, expected, actual time.Time, field string) {
	t.Helper()
	if !expected.Equal(actual) {
		t.Fatalf("unexpected %s: expected %s, got %s", field, expected.Format(time.RFC3339), actual.Format(time.RFC3339))
	}
}
