package config

import (
	"strings"
	"testing"
)

func TestValidateYAMLContent_RejectsUnsupportedMapper(t *testing.T) {
	t.Parallel()

	content := []byte(`onepoint:
  url: "https://onepoint.virtual7.io/onepoint/faces/home"
import:
  auto_reconcile_after_import: true
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
import:
  auto_reconcile_after_import: true
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
