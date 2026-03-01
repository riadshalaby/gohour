package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gohour/config"
	"gohour/internal/timeutil"
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

	client := &fakeClient{
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

	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
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

	client := &fakeClient{
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

	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
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

func TestPatchWorklog_ValidBody(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local))})
	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	id := entries[0].ID

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	body := strings.NewReader(`{"date":"2026-03-01","start":"10:00","end":"11:30","project":"P2","activity":"A2","skill":"S2","billable":90,"description":"updated"}`)
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/worklog/"+strconvI64(id), body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 204, got %d body=%s", resp.StatusCode, string(payload))
	}

	entries, err = store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if got := entries[0]; got.Project != "P2" || got.Activity != "A2" || got.Skill != "S2" || got.Billable != 90 || got.Description != "updated" || got.StartDateTime.Format("15:04") != "10:00" || got.EndDateTime.Format("15:04") != "11:30" {
		t.Fatalf("unexpected updated entry: %+v", got)
	}
}

func TestPatchWorklog_InvalidTime(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local))})
	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	id := entries[0].ID

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	body := strings.NewReader(`{"date":"2026-03-01","start":"99:00","end":"11:30","project":"P2","activity":"A2","skill":"S2","billable":90,"description":"updated"}`)
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/worklog/"+strconvI64(id), body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d body=%s", resp.StatusCode, string(payload))
	}
}

func TestDeleteWorklog_Exists(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local))})
	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	id := entries[0].ID

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/worklog/"+strconvI64(id), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 204, got %d body=%s", resp.StatusCode, string(payload))
	}

	entries, err = store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty store after delete, got %d", len(entries))
	}
}

func TestCreateWorklog_ReturnsID(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	body := strings.NewReader(`{"date":"2026-03-01","start":"09:00","end":"10:30","project":"P","activity":"A","skill":"S","billable":90,"description":"created"}`)
	resp, err := http.Post(ts.URL+"/api/worklog", "application/json", body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d body=%s", resp.StatusCode, string(payload))
	}

	var payload map[string]int64
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	id := payload["id"]
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	entry, found, err := store.GetWorklogByID(id)
	if err != nil {
		t.Fatalf("get worklog by id: %v", err)
	}
	if !found {
		t.Fatalf("expected created entry to exist")
	}
	if entry.Description != "created" || entry.Project != "P" || entry.Activity != "A" || entry.Skill != "S" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
}

func TestSubmitDay_LockedDay(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(day)})

	client := &fakeClient{
		dayWorklogs: map[string][]onepoint.DayWorklog{
			"2026-03-01": {
				{
					WorklogDate: onepoint.FormatDay(day),
					Locked:      1,
					StartTime:   9 * 60,
					FinishTime:  10 * 60,
					ProjectID:   11,
					ActivityID:  22,
					SkillID:     33,
				},
			},
		},
	}

	ts := httptest.NewServer(NewServer(store, client, testConfig([]config.Rule{ruleForLocal()})))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/submit/day/2026-03-01", "application/json", nil)
	if err != nil {
		t.Fatalf("submit day request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	var payload submitResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.LockedDays) != 1 || payload.LockedDays[0] != "2026-03-01" {
		t.Fatalf("unexpected locked days: %+v", payload.LockedDays)
	}
	if client.persistCalls != 0 {
		t.Fatalf("expected no persist calls, got %d", client.persistCalls)
	}
}

func TestSubmitDay_NewEntry(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(day)})

	client := &fakeClient{dayWorklogs: map[string][]onepoint.DayWorklog{}}
	ts := httptest.NewServer(NewServer(store, client, testConfig([]config.Rule{ruleForLocal()})))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/submit/day/2026-03-01", "application/json", nil)
	if err != nil {
		t.Fatalf("submit day request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	var payload submitResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Submitted != 1 {
		t.Fatalf("expected submitted=1, got %d", payload.Submitted)
	}
	if client.persistCalls != 1 {
		t.Fatalf("expected persist call, got %d", client.persistCalls)
	}
}

