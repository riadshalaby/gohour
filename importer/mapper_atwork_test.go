package importer

import (
	"gohour/config"
	"testing"
)

func atworkConfig() config.Config {
	return config.Config{
		ImportProject:  "test-project",
		ImportActivity: "test-activity",
		ImportSkill:    "test-skill",
		ImportBillable: true,
	}
}

func newATWorkRecord(row int, beginn, ende, dauer, kunde, projekt, aufgabe, notiz string) Record {
	return Record{
		RowNumber: row,
		Values: map[string]string{
			normalizeHeader("Beginn"):  beginn,
			normalizeHeader("Ende"):    ende,
			normalizeHeader("Dauer"):   dauer,
			normalizeHeader("Kunde"):   kunde,
			normalizeHeader("Projekt"): projekt,
			normalizeHeader("Aufgabe"): aufgabe,
			normalizeHeader("Notiz"):   notiz,
		},
	}
}

func TestATWorkMapper_HappyPath(t *testing.T) {
	t.Parallel()
	mapper := &ATWorkMapper{}
	cfg := atworkConfig()

	record := newATWorkRecord(3, "03.03.2026 08:30", "03.03.2026 10:00", "1,5", "Virtual7", "Intern", "Travel", "Fake commute")
	entry, ok, err := mapper.Map(record, cfg, "csv", "atwork.csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || entry == nil {
		t.Fatal("expected entry to be mapped")
	}

	if entry.Billable != 90 {
		t.Errorf("Billable = %d, want 90", entry.Billable)
	}
	if entry.Project != "test-project" {
		t.Errorf("Project = %q, want %q", entry.Project, "test-project")
	}
	if entry.Activity != "test-activity" {
		t.Errorf("Activity = %q, want %q", entry.Activity, "test-activity")
	}
	if entry.Skill != "test-skill" {
		t.Errorf("Skill = %q, want %q", entry.Skill, "test-skill")
	}
	if entry.Description != "[Intern/Travel] Fake commute" {
		t.Errorf("Description = %q, want %q", entry.Description, "[Intern/Travel] Fake commute")
	}
}

func TestATWorkMapper_SkipZeroDuration(t *testing.T) {
	t.Parallel()
	mapper := &ATWorkMapper{}
	cfg := atworkConfig()

	// Start < End but Dauer = 0 → entry should be skipped.
	record := newATWorkRecord(3, "03.03.2026 08:30", "03.03.2026 09:00", "0", "Virtual7", "Intern", "Travel", "")
	_, ok, err := mapper.Map(record, cfg, "csv", "atwork.csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected zero-duration row to be skipped")
	}
}

func TestATWorkMapper_DescriptionFallbackToAufgabe(t *testing.T) {
	t.Parallel()
	mapper := &ATWorkMapper{}
	cfg := atworkConfig()

	record := newATWorkRecord(3, "12.03.2026 13:00", "12.03.2026 14:00", "1", "Virtual7", "Intern", "Travel", "")
	entry, ok, err := mapper.Map(record, cfg, "csv", "atwork.csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || entry == nil {
		t.Fatal("expected entry to be mapped")
	}

	if entry.Description != "Travel" {
		t.Errorf("Description = %q, want %q (fallback to Aufgabe)", entry.Description, "Travel")
	}
}

func TestATWorkMapper_DescriptionWithNotizAndContext(t *testing.T) {
	t.Parallel()
	mapper := &ATWorkMapper{}
	cfg := atworkConfig()

	record := newATWorkRecord(3, "18.03.2026 15:00", "18.03.2026 17:00", "2", "Virtual7", "Role Activity", "Allgemein", "Code review")
	entry, ok, err := mapper.Map(record, cfg, "csv", "atwork.csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || entry == nil {
		t.Fatal("expected entry to be mapped")
	}

	expected := "[Role Activity/Allgemein] Code review"
	if entry.Description != expected {
		t.Errorf("Description = %q, want %q", entry.Description, expected)
	}
}

func TestATWorkMapper_IntegerDuration(t *testing.T) {
	t.Parallel()
	mapper := &ATWorkMapper{}
	cfg := atworkConfig()

	record := newATWorkRecord(3, "12.03.2026 13:00", "12.03.2026 14:00", "1", "Virtual7", "Intern", "Travel", "")
	entry, ok, err := mapper.Map(record, cfg, "csv", "atwork.csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || entry == nil {
		t.Fatal("expected entry to be mapped")
	}

	if entry.Billable != 60 {
		t.Errorf("Billable = %d, want 60 (1 hour = 60 minutes)", entry.Billable)
	}
}

func TestATWorkMapper_Name(t *testing.T) {
	t.Parallel()
	mapper := &ATWorkMapper{}
	if mapper.Name() != "atwork" {
		t.Errorf("Name() = %q, want %q", mapper.Name(), "atwork")
	}
}
