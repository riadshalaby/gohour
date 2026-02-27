package onepoint

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHTTPClient_KnownEndpointsAndHeaders(t *testing.T) {
	t.Parallel()

	type seenRequest struct {
		method  string
		path    string
		cookie  string
		referer string
		xrw     string
	}

	seen := make([]seenRequest, 0, 5)
	appendSeen := func(r *http.Request) {
		seen = append(seen, seenRequest{
			method:  r.Method,
			path:    r.URL.Path,
			cookie:  r.Header.Get("Cookie"),
			referer: r.Header.Get("Referer"),
			xrw:     r.Header.Get("X-Requested-With"),
		})
	}

	doer := fakeDoer{fn: func(r *http.Request) (*http.Response, error) {
		appendSeen(r)
		if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			t.Fatalf("missing X-Requested-With header")
		}
		if r.Header.Get("Cookie") == "" {
			t.Fatalf("missing Cookie header")
		}
		if r.Header.Get("Referer") != "https://onepoint.virtual7.io/onepoint/faces/home" {
			t.Fatalf("unexpected Referer: %q", r.Header.Get("Referer"))
		}

		key := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		switch key {
		case "POST /OPServices/resources/OpProjects/getAllUserProjects":
			if got := r.URL.Query().Get("mode"); got != "all" {
				t.Fatalf("unexpected projects mode: %q", got)
			}
			return jsonResponse([]Project{{ID: 432904811, Name: "bfa211102 - ISO RVSE9 Los2", Archived: "0"}}), nil
		case "POST /OPServices/resources/OpProjects/getAllUserActivities":
			if got := r.URL.Query().Get("mode"); got != "all" {
				t.Fatalf("unexpected activities mode: %q", got)
			}
			return jsonResponse([]Activity{{ID: 436142369, Name: "RISH - Travel", ProjectNodeID: 432904811, Locked: false}}), nil
		case "POST /OPServices/resources/OpProjects/getAllUserSkills":
			if got := r.URL.Query().Get("mode"); got != "all" {
				t.Fatalf("unexpected skills mode: %q", got)
			}
			return jsonResponse([]Skill{{ActivityID: 436142369, Name: "Realisation (pm)", SkillID: 44498948}}), nil
		case "GET /OPServices/resources/OpWorklogs/22-02-2026:22-02-2026/getFilteredWorklogs":
			return jsonResponse(getFilteredWorklogsResponse{
				Worklogs: []DayWorklog{
					{
						TimeRecordID: 437654923,
						WorkSlipID:   436227248,
						WorkRecordID: 437043599,
						WorklogDate:  "22-02-2026",
						StartTime:    720,
						FinishTime:   780,
						Duration:     60,
						Billable:     60,
						Valuable:     0,
						ProjectID:    432904811,
						ActivityID:   436142369,
						SkillID:      44498948,
					},
				},
			}), nil
		case "POST /OPServices/resources/OpWorklogs/22-02-2026/persistWorklogs":
			var payload []PersistWorklog
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode persist payload: %v", err)
			}
			if len(payload) != 1 {
				t.Fatalf("expected 1 persist entry, got %d", len(payload))
			}
			if !payload[0].ProjectID.Valid || payload[0].ProjectID.Value != 432904811 {
				t.Fatalf("unexpected project id in payload: %+v", payload[0].ProjectID)
			}
			return jsonResponse([]PersistResult{
				{
					Message:         "Worklog successfully created",
					NewTimeRecordID: 437654923,
					OldTimeRecordID: 437654918,
					WorkRecordID:    436227248,
					WorkSlipID:      437043599,
					WorklogDate:     "2026-02-22T00:00:00+01:00",
				},
			}), nil
		default:
			return nil, fmt.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}}

	client, err := NewClient(ClientConfig{
		BaseURL:        "https://onepoint.virtual7.io",
		RefererURL:     "https://onepoint.virtual7.io/onepoint/faces/home",
		SessionCookies: "JSESSIONID=test; _WL_AUTHCOOKIE_JSESSIONID=test2",
		UserAgent:      "gohour-test",
		HTTPClient:     doer,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := context.Background()
	day := time.Date(2026, 2, 22, 10, 0, 0, 0, time.FixedZone("CET", 3600))

	if _, err := client.ListProjects(ctx); err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if _, err := client.ListActivities(ctx); err != nil {
		t.Fatalf("list activities: %v", err)
	}
	if _, err := client.ListSkills(ctx); err != nil {
		t.Fatalf("list skills: %v", err)
	}
	dayWorklogs, err := client.GetDayWorklogs(ctx, day)
	if err != nil {
		t.Fatalf("get day worklogs: %v", err)
	}
	if len(dayWorklogs) != 1 {
		t.Fatalf("expected one day worklog, got %d", len(dayWorklogs))
	}

	payload := []PersistWorklog{dayWorklogs[0].ToPersistWorklog()}
	results, err := client.PersistWorklogs(ctx, day, payload)
	if err != nil {
		t.Fatalf("persist worklogs: %v", err)
	}
	if len(results) != 1 || results[0].NewTimeRecordID != 437654923 {
		t.Fatalf("unexpected persist results: %+v", results)
	}

	if len(seen) != 5 {
		t.Fatalf("expected 5 requests, got %d", len(seen))
	}
	for _, request := range seen {
		if request.cookie == "" {
			t.Fatalf("expected cookie header for request %s %s", request.method, request.path)
		}
		if request.referer != "https://onepoint.virtual7.io/onepoint/faces/home" {
			t.Fatalf("unexpected referer for %s %s: %q", request.method, request.path, request.referer)
		}
		if request.xrw != "XMLHttpRequest" {
			t.Fatalf("missing X-Requested-With for %s %s", request.method, request.path)
		}
	}
}

type fakeDoer struct {
	fn func(*http.Request) (*http.Response, error)
}

func (f fakeDoer) Do(req *http.Request) (*http.Response, error) {
	return f.fn(req)
}

func jsonResponse(payload any) *http.Response {
	body, _ := json.Marshal(payload)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Header:     make(http.Header),
	}
}