func TestSubmitDay_LocalErrorReturns500(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	entry := newLocalEntry(day)
	entry.Activity = ""
	insertWorklogs(t, store, []worklog.Entry{entry})

	client := &fakeClient{}
	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/submit/day/2026-03-01", "application/json", nil)
	if err != nil {
		t.Fatalf("submit day request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 500, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestSubmitDay_UpstreamErrorReturns502(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(day)})

	client := &fakeClient{getDayErr: errors.New("onepoint unavailable")}
	ts := httptest.NewServer(NewServer(store, client, testConfig([]config.Rule{ruleForLocal()})))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/submit/day/2026-03-01", "application/json", nil)
	if err != nil {
		t.Fatalf("submit day request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 502, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestImport_ValidFile(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	client := &fakeClient{}
	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("description,startdatetime,enddatetime,project,activity,skill\nTask,2026-03-01 09:00,2026-03-01 10:00,P,A,S\n"))
	_ = writer.WriteField("mapper", "generic")
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	resp, err := http.Post(ts.URL+"/api/import", writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatalf("import request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(payload))
	}

	var payload importResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.RowsPersisted != 1 {
		t.Fatalf("expected rowsPersisted=1, got %d", payload.RowsPersisted)
	}

	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one imported entry, got %d", len(entries))
	}
}

func TestGetLookup_ReturnsJSON(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	client := &fakeClient{
		snapshot: onepoint.LookupSnapshot{
			Projects: []onepoint.Project{
				{ID: 1, Name: "Project A", Archived: "0"},
			},
			Activities: []onepoint.Activity{
				{ID: 2, Name: "Activity B", ProjectNodeID: 1, Locked: false},
			},
			Skills: []onepoint.Skill{
				{SkillID: 3, Name: "Skill C", ActivityID: 2},
			},
		},
	}
	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/lookup")
	if err != nil {
		t.Fatalf("lookup request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	var payload map[string][]map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload["projects"]) != 1 || payload["projects"][0]["name"] != "Project A" {
		t.Fatalf("unexpected projects payload: %+v", payload["projects"])
	}
	if payload["projects"][0]["id"] != float64(1) {
		t.Fatalf("unexpected project id payload: %+v", payload["projects"][0])
	}
	if len(payload["activities"]) != 1 || payload["activities"][0]["name"] != "Activity B" {
		t.Fatalf("unexpected activities payload: %+v", payload["activities"])
	}
	if payload["activities"][0]["id"] != float64(2) || payload["activities"][0]["projectId"] != float64(1) {
		t.Fatalf("unexpected activity ids payload: %+v", payload["activities"][0])
	}
	if len(payload["skills"]) != 1 || payload["skills"][0]["name"] != "Skill C" {
		t.Fatalf("unexpected skills payload: %+v", payload["skills"])
	}
	if payload["skills"][0]["id"] != float64(3) || payload["skills"][0]["activityId"] != float64(2) {
		t.Fatalf("unexpected skill ids payload: %+v", payload["skills"][0])
	}
}

func TestGetLookup_CachedOnSecondCall(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	client := &fakeClient{
		snapshot: onepoint.LookupSnapshot{
			Projects: []onepoint.Project{{ID: 1, Name: "Project A", Archived: "0"}},
		},
	}
	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	for i := 0; i < 2; i++ {
		resp, err := http.Get(ts.URL + "/api/lookup")
		if err != nil {
			t.Fatalf("lookup request %d: %v", i+1, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 on request %d, got %d", i+1, resp.StatusCode)
		}
	}

	if client.snapshotCalls != 1 {
		t.Fatalf("expected one snapshot fetch, got %d", client.snapshotCalls)
	}
}

func newLocalEntry(start time.Time) worklog.Entry {
	return worklog.Entry{
		StartDateTime: start,
		EndDateTime:   start.Add(1 * time.Hour),
		Billable:      60,
		Description:   "task",
		Project:       "P",
		Activity:      "A",
		Skill:         "S",
		SourceFormat:  "csv",
		SourceMapper:  "generic",
		SourceFile:    "source.csv",
	}
}

func ruleForLocal() config.Rule {
	return config.Rule{
		Mapper:     "generic",
		Project:    "P",
		Activity:   "A",
		Skill:      "S",
		ProjectID:  100,
		ActivityID: 200,
		SkillID:    300,
	}
}

func testConfig(rules []config.Rule) config.Config {
	return config.Config{
		OnePoint: config.OnePointConfig{URL: "https://onepoint.virtual7.io/onepoint/faces/home"},
		Import:   config.ImportConfig{AutoReconcileAfterImport: false},
		Rules:    rules,
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
	worklogs      []onepoint.DayWorklog
	dayWorklogs   map[string][]onepoint.DayWorklog
	snapshot      onepoint.LookupSnapshot
	snapshotCalls int
	persistCalls  int
	persistByDate map[string][]onepoint.PersistWorklog
	getDayErr     error
	persistErr    error
	snapshotErr   error
}

func (f *fakeClient) ListProjects(ctx context.Context) ([]onepoint.Project, error) {
	return nil, errors.New("not implemented in test fake")
}

func (f *fakeClient) ListActivities(ctx context.Context) ([]onepoint.Activity, error) {
	return nil, errors.New("not implemented in test fake")
}

func (f *fakeClient) ListSkills(ctx context.Context) ([]onepoint.Skill, error) {
	return nil, errors.New("not implemented in test fake")
}

func (f *fakeClient) GetFilteredWorklogs(ctx context.Context, from, to time.Time) ([]onepoint.DayWorklog, error) {
	out := make([]onepoint.DayWorklog, 0, len(f.worklogs))
	for _, item := range f.worklogs {
		day, err := onepoint.ParseDay(item.WorklogDate)
		if err != nil {
			continue
		}
		day = timeutil.StartOfDay(day)
		if day.Before(timeutil.StartOfDay(from)) || day.After(timeutil.StartOfDay(to)) {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (f *fakeClient) GetDayWorklogs(ctx context.Context, day time.Time) ([]onepoint.DayWorklog, error) {
	if f.getDayErr != nil {
		return nil, f.getDayErr
	}
	key := timeutil.StartOfDay(day).Format("2006-01-02")
	if f.dayWorklogs != nil {
		if values, ok := f.dayWorklogs[key]; ok {
			return append([]onepoint.DayWorklog(nil), values...), nil
		}
	}
	return f.GetFilteredWorklogs(ctx, day, day)
}

func (f *fakeClient) PersistWorklogs(ctx context.Context, day time.Time, worklogs []onepoint.PersistWorklog) ([]onepoint.PersistResult, error) {
	if f.persistErr != nil {
		return nil, f.persistErr
	}
	f.persistCalls++
	if f.persistByDate == nil {
		f.persistByDate = make(map[string][]onepoint.PersistWorklog)
	}
	key := timeutil.StartOfDay(day).Format("2006-01-02")
	f.persistByDate[key] = append([]onepoint.PersistWorklog(nil), worklogs...)
	return []onepoint.PersistResult{{OldTimeRecordID: -1, NewTimeRecordID: 1}}, nil
}

func (f *fakeClient) FetchLookupSnapshot(ctx context.Context) (onepoint.LookupSnapshot, error) {
	f.snapshotCalls++
	if f.snapshotErr != nil {
		return onepoint.LookupSnapshot{}, f.snapshotErr
	}
	if len(f.snapshot.Projects) == 0 && len(f.snapshot.Activities) == 0 && len(f.snapshot.Skills) == 0 {
		return onepoint.LookupSnapshot{}, errors.New("not implemented in test fake")
	}
	return f.snapshot, nil
}

func (f *fakeClient) ResolveIDs(ctx context.Context, projectName, activityName, skillName string, options onepoint.ResolveOptions) (onepoint.ResolvedIDs, error) {
	return onepoint.ResolvedIDs{}, errors.New("not implemented in test fake")
}

func strconvI64(value int64) string {
	return strconv.FormatInt(value, 10)
}
