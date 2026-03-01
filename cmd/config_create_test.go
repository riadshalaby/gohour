package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gohour/config"

	"github.com/spf13/viper"
)

func TestSaveDefaultConfigCreatesExampleTemplate(t *testing.T) {
	t.Cleanup(func() {
		cfgFile = ""
		viper.Reset()
	})

	tmpConfig := filepath.Join(t.TempDir(), "create-template.yaml")
	cfgFile = tmpConfig
	viper.Reset()

	if err := saveDefaultConfig(); err != nil {
		t.Fatalf("unexpected error creating config: %v", err)
	}

	content, err := os.ReadFile(tmpConfig)
	if err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "# gohour configuration") {
		t.Fatalf("expected example header in config file, got:\n%s", text)
	}
	if !strings.Contains(text, "onepoint:") || !strings.Contains(text, "url: \"https://onepoint.virtual7.io/onepoint/faces/home\"") {
		t.Fatalf("expected onepoint URL example in config file, got:\n%s", text)
	}
	if strings.Contains(text, "file_template:") {
		t.Fatalf("did not expect demo rule in default config template, got:\n%s", text)
	}
	if !strings.Contains(text, "rules: []") {
		t.Fatalf("expected empty rules list in default config template, got:\n%s", text)
	}
}

func TestSaveDefaultConfigDoesNotOverwriteExistingFile(t *testing.T) {
	t.Cleanup(func() {
		cfgFile = ""
		viper.Reset()
	})

	tmpConfig := filepath.Join(t.TempDir(), "existing.yaml")
	original := "onepoint:\n  url: \"https://onepoint.virtual7.io/onepoint/faces/home\"\nimport:\n  auto_reconcile_after_import: true\n"
	if err := os.WriteFile(tmpConfig, []byte(original), 0o644); err != nil {
		t.Fatalf("failed writing initial config: %v", err)
	}

	cfgFile = tmpConfig
	viper.Reset()

	if err := saveDefaultConfig(); err != nil {
		t.Fatalf("unexpected error creating config: %v", err)
	}

	content, err := os.ReadFile(tmpConfig)
	if err != nil {
		t.Fatalf("failed reading existing config after create: %v", err)
	}
	if string(content) != original {
		t.Fatalf("expected existing config to remain unchanged")
	}
}

func TestConfigShow_PrintsRuleBillable(t *testing.T) {
	t.Cleanup(func() {
		cfgFile = ""
		viper.Reset()
	})

	tmpConfig := filepath.Join(t.TempDir(), "config-show.yaml")
	content := `onepoint:
  url: "https://onepoint.virtual7.io/onepoint/faces/home"
import:
  auto_reconcile_after_import: true
rules:
  - name: "rule-false"
    mapper: "generic"
    file_template: "a.csv"
    billable: false
    project_id: 1
    project: "P1"
    activity_id: 2
    activity: "A1"
    skill_id: 3
    skill: "S1"
  - name: "rule-default"
    mapper: "generic"
    file_template: "b.csv"
    project_id: 4
    project: "P2"
    activity_id: 5
    activity: "A2"
    skill_id: 6
    skill: "S2"
`
	if err := os.WriteFile(tmpConfig, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	viper.Reset()
	config.SetDefaults()
	viper.SetConfigFile(tmpConfig)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("read config: %v", err)
	}

	out := captureStdout(t, func() {
		configShowCmd.Run(configShowCmd, nil)
	})

	if !strings.Contains(out, "rules[0].billable: false") {
		t.Fatalf("expected explicit false billable in output, got:\n%s", out)
	}
	if !strings.Contains(out, "rules[1].billable: true (default)") {
		t.Fatalf("expected default billable in output, got:\n%s", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = w

	runDone := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(runDone)
	}()

	fn()

	_ = w.Close()
	os.Stdout = old
	<-runDone
	_ = r.Close()

	return buf.String()
}
