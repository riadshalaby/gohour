package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"gohour/onepoint"
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

	_, _, _, err := resolveOnePointURLs("https://onepoint.virtual7.io")
	if err == nil {
		t.Fatalf("expected error for base URL without path")
	}
}

func TestResolveOnePointURLs_WithHomeURLOverride(t *testing.T) {
	t.Cleanup(func() {
		viper.Reset()
		cfgFile = ""
	})

	base, home, host, err := resolveOnePointURLs("https://onepoint.virtual7.io/onepoint/faces/home")
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

func TestEnsureAuthenticated_AlreadyLoggedIn(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "state.json")
	stateJSON := `{
  "cookies": [
    {"name":"JSESSIONID","value":"abc","domain":"onepoint.virtual7.io","path":"/"}
  ]
}`
	if err := os.WriteFile(stateFile, []byte(stateJSON), 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	called := false
	previous := runBrowserLogin
	runBrowserLogin = func(baseURL, homeURL, host, stateFile string, timeout time.Duration, debugCookies bool) (string, error) {
		called = true
		return "", nil
	}
	t.Cleanup(func() {
		runBrowserLogin = previous
	})

	cookieHeader, baseURL, homeURL, host, err := ensureAuthenticated(
		"https://onepoint.virtual7.io/onepoint/faces/home",
		stateFile,
	)
	if err != nil {
		t.Fatalf("ensure authenticated: %v", err)
	}
	if called {
		t.Fatalf("did not expect browser login to be called")
	}
	if cookieHeader != "JSESSIONID=abc" {
		t.Fatalf("unexpected cookie header: %q", cookieHeader)
	}
	if baseURL != "https://onepoint.virtual7.io" || homeURL != "https://onepoint.virtual7.io/onepoint/faces/home" || host != "onepoint.virtual7.io" {
		t.Fatalf("unexpected urls/host: base=%q home=%q host=%q", baseURL, homeURL, host)
	}
}

func TestEnsureAuthenticated_MissingFile(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "missing.json")

	called := false
	previous := runBrowserLogin
	runBrowserLogin = func(baseURL, homeURL, host, stateFile string, timeout time.Duration, debugCookies bool) (string, error) {
		called = true
		return "JSESSIONID=from-browser", nil
	}
	t.Cleanup(func() {
		runBrowserLogin = previous
	})

	cookieHeader, _, _, _, err := ensureAuthenticated(
		"https://onepoint.virtual7.io/onepoint/faces/home",
		stateFile,
	)
	if err != nil {
		t.Fatalf("ensure authenticated: %v", err)
	}
	if !called {
		t.Fatalf("expected browser login to be called")
	}
	if cookieHeader != "JSESSIONID=from-browser" {
		t.Fatalf("unexpected cookie header: %q", cookieHeader)
	}
}

func TestEnsureAuthenticated_MissingCookies(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "state.json")
	stateJSON := `{
  "cookies": [
    {"name":"_WL_AUTHCOOKIE_JSESSIONID","value":"def","domain":"onepoint.virtual7.io","path":"/"}
  ]
}`
	if err := os.WriteFile(stateFile, []byte(stateJSON), 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	called := false
	previous := runBrowserLogin
	runBrowserLogin = func(baseURL, homeURL, host, stateFile string, timeout time.Duration, debugCookies bool) (string, error) {
		called = true
		return "JSESSIONID=from-browser", nil
	}
	t.Cleanup(func() {
		runBrowserLogin = previous
	})

	cookieHeader, _, _, _, err := ensureAuthenticated(
		"https://onepoint.virtual7.io/onepoint/faces/home",
		stateFile,
	)
	if err != nil {
		t.Fatalf("ensure authenticated: %v", err)
	}
	if !called {
		t.Fatalf("expected browser login to be called")
	}
	if cookieHeader != "JSESSIONID=from-browser" {
		t.Fatalf("unexpected cookie header: %q", cookieHeader)
	}
}

func TestRetryWithRelogin_RetriesOnceOnUnauthorized(t *testing.T) {
	cookieHeader := "JSESSIONID=old"
	stateFile := filepath.Join(t.TempDir(), "state.json")
	called := false
	previous := runBrowserLogin
	runBrowserLogin = func(baseURL, homeURL, host, stateFile string, timeout time.Duration, debugCookies bool) (string, error) {
		called = true
		return "JSESSIONID=new", nil
	}
	t.Cleanup(func() {
		runBrowserLogin = previous
	})

	attempts := 0
	result, err := retryWithRelogin(
		"https://onepoint.virtual7.io",
		"https://onepoint.virtual7.io/onepoint/faces/home",
		"onepoint.virtual7.io",
		stateFile,
		"gohour-test/1.0",
		&cookieHeader,
		func(client onepoint.Client) (string, error) {
			attempts++
			if attempts == 1 {
				return "", fmt.Errorf("request failed: %w", onepoint.ErrAuthUnauthorized)
			}
			return "ok", nil
		},
	)
	if err != nil {
		t.Fatalf("retryWithRelogin returned error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("unexpected result: %q", result)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if !called {
		t.Fatalf("expected browser login call")
	}
	if cookieHeader != "JSESSIONID=new" {
		t.Fatalf("expected refreshed cookie header, got %q", cookieHeader)
	}
}

func TestRetryWithRelogin_DoesNotRetryOnNonUnauthorized(t *testing.T) {
	cookieHeader := "JSESSIONID=old"
	called := false
	previous := runBrowserLogin
	runBrowserLogin = func(baseURL, homeURL, host, stateFile string, timeout time.Duration, debugCookies bool) (string, error) {
		called = true
		return "JSESSIONID=new", nil
	}
	t.Cleanup(func() {
		runBrowserLogin = previous
	})

	wantErr := errors.New("boom")
	_, err := retryWithRelogin(
		"https://onepoint.virtual7.io",
		"https://onepoint.virtual7.io/onepoint/faces/home",
		"onepoint.virtual7.io",
		filepath.Join(t.TempDir(), "state.json"),
		"gohour-test/1.0",
		&cookieHeader,
		func(client onepoint.Client) (string, error) {
			return "", wantErr
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected original error, got %v", err)
	}
	if called {
		t.Fatalf("did not expect browser login call")
	}
}
