package cmd

import (
	"strings"
	"testing"

	"gohour/config"
)

func TestAppendEPMRuleToConfigYAML_AppendsRule(t *testing.T) {
	t.Parallel()

	input := []byte(`user: "john.doe"
url: "https://onepoint.virtual7.io"
port: 443
auto_reconcile_after_import: true
epm:
  rules:
    - name: "rz"
      file_template: "EPMExportRZ*.xlsx"
      project: "Project A"
      activity: "Activity A"
      skill: "Skill A"
`)

	newRule := config.EPMRule{
		Name:         "sz",
		FileTemplate: "EPMExportSZ*.xlsx",
		Project:      "Project B",
		Activity:     "Activity B",
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
	if last.Name != "sz" || last.FileTemplate != "EPMExportSZ*.xlsx" || last.Project != "Project B" || last.Activity != "Activity B" || last.Skill != "Skill B" {
		t.Fatalf("unexpected last rule: %+v", last)
	}
}

func TestAppendEPMRuleToConfigYAML_DuplicateName(t *testing.T) {
	t.Parallel()

	input := []byte(`user: "john.doe"
url: "https://onepoint.virtual7.io"
port: 443
auto_reconcile_after_import: true
epm:
  rules:
    - name: "rz"
      file_template: "EPMExportRZ*.xlsx"
      project: "Project A"
      activity: "Activity A"
      skill: "Skill A"
`)

	_, err := appendEPMRuleToConfigYAML(input, config.EPMRule{
		Name:         "RZ",
		FileTemplate: "Other*.xlsx",
		Project:      "Project B",
		Activity:     "Activity B",
		Skill:        "Skill B",
	})
	if err == nil {
		t.Fatalf("expected duplicate rule error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}
