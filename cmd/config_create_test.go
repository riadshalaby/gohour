package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	if !strings.Contains(text, "user: \"your.user\"") {
		t.Fatalf("expected example user value in config file, got:\n%s", text)
	}
}

func TestSaveDefaultConfigDoesNotOverwriteExistingFile(t *testing.T) {
	t.Cleanup(func() {
		cfgFile = ""
		viper.Reset()
	})

	tmpConfig := filepath.Join(t.TempDir(), "existing.yaml")
	original := "user: \"existing\"\nurl: \"https://company.example\"\nport: 443\n"
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
