package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gohour/onepoint"
	"gohour/storage"
	"gohour/worklog"
)

func TestServer_MonthPageRendersMonthDays(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		{
			StartDateTime: time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local),
			EndDateTime:   time.Date(2026, 3, 1, 10, 0, 0, 0, time.Local),
			Billable:      60,
			Description:   "local-1",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "a.csv",
		},
	})

	client := fakeClient{
		worklogs: []onepoint.DayWorklog{
			{
				WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)),
				StartTime:   9 * 60,
				FinishTime:  10 * 60,
				Billable:    60,
				ProjectID:   901,
				ActivityID:  902,
				SkillID:     903,
			},
		},
	}

	ts := httptest.NewServer(NewServer(store, client))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/month/2026-03")
	if err != nil {
		t.Fatalf("request month page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if !strings.Contains(text, "2026-03-01") {
		t.Fatalf("month page missing first day: %s", text)
	}
	if !strings.Contains(text, "2026-03-31") {
		t.Fatalf("month page missing last day: %s", text)
	}
}

func TestServer_DayPageShowsClassificationBadges(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	day := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
	insertWorklogs(t, store, []worklog.Entry{
		{
			StartDateTime: day.Add(9 * time.Hour),
			EndDateTime:   day.Add(10 * time.Hour),
			Billable:      60,
			Description:   "duplicate-entry",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "a.csv",
		},
		{
			StartDateTime: day.Add(10 * time.Hour),
			EndDateTime:   day.Add(11 * time.Hour),
			Billable:      60,
			Description:   "overlap-entry",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "a.csv",
		},
		{
			StartDateTime: day.Add(12 * time.Hour),
			EndDateTime:   day.Add(13 * time.Hour),
			Billable:      60,
			Description:   "new-entry",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "a.csv",
		},
	})

	client := fakeClient{
		worklogs: []onepoint.DayWorklog{
			{
				WorklogDate: onepoint.FormatDay(day),
				StartTime:   9 * 60,
				FinishTime:  10 * 60,
				Billable:    60,
				ProjectID:   901,
				ActivityID:  902,
				SkillID:     903,
			},
			{
				WorklogDate: onepoint.FormatDay(day),
				StartTime:   10*60 + 30,
				FinishTime:  11*60 + 30,
				Billable:    60,
				ProjectID:   904,
				ActivityID:  905,
				SkillID:     906,
			},
			{
				WorklogDate: onepoint.FormatDay(day),
				StartTime:   14 * 60,
				FinishTime:  15 * 60,
				Billable:    60,
				ProjectID:   907,
				ActivityID:  908,
				SkillID:     909,
			},
		},
	}

	ts := httptest.NewServer(NewServer(store, client))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/day/2026-03-01")
	if err != nil {
		t.Fatalf("request day page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	for _, label := range []string{"duplicate", "overlap", "new", "remote"} {
		if !strings.Contains(text, label) {
			t.Fatalf("expected badge label %q in response body", label)
		}
	}
}

func openTestStore(t *testing.T) *storage.SQLiteStore {
	t.Helper()

	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "gohour_test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func insertWorklogs(t *testing.T, store *storage.SQLiteStore, entries []worklog.Entry) {
	t.Helper()
	inserted, err := store.InsertWorklogs(entries)
	if err != nil {
		t.Fatalf("insert worklogs: %v", err)
	}
	if inserted != len(entries) {
		t.Fatalf("expected %d inserted rows, got %d", len(entries), inserted)
	}
}

type fakeClient struct {
	worklogs []onepoint.DayWorklog
}

func (f fakeClient) ListProjects(ctx context.Context) ([]onepoint.Project, error) {
	return nil, errors.New("not implemented in test fake")
}

func (f fakeClient) ListActivities(ctx context.Context) ([]onepoint.Activity, error) {
	return nil, errors.New("not implemented in test fake")
}

func (f fakeClient) ListSkills(ctx context.Context) ([]onepoint.Skill, error) {
	return nil, errors.New("not implemented in test fake")
}

func (f fakeClient) GetFilteredWorklogs(ctx context.Context, from, to time.Time) ([]onepoint.DayWorklog, error) {
	out := make([]onepoint.DayWorklog, 0, len(f.worklogs))
	for _, item := range f.worklogs {
		day, err := onepoint.ParseDay(item.WorklogDate)
		if err != nil {
			continue
		}
		day = startOfDay(day)
		if day.Before(startOfDay(from)) || day.After(startOfDay(to)) {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (f fakeClient) GetDayWorklogs(ctx context.Context, day time.Time) ([]onepoint.DayWorklog, error) {
	return f.GetFilteredWorklogs(ctx, day, day)
}

func (f fakeClient) PersistWorklogs(ctx context.Context, day time.Time, worklogs []onepoint.PersistWorklog) ([]onepoint.PersistResult, error) {
	return nil, errors.New("not implemented in test fake")
}

func (f fakeClient) FetchLookupSnapshot(ctx context.Context) (onepoint.LookupSnapshot, error) {
	return onepoint.LookupSnapshot{}, errors.New("not implemented in test fake")
}

func (f fakeClient) ResolveIDs(ctx context.Context, projectName, activityName, skillName string, options onepoint.ResolveOptions) (onepoint.ResolvedIDs, error) {
	return onepoint.ResolvedIDs{}, errors.New("not implemented in test fake")
}