func TestResolveIDsFromSnapshot_Success(t *testing.T) {
	t.Parallel()

	snapshot := LookupSnapshot{
		Projects: []Project{
			{ID: 432904811, Name: "bfa211102 - ISO RVSE9 Los2", Archived: "0"},
			{ID: 432904810, Name: "bfa211101 - ISO RVSE8 Los1", Archived: "0"},
		},
		Activities: []Activity{
			{ID: 436142369, Name: "RISH - Travel", ProjectNodeID: 432904811, Locked: false},
			{ID: 436142368, Name: "RISH - Travel", ProjectNodeID: 432904811, Locked: true},
			{ID: 436117539, Name: "RSH - Travel", ProjectNodeID: 432904810, Locked: true},
		},
		Skills: []Skill{
			{ActivityID: 436142369, Name: "Realisation (pm)", SkillID: 44498948},
			{ActivityID: 436142369, Name: "Realisation (pm)", SkillID: 44498948}, // duplicated from API
			{ActivityID: 436142369, Name: "Internal Discussion (pm)", SkillID: 44498946},
		},
	}

	resolved, err := ResolveIDsFromSnapshot(
		snapshot,
		"bfa211102 - ISO RVSE9 Los2",
		"RISH - Travel",
		"Realisation (pm)",
		ResolveOptions{},
	)
	if err != nil {
		t.Fatalf("resolve ids: %v", err)
	}

	if resolved.ProjectID != 432904811 || resolved.ActivityID != 436142369 || resolved.SkillID != 44498948 {
		t.Fatalf("unexpected resolved ids: %+v", resolved)
	}
}

func TestResolveIDsFromSnapshot_ArchivedProjectFilteredByDefault(t *testing.T) {
	t.Parallel()

	snapshot := LookupSnapshot{
		Projects: []Project{
			{ID: 432906972, Name: "bfa231101 - ISO RVSE8 Los1 Neu", Archived: "1"},
		},
	}

	_, err := ResolveIDsFromSnapshot(
		snapshot,
		"bfa231101 - ISO RVSE8 Los1 Neu",
		"RISH - Travel",
		"Realisation (pm)",
		ResolveOptions{},
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "archived") {
		t.Fatalf("expected archived hint in error, got %v", err)
	}
}

func TestResolveIDsFromSnapshot_AmbiguousSkill(t *testing.T) {
	t.Parallel()

	snapshot := LookupSnapshot{
		Projects: []Project{
			{ID: 432904811, Name: "bfa211102 - ISO RVSE9 Los2", Archived: "0"},
		},
		Activities: []Activity{
			{ID: 436142369, Name: "RISH - Travel", ProjectNodeID: 432904811, Locked: false},
		},
		Skills: []Skill{
			{ActivityID: 436142369, Name: "Realisation (pm)", SkillID: 44498948},
			{ActivityID: 436142369, Name: "Realisation (pm)", SkillID: 44498999},
		},
	}

	_, err := ResolveIDsFromSnapshot(
		snapshot,
		"bfa211102 - ISO RVSE9 Los2",
		"RISH - Travel",
		"Realisation (pm)",
		ResolveOptions{},
	)
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
}

func TestFlexibleInt64_MarshalAndUnmarshal(t *testing.T) {
	t.Parallel()

	var id FlexibleInt64
	if err := json.Unmarshal([]byte(`44498948`), &id); err != nil {
		t.Fatalf("unmarshal number: %v", err)
	}
	if !id.Valid || id.Value != 44498948 {
		t.Fatalf("unexpected decoded id: %+v", id)
	}

	if err := json.Unmarshal([]byte(`""`), &id); err != nil {
		t.Fatalf("unmarshal empty string: %v", err)
	}
	if id.Valid {
		t.Fatalf("expected invalid id after empty string, got %+v", id)
	}

	encoded, err := json.Marshal(id)
	if err != nil {
		t.Fatalf("marshal empty id: %v", err)
	}
	if string(encoded) != `""` {
		t.Fatalf("expected empty string marshal, got %s", string(encoded))
	}

	encoded, err = json.Marshal(ID(123))
	if err != nil {
		t.Fatalf("marshal valid id: %v", err)
	}
	if string(encoded) != "123" {
		t.Fatalf("expected numeric marshal, got %s", string(encoded))
	}
}
