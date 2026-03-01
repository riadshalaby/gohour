package submitter

import (
	"testing"
	"time"

	"gohour/config"
	"gohour/onepoint"
	"gohour/worklog"
)

func TestClassifyWorklogs_Duplicate(t *testing.T) {
	t.Parallel()

	local := []onepoint.PersistWorklog{
		{
			StartTime:  submitterIntPtr(9 * 60),
			FinishTime: submitterIntPtr(10 * 60),
			ProjectID:  onepoint.ID(1),
			ActivityID: onepoint.ID(2),
			SkillID:    onepoint.ID(3),
		},
	}
	existing := []onepoint.PersistWorklog{
		{
			StartTime:  submitterIntPtr(9 * 60),
			FinishTime: submitterIntPtr(10 * 60),
			ProjectID:  onepoint.ID(1),
			ActivityID: onepoint.ID(2),
			SkillID:    onepoint.ID(3),
		},
	}

	toAdd, overlaps, duplicates := ClassifyWorklogs(local, existing)
	if len(toAdd) != 0 {
		t.Fatalf("expected 0 toAdd, got %d", len(toAdd))
	}
	if len(overlaps) != 0 {
		t.Fatalf("expected 0 overlaps, got %d", len(overlaps))
	}
	if duplicates != 1 {
		t.Fatalf("expected 1 duplicate, got %d", duplicates)
	}
}

func TestClassifyWorklogs_Overlap(t *testing.T) {
	t.Parallel()

	local := []onepoint.PersistWorklog{
		{
			StartTime:  submitterIntPtr(9 * 60),
			FinishTime: submitterIntPtr(10 * 60),
			ProjectID:  onepoint.ID(1),
			ActivityID: onepoint.ID(2),
			SkillID:    onepoint.ID(3),
		},
	}
	existing := []onepoint.PersistWorklog{
		{
			StartTime:  submitterIntPtr(9*60 + 30),
			FinishTime: submitterIntPtr(10*60 + 30),
			ProjectID:  onepoint.ID(10),
			ActivityID: onepoint.ID(20),
			SkillID:    onepoint.ID(30),
		},
	}

	toAdd, overlaps, duplicates := ClassifyWorklogs(local, existing)
	if len(toAdd) != 0 {
		t.Fatalf("expected 0 toAdd, got %d", len(toAdd))
	}
	if len(overlaps) != 1 {
		t.Fatalf("expected 1 overlap, got %d", len(overlaps))
	}
	if duplicates != 0 {
		t.Fatalf("expected 0 duplicates, got %d", duplicates)
	}
}

func TestClassifyWorklogs_New(t *testing.T) {
	t.Parallel()

	local := []onepoint.PersistWorklog{
		{
			StartTime:  submitterIntPtr(9 * 60),
			FinishTime: submitterIntPtr(10 * 60),
			ProjectID:  onepoint.ID(1),
			ActivityID: onepoint.ID(2),
			SkillID:    onepoint.ID(3),
		},
	}

	toAdd, overlaps, duplicates := ClassifyWorklogs(local, nil)
	if len(toAdd) != 1 {
		t.Fatalf("expected 1 toAdd, got %d", len(toAdd))
	}
	if len(overlaps) != 0 {
		t.Fatalf("expected 0 overlaps, got %d", len(overlaps))
	}
	if duplicates != 0 {
		t.Fatalf("expected 0 duplicates, got %d", duplicates)
	}
}

func TestBuildDayBatches_CrossDay(t *testing.T) {
	t.Parallel()

	entries := []worklog.Entry{
		{
			ID:            7,
			StartDateTime: time.Date(2026, 3, 1, 23, 0, 0, 0, time.Local),
			EndDateTime:   time.Date(2026, 3, 2, 0, 30, 0, 0, time.Local),
			Billable:      90,
			Project:       "P",
			Activity:      "A",
			Skill:         "S",
			SourceMapper:  "epm",
		},
	}
	ids := map[NameTuple]ResolvedIDs{
		{Mapper: "epm", Project: "p", Activity: "a", Skill: "s"}: {
			ProjectID:  1,
			ActivityID: 2,
			SkillID:    3,
		},
	}

	_, err := BuildDayBatches(entries, ids)
	if err == nil {
		t.Fatalf("expected cross-day error")
	}
}

func TestBuildRuleIDMap_SkipsIncomplete(t *testing.T) {
	t.Parallel()

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
		{
			Mapper:     "epm",
			Project:    "Project B",
			Activity:   "Delivery",
			Skill:      "Go",
			ProjectID:  0,
			ActivityID: 12,
			SkillID:    13,
		},
		{
			Mapper:     "epm",
			Project:    "",
			Activity:   "Delivery",
			Skill:      "Go",
			ProjectID:  21,
			ActivityID: 22,
			SkillID:    23,
		},
	}

	got := BuildRuleIDMap(rules)
	if len(got) != 1 {
		t.Fatalf("expected 1 complete rule in map, got %d", len(got))
	}
	key := NameTuple{Mapper: "epm", Project: "project a", Activity: "delivery", Skill: "go"}
	if _, ok := got[key]; !ok {
		t.Fatalf("expected complete rule key to exist")
	}
}

func submitterIntPtr(value int) *int {
	out := value
	return &out
}
