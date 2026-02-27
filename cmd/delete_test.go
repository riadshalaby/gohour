package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestConfirmDeletePrompt(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "uppercase Y confirms", input: "Y\n", want: true},
		{name: "lowercase y does not confirm", input: "y\n", want: false},
		{name: "N does not confirm", input: "N\n", want: false},
		{name: "empty does not confirm", input: "\n", want: false},
		{name: "Y without newline confirms", input: "Y", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := confirmDeletePrompt(bytes.NewBufferString(tt.input), &out, "./gohour.db")
			if err != nil {
				t.Fatalf("confirm prompt returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
			if out.Len() == 0 {
				t.Fatalf("expected prompt output")
			}
		})
	}
}

func TestRemoveDatabaseFile(t *testing.T) {
	t.Run("deletes existing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "gohour.db")
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("write temp db file: %v", err)
		}

		if err := removeDatabaseFile(path); err != nil {
			t.Fatalf("remove db file: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected file to be deleted")
		}
	})

	t.Run("fails for directory path", func(t *testing.T) {
		dir := t.TempDir()
		if err := removeDatabaseFile(dir); err == nil {
			t.Fatalf("expected error for directory path")
		}
	})

	t.Run("fails for missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.db")
		if err := removeDatabaseFile(path); err == nil {
			t.Fatalf("expected error for missing file")
		}
	})
}
