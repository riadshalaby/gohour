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
	if strings.Contains(text, `id="month-submit-result"`) {
		t.Fatalf("month page still contains inline month submit result placeholder")
	}
	if !strings.Contains(text, `id="submit-dialog"`) {
		t.Fatalf("month page missing shared submit dialog: %s", text)
	}
	if strings.Contains(text, "Preview submit") {
		t.Fatalf("month page still contains preview submit control")
	}
	if !strings.Contains(text, `id="submit-dry-run"`) {
		t.Fatalf("month page missing unified submit dry-run toggle")
	}
	if !strings.Contains(text, `id="month-remote-refreshed-at"`) {
		t.Fatalf("month page missing remote refresh timestamp field")
	}
	if !strings.Contains(text, `id="month-auth-error"`) {
		t.Fatalf("month page missing dynamic auth error surface for in-place reloads")
	}
	// Actions menu is now an Alpine.js dropdown (x-data) instead of <details>
	if !strings.Contains(text, `class="actions-menu"`) {
		t.Fatalf("month page missing actions menu")
	}
	// JS is now external; verify the app.js script tag is present
	if !strings.Contains(text, `src="/static/js/app.js"`) {
		t.Fatalf("month page missing external app.js script tag")
	}
	for _, action := range []string{"Refresh remote", "Copy from remote", "Delete all local", "Delete all remote", "Import file"} {
		if !strings.Contains(text, action) {
			t.Fatalf("month page missing action %q", action)
		}
	}
}

func TestServer_MonthPageRemoteErrorRendersAuthBanner(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})

	client := &fakeClient{filteredErr: errors.New("session expired")}
	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/month/2026-03")
	if err != nil {
		t.Fatalf("request month page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if !strings.Contains(text, "OnePoint session may have expired") {
		t.Fatalf("expected auth error banner, got: %s", text)
	}
	if !strings.Contains(text, "gohour auth login") {
		t.Fatalf("expected auth login hint, got: %s", text)
	}
	if !strings.Contains(text, "2026-03-01") {
		t.Fatalf("expected local month data to render, got: %s", text)
	}
}

func TestServer_APIMonth_ReturnsRowsAndRefreshTimestamp(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})

	client := &fakeClient{
		worklogs: []onepoint.DayWorklog{
			{
				WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)),
				StartTime:   9 * 60,
				FinishTime:  10 * 60,
				Billable:    60,
			},
		},
	}
	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/month/2026-03")
	if err != nil {
		t.Fatalf("request month api: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	var payload monthAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Month != "2026-03" {
		t.Fatalf("unexpected month in payload: %+v", payload)
	}
	if len(payload.Rows) != 31 {
		t.Fatalf("expected 31 rows in month payload, got %d", len(payload.Rows))
	}
	if payload.RemoteRefreshedAt == "" {
		t.Fatalf("expected non-empty refresh timestamp: %+v", payload)
	}
	if _, err := time.Parse(time.RFC3339, payload.RemoteRefreshedAt); err != nil {
		t.Fatalf("expected RFC3339 refresh timestamp, got %q", payload.RemoteRefreshedAt)
	}
}

func TestServer_APIMonth_RemoteErrorWithoutRefresh_DegradesGracefully(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})
	client := &fakeClient{filteredErr: errors.New("session expired")}
	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/month/2026-03")
	if err != nil {
		t.Fatalf("request month api: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	var payload monthAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.AuthErrorMsg == "" {
		t.Fatalf("expected auth error message in payload")
	}
	if payload.TotalLocal <= 0 {
		t.Fatalf("expected local totals in degraded payload: %+v", payload)
	}
}

func TestServer_APIMonth_RemoteErrorWithRefresh_ReturnsBadGateway(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})
	client := &fakeClient{filteredErr: errors.New("session expired")}
	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/month/2026-03?refresh=1")
	if err != nil {
		t.Fatalf("request month api refresh: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 502, got %d body=%s", resp.StatusCode, string(body))
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
	for _, label := range []string{"synced", "conflict", "local", "remote"} {
		if !strings.Contains(text, label) {
			t.Fatalf("expected badge label %q in response body", label)
		}
	}
	if strings.Contains(text, "Preview submit") {
		t.Fatalf("day page still contains preview submit control")
	}
	if !strings.Contains(text, `id="day-remote-refreshed-at"`) {
		t.Fatalf("day page missing remote refresh timestamp field")
	}
}

func TestServer_DayPageRemoteErrorRendersAuthBanner(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})

	client := &fakeClient{filteredErr: errors.New("session expired")}
	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/day/2026-03-01")
	if err != nil {
		t.Fatalf("request day page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if !strings.Contains(text, "OnePoint session may have expired") {
		t.Fatalf("expected auth error banner, got: %s", text)
	}
	if !strings.Contains(text, "gohour auth login") {
		t.Fatalf("expected auth login hint, got: %s", text)
	}
	if !strings.Contains(text, "badge-local") {
		t.Fatalf("expected local data to render, got: %s", text)
	}
}

func TestServer_DayPageIncludesResponsiveTableAndEditDialog(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local))})

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
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
	for _, needle := range []string{
		// HTML structure still in templates
		`<div class="table-wrap">`,
		`id="edit-dialog"`,
		`id="confirm-dialog"`,
		`id="submit-dialog"`,
		`id="submit-dry-run"`,
		`class="dialog-row"`,
		`<textarea id="edit-description" name="description" rows="3"`,
		`class="dialog-readonly"`,
		`class="dialog-footer"`,
		`id="edit-dialog-error"`,
		`x-bind:hx-post="$store.edit.endpoint"`,
		`@htmx:before-request="handleSubmitBeforeRequest($event)"`,
		`id="day-refresh-head"`,
		`id="preview-status"`,
		// JS and CSS are now external — verify the asset tags are present
		`src="/static/js/app.js"`,
		`href="/static/css/tokens.css"`,
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("day page missing %q", needle)
		}
	}
	if strings.Contains(text, "Import file") {
		t.Fatalf("day page should not expose import action")
	}
	if strings.Contains(text, `id="day-refresh-spinner"`) {
		t.Fatalf("day page should not render inline refresh spinner in button")
	}
	if strings.Contains(text, `id="day-import-dialog"`) {
		t.Fatalf("day page should not include day import dialog")
	}
	if strings.Contains(text, `id="edit-date"`) {
		t.Fatalf("day page still contains separate dialog date field")
	}
	if strings.Contains(text, `onclick="saveRow(this)"`) {
		t.Fatalf("day page still contains inline save action")
	}
	if strings.Contains(text, `>JSON<`) {
		t.Fatalf("day page still contains JSON nav link")
	}
	if strings.Contains(text, `id="day-submit-result"`) {
		t.Fatalf("day page still contains inline day submit result placeholder")
	}
}

