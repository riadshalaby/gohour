package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/riadshalaby/gohour/config"
)

func TestDefaultServeMonthBoundsUsesCurrentMonth(t *testing.T) {
	t.Parallel()

	bounds := defaultServeMonthBounds()
	want := time.Now().In(time.Local).Format("2006-01")
	if bounds.defaultMonth != want {
		t.Fatalf("expected default month %q, got %q", want, bounds.defaultMonth)
	}
}

func TestWithServeMonthRedirect(t *testing.T) {
	t.Parallel()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	handler := withServeMonthRedirect(next, serveMonthBounds{defaultMonth: "2026-03"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("expected redirect status, got %d", res.Code)
	}
	if got := res.Header().Get("Location"); got != "/month/2026-03" {
		t.Fatalf("unexpected redirect target: %q", got)
	}
	if nextCalled {
		t.Fatalf("expected wrapper to intercept root redirect")
	}
}

func TestServeFlagsOnlyExposePortAndNoOpen(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"port", "no-open"} {
		if serveCmd.Flags().Lookup(name) == nil {
			t.Fatalf("expected --%s flag", name)
		}
	}
	for _, name := range []string{"db", "url", "state-file", "from", "to"} {
		if serveCmd.Flags().Lookup(name) != nil {
			t.Fatalf("did not expect removed --%s flag", name)
		}
	}
}

func TestBuildServeClient_UsesE2EStubWhenEnabled(t *testing.T) {
	t.Setenv(e2eStubRemoteEnv, "1")
	client, err := buildServeClient(config.Config{
		Rules: []config.Rule{
			{
				Name:         "generic-local",
				Mapper:       "generic",
				FileTemplate: "*.csv",
				ProjectID:    100,
				Project:      "P",
				ActivityID:   200,
				Activity:     "A",
				SkillID:      300,
				Skill:        "S",
			},
		},
	})
	if err != nil {
		t.Fatalf("buildServeClient returned error: %v", err)
	}

	snapshot, err := client.FetchLookupSnapshot(t.Context())
	if err != nil {
		t.Fatalf("FetchLookupSnapshot returned error: %v", err)
	}
	if len(snapshot.Projects) != 1 || snapshot.Projects[0].Name != "P" {
		t.Fatalf("unexpected projects: %+v", snapshot.Projects)
	}
	if len(snapshot.Activities) != 1 || snapshot.Activities[0].Name != "A" {
		t.Fatalf("unexpected activities: %+v", snapshot.Activities)
	}
	if len(snapshot.Skills) != 1 || snapshot.Skills[0].Name != "S" {
		t.Fatalf("unexpected skills: %+v", snapshot.Skills)
	}
}

func TestBuildServeClient_StubRefreshFailsButDayLookupWorks(t *testing.T) {
	t.Setenv(e2eStubRemoteEnv, "1")
	client, err := buildServeClient(config.Config{})
	if err != nil {
		t.Fatalf("buildServeClient returned error: %v", err)
	}

	if _, err := client.GetFilteredWorklogs(context.Background(), time.Now(), time.Now()); err == nil {
		t.Fatalf("expected GetFilteredWorklogs to fail in e2e stub mode")
	}
	worklogs, err := client.GetDayWorklogs(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("GetDayWorklogs returned error: %v", err)
	}
	if len(worklogs) != 0 {
		t.Fatalf("expected empty day worklogs, got %+v", worklogs)
	}
}
