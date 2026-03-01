package web

import (
	"testing"
	"time"

	"gohour/onepoint"
	"gohour/worklog"
)

func TestBuildDailyView_NewEntry(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	local := []worklog.Entry{
		{
			StartDateTime: day,
			EndDateTime:   day.Add(1 * time.Hour),
			Billable:      60,
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
		},
	}

	rows := BuildDailyView(local, nil)
	if len(rows) != 1 {
		t.Fatalf("expected 1 day row, got %d", len(rows))
	}
	if len(rows[0].Entries) != 1 {
		t.Fatalf("expected 1 entry row, got %d", len(rows[0].Entries))
	}
	if got := rows[0].Entries[0].Source; got != "new" {
		t.Fatalf("expected source=new, got %q", got)
	}
}

func TestBuildDailyView_Duplicate(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	local := []worklog.Entry{
		{
			StartDateTime: day,
			EndDateTime:   day.Add(1 * time.Hour),
			Billable:      60,
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
		},
	}
	remote := []onepoint.DayWorklog{
		{
			WorklogDate: onepoint.FormatDay(day),
			StartTime:   9 * 60,
			FinishTime:  10 * 60,
			Billable:    60,
			ProjectID:   101,
			ActivityID:  202,
			SkillID:     303,
		},
	}

	rows := BuildDailyView(local, remote)
	if len(rows) != 1 || len(rows[0].Entries) != 1 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if got := rows[0].Entries[0].Source; got != "duplicate" {
		t.Fatalf("expected source=duplicate, got %q", got)
	}
}

func TestBuildDailyView_Overlap(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	local := []worklog.Entry{
		{
			StartDateTime: day,
			EndDateTime:   day.Add(1 * time.Hour),
			Billable:      60,
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
		},
	}
	remote := []onepoint.DayWorklog{
		{
			WorklogDate: onepoint.FormatDay(day),
			StartTime:   9*60 + 30,
			FinishTime:  10*60 + 30,
			Billable:    60,
			ProjectID:   101,
			ActivityID:  202,
			SkillID:     303,
		},
	}

	rows := BuildDailyView(local, remote)
	if len(rows) != 1 {
		t.Fatalf("expected 1 day row, got %d", len(rows))
	}

	found := false
	for _, entry := range rows[0].Entries {
		if entry.Source == "overlap" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected at least one overlap row, got %+v", rows[0].Entries)
	}
}

func TestBuildDailyView_RemoteOnly(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
	remote := []onepoint.DayWorklog{
		{
			WorklogDate: onepoint.FormatDay(day),
			StartTime:   11 * 60,
			FinishTime:  12 * 60,
			Billable:    60,
			ProjectID:   101,
			ActivityID:  202,
			SkillID:     303,
		},
	}

	rows := BuildDailyView(nil, remote)
	if len(rows) != 1 || len(rows[0].Entries) != 1 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if got := rows[0].Entries[0].Source; got != "remote" {
		t.Fatalf("expected source=remote, got %q", got)
	}
}

func TestBuildMonthlyView(t *testing.T) {
	t.Parallel()

	days := []DayRow{
		{
			Date:        time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local),
			LocalHours:  2.0,
			RemoteHours: 1.0,
		},
		{
			Date:        time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local),
			LocalHours:  1.5,
			RemoteHours: 2.0,
		},
	}

	summary := BuildMonthlyView(days)
	if len(summary.Days) != 2 {
		t.Fatalf("expected 2 summary days, got %d", len(summary.Days))
	}
	if summary.TotalLocalHours != 3.5 {
		t.Fatalf("unexpected total local hours: %.2f", summary.TotalLocalHours)
	}
	if summary.TotalRemoteHours != 3.0 {
		t.Fatalf("unexpected total remote hours: %.2f", summary.TotalRemoteHours)
	}
	if summary.TotalDeltaHours != 0.5 {
		t.Fatalf("unexpected total delta: %.2f", summary.TotalDeltaHours)
	}
}

func TestBuildDailyView_DurationMins(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	local := []worklog.Entry{
		{
			StartDateTime: day,
			EndDateTime:   day.Add(90 * time.Minute),
			Billable:      90,
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
		},
	}

	rows := BuildDailyView(local, nil)
	if len(rows) != 1 || len(rows[0].Entries) != 1 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if got := rows[0].Entries[0].DurationMins; got != 90 {
		t.Fatalf("expected duration mins = 90, got %d", got)
	}
}

func TestBuildDailyView_DurationIndependentOfBillable(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	local := []worklog.Entry{
		{
			StartDateTime: day,
			EndDateTime:   day.Add(90 * time.Minute),
			Billable:      0,
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
		},
	}

	rows := BuildDailyView(local, nil)
	if len(rows) != 1 || len(rows[0].Entries) != 1 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if got := rows[0].Entries[0].DurationMins; got != 90 {
		t.Fatalf("expected duration mins = 90, got %d", got)
	}
	if got := rows[0].Entries[0].BillableMins; got != 0 {
		t.Fatalf("expected billable mins = 0, got %d", got)
	}
}