func TestServer_APIDay_RefreshForcesRemoteReload(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})
	client := &fakeClient{
		worklogs: []onepoint.DayWorklog{
			{
				WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)),
				StartTime:   9 * 60,
				FinishTime:  10 * 60,
				Billable:    60,
			},
		},
	}
	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	firstResp, err := http.Get(ts.URL + "/api/day/2026-03-01")
	if err != nil {
		t.Fatalf("first day request: %v", err)
	}
	defer firstResp.Body.Close()
	if firstResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(firstResp.Body)
		t.Fatalf("expected 200, got %d body=%s", firstResp.StatusCode, string(body))
	}
	var first dayAPIResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if first.RemoteHours != 1.0 {
		t.Fatalf("expected first remote hours=1.0, got %+v", first)
	}
	if first.RemoteRefreshedAt == "" {
		t.Fatalf("expected first response refresh timestamp")
	}
	if client.filteredCalls != 1 {
		t.Fatalf("expected one remote fetch, got %d", client.filteredCalls)
	}

	client.worklogs = []onepoint.DayWorklog{
		{
			WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)),
			StartTime:   9 * 60,
			FinishTime:  10 * 60,
			Billable:    30,
		},
	}

	cachedResp, err := http.Get(ts.URL + "/api/day/2026-03-01")
	if err != nil {
		t.Fatalf("cached day request: %v", err)
	}
	defer cachedResp.Body.Close()
	var cached dayAPIResponse
	if err := json.NewDecoder(cachedResp.Body).Decode(&cached); err != nil {
		t.Fatalf("decode cached response: %v", err)
	}
	if cached.RemoteHours != 1.0 {
		t.Fatalf("expected cached remote hours=1.0, got %+v", cached)
	}
	if client.filteredCalls != 1 {
		t.Fatalf("expected cached request to avoid refetch, got %d", client.filteredCalls)
	}

	refreshedResp, err := http.Get(ts.URL + "/api/day/2026-03-01?refresh=1")
	if err != nil {
		t.Fatalf("refresh day request: %v", err)
	}
	defer refreshedResp.Body.Close()
	var refreshed dayAPIResponse
	if err := json.NewDecoder(refreshedResp.Body).Decode(&refreshed); err != nil {
		t.Fatalf("decode refreshed response: %v", err)
	}
	if refreshed.RemoteHours != 0.5 {
		t.Fatalf("expected refreshed remote hours=0.5, got %+v", refreshed)
	}
	if refreshed.RemoteRefreshedAt == "" {
		t.Fatalf("expected refreshed response timestamp")
	}
	if client.filteredCalls != 2 {
		t.Fatalf("expected forced refresh to refetch remote data, got %d calls", client.filteredCalls)
	}
}

// ── HTMX partial route tests ──────────────────────────────────────────────────

func TestServer_PartialMonth_ReturnsRows(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/partials/month/2026-03")
	if err != nil {
		t.Fatalf("request partial month: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if !strings.Contains(text, `data-date="2026-03-01"`) {
		t.Fatalf("partial month missing first day row: %s", text)
	}
	if !strings.Contains(text, `data-date="2026-03-31"`) {
		t.Fatalf("partial month missing last day row: %s", text)
	}
	// OOB stat card block should be present
	if !strings.Contains(text, `id="month-stats"`) {
		t.Fatalf("partial month missing OOB month-stats block: %s", text)
	}
}

func TestServer_PartialMonth_RefreshForcesRemoteReload(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})
	client := &fakeClient{
		worklogs: []onepoint.DayWorklog{
			{
				WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)),
				StartTime:   9 * 60,
				FinishTime:  10 * 60,
				Billable:    60,
			},
		},
	}
	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	// First request: primes the cache.
	resp1, err := http.Get(ts.URL + "/partials/month/2026-03")
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	io.ReadAll(resp1.Body)
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on first request, got %d", resp1.StatusCode)
	}
	callsAfterFirst := client.filteredCalls

	// Change remote data.
	client.worklogs = []onepoint.DayWorklog{
		{
			WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)),
			StartTime:   9 * 60,
			FinishTime:  11 * 60,
			Billable:    120,
		},
	}

	// Cached request: should not re-fetch remote.
	resp2, err := http.Get(ts.URL + "/partials/month/2026-03")
	if err != nil {
		t.Fatalf("cached request: %v", err)
	}
	io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if client.filteredCalls != callsAfterFirst {
		t.Fatalf("expected cached request to skip remote fetch, got %d total calls", client.filteredCalls)
	}

	// Refresh request: must force a new remote fetch.
	resp3, err := http.Get(ts.URL + "/partials/month/2026-03?refresh=1")
	if err != nil {
		t.Fatalf("refresh request: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp3.Body)
		t.Fatalf("expected 200 on refresh, got %d body=%s", resp3.StatusCode, string(body))
	}
	if client.filteredCalls <= callsAfterFirst {
		t.Fatalf("expected forced refresh to re-fetch remote, got %d total calls", client.filteredCalls)
	}
}

func TestServer_PartialMonth_AuthError_Returns502OnRefresh(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})
	client := &fakeClient{filteredErr: errors.New("session expired")}
	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	// Without ?refresh=1: graceful degradation (200).
	resp, err := http.Get(ts.URL + "/partials/month/2026-03")
	if err != nil {
		t.Fatalf("request partial month without refresh: %v", err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 without refresh, got %d", resp.StatusCode)
	}

	// With ?refresh=1: must return 502.
	resp2, err := http.Get(ts.URL + "/partials/month/2026-03?refresh=1")
	if err != nil {
		t.Fatalf("request partial month with refresh: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadGateway {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 502 with refresh on auth error, got %d body=%s", resp2.StatusCode, string(body))
	}
}

