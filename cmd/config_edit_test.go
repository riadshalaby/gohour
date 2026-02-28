package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveConfigEditPath(t *testing.T) {
	t.Run("uses explicit flag first", func(t *testing.T) {
		got, err := resolveConfigEditPath("./custom.yaml", "/tmp/active.yaml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "./custom.yaml" {
			t.Fatalf("expected explicit config path, got %q", got)
		}
	})

	t.Run("uses active config when flag is empty", func(t *testing.T) {
		got, err := resolveConfigEditPath("", "/tmp/active.yaml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/tmp/active.yaml" {
			t.Fatalf("expected active config path, got %q", got)
		}
	})

	t.Run("falls back to home config path", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		got, err := resolveConfigEditPath("", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(home, ".gohour.yaml")
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})
}

func TestEnsureConfigFileWithTemplate(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "nested", "myconfig.yaml")

	created, err := ensureConfigFileWithTemplate(configPath)
	if err != nil {
		t.Fatalf("unexpected error creating template config: %v", err)
	}
	if !created {
		t.Fatalf("expected file to be created")
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("unexpected error reading config file: %v", err)
	}
	if !strings.Contains(string(content), "# gohour configuration") {
		t.Fatalf("expected example config content, got:\n%s", string(content))
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("unexpected error stat config file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected config file mode 0600, got %o", info.Mode().Perm())
	}

	created, err = ensureConfigFileWithTemplate(configPath)
	if err != nil {
		t.Fatalf("unexpected error on existing config file: %v", err)
	}
	if created {
		t.Fatalf("did not expect existing file to be recreated")
	}
}

func TestResolveEditorValue(t *testing.T) {
	tests := []struct {
		name   string
		visual string
		editor string
		want   string
	}{
		{name: "visual wins", visual: "code --wait", editor: "nano", want: "code --wait"},
		{name: "editor fallback", visual: "", editor: "nano", want: "nano"},
		{name: "default vi", visual: "", editor: "", want: "vi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEditorValue(tt.visual, tt.editor)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestBuildEditorCommand(t *testing.T) {
	t.Run("splits editor args and appends config path", func(t *testing.T) {
		cmd, err := buildEditorCommand("code --wait", "/tmp/cfg.yaml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cmd.Path != "code" {
			t.Fatalf("expected command path %q, got %q", "code", cmd.Path)
		}
		if len(cmd.Args) != 3 {
			t.Fatalf("expected 3 args, got %d", len(cmd.Args))
		}
		if cmd.Args[1] != "--wait" || cmd.Args[2] != "/tmp/cfg.yaml" {
			t.Fatalf("unexpected command args: %#v", cmd.Args)
		}
	})

	t.Run("fails on empty editor", func(t *testing.T) {
		if _, err := buildEditorCommand("   ", "/tmp/cfg.yaml"); err == nil {
			t.Fatalf("expected error for empty editor")
		}
	})
}
