package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gohour/config"
	"gohour/onepoint"
	"gohour/worklog"
)

type submitFakeDoer struct {
	fn func(*http.Request) (*http.Response, error)
}

func (f submitFakeDoer) Do(req *http.Request) (*http.Response, error) {
	return f.fn(req)
}

func submitJSONResponse(payload any) *http.Response {
	body, _ := json.Marshal(payload)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Header:     make(http.Header),
	}
}

func TestBuildSubmitDayBatches_GroupsAndBuildsPayload(t *testing.T) {
	t.Parallel()

	entries := []worklog.Entry{
		{
			ID:            1,
			StartDateTime: time.Date(2026, 3, 5, 9, 0, 0, 0, time.Local),
			EndDateTime:   time.Date(2026, 3, 5, 10, 0, 0, 0, time.Local),
			Billable:      60,
			Description:   "API bugfix",
			Project:       "Project A",
			Activity:      "Delivery",
			Skill:         "Go",
		},
		{
			ID:            2,
			StartDateTime: time.Date(2026, 3, 5, 10, 15, 0, 0, time.Local),
			EndDateTime:   time.Date(2026, 3, 5, 11, 0, 0, 0, time.Local),
			Billable:      45,
			Description:   "Unit tests",
			Project:       "Project A",
			Activity:      "Delivery",
			Skill:         "Go",
		},
	}
	ids := map[submitNameTuple]submitResolvedIDs{
		{
			Project:  "project a",
			Activity: "delivery",
			Skill:    "go",
		}: {
			ProjectID:  100,
			ActivityID: 200,
			SkillID:    300,
		},
	}

	batches, err := buildSubmitDayBatches(entries, ids)
	if err != nil {
		t.Fatalf("build day batches: %v", err)
	}
	if len(batches) != 1 {
		t.Fatalf("expected 1 day batch, got %d", len(batches))
	}
	if len(batches[0].Worklogs) != 2 {
		t.Fatalf("expected 2 worklogs, got %d", len(batches[0].Worklogs))
	}

	first := batches[0].Worklogs[0]
	if first.TimeRecordID >= 0 {
		t.Fatalf("expected negative temporary timerecordId, got %d", first.TimeRecordID)
	}
	if !first.ProjectID.Valid || first.ProjectID.Value != 100 {
		t.Fatalf("unexpected project id: %+v", first.ProjectID)
	}
	if first.WorklogDate != "05-03-2026" {
		t.Fatalf("unexpected worklog date: %s", first.WorklogDate)
	}
}

func TestResolveIDsForEntries_UsesRulesThenSnapshotFallback(t *testing.T) {
	t.Parallel()

	doer := submitFakeDoer{fn: func(r *http.Request) (*http.Response, error) {
		switch fmt.Sprintf("%s %s", r.Method, r.URL.Path) {
		case "POST /OPServices/resources/OpProjects/getAllUserProjects":
			return submitJSONResponse([]onepoint.Project{{ID: 22, Name: "Project B", Archived: "0"}}), nil
		case "POST /OPServices/resources/OpProjects/getAllUserActivities":
			return submitJSONResponse([]onepoint.Activity{{ID: 33, Name: "Development", ProjectNodeID: 22}}), nil
		case "POST /OPServices/resources/OpProjects/getAllUserSkills":
			return submitJSONResponse([]onepoint.Skill{{SkillID: 44, Name: "Go", ActivityID: 33}}), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}}

	client, err := onepoint.NewClient(onepoint.ClientConfig{
		BaseURL:        "https://onepoint.virtual7.io",
		RefererURL:     "https://onepoint.virtual7.io/onepoint/faces/home",
		SessionCookies: "JSESSIONID=test",
		HTTPClient:     doer,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	entries := []worklog.Entry{
		{
			Project:  "Project A",
			Activity: "Delivery",
			Skill:    "Go",
		},
		{
			Project:  "Project B",
			Activity: "Development",
			Skill:    "Go",
		},
	}
	rules := []config.EPMRule{
		{
			Project:    "Project A",
			Activity:   "Delivery",
			Skill:      "Go",
			ProjectID:  11,
			ActivityID: 12,
			SkillID:    13,
		},
	}

	resolved, err := resolveIDsForEntries(context.Background(), client, rules, entries, onepoint.ResolveOptions{})
	if err != nil {
		t.Fatalf("resolve ids: %v", err)
	}

	ruleTuple := submitNameTuple{Project: "project a", Activity: "delivery", Skill: "go"}
	if got := resolved[ruleTuple]; got.ProjectID != 11 || got.ActivityID != 12 || got.SkillID != 13 {
		t.Fatalf("unexpected rule-resolved ids: %+v", got)
	}

	fallbackTuple := submitNameTuple{Project: "project b", Activity: "development", Skill: "go"}
	if got := resolved[fallbackTuple]; got.ProjectID != 22 || got.ActivityID != 33 || got.SkillID != 44 {
		t.Fatalf("unexpected fallback-resolved ids: %+v", got)
	}
}