func TestServer_PartialDay_ReturnsEntries(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/partials/day/2026-03-01")
	if err != nil {
		t.Fatalf("request partial day: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	// Local entry row with data-source attribute should be present.
	if !strings.Contains(text, `data-source="local"`) {
		t.Fatalf("partial day missing local entry row: %s", text)
	}
	// OOB stat card for day should be present.
	if !strings.Contains(text, `id="day-local-worked"`) {
		t.Fatalf("partial day missing OOB day-local-worked element: %s", text)
	}
}

func TestServer_PartialDay_AuthError_GracefulWithoutRefreshAnd502WithRefresh(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})
	client := &fakeClient{filteredErr: errors.New("session expired")}
	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	// Without refresh, day partial should degrade gracefully to local-only data.
	resp, err := http.Get(ts.URL + "/partials/day/2026-03-01")
	if err != nil {
		t.Fatalf("request partial day without refresh: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 without refresh, got %d body=%s", resp.StatusCode, string(body))
	}
	if !strings.Contains(string(body), `data-source="local"`) {
		t.Fatalf("expected local row in graceful partial response, got %s", string(body))
	}

	// Refresh should fail closed.
	resp, err = http.Get(ts.URL + "/partials/day/2026-03-01?refresh=1")
	if err != nil {
		t.Fatalf("request partial day with refresh: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 502 on auth error with refresh, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestServer_PartialWorklogCreate_ReturnsUpdatedDayPartial(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	form := "date=2026-03-01&start=09:00&end=10:00&project=P&activity=A&skill=S&billableHours=1&description=partial-create"
	req, _ := http.NewRequest(
		http.MethodPost,
		ts.URL+"/partials/day/2026-03-01/worklog",
		strings.NewReader(form),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("partial create request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if !strings.Contains(text, `data-source="local"`) {
		t.Fatalf("expected updated day rows in partial response, got %s", text)
	}
	if !strings.Contains(text, `id="day-local-worked"`) {
		t.Fatalf("expected OOB day stat updates in partial response, got %s", text)
	}

	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one inserted worklog, got %d", len(entries))
	}
}

func TestServer_PartialWorklogUpdate_ReturnsUpdatedDayPartial(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})
	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	id := entries[0].ID

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	form := "date=2026-03-01&start=10:00&end=11:30&project=P2&activity=A2&skill=S2&billableHours=1.5&description=partial-update"
	req, _ := http.NewRequest(
		http.MethodPost,
		ts.URL+"/partials/day/2026-03-01/worklog/"+strconvI64(id),
		strings.NewReader(form),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("partial update request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}
	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if !strings.Contains(text, `data-id="`+strconvI64(id)+`"`) {
		t.Fatalf("expected updated row id in partial response, got %s", text)
	}
	if !strings.Contains(text, "partial-update") {
		t.Fatalf("expected updated description in partial response, got %s", text)
	}

	updated, found, err := store.GetWorklogByID(id)
	if err != nil {
		t.Fatalf("get updated worklog: %v", err)
	}
	if !found {
		t.Fatalf("updated worklog missing")
	}
	if updated.Description != "partial-update" {
		t.Fatalf("expected description to be updated, got %q", updated.Description)
	}
}

func TestServer_PartialWorklogDelete_ReturnsUpdatedDayPartial(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})
	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	id := entries[0].ID

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	req, _ := http.NewRequest(
		http.MethodPost,
		ts.URL+"/partials/day/2026-03-01/worklog/"+strconvI64(id)+"/delete",
		strings.NewReader(""),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("partial delete request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), `data-id="`+strconvI64(id)+`"`) {
		t.Fatalf("deleted row id still present in partial response: %s", string(body))
	}

	remaining, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected no remaining worklogs, got %d", len(remaining))
	}
}

func TestServer_PartialSubmitDay_DryRunAndSubmit(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(day)})
	client := &fakeClient{dayWorklogs: map[string][]onepoint.DayWorklog{}}

	ts := httptest.NewServer(NewServer(store, client, testConfig([]config.Rule{ruleForLocal()})))
	defer ts.Close()

	dryRunReq, _ := http.NewRequest(
		http.MethodPost,
		ts.URL+"/partials/submit/day/2026-03-01",
		strings.NewReader("dry_run=true"),
	)
	dryRunReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	dryRunResp, err := http.DefaultClient.Do(dryRunReq)
	if err != nil {
		t.Fatalf("partial dry-run submit request: %v", err)
	}
	defer dryRunResp.Body.Close()

	if dryRunResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(dryRunResp.Body)
		t.Fatalf("expected 200, got %d body=%s", dryRunResp.StatusCode, string(body))
	}
	if dryRunResp.Header.Get("HX-Trigger") != "" {
		t.Fatalf("expected no HX-Trigger on dry-run, got %q", dryRunResp.Header.Get("HX-Trigger"))
	}
	dryRunBody, _ := io.ReadAll(dryRunResp.Body)
	if !strings.Contains(string(dryRunBody), "Preview only") {
		t.Fatalf("expected dry-run preview message, got %s", string(dryRunBody))
	}

	submitResp, err := http.Post(
		ts.URL+"/partials/submit/day/2026-03-01",
		"application/x-www-form-urlencoded",
		strings.NewReader(""),
	)
	if err != nil {
		t.Fatalf("partial submit request: %v", err)
	}
	defer submitResp.Body.Close()

	if submitResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(submitResp.Body)
		t.Fatalf("expected 200, got %d body=%s", submitResp.StatusCode, string(body))
	}
	if !strings.Contains(submitResp.Header.Get("HX-Trigger"), "refresh-day") {
		t.Fatalf("expected refresh-day HX-Trigger header, got %q", submitResp.Header.Get("HX-Trigger"))
	}
	submitBody, _ := io.ReadAll(submitResp.Body)
	if !strings.Contains(string(submitBody), "Added") {
		t.Fatalf("expected submit result fragment, got %s", string(submitBody))
	}
}

func TestServer_PartialSubmitMonth_DryRunAndSubmit(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(day)})
	client := &fakeClient{dayWorklogs: map[string][]onepoint.DayWorklog{}}

	ts := httptest.NewServer(NewServer(store, client, testConfig([]config.Rule{ruleForLocal()})))
	defer ts.Close()

	dryRunReq, _ := http.NewRequest(
		http.MethodPost,
		ts.URL+"/partials/submit/month/2026-03",
		strings.NewReader("dry_run=true"),
	)
	dryRunReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	dryRunResp, err := http.DefaultClient.Do(dryRunReq)
	if err != nil {
		t.Fatalf("partial month dry-run submit request: %v", err)
	}
	defer dryRunResp.Body.Close()

	if dryRunResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(dryRunResp.Body)
		t.Fatalf("expected 200, got %d body=%s", dryRunResp.StatusCode, string(body))
	}
	if dryRunResp.Header.Get("HX-Trigger") != "" {
		t.Fatalf("expected no HX-Trigger on dry-run, got %q", dryRunResp.Header.Get("HX-Trigger"))
	}
	dryRunBody, _ := io.ReadAll(dryRunResp.Body)
	if !strings.Contains(string(dryRunBody), "Preview only") {
		t.Fatalf("expected dry-run preview message, got %s", string(dryRunBody))
	}

	submitResp, err := http.Post(
		ts.URL+"/partials/submit/month/2026-03",
		"application/x-www-form-urlencoded",
		strings.NewReader(""),
	)
	if err != nil {
		t.Fatalf("partial month submit request: %v", err)
	}
	defer submitResp.Body.Close()

	if submitResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(submitResp.Body)
		t.Fatalf("expected 200, got %d body=%s", submitResp.StatusCode, string(body))
	}
	if !strings.Contains(submitResp.Header.Get("HX-Trigger"), "refresh-month") {
		t.Fatalf("expected refresh-month HX-Trigger header, got %q", submitResp.Header.Get("HX-Trigger"))
	}
	submitBody, _ := io.ReadAll(submitResp.Body)
	if !strings.Contains(string(submitBody), "Locked days") {
		t.Fatalf("expected month submit result fragment, got %s", string(submitBody))
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

func TestCreateWorklog_EmptyProjectRejected(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	body := strings.NewReader(`{"date":"2026-03-01","start":"09:00","end":"10:00","project":"   ","activity":"A","skill":"S","billable":60,"description":"created"}`)
	resp, err := http.Post(ts.URL+"/api/worklog", "application/json", body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d body=%s", resp.StatusCode, string(payload))
	}
}

func TestCreateWorklog_DuplicateConflict(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(day)})

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	body := strings.NewReader(`{"date":"2026-03-01","start":"09:00","end":"10:00","project":"P","activity":"A","skill":"S","billable":60,"description":"duplicate"}`)
	resp, err := http.Post(ts.URL+"/api/worklog", "application/json", body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 409, got %d body=%s", resp.StatusCode, string(payload))
	}

	var payload worklogConflictResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Type != "duplicate" {
		t.Fatalf("expected duplicate conflict, got %+v", payload)
	}
	if payload.ExistingID <= 0 {
		t.Fatalf("expected positive existing id, got %+v", payload)
	}
}

