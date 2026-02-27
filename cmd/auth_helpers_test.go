package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestResolveDefaultAuthStatePath(t *testing.T) {
	t.Run("explicit path wins", func(t *testing.T) {
		got, err := resolveDefaultAuthStatePath("./state.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "./state.json" {
			t.Fatalf("expected explicit path, got %q", got)
		}
	})

	t.Run("uses home fallback", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		got, err := resolveDefaultAuthStatePath("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(home, ".gohour", "onepoint-auth-state.json")
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})
}

func TestResolveProfileDir(t *testing.T) {
	t.Run("explicit path wins", func(t *testing.T) {
		got, isTemp, err := resolveProfileDir("./profile")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "./profile" {
			t.Fatalf("expected explicit path, got %q", got)
		}
		if isTemp {
			t.Fatalf("did not expect explicit profile to be marked as temp")
		}
	})

	t.Run("creates temp profile dir by default", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		got, isTemp, err := resolveProfileDir("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isTemp {
			t.Fatalf("expected temp profile flag")
		}
		if !strings.HasPrefix(got, filepath.Join(home, ".gohour", "chrome-profile-")) {
			t.Fatalf("unexpected temp profile path: %q", got)
		}
	})
}

func TestResolveOnePointURLs_WithOverride(t *testing.T) {
	t.Cleanup(func() {
		viper.Reset()
		cfgFile = ""
	})

	base, home, host, err := resolveOnePointURLs("https://onepoint.virtual7.io")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if base != "https://onepoint.virtual7.io" {
		t.Fatalf("unexpected base URL: %q", base)
	}
	if home != "https://onepoint.virtual7.io/onepoint/faces/home" {
		t.Fatalf("unexpected home URL: %q", home)
	}
	if host != "onepoint.virtual7.io" {
		t.Fatalf("unexpected host: %q", host)
	}
}

func TestResolveOnePointURLs_NoConfigAndNoOverride(t *testing.T) {
	t.Cleanup(func() {
		viper.Reset()
		cfgFile = ""
	})
	viper.Reset()
	cfgFile = ""

	_, _, _, err := resolveOnePointURLs("")
	if err == nil {
		t.Fatalf("expected error")
	}
}
