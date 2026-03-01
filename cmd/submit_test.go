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
			SourceMapper:  "epm",
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
			SourceMapper:  "epm",
		},
	}
	ids := map[submitNameTuple]submitResolvedIDs{
		{
			Mapper:   "epm",
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

func TestBuildSubmitDayBatches_PreservesZeroBillable(t *testing.T) {
	t.Parallel()

	entries := []worklog.Entry{
		{
			ID:            1,
			StartDateTime: time.Date(2026, 3, 5, 9, 0, 0, 0, time.Local),
			EndDateTime:   time.Date(2026, 3, 5, 10, 0, 0, 0, time.Local),
			Billable:      0,
			Description:   "Internal support",
			Project:       "Project A",
			Activity:      "Delivery",
			Skill:         "Go",
			SourceMapper:  "epm",
		},
	}
	ids := map[submitNameTuple]submitResolvedIDs{
		{
			Mapper:   "epm",
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
	if len(batches) != 1 || len(batches[0].Worklogs) != 1 {
		t.Fatalf("expected one worklog, got %+v", batches)
	}
	if got := batches[0].Worklogs[0].Billable; got != 0 {
		t.Fatalf("expected billable to stay 0, got %d", got)
	}
}

func TestBuildSubmitDayBatches_RejectsNegativeBillable(t *testing.T) {
	t.Parallel()

	entries := []worklog.Entry{
		{
			ID:            77,
			StartDateTime: time.Date(2026, 3, 5, 9, 0, 0, 0, time.Local),
			EndDateTime:   time.Date(2026, 3, 5, 10, 0, 0, 0, time.Local),
			Billable:      -1,
			Description:   "Invalid billable",
			Project:       "Project A",
			Activity:      "Delivery",
			Skill:         "Go",
			SourceMapper:  "epm",
		},
	}
	ids := map[submitNameTuple]submitResolvedIDs{
		{
			Mapper:   "epm",
			Project:  "project a",
			Activity: "delivery",
			Skill:    "go",
		}: {
			ProjectID:  100,
			ActivityID: 200,
			SkillID:    300,
		},
	}

	_, err := buildSubmitDayBatches(entries, ids)
	if err == nil {
		t.Fatalf("expected error for negative billable")
	}
	if !strings.Contains(err.Error(), "negative billable value") {
		t.Fatalf("unexpected error: %v", err)
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
			Project:      "Project A",
			Activity:     "Delivery",
			Skill:        "Go",
			SourceMapper: "epm",
		},
		{
			Project:      "Project B",
			Activity:     "Development",
			Skill:        "Go",
			SourceMapper: "epm",
		},
	}
	rules := []config.Rule{
		{
			Mapper:     "epm",
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

	ruleTuple := submitNameTuple{Mapper: "epm", Project: "project a", Activity: "delivery", Skill: "go"}
	if got := resolved[ruleTuple]; got.ProjectID != 11 || got.ActivityID != 12 || got.SkillID != 13 {
		t.Fatalf("unexpected rule-resolved ids: %+v", got)
	}

	fallbackTuple := submitNameTuple{Mapper: "epm", Project: "project b", Activity: "development", Skill: "go"}
	if got := resolved[fallbackTuple]; got.ProjectID != 22 || got.ActivityID != 33 || got.SkillID != 44 {
		t.Fatalf("unexpected fallback-resolved ids: %+v", got)
	}
}

func TestResolveIDsForEntries_ErrorsEarlyOnEmptyNames(t *testing.T) {
	t.Parallel()

	entries := []worklog.Entry{
		{
			ID:           42,
			Project:      "Project A",
			Activity:     "",
			Skill:        "Go",
			SourceMapper: "epm",
		},
	}

	_, err := resolveIDsForEntries(context.Background(), nil, nil, entries, onepoint.ResolveOptions{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "worklog id=42") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormatDryRunWorklog(t *testing.T) {
	t.Parallel()

	start := 540
	finish := 615
	value := onepoint.PersistWorklog{
		StartTime:  &start,
		FinishTime: &finish,
		Duration:   75,
		Billable:   75,
		ProjectID:  onepoint.ID(11),
		ActivityID: onepoint.ID(22),
		SkillID:    onepoint.ID(33),
		Comment:    "  Implement feature X  ",
	}

	got := formatDryRunWorklog(value)
	if !strings.Contains(got, "time=09:00-10:15") {
		t.Fatalf("unexpected time output: %s", got)
	}
	if !strings.Contains(got, "duration=75") || !strings.Contains(got, "billable=75") {
		t.Fatalf("missing duration/billable output: %s", got)
	}
	if !strings.Contains(got, "projectId=11") || !strings.Contains(got, "activityId=22") || !strings.Contains(got, "skillId=33") {
		t.Fatalf("missing id output: %s", got)
	}
	if !strings.Contains(got, `comment="Implement feature X"`) {
		t.Fatalf("unexpected comment output: %s", got)
	}
}

func TestFormatFlexibleIDForDryRun_Empty(t *testing.T) {
	t.Parallel()

	if got := formatFlexibleIDForDryRun(onepoint.FlexibleInt64{}); got != "<empty>" {
		t.Fatalf("expected <empty>, got %q", got)
	}
}

func TestClassifySubmitWorklogs_DuplicateOverlapAndAdd(t *testing.T) {
	t.Parallel()

	existing := []onepoint.PersistWorklog{
		{
			StartTime:  submitIntPtr(540),
			FinishTime: submitIntPtr(600),
			ProjectID:  onepoint.ID(10),
			ActivityID: onepoint.ID(20),
			SkillID:    onepoint.ID(30),
			Comment:    "existing",
			Billable:   0,
		},
	}
	local := []onepoint.PersistWorklog{
		{
			StartTime:  submitIntPtr(540),
			FinishTime: submitIntPtr(600),
			ProjectID:  onepoint.ID(10),
			ActivityID: onepoint.ID(20),
			SkillID:    onepoint.ID(30),
			Comment:    "duplicate with different comment",
			Billable:   60,
		},
		{
			StartTime:  submitIntPtr(570),
			FinishTime: submitIntPtr(630),
			ProjectID:  onepoint.ID(10),
			ActivityID: onepoint.ID(20),
			SkillID:    onepoint.ID(30),
			Comment:    "overlap",
		},
		{
			StartTime:  submitIntPtr(630),
			FinishTime: submitIntPtr(690),
			ProjectID:  onepoint.ID(10),
			ActivityID: onepoint.ID(20),
			SkillID:    onepoint.ID(30),
			Comment:    "new",
		},
	}

	toAdd, overlaps, duplicates := classifySubmitWorklogs(local, existing)
	if duplicates != 1 {
		t.Fatalf("expected 1 duplicate, got %d", duplicates)
	}
	if len(overlaps) != 1 {
		t.Fatalf("expected 1 overlap, got %d", len(overlaps))
	}
	if len(toAdd) != 1 {
		t.Fatalf("expected 1 non-overlapping add candidate, got %d", len(toAdd))
	}
	if got := strings.TrimSpace(toAdd[0].Comment); got != "new" {
		t.Fatalf("expected new entry to be added, got %q", got)
	}
}

func TestHandleOverlaps_DryRunSkipsWithoutPrompt(t *testing.T) {
	t.Parallel()

	overlaps := []onepoint.OverlapInfo{
		{
			Local: onepoint.PersistWorklog{
				StartTime:  submitIntPtr(540),
				FinishTime: submitIntPtr(600),
				ProjectID:  onepoint.ID(10),
			},
			Existing: onepoint.PersistWorklog{
				StartTime:  submitIntPtr(570),
				FinishTime: submitIntPtr(630),
			},
		},
	}

	out, err := handleOverlaps(overlaps, true, new(bool), new(bool))
	if err != nil {
		t.Fatalf("handle overlaps: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected dry-run to skip overlaps, got %d", len(out))
	}
}

func TestHandleOverlaps_GlobalWriteAll(t *testing.T) {
	t.Parallel()

	writeAll := true
	skipAll := false
	overlaps := []onepoint.OverlapInfo{
		{
			Local: onepoint.PersistWorklog{
				StartTime:  submitIntPtr(540),
				FinishTime: submitIntPtr(600),
			},
		},
	}

	out, err := handleOverlaps(overlaps, false, &skipAll, &writeAll)
	if err != nil {
		t.Fatalf("handle overlaps: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected overlapping entry to be returned, got %d", len(out))
	}
}

func TestHandleOverlaps_GlobalSkipAll(t *testing.T) {
	t.Parallel()

	writeAll := false
	skipAll := true
	overlaps := []onepoint.OverlapInfo{
		{
			Local: onepoint.PersistWorklog{
				StartTime:  submitIntPtr(540),
				FinishTime: submitIntPtr(600),
			},
		},
	}

	out, err := handleOverlaps(overlaps, false, &skipAll, &writeAll)
	if err != nil {
		t.Fatalf("handle overlaps: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected overlaps to be skipped, got %d", len(out))
	}
}

func submitIntPtr(value int) *int {
	out := value
	return &out
}