func TestCreateWorklog_DuplicateConflict_NormalizedWhitespace(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	entry := newLocalEntry(day)
	entry.Project = "Project A"
	entry.Activity = "Activity Alpha"
	entry.Skill = "Skill One"
	insertWorklogs(t, store, []worklog.Entry{entry})

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	body := strings.NewReader(`{"date":"2026-03-01","start":"09:00","end":"10:00","project":"  project   a ","activity":" activity    alpha ","skill":" skill   one ","billable":60,"description":"duplicate"}`)
	resp, err := http.Post(ts.URL+"/api/worklog", "application/json", body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 409, got %d body=%s", resp.StatusCode, string(payload))
	}

	var payload worklogConflictResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Type != "duplicate" {
		t.Fatalf("expected duplicate conflict, got %+v", payload)
	}
}

func TestCreateWorklog_OverlapConflict(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(day)})

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	body := strings.NewReader(`{"date":"2026-03-01","start":"09:30","end":"10:30","project":"Other","activity":"Other","skill":"Other","billable":60,"description":"overlap"}`)
	resp, err := http.Post(ts.URL+"/api/worklog", "application/json", body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 409, got %d body=%s", resp.StatusCode, string(payload))
	}

	var payload worklogConflictResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Type != "overlap" {
		t.Fatalf("expected overlap conflict, got %+v", payload)
	}
	if payload.ExistingID <= 0 {
		t.Fatalf("expected positive existing id, got %+v", payload)
	}
}

