package cmd

import (
	"strings"
	"testing"

	"github.com/chromedp/cdproto/network"
)

func TestHasRequiredSessionCookies(t *testing.T) {
	t.Parallel()

	host := "onepoint.virtual7.io"
	cookies := []*network.Cookie{
		{Name: "JSESSIONID", Value: "a", Domain: "onepoint.virtual7.io", Path: "/"},
		{Name: "_WL_AUTHCOOKIE_JSESSIONID", Value: "b", Domain: "onepoint.virtual7.io", Path: "/"},
	}
	if !hasRequiredSessionCookies(cookies, host) {
		t.Fatalf("expected required cookies to be detected")
	}
}

func TestHasRequiredSessionCookies_RequiresJSessionID(t *testing.T) {
	t.Parallel()

	host := "onepoint.virtual7.io"
	cookies := []*network.Cookie{
		{Name: "JSESSIONID", Value: "a", Domain: "onepoint.virtual7.io", Path: "/"},
	}
	if !hasRequiredSessionCookies(cookies, host) {
		t.Fatalf("expected success when JSESSIONID is present")
	}

	cookies = []*network.Cookie{
		{Name: "_WL_AUTHCOOKIE_JSESSIONID", Value: "b", Domain: "onepoint.virtual7.io", Path: "/"},
	}
	if hasRequiredSessionCookies(cookies, host) {
		t.Fatalf("did not expect success when JSESSIONID is missing")
	}
}

func TestHasRequiredSessionCookies_AnyHostMode(t *testing.T) {
	t.Parallel()

	cookies := []*network.Cookie{
		{Name: "JSESSIONID", Value: "a", Domain: "onepoint.virtual7.io", Path: "/"},
		{Name: "_WL_AUTHCOOKIE_JSESSIONID", Value: "b", Domain: "onepoint.virtual7.io", Path: "/"},
	}
	if !hasRequiredSessionCookies(cookies, "") {
		t.Fatalf("expected cookie detection without host filter")
	}
}

func TestFindSessionCookieHost(t *testing.T) {
	t.Parallel()

	cookies := []*network.Cookie{
		{Name: "JSESSIONID", Value: "a", Domain: ".onepoint.virtual7.io", Path: "/"},
		{Name: "_WL_AUTHCOOKIE_JSESSIONID", Value: "b", Domain: "onepoint.virtual7.io", Path: "/"},
		{Name: "JSESSIONID", Value: "x", Domain: "example.com", Path: "/"},
	}
	got := findSessionCookieHost(cookies)
	if got != "onepoint.virtual7.io" {
		t.Fatalf("unexpected host: %q", got)
	}
}

func TestSummarizeCookieInventory(t *testing.T) {
	t.Parallel()

	cookies := []*network.Cookie{
		{Name: "JSESSIONID", Domain: ".onepoint.virtual7.io"},
		{Name: "_WL_AUTHCOOKIE_JSESSIONID", Domain: "onepoint.virtual7.io"},
		{Name: "ESTSAUTH", Domain: ".login.microsoftonline.com"},
	}

	summary := summarizeCookieInventory(cookies)
	if !strings.Contains(summary, "cookies=3") {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if !strings.Contains(summary, "onepoint.virtual7.io=[") {
		t.Fatalf("expected onepoint domain in summary: %q", summary)
	}
	if !strings.Contains(summary, "JSESSIONID") {
		t.Fatalf("expected JSESSIONID in summary: %q", summary)
	}
}
