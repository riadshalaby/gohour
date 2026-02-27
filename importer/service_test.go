package importer

import (
	"gohour/config"
	"testing"
)

func TestResolveConfigForFile_EPMRuleMatch(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{Mapper: "epm", FileTemplate: "EPMExportRZ*.xlsx", ProjectID: 1, Project: "RZ Project", ActivityID: 2, Activity: "Delivery", SkillID: 3, Skill: "Go"},
		},
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
		Rules: []config.Rule{
			{Mapper: "epm", FileTemplate: "EPMExportRZ*.xlsx", ProjectID: 1, Project: "RZ Project", ActivityID: 2, Activity: "Delivery", SkillID: 3, Skill: "Go"},
		},
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
	cfg := config.Config{}

	_, err := resolveConfigForFile("EPMExportRZ202601.xlsx", "epm", cfg, RunOptions{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestResolveConfigForFile_UsesTemplateMatchedRule(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{Mapper: "generic", FileTemplate: "EPMExportRZ*.xlsx", ProjectID: 1, Project: "FromRule", ActivityID: 2, Activity: "FromRule", SkillID: 3, Skill: "FromRule"},
		},
	}

	resolved, err := resolveConfigForFile("EPMExportRZ202601.xlsx", "epm", cfg, RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.ImportProject != "FromRule" || resolved.ImportActivity != "FromRule" || resolved.ImportSkill != "FromRule" {
		t.Fatalf("expected values from matched rule, got project=%q activity=%q skill=%q", resolved.ImportProject, resolved.ImportActivity, resolved.ImportSkill)
	}
}

func TestMatchRuleByTemplate(t *testing.T) {
	rules := []config.Rule{
		{Name: "a", Mapper: "epm", FileTemplate: "EPMExportRZ*.xlsx"},
	}

	rule := MatchRuleByTemplate("EPMExportRZ202601.xlsx", rules)
	if rule.Name != "a" {
		t.Fatalf("expected rule a, got %+v", rule)
	}
}

func TestResolveConfigForFile_EPMNoRuleButExplicitValuesWorks(t *testing.T) {
	cfg := config.Config{}

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
	cfg := config.Config{}

	resolved, err := resolveConfigForFile("generic.csv", "generic", cfg, RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.ImportProject != "" || resolved.ImportActivity != "" || resolved.ImportSkill != "" {
		t.Fatalf("expected empty import values for generic mapper")
	}
}
