package cmd

import (
	"regexp"
	"testing"
)

func TestVersionLiteralFormat(t *testing.T) {
	t.Parallel()
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
	semver := regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[\w.\-]+)?(\+[\w.\-]+)?$`)
	if Version == "dev" {
		return
	}
	if !semver.MatchString(Version) {
		t.Fatalf("Version %q must be %q or a semver string", Version, "dev")
	}
}