func TestServer_WorklogCreate_ForceOverlapHeader_Succeeds(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(day)})

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	body := strings.NewReader(`{"date":"2026-03-01","start":"09:30","end":"10:30","project":"Other","activity":"Other","skill":"Other","billable":60,"description":"overlap"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/worklog", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Force-Overlap", "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d body=%s", resp.StatusCode, string(payload))
	}

	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after forced overlap create, got %d", len(entries))
	}
}

func TestServer_WorklogPatch_ForceOverlapHeader_Succeeds(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		{
			StartDateTime: day.Add(9 * time.Hour),
			EndDateTime:   day.Add(10 * time.Hour),
			Billable:      60,
			Description:   "base",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "a.csv",
		},
		{
			StartDateTime: day.Add(11 * time.Hour),
			EndDateTime:   day.Add(12 * time.Hour),
			Billable:      60,
			Description:   "to-edit",
			Project:       "P2",
			Activity:      "A2",
			Skill:         "S2",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "a.csv",
		},
	})

	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	editID := entries[1].ID

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	body := strings.NewReader(`{"date":"2026-03-01","start":"09:30","end":"10:30","project":"P2","activity":"A2","skill":"S2","billable":60,"description":"forced-overlap"}`)
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/worklog/"+strconvI64(editID), body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Force-Overlap", "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 204, got %d body=%s", resp.StatusCode, string(payload))
	}

	updated, found, err := store.GetWorklogByID(editID)
	if err != nil {
		t.Fatalf("get updated worklog: %v", err)
	}
	if !found {
		t.Fatalf("expected updated row to exist")
	}
	if got := updated.StartDateTime.Format("15:04"); got != "09:30" {
		t.Fatalf("expected updated start time 09:30, got %s", got)
	}
}

func TestSubmitDay_LockedDay(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(day)})

	client := &fakeClient{
		snapshot: onepoint.LookupSnapshot{
			Projects: []onepoint.Project{
				{ID: 100, Name: "P", Archived: "0"},
			},
			Activities: []onepoint.Activity{
				{ID: 200, Name: "A", ProjectNodeID: 100, Locked: false},
			},
			Skills: []onepoint.Skill{
				{SkillID: 300, Name: "S", ActivityID: 200},
			},
		},
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

func TestSubmitDay_ChangedSyncedEntry_PropagatesUpdate(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		{
			StartDateTime: day,
			EndDateTime:   day.Add(1 * time.Hour),
			Billable:      30,
			Description:   "updated locally",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "remote",
			SourceMapper:  "onepoint",
			SourceFile:    "onepoint-sync-2026-03",
		},
	})

	client := &fakeClient{
		snapshot: onepoint.LookupSnapshot{
			Projects: []onepoint.Project{
				{ID: 100, Name: "P", Archived: "0"},
			},
			Activities: []onepoint.Activity{
				{ID: 200, Name: "A", ProjectNodeID: 100, Locked: false},
			},
			Skills: []onepoint.Skill{
				{SkillID: 300, Name: "S", ActivityID: 200},
			},
		},
		dayWorklogs: map[string][]onepoint.DayWorklog{
			"2026-03-01": {
				{
					WorklogDate:  onepoint.FormatDay(day),
					StartTime:    9 * 60,
					FinishTime:   10 * 60,
					Billable:     60,
					Comment:      "remote old",
					ProjectID:    100,
					ActivityID:   200,
					SkillID:      300,
					TimeRecordID: 1,
					WorkRecordID: 2,
					WorkSlipID:   3,
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
	if payload.Submitted != 1 {
		t.Fatalf("expected submitted=1 for update, got %+v", payload)
	}
	if client.persistCalls != 1 {
		t.Fatalf("expected persist call, got %d", client.persistCalls)
	}

	persisted := client.persistByDate["2026-03-01"]
	if len(persisted) != 1 {
		t.Fatalf("expected replacement payload with 1 entry, got %+v", persisted)
	}
	if persisted[0].Billable != 30 || persisted[0].Comment != "updated locally" {
		t.Fatalf("expected updated billable/comment in payload, got %+v", persisted[0])
	}
}

func TestServer_SubmitDay_DryRun_DoesNotPersist(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(day)})

	client := &fakeClient{dayWorklogs: map[string][]onepoint.DayWorklog{}}
	ts := httptest.NewServer(NewServer(store, client, testConfig([]config.Rule{ruleForLocal()})))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/submit/day/2026-03-01?dry_run=1", "application/json", nil)
	if err != nil {
		t.Fatalf("submit day dry-run request: %v", err)
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
	if !payload.DryRun {
		t.Fatalf("expected dryRun=true, got %+v", payload)
	}
	if payload.Submitted != 0 {
		t.Fatalf("expected submitted=0 in dry-run, got %d", payload.Submitted)
	}
	if len(payload.Days) != 1 || payload.Days[0].Added != 1 {
		t.Fatalf("expected would-add count in day result, got %+v", payload.Days)
	}
	if client.persistCalls != 0 {
		t.Fatalf("expected no persist calls in dry-run, got %d", client.persistCalls)
	}
}

func TestServer_SubmitMonth_DryRun_DoesNotPersist(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{newLocalEntry(day)})

	client := &fakeClient{dayWorklogs: map[string][]onepoint.DayWorklog{}}
	ts := httptest.NewServer(NewServer(store, client, testConfig([]config.Rule{ruleForLocal()})))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/submit/month/2026-03?dry_run=1", "application/json", nil)
	if err != nil {
		t.Fatalf("submit month dry-run request: %v", err)
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
	if !payload.DryRun {
		t.Fatalf("expected dryRun=true, got %+v", payload)
	}
	if payload.Submitted != 0 {
		t.Fatalf("expected submitted=0 in dry-run, got %d", payload.Submitted)
	}
	if len(payload.Days) != 1 || payload.Days[0].Added != 1 {
		t.Fatalf("expected would-add count in month result, got %+v", payload.Days)
	}
	if client.persistCalls != 0 {
		t.Fatalf("expected no persist calls in dry-run, got %d", client.persistCalls)
	}
}

func TestServer_SubmitDay_AuditSuccessAndFailure(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
		store := openTestStore(t)
		insertWorklogs(t, store, []worklog.Entry{newLocalEntry(day)})

		client := &fakeClient{dayWorklogs: map[string][]onepoint.DayWorklog{}}
		auditSink := &testAuditLogger{}
		handler, ok := NewServer(store, client, testConfig([]config.Rule{ruleForLocal()})).(*Server)
		if !ok {
			t.Fatalf("expected *Server handler")
		}
		handler.audit = auditSink
		ts := httptest.NewServer(handler)
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

		if len(auditSink.records) < 2 {
			t.Fatalf("expected attempt and success audit records, got %+v", auditSink.records)
		}
		attempt := auditSink.records[0]
		success := auditSink.records[len(auditSink.records)-1]
		if attempt.Operation != "submit" || attempt.Scope != "day" || attempt.Target != "2026-03-01" || attempt.Outcome != "attempt" {
			t.Fatalf("unexpected attempt audit record: %+v", attempt)
		}
		if success.Outcome != "success" || success.Submitted != 1 {
			t.Fatalf("unexpected success audit record: %+v", success)
		}
	})

	t.Run("failure", func(t *testing.T) {
		day := time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)
		store := openTestStore(t)
		insertWorklogs(t, store, []worklog.Entry{newLocalEntry(day)})

		client := &fakeClient{getDayErr: errors.New("upstream unavailable")}
		auditSink := &testAuditLogger{}
		handler, ok := NewServer(store, client, testConfig([]config.Rule{ruleForLocal()})).(*Server)
		if !ok {
			t.Fatalf("expected *Server handler")
		}
		handler.audit = auditSink
		ts := httptest.NewServer(handler)
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

		if len(auditSink.records) < 2 {
			t.Fatalf("expected attempt and error audit records, got %+v", auditSink.records)
		}
		failure := auditSink.records[len(auditSink.records)-1]
		if failure.Outcome != "error" || failure.Error == "" {
			t.Fatalf("unexpected failure audit record: %+v", failure)
		}
	})
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

func TestImport_AutoReconcileFailure_ReturnsSuccessWithWarning(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	cfg := testConfig(nil)
	cfg.Import.AutoReconcileAfterImport = true
	client := &fakeClient{filteredErr: errors.New("onepoint unavailable")}
	ts := httptest.NewServer(NewServer(store, client, cfg))
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
		t.Fatalf("expected rowsPersisted=1, got %+v", payload)
	}
	if payload.ReconcileWarning == "" {
		t.Fatalf("expected reconcile warning in partial-success response: %+v", payload)
	}

	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected inserted row despite reconcile failure, got %d", len(entries))
	}
}

func TestServer_Import_AutoReconcileOnlyUnsyncedEntriesInRange(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		{
			StartDateTime: day.Add(9 * time.Hour),
			EndDateTime:   day.Add(10 * time.Hour),
			Billable:      60,
			Description:   "generic-fixed",
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
			Description:   "synced-epm",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "excel",
			SourceMapper:  "epm",
			SourceFile:    "EPMExportRZ202603.xlsx",
		},
		{
			StartDateTime: day.Add(9*time.Hour + 30*time.Minute),
			EndDateTime:   day.Add(10*time.Hour + 30*time.Minute),
			Billable:      60,
			Description:   "unsynced-epm",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "excel",
			SourceMapper:  "epm",
			SourceFile:    "EPMExportRZ202603.xlsx",
		},
	})

	cfg := testConfig(nil)
	cfg.Import.AutoReconcileAfterImport = true
	client := &fakeClient{
		worklogs: []onepoint.DayWorklog{
			{
				WorklogDate: onepoint.FormatDay(day),
				StartTime:   10 * 60,
				FinishTime:  11 * 60,
				Billable:    60,
			},
		},
	}
	handler, ok := NewServer(store, client, cfg).(*Server)
	if !ok {
		t.Fatalf("expected *Server handler")
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("description,startdatetime,enddatetime,project,activity,skill\ntrigger,2026-03-01 13:00,2026-03-01 14:00,P,A,S\n"))
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

	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	var synced, unsynced worklog.Entry
	foundImport := false
	for _, entry := range entries {
		switch entry.Description {
		case "synced-epm":
			synced = entry
		case "unsynced-epm":
			unsynced = entry
		case "trigger":
			foundImport = true
		}
	}
	if !foundImport {
		t.Fatalf("expected imported trigger entry")
	}
	if got := synced.StartDateTime.Format("15:04"); got != "10:00" {
		t.Fatalf("expected synced epm start to stay 10:00, got %s", got)
	}
	if got := synced.EndDateTime.Format("15:04"); got != "11:00" {
		t.Fatalf("expected synced epm end to stay 11:00, got %s", got)
	}
	if got := unsynced.StartDateTime.Format("15:04"); got != "11:00" {
		t.Fatalf("expected unsynced epm start to move to 11:00, got %s", got)
	}
	if got := unsynced.EndDateTime.Format("15:04"); got != "12:00" {
		t.Fatalf("expected unsynced epm end to move to 12:00, got %s", got)
	}
}

func TestServer_Import_AutoReconcile_UsesFreshRemoteData(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		{
			StartDateTime: day.Add(9 * time.Hour),
			EndDateTime:   day.Add(10 * time.Hour),
			Billable:      60,
			Description:   "generic-fixed",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "a.csv",
		},
		{
			StartDateTime: day.Add(9*time.Hour + 30*time.Minute),
			EndDateTime:   day.Add(10*time.Hour + 30*time.Minute),
			Billable:      60,
			Description:   "synced-epm",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "excel",
			SourceMapper:  "epm",
			SourceFile:    "EPMExportRZ202603.xlsx",
		},
	})

	cfg := testConfig(nil)
	cfg.Import.AutoReconcileAfterImport = true
	client := &fakeClient{
		// Prime cache with stale remote data that does NOT include synced time range.
		worklogs: []onepoint.DayWorklog{
			{
				WorklogDate: onepoint.FormatDay(day),
				StartTime:   8 * 60,
				FinishTime:  9 * 60,
				Billable:    60,
			},
		},
	}
	ts := httptest.NewServer(NewServer(store, client, cfg))
	defer ts.Close()

	primeResp, err := http.Get(ts.URL + "/api/day/2026-03-01")
	if err != nil {
		t.Fatalf("prime cache request: %v", err)
	}
	_ = primeResp.Body.Close()
	if primeResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 priming response, got %d", primeResp.StatusCode)
	}
	if client.filteredCalls != 1 {
		t.Fatalf("expected one priming remote call, got %d", client.filteredCalls)
	}

	// Update remote source to current data where synced-epm should be classified as synced.
	client.worklogs = []onepoint.DayWorklog{
		{
			WorklogDate: onepoint.FormatDay(day),
			StartTime:   9*60 + 30,
			FinishTime:  10*60 + 30,
			Billable:    60,
		},
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("description,startdatetime,enddatetime,project,activity,skill\ntrigger,2026-03-01 13:00,2026-03-01 14:00,P,A,S\n"))
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

	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	var synced worklog.Entry
	for _, entry := range entries {
		if entry.Description == "synced-epm" {
			synced = entry
			break
		}
	}
	if synced.ID == 0 {
		t.Fatalf("expected synced epm row")
	}
	if got := synced.StartDateTime.Format("15:04"); got != "09:30" {
		t.Fatalf("expected synced row to remain unchanged with fresh remote lookup, got %s", got)
	}
	if client.filteredCalls < 2 {
		t.Fatalf("expected auto-reconcile to force remote refresh, got %d remote calls", client.filteredCalls)
	}
}

func TestServer_Import_BillableOverrideNonBillable(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte(
		"description,startdatetime,enddatetime,project,activity,skill\n" +
			"Task1,2026-03-01 09:00,2026-03-01 10:00,P,A,S\n" +
			"Task2,2026-03-01 10:00,2026-03-01 11:00,P,A,S\n",
	))
	_ = writer.WriteField("mapper", "generic")
	_ = writer.WriteField("billable", "non-billable")
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

	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(entries))
	}
	for _, entry := range entries {
		if entry.Billable != 0 {
			t.Fatalf("expected non-billable row, got %+v", entry)
		}
	}
}

func TestServer_ImportPreview_ReturnsClassifiedEntries(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
	insertWorklogs(t, store, []worklog.Entry{
		{
			StartDateTime: base.Add(9 * time.Hour),
			EndDateTime:   base.Add(10 * time.Hour),
			Billable:      60,
			Description:   "existing-duplicate",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "existing.csv",
		},
		{
			StartDateTime: base.Add(12 * time.Hour),
			EndDateTime:   base.Add(13 * time.Hour),
			Billable:      60,
			Description:   "existing-overlap",
			Project:       "X",
			Activity:      "Y",
			Skill:         "Z",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "existing.csv",
		},
	})

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte(
		"description,startdatetime,enddatetime,project,activity,skill\n" +
			"dup,2026-03-01 09:00,2026-03-01 10:00,P,A,S\n" +
			"ovl,2026-03-01 12:30,2026-03-01 13:30,Q,R,T\n" +
			"ok,2026-03-01 14:00,2026-03-01 15:00,Q,R,T\n",
	))
	_ = writer.WriteField("mapper", "generic")
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	resp, err := http.Post(ts.URL+"/api/import-preview", writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatalf("import preview request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(payload))
	}

	var payload importPreviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Entries) != 3 {
		t.Fatalf("expected 3 preview entries, got %d", len(payload.Entries))
	}
	if payload.Entries[0].Status != "duplicate" {
		t.Fatalf("expected first row duplicate, got %+v", payload.Entries[0])
	}
	if payload.Entries[1].Status != "overlap" {
		t.Fatalf("expected second row overlap, got %+v", payload.Entries[1])
	}
	if payload.Entries[2].Status != "clean" {
		t.Fatalf("expected third row clean, got %+v", payload.Entries[2])
	}
	if payload.Entries[0].ConflictID <= 0 || payload.Entries[1].ConflictID <= 0 {
		t.Fatalf("expected conflict IDs for duplicate/overlap, got %+v", payload.Entries)
	}
}

func TestServer_Import_SkipIndices(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte(
		"description,startdatetime,enddatetime,project,activity,skill\n" +
			"Task1,2026-03-01 09:00,2026-03-01 10:00,P,A,S\n" +
			"Task2,2026-03-01 10:00,2026-03-01 11:00,P,A,S\n" +
			"Task3,2026-03-01 11:00,2026-03-01 12:00,P,A,S\n",
	))
	_ = writer.WriteField("mapper", "generic")
	_ = writer.WriteField("skipIndices", "0,1")
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

	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only 1 inserted row, got %d", len(entries))
	}
	if got := entries[0].StartDateTime.Format("15:04"); got != "11:00" {
		t.Fatalf("expected only third row to remain, got %s", got)
	}
}

func TestImport_OverlapConflictReturns409(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})
	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("description,startdatetime,enddatetime,project,activity,skill\nTask,2026-03-01 09:30,2026-03-01 10:30,P2,A2,S2\n"))
	_ = writer.WriteField("mapper", "generic")
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	resp, err := http.Post(ts.URL+"/api/import", writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatalf("import request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 409, got %d body=%s", resp.StatusCode, string(payload))
	}

	var payload importConflictResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Overlaps) != 1 {
		t.Fatalf("expected one overlap, got %+v", payload)
	}
	if payload.Overlaps[0].ExistingID <= 0 {
		t.Fatalf("expected overlap existing id, got %+v", payload.Overlaps[0])
	}
}

func TestImport_OverlapSkipPersistsOnlyCleanRows(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})
	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte(
		"description,startdatetime,enddatetime,project,activity,skill\n" +
			"Overlap,2026-03-01 09:30,2026-03-01 10:30,P2,A2,S2\n" +
			"Clean,2026-03-01 11:00,2026-03-01 12:00,P3,A3,S3\n",
	))
	_ = writer.WriteField("mapper", "generic")
	_ = writer.WriteField("skipOverlapping", "true")
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
	if payload.RowsPersisted != 1 || payload.OverlapsSkipped != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestImport_OverlapForcePersistsOverlappingRows(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		newLocalEntry(time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local)),
	})
	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("description,startdatetime,enddatetime,project,activity,skill\nOverlap,2026-03-01 09:30,2026-03-01 10:30,P2,A2,S2\n"))
	_ = writer.WriteField("mapper", "generic")
	_ = writer.WriteField("forceOverlapping", "true")
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
	if payload.RowsPersisted != 1 || payload.OverlapsSkipped != 0 {
		t.Fatalf("unexpected payload: %+v", payload)
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

func TestServer_DeleteMonthWorklogs(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		{
			StartDateTime: time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local),
			EndDateTime:   time.Date(2026, 3, 1, 10, 0, 0, 0, time.Local),
			Billable:      60,
			Description:   "march-a",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "a.csv",
		},
		{
			StartDateTime: time.Date(2026, 3, 15, 9, 0, 0, 0, time.Local),
			EndDateTime:   time.Date(2026, 3, 15, 10, 0, 0, 0, time.Local),
			Billable:      60,
			Description:   "march-b",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "b.csv",
		},
		{
			StartDateTime: time.Date(2026, 4, 2, 9, 0, 0, 0, time.Local),
			EndDateTime:   time.Date(2026, 4, 2, 10, 0, 0, 0, time.Local),
			Billable:      60,
			Description:   "april-a",
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceFormat:  "csv",
			SourceMapper:  "generic",
			SourceFile:    "c.csv",
		},
	})

	ts := httptest.NewServer(NewServer(store, &fakeClient{}, testConfig(nil)))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/month/2026-03/worklogs", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete month request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	var payload map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["deleted"] != 2 {
		t.Fatalf("expected deleted=2, got %+v", payload)
	}

	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one remaining row, got %d", len(entries))
	}
	if got := entries[0].StartDateTime.Format("2006-01"); got != "2026-04" {
		t.Fatalf("expected remaining row in april, got %s", got)
	}
}

func TestServer_DeleteMonthRemoteWorklogs_SkipsLocked(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	day1 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
	day2 := time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local)

	client := &fakeClient{
		worklogs: []onepoint.DayWorklog{
			{
				WorklogDate: onepoint.FormatDay(day1),
				StartTime:   9 * 60,
				FinishTime:  10 * 60,
				Billable:    60,
				ProjectID:   11,
				ActivityID:  22,
				SkillID:     33,
			},
			{
				WorklogDate: onepoint.FormatDay(day2),
				StartTime:   10 * 60,
				FinishTime:  11 * 60,
				Billable:    60,
				ProjectID:   11,
				ActivityID:  22,
				SkillID:     33,
			},
		},
		dayWorklogs: map[string][]onepoint.DayWorklog{
			"2026-03-01": {
				{
					WorklogDate: onepoint.FormatDay(day1),
					Locked:      1,
					StartTime:   9 * 60,
					FinishTime:  10 * 60,
					ProjectID:   11,
					ActivityID:  22,
					SkillID:     33,
				},
			},
			"2026-03-02": {
				{
					WorklogDate: onepoint.FormatDay(day2),
					Locked:      0,
					StartTime:   10 * 60,
					FinishTime:  11 * 60,
					ProjectID:   11,
					ActivityID:  22,
					SkillID:     33,
				},
			},
		},
	}

	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/month/2026-03/remote-worklogs", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete remote month request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if int(payload["deleted"].(float64)) != 1 {
		t.Fatalf("expected deleted=1, got %+v", payload)
	}
	if int(payload["skippedLocked"].(float64)) != 1 {
		t.Fatalf("expected skippedLocked=1, got %+v", payload)
	}
	lockedDays, ok := payload["lockedDays"].([]any)
	if !ok || len(lockedDays) != 1 || lockedDays[0].(string) != "2026-03-01" {
		t.Fatalf("unexpected lockedDays payload: %+v", payload["lockedDays"])
	}

	expectedPersistCalls := 31 - 1 // March days minus one locked day
	if client.persistCalls != expectedPersistCalls {
		t.Fatalf("expected %d clear persist calls, got %d", expectedPersistCalls, client.persistCalls)
	}
	day1Payload := client.persistByDate["2026-03-01"]
	if day1Payload != nil {
		t.Fatalf("expected no persist payload for locked day, got %+v", day1Payload)
	}
	day2Payload, ok := client.persistByDate["2026-03-02"]
	if !ok {
		t.Fatalf("expected persist call for unlocked day")
	}
	if len(day2Payload) != 0 {
		t.Fatalf("expected empty payload to clear day, got %+v", day2Payload)
	}
	if _, ok := client.persistByDate["2026-03-03"]; !ok {
		t.Fatalf("expected clear call for empty day to catch stale month totals")
	}
}

func TestServer_DeleteMonthRemoteWorklogs_AuditSuccessAndFailure(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		store := openTestStore(t)
		client := &fakeClient{}
		auditSink := &testAuditLogger{}
		handler, ok := NewServer(store, client, testConfig(nil)).(*Server)
		if !ok {
			t.Fatalf("expected *Server handler")
		}
		handler.audit = auditSink
		ts := httptest.NewServer(handler)
		defer ts.Close()

		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/month/2026-03/remote-worklogs", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete remote month request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
		}

		if len(auditSink.records) < 2 {
			t.Fatalf("expected attempt and success audit records, got %+v", auditSink.records)
		}
		last := auditSink.records[len(auditSink.records)-1]
		if last.Operation != "delete_remote_month" || last.Outcome != "success" {
			t.Fatalf("unexpected success audit record: %+v", last)
		}
	})

	t.Run("failure", func(t *testing.T) {
		store := openTestStore(t)
		client := &fakeClient{getDayErr: errors.New("cannot load day")}
		auditSink := &testAuditLogger{}
		handler, ok := NewServer(store, client, testConfig(nil)).(*Server)
		if !ok {
			t.Fatalf("expected *Server handler")
		}
		handler.audit = auditSink
		ts := httptest.NewServer(handler)
		defer ts.Close()

		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/month/2026-03/remote-worklogs", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete remote month request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadGateway {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 502, got %d body=%s", resp.StatusCode, string(body))
		}

		if len(auditSink.records) < 2 {
			t.Fatalf("expected attempt and error audit records, got %+v", auditSink.records)
		}
		last := auditSink.records[len(auditSink.records)-1]
		if last.Operation != "delete_remote_month" || last.Outcome != "error" || last.Error == "" {
			t.Fatalf("unexpected failure audit record: %+v", last)
		}
	})
}

func TestServer_CopyMonthRemote(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	client := &fakeClient{
		snapshot: onepoint.LookupSnapshot{
			Projects: []onepoint.Project{{ID: 11, Name: "Project A", Archived: "0"}},
			Activities: []onepoint.Activity{
				{ID: 22, Name: "Activity B", ProjectNodeID: 11, Locked: false},
			},
			Skills: []onepoint.Skill{
				{SkillID: 33, Name: "Skill C", ActivityID: 22},
			},
		},
		worklogs: []onepoint.DayWorklog{
			{
				WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)),
				StartTime:   9 * 60,
				FinishTime:  10 * 60,
				Billable:    60,
				Comment:     "remote-a",
				ProjectID:   11,
				ActivityID:  22,
				SkillID:     33,
			},
			{
				WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local)),
				StartTime:   10 * 60,
				FinishTime:  11 * 60,
				Billable:    60,
				Comment:     "remote-b",
				ProjectID:   11,
				ActivityID:  22,
				SkillID:     33,
			},
			{
				WorklogDate: onepoint.FormatDay(time.Date(2026, 4, 1, 0, 0, 0, 0, time.Local)),
				StartTime:   9 * 60,
				FinishTime:  10 * 60,
				Billable:    60,
				Comment:     "outside-range",
				ProjectID:   11,
				ActivityID:  22,
				SkillID:     33,
			},
		},
	}

	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/month/2026-03/copy-from-remote", "application/json", nil)
	if err != nil {
		t.Fatalf("sync request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	var payload map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["copied"] != 2 || payload["total"] != 2 {
		t.Fatalf("unexpected copy payload: %+v", payload)
	}

	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 synced entries, got %d", len(entries))
	}
	for _, entry := range entries {
		if entry.Project != "Project A" || entry.Activity != "Activity B" || entry.Skill != "Skill C" {
			t.Fatalf("expected lookup names on synced row, got %+v", entry)
		}
		if entry.SourceMapper != "onepoint" || entry.SourceFormat != "remote" {
			t.Fatalf("expected onepoint source metadata, got %+v", entry)
		}
	}
}

func TestServer_CopyMonthRemote_SkipsEntriesAlreadyInLocal(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	insertWorklogs(t, store, []worklog.Entry{
		{
			StartDateTime: time.Date(2026, 3, 1, 9, 0, 0, 0, time.Local),
			EndDateTime:   time.Date(2026, 3, 1, 10, 0, 0, 0, time.Local),
			Billable:      60,
			Description:   "existing",
			Project:       "Project A",
			Activity:      "Activity B",
			Skill:         "Skill C",
			SourceFormat:  "manual",
			SourceMapper:  "manual",
			SourceFile:    "web-ui",
		},
	})

	client := &fakeClient{
		snapshot: onepoint.LookupSnapshot{
			Projects: []onepoint.Project{{ID: 11, Name: "Project A", Archived: "0"}},
			Activities: []onepoint.Activity{
				{ID: 22, Name: "Activity B", ProjectNodeID: 11, Locked: false},
			},
			Skills: []onepoint.Skill{
				{SkillID: 33, Name: "Skill C", ActivityID: 22},
			},
		},
		worklogs: []onepoint.DayWorklog{
			{
				WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)),
				StartTime:   9 * 60,
				FinishTime:  10 * 60,
				Billable:    60,
				Comment:     "remote-same",
				ProjectID:   11,
				ActivityID:  22,
				SkillID:     33,
			},
			{
				WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local)),
				StartTime:   10 * 60,
				FinishTime:  11 * 60,
				Billable:    60,
				Comment:     "remote-new",
				ProjectID:   11,
				ActivityID:  22,
				SkillID:     33,
			},
		},
	}

	ts := httptest.NewServer(NewServer(store, client, testConfig(nil)))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/month/2026-03/copy-from-remote", "application/json", nil)
	if err != nil {
		t.Fatalf("copy request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	var payload map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["copied"] != 1 || payload["total"] != 2 {
		t.Fatalf("unexpected copy payload: %+v", payload)
	}

	entries, err := store.ListWorklogs()
	if err != nil {
		t.Fatalf("list worklogs: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected one existing + one copied entry, got %d", len(entries))
	}
}

func TestLoadRemoteRange_SortsOnceAndUsesCache(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	client := &fakeClient{
		worklogs: []onepoint.DayWorklog{
			{WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local)), StartTime: 11 * 60, FinishTime: 12 * 60},
			{WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)), StartTime: 14 * 60, FinishTime: 15 * 60},
			{WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)), StartTime: 9 * 60, FinishTime: 10 * 60},
			{WorklogDate: onepoint.FormatDay(time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local)), StartTime: 8 * 60, FinishTime: 9 * 60},
		},
	}
	server, ok := NewServer(store, client, testConfig(nil)).(*Server)
	if !ok {
		t.Fatalf("expected *Server handler")
	}

	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local)

	first, firstRefreshedAt, err := server.loadRemoteRange(context.Background(), from, to, false)
	if err != nil {
		t.Fatalf("first loadRemoteRange: %v", err)
	}
	second, secondRefreshedAt, err := server.loadRemoteRange(context.Background(), from, to, false)
	if err != nil {
		t.Fatalf("second loadRemoteRange: %v", err)
	}

	if client.filteredCalls != 1 {
		t.Fatalf("expected one filtered fetch call, got %d", client.filteredCalls)
	}
	if firstRefreshedAt.IsZero() || secondRefreshedAt.IsZero() {
		t.Fatalf("expected refresh timestamps for cached days")
	}
	if !firstRefreshedAt.Equal(secondRefreshedAt) {
		t.Fatalf("expected cached fetch timestamp to be stable, got first=%s second=%s", firstRefreshedAt, secondRefreshedAt)
	}
	for i, values := range [][]onepoint.DayWorklog{first, second} {
		got := make([]string, 0, len(values))
		for _, item := range values {
			got = append(got, item.WorklogDate+"|"+strconv.Itoa(item.StartTime))
		}
		want := []string{
			onepoint.FormatDay(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)) + "|540",
			onepoint.FormatDay(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)) + "|840",
			onepoint.FormatDay(time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local)) + "|480",
			onepoint.FormatDay(time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local)) + "|660",
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("unexpected order on run %d: got=%v want=%v", i+1, got, want)
		}
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
	filteredCalls int
	persistCalls  int
	persistByDate map[string][]onepoint.PersistWorklog
	filteredErr   error
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
	f.filteredCalls++
	if f.filteredErr != nil {
		return nil, f.filteredErr
	}
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

type testAuditLogger struct {
	records []auditRecord
}

func (l *testAuditLogger) Log(record auditRecord) error {
	l.records = append(l.records, record)
	return nil
}

func strconvI64(value int64) string {
	return strconv.FormatInt(value, 10)
}
