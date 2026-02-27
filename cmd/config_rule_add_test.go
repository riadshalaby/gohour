package cmd

import (
	"strings"
	"testing"

	"gohour/config"
)

func TestAppendRuleToConfigYAML_AppendsRule(t *testing.T) {
	t.Parallel()

	input := []byte(`onepoint:
  url: "https://onepoint.virtual7.io"
import:
  auto_reconcile_after_import: true
rules:
  - name: "rz"
    mapper: "epm"
    file_template: "EPMExportRZ*.xlsx"
    project_id: 1
    project: "Project A"
    activity_id: 2
    activity: "Activity A"
    skill_id: 3
    skill: "Skill A"
`)

	newRule := config.Rule{
		Name:         "sz",
		Mapper:       "epm",
		FileTemplate: "EPMExportSZ*.xlsx",
		ProjectID:    10,
		Project:      "Project B",
		ActivityID:   20,
		Activity:     "Activity B",
		SkillID:      30,
		Skill:        "Skill B",
	}

	updated, err := appendRuleToConfigYAML(input, newRule)
	if err != nil {
		t.Fatalf("append rule failed: %v", err)
	}

	cfg, err := config.ValidateYAMLContent(updated)
	if err != nil {
		t.Fatalf("updated yaml should validate: %v", err)
	}

	if len(cfg.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(cfg.Rules))
	}
	last := cfg.Rules[1]
	if last.Name != "sz" || last.Mapper != "epm" || last.FileTemplate != "EPMExportSZ*.xlsx" || last.ProjectID != 10 || last.Project != "Project B" || last.ActivityID != 20 || last.Activity != "Activity B" || last.SkillID != 30 || last.Skill != "Skill B" {
		t.Fatalf("unexpected last rule: %+v", last)
	}
}

func TestAppendRuleToConfigYAML_DuplicateName(t *testing.T) {
	t.Parallel()

	input := []byte(`onepoint:
  url: "https://onepoint.virtual7.io"
import:
  auto_reconcile_after_import: true
rules:
  - name: "rz"
    mapper: "epm"
    file_template: "EPMExportRZ*.xlsx"
    project_id: 1
    project: "Project A"
    activity_id: 2
    activity: "Activity A"
    skill_id: 3
    skill: "Skill A"
`)

	_, err := appendRuleToConfigYAML(input, config.Rule{
		Name:         "RZ",
		Mapper:       "epm",
		FileTemplate: "Other*.xlsx",
		ProjectID:    10,
		Project:      "Project B",
		ActivityID:   20,
		Activity:     "Activity B",
		SkillID:      30,
		Skill:        "Skill B",
	})
	if err == nil {
		t.Fatalf("expected duplicate rule error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppendRuleToConfigYAML_AddsRulesBlockWhenMissing(t *testing.T) {
	t.Parallel()

	input := []byte(`onepoint:
  url: "https://onepoint.virtual7.io/onepoint/faces/home"
import:
  auto_reconcile_after_import: true
`)

	newRule := config.Rule{
		Name:         "rz",
		Mapper:       "epm",
		FileTemplate: "EPMExportRZ*.xlsx",
		ProjectID:    10,
		Project:      "Project A",
		ActivityID:   20,
		Activity:     "Activity A",
		SkillID:      30,
		Skill:        "Skill A",
	}

	updated, err := appendRuleToConfigYAML(input, newRule)
	if err != nil {
		t.Fatalf("append rule failed: %v", err)
	}

	cfg, err := config.ValidateYAMLContent(updated)
	if err != nil {
		t.Fatalf("updated yaml should validate: %v", err)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Name != "rz" || cfg.Rules[0].Mapper != "epm" {
		t.Fatalf("unexpected added rule: %+v", cfg.Rules[0])
	}
}
