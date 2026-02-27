package cmd

import (
	"strings"
	"testing"

	"gohour/config"
)

func TestAppendEPMRuleToConfigYAML_AppendsRule(t *testing.T) {
	t.Parallel()

	input := []byte(`onepoint:
  url: "https://onepoint.virtual7.io"
import:
  auto_reconcile_after_import: true
epm:
  rules:
    - name: "rz"
      file_template: "EPMExportRZ*.xlsx"
      project_id: 1
      project: "Project A"
      activity_id: 2
      activity: "Activity A"
      skill_id: 3
      skill: "Skill A"
`)

	newRule := config.EPMRule{
		Name:         "sz",
		FileTemplate: "EPMExportSZ*.xlsx",
		ProjectID:    10,
		Project:      "Project B",
		ActivityID:   20,
		Activity:     "Activity B",
		SkillID:      30,
		Skill:        "Skill B",
	}

	updated, err := appendEPMRuleToConfigYAML(input, newRule)
	if err != nil {
		t.Fatalf("append rule failed: %v", err)
	}

	cfg, err := config.ValidateYAMLContent(updated)
	if err != nil {
		t.Fatalf("updated yaml should validate: %v", err)
	}

	if len(cfg.EPM.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(cfg.EPM.Rules))
	}
	last := cfg.EPM.Rules[1]
	if last.Name != "sz" || last.FileTemplate != "EPMExportSZ*.xlsx" || last.ProjectID != 10 || last.Project != "Project B" || last.ActivityID != 20 || last.Activity != "Activity B" || last.SkillID != 30 || last.Skill != "Skill B" {
		t.Fatalf("unexpected last rule: %+v", last)
	}
}

func TestAppendEPMRuleToConfigYAML_DuplicateName(t *testing.T) {
	t.Parallel()

	input := []byte(`onepoint:
  url: "https://onepoint.virtual7.io"
import:
  auto_reconcile_after_import: true
epm:
  rules:
    - name: "rz"
      file_template: "EPMExportRZ*.xlsx"
      project_id: 1
      project: "Project A"
      activity_id: 2
      activity: "Activity A"
      skill_id: 3
      skill: "Skill A"
`)

	_, err := appendEPMRuleToConfigYAML(input, config.EPMRule{
		Name:         "RZ",
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
