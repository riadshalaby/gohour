package onepoint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionCookieHeaderFromStateFile(t *testing.T) {
	t.Parallel()

	stateJSON := `{
  "cookies": [
    {"name":"JSESSIONID","value":"abc","domain":"onepoint.virtual7.io","path":"/"},
    {"name":"_WL_AUTHCOOKIE_JSESSIONID","value":"def","domain":"onepoint.virtual7.io","path":"/"}
  ],
  "origins": []
}`

	file := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(file, []byte(stateJSON), 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	header, err := SessionCookieHeaderFromStateFile(file, "onepoint.virtual7.io")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if header != "JSESSIONID=abc; _WL_AUTHCOOKIE_JSESSIONID=def" {
		t.Fatalf("unexpected header: %q", header)
	}
}

func TestSessionCookieHeaderFromStateFile_MissingCookie(t *testing.T) {
	t.Parallel()

	stateJSON := `{
  "cookies": [
    {"name":"JSESSIONID","value":"abc","domain":"onepoint.virtual7.io","path":"/"}
  ]
}`

	file := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(file, []byte(stateJSON), 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	header, err := SessionCookieHeaderFromStateFile(file, "onepoint.virtual7.io")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(header, "JSESSIONID=abc") {
		t.Fatalf("unexpected header: %q", header)
	}
}

func TestCookieDomainMatches(t *testing.T) {
	t.Parallel()

	if !cookieDomainMatches(".onepoint.virtual7.io", "onepoint.virtual7.io") {
		t.Fatalf("expected exact domain match")
	}
	if !cookieDomainMatches("virtual7.io", "onepoint.virtual7.io") {
		t.Fatalf("expected parent domain match")
	}
	if cookieDomainMatches("example.com", "onepoint.virtual7.io") {
		t.Fatalf("did not expect unrelated domain to match")
	}
}
