package classify

import (
	"strings"
	"testing"

	"gohour/onepoint"
)

func TestClassifySubmitWorklogs_Duplicate(t *testing.T) {
	t.Parallel()

	existing := []onepoint.PersistWorklog{baseExistingWorklog()}
	local := []onepoint.PersistWorklog{
		{
			StartTime:  intPtr(540),
			FinishTime: intPtr(600),
			ProjectID:  onepoint.ID(10),
			ActivityID: onepoint.ID(20),
			SkillID:    onepoint.ID(30),
			Comment:    "duplicate with different comment",
			Billable:   60,
		},
	}

	toAdd, overlaps, duplicates := ClassifySubmitWorklogs(local, existing)
	if duplicates != 1 {
		t.Fatalf("expected 1 duplicate, got %d", duplicates)
	}
	if len(overlaps) != 0 {
		t.Fatalf("expected no overlaps, got %d", len(overlaps))
	}
	if len(toAdd) != 0 {
		t.Fatalf("expected no add candidates, got %d", len(toAdd))
	}
}

func TestClassifySubmitWorklogs_Overlap(t *testing.T) {
	t.Parallel()

	existing := []onepoint.PersistWorklog{baseExistingWorklog()}
	local := []onepoint.PersistWorklog{
		{
			StartTime:  intPtr(570),
			FinishTime: intPtr(630),
			ProjectID:  onepoint.ID(10),
			ActivityID: onepoint.ID(20),
			SkillID:    onepoint.ID(30),
			Comment:    "overlap",
		},
	}

	toAdd, overlaps, duplicates := ClassifySubmitWorklogs(local, existing)
	if duplicates != 0 {
		t.Fatalf("expected 0 duplicates, got %d", duplicates)
	}
	if len(overlaps) != 1 {
		t.Fatalf("expected 1 overlap, got %d", len(overlaps))
	}
	if len(toAdd) != 0 {
		t.Fatalf("expected no add candidates, got %d", len(toAdd))
	}
}

func TestClassifySubmitWorklogs_New(t *testing.T) {
	t.Parallel()

	existing := []onepoint.PersistWorklog{baseExistingWorklog()}
	local := []onepoint.PersistWorklog{
		{
			StartTime:  intPtr(630),
			FinishTime: intPtr(690),
			ProjectID:  onepoint.ID(10),
			ActivityID: onepoint.ID(20),
			SkillID:    onepoint.ID(30),
			Comment:    "new",
		},
	}

	toAdd, overlaps, duplicates := ClassifySubmitWorklogs(local, existing)
	if duplicates != 0 {
		t.Fatalf("expected 0 duplicates, got %d", duplicates)
	}
	if len(overlaps) != 0 {
		t.Fatalf("expected no overlaps, got %d", len(overlaps))
	}
	if len(toAdd) != 1 {
		t.Fatalf("expected 1 add candidate, got %d", len(toAdd))
	}
	if got := strings.TrimSpace(toAdd[0].Comment); got != "new" {
		t.Fatalf("expected new entry to be added, got %q", got)
	}
}

func baseExistingWorklog() onepoint.PersistWorklog {
	return onepoint.PersistWorklog{
		StartTime:  intPtr(540),
		FinishTime: intPtr(600),
		ProjectID:  onepoint.ID(10),
		ActivityID: onepoint.ID(20),
		SkillID:    onepoint.ID(30),
		Comment:    "existing",
		Billable:   0,
	}
}

func intPtr(value int) *int {
	out := value
	return &out
}
