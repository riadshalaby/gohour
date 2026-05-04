package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMigrationExistingNewConfigNoOp(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "config.yaml"), []byte("onepoint:\n  url: \"https://existing.example.test/home\"\n"), 0o600); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	var out bytes.Buffer
	err := runMigration(migrationOptions{
		cwd:     t.TempDir(),
		home:    t.TempDir(),
		dataDir: dataDir,
		input:   strings.NewReader("fresh\n"),
		output:  &out,
	})
	if err != nil {
		t.Fatalf("runMigration() returned error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no prompt when new config exists, got %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(dataDir, "config.yaml")); err != nil {
		t.Fatalf("new config missing after no-op: %v", err)
	}
}

func TestRunMigrationMovesOldFilesFromCurrentDirectory(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	dataDir := t.TempDir()
	oldConfig := filepath.Join(cwd, ".gohour.yaml")
	oldDB := filepath.Join(cwd, "gohour.db")
	if err := os.WriteFile(oldConfig, []byte("onepoint:\n  url: \"https://old.example.test/home\"\nrules: []\n"), 0o600); err != nil {
		t.Fatalf("write old config: %v", err)
	}
	if err := os.WriteFile(oldDB, []byte("sqlite"), 0o600); err != nil {
		t.Fatalf("write old db: %v", err)
	}

	var out bytes.Buffer
	err := runMigration(migrationOptions{
		cwd:     cwd,
		home:    home,
		dataDir: dataDir,
		input:   strings.NewReader("move\n"),
		output:  &out,
	})
	if err != nil {
		t.Fatalf("runMigration() returned error: %v", err)
	}
	assertFileContent(t, filepath.Join(dataDir, "config.yaml"), "https://old.example.test/home")
	assertFileContent(t, filepath.Join(dataDir, "gohour.db"), "sqlite")
	if _, err := os.Stat(oldConfig + ".bak"); err != nil {
		t.Fatalf("expected old config backup: %v", err)
	}
	if _, err := os.Stat(oldDB + ".bak"); err != nil {
		t.Fatalf("expected old db backup: %v", err)
	}
	if !strings.Contains(out.String(), "Move existing files") {
		t.Fatalf("expected migration prompt, got %q", out.String())
	}
}

func TestRunMigrationFreshKeepsOldFilesAndWritesDefault(t *testing.T) {
	cwd := t.TempDir()
	dataDir := t.TempDir()
	oldConfig := filepath.Join(cwd, ".gohour.yaml")
	if err := os.WriteFile(oldConfig, []byte("onepoint:\n  url: \"https://old.example.test/home\"\n"), 0o600); err != nil {
		t.Fatalf("write old config: %v", err)
	}

	err := runMigration(migrationOptions{
		cwd:     cwd,
		home:    t.TempDir(),
		dataDir: dataDir,
		input:   strings.NewReader("fresh\n"),
		output:  &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("runMigration() returned error: %v", err)
	}
	assertFileContent(t, filepath.Join(dataDir, "config.yaml"), "https://onepoint.virtual7.io/onepoint/faces/home")
	if _, err := os.Stat(oldConfig); err != nil {
		t.Fatalf("expected old config to remain in place: %v", err)
	}
	if _, err := os.Stat(oldConfig + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("old config should not have been backed up on fresh migration")
	}
}

func TestRunMigrationNoOldFilesWritesDefaultConfig(t *testing.T) {
	dataDir := t.TempDir()

	err := runMigration(migrationOptions{
		cwd:     t.TempDir(),
		home:    t.TempDir(),
		dataDir: dataDir,
		input:   strings.NewReader(""),
		output:  &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("runMigration() returned error: %v", err)
	}
	assertFileContent(t, filepath.Join(dataDir, "config.yaml"), "https://onepoint.virtual7.io/onepoint/faces/home")
	if _, err := os.Stat(filepath.Join(dataDir, "gohour.db")); !os.IsNotExist(err) {
		t.Fatalf("database should be created on first open, stat err=%v", err)
	}
}

func TestRunMigrationDetectsHomeConfigWhenCurrentDirectoryHasNone(t *testing.T) {
	home := t.TempDir()
	dataDir := t.TempDir()
	oldConfig := filepath.Join(home, ".gohour.yaml")
	if err := os.WriteFile(oldConfig, []byte("onepoint:\n  url: \"https://home.example.test/home\"\nrules: []\n"), 0o600); err != nil {
		t.Fatalf("write old home config: %v", err)
	}

	err := runMigration(migrationOptions{
		cwd:     t.TempDir(),
		home:    home,
		dataDir: dataDir,
		input:   strings.NewReader("move\n"),
		output:  &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("runMigration() returned error: %v", err)
	}
	assertFileContent(t, filepath.Join(dataDir, "config.yaml"), "https://home.example.test/home")
	if _, err := os.Stat(oldConfig + ".bak"); err != nil {
		t.Fatalf("expected old home config backup: %v", err)
	}
}

func assertFileContent(t *testing.T, path string, needle string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(content), needle) {
		t.Fatalf("expected %s to contain %q, got:\n%s", path, needle, string(content))
	}
}
