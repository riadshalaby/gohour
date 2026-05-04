package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFixedPathsUseConfiguredDataDir(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("GOHOUR_DATA_DIR", dataDir)

	if got := DataDir(); got != dataDir {
		t.Fatalf("DataDir() = %q, want %q", got, dataDir)
	}
	if got := ConfigPath(); got != filepath.Join(dataDir, "config.yaml") {
		t.Fatalf("ConfigPath() = %q", got)
	}
	if got := DBPath(); got != filepath.Join(dataDir, "gohour.db") {
		t.Fatalf("DBPath() = %q", got)
	}
	if got := AuthStatePath(); got != filepath.Join(dataDir, "onepoint-auth-state.json") {
		t.Fatalf("AuthStatePath() = %q", got)
	}
}

func TestWriteConfigRoundTrip(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("GOHOUR_DATA_DIR", dataDir)
	billable := false

	cfg := &Config{
		OnePoint: OnePointConfig{URL: "https://onepoint.example.test/onepoint/faces/home"},
		Rules: []Rule{
			{
				Name:         "rz",
				Mapper:       "epm",
				FileTemplate: "EPMExportRZ*.xlsx",
				Billable:     &billable,
				ProjectID:    1,
				Project:      "Project A",
				ActivityID:   2,
				Activity:     "Activity A",
				SkillID:      3,
				Skill:        "Skill A",
			},
		},
	}

	if err := WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig() returned error: %v", err)
	}
	content, err := os.ReadFile(ConfigPath())
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	if strings.Contains(string(content), "auto_reconcile_after_import") {
		t.Fatalf("written config still contains removed reconcile setting:\n%s", string(content))
	}

	loaded, err := ValidateYAMLContent(content)
	if err != nil {
		t.Fatalf("written config did not validate: %v", err)
	}
	if loaded.OnePoint.URL != cfg.OnePoint.URL || len(loaded.Rules) != 1 || loaded.Rules[0].Name != "rz" {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
	if loaded.Rules[0].Billable == nil || *loaded.Rules[0].Billable {
		t.Fatalf("expected billable=false round trip, got %+v", loaded.Rules[0].Billable)
	}
}

func TestValidateYAMLContent_RejectsUnsupportedMapper(t *testing.T) {
	t.Parallel()

	content := []byte(`onepoint:
  url: "https://onepoint.virtual7.io/onepoint/faces/home"
rules:
  - name: "rz"
    mapper: "expm"
    file_template: "EPMExportRZ*.xlsx"
    project_id: 1
    project: "Project A"
    activity_id: 2
    activity: "Activity A"
    skill_id: 3
    skill: "Skill A"
`)

	_, err := ValidateYAMLContent(content)
	if err == nil {
		t.Fatalf("expected validation error for unsupported mapper")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateYAMLContent_AcceptsSupportedMapperCaseInsensitive(t *testing.T) {
	t.Parallel()

	content := []byte(`onepoint:
  url: "https://onepoint.virtual7.io/onepoint/faces/home"
rules:
  - name: "rz"
    mapper: "EPM"
    file_template: "EPMExportRZ*.xlsx"
    project_id: 1
    project: "Project A"
    activity_id: 2
    activity: "Activity A"
    skill_id: 3
    skill: "Skill A"
`)

	if _, err := ValidateYAMLContent(content); err != nil {
		t.Fatalf("expected config to validate: %v", err)
	}
}
