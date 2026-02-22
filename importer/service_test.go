package importer

import (
	"gohour/config"
	"testing"
)

func TestResolveConfigForFile_EPMRuleMatch(t *testing.T) {
	cfg := config.Config{
		User: "u", URL: "http://localhost", Port: 8080,
		EPM: config.EPMConfig{Rules: []config.EPMRule{
			{FileTemplate: "EPMExportRZ*.xlsx", Project: "RZ Project", Activity: "Delivery", Skill: "Go"},
		}},
	}

	resolved, err := resolveConfigForFile("/tmp/EPMExportRZ202601.xlsx", "epm", cfg, RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.ImportProject != "RZ Project" || resolved.ImportActivity != "Delivery" || resolved.ImportSkill != "Go" {
		t.Fatalf("unexpected resolved values: project=%q activity=%q skill=%q", resolved.ImportProject, resolved.ImportActivity, resolved.ImportSkill)
	}
}

func TestResolveConfigForFile_EPMExplicitOverridesRule(t *testing.T) {
	cfg := config.Config{
		User: "u", URL: "http://localhost", Port: 8080,
		EPM: config.EPMConfig{Rules: []config.EPMRule{
			{FileTemplate: "EPMExportRZ*.xlsx", Project: "RZ Project", Activity: "Delivery", Skill: "Go"},
		}},
	}

	resolved, err := resolveConfigForFile("EPMExportRZ202601.xlsx", "epm", cfg, RunOptions{
		EPMProject: "Override Project",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.ImportProject != "Override Project" {
		t.Fatalf("expected override project, got %q", resolved.ImportProject)
	}
	if resolved.ImportActivity != "Delivery" || resolved.ImportSkill != "Go" {
		t.Fatalf("unexpected fallback values: activity=%q skill=%q", resolved.ImportActivity, resolved.ImportSkill)
	}
}

func TestResolveConfigForFile_EPMNoRuleAndNoExplicitFails(t *testing.T) {
	cfg := config.Config{User: "u", URL: "http://localhost", Port: 8080}

	_, err := resolveConfigForFile("EPMExportRZ202601.xlsx", "epm", cfg, RunOptions{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestResolveConfigForFile_EPMNoRuleButExplicitValuesWorks(t *testing.T) {
	cfg := config.Config{User: "u", URL: "http://localhost", Port: 8080}

	resolved, err := resolveConfigForFile("EPMExportRZ202601.xlsx", "epm", cfg, RunOptions{
		EPMProject:  "Manual Project",
		EPMActivity: "Manual Activity",
		EPMSkill:    "Manual Skill",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.ImportProject != "Manual Project" || resolved.ImportActivity != "Manual Activity" || resolved.ImportSkill != "Manual Skill" {
		t.Fatalf("unexpected resolved values: project=%q activity=%q skill=%q", resolved.ImportProject, resolved.ImportActivity, resolved.ImportSkill)
	}
}

func TestResolveConfigForFile_GenericMapperNeedsNoEPMValues(t *testing.T) {
	cfg := config.Config{User: "u", URL: "http://localhost", Port: 8080}

	resolved, err := resolveConfigForFile("generic.csv", "generic", cfg, RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.ImportProject != "" || resolved.ImportActivity != "" || resolved.ImportSkill != "" {
		t.Fatalf("expected empty import values for generic mapper")
	}
}
