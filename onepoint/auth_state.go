package onepoint

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	SessionCookieJSESSIONID    = "JSESSIONID"
	SessionCookieWLAuthSession = "_WL_AUTHCOOKIE_JSESSIONID"
)

type storageState struct {
	Cookies []stateCookie `json:"cookies"`
}

type stateCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

func DefaultAuthStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".gohour", "onepoint-auth-state.json"), nil
}

func SessionCookieHeaderFromStateFile(path, targetHost string) (string, error) {
	content, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return "", fmt.Errorf("read auth state file: %w", err)
	}

	var state storageState
	if err := json.Unmarshal(content, &state); err != nil {
		return "", fmt.Errorf("decode auth state file: %w", err)
	}

	return sessionCookieHeaderFromState(state, targetHost)
}

func sessionCookieHeaderFromState(state storageState, targetHost string) (string, error) {
	host := normalizeHost(targetHost)
	if host == "" {
		return "", errors.New("target host is required")
	}

	values := map[string]string{}
	for _, cookie := range state.Cookies {
		if cookie.Name == "" || cookie.Value == "" {
			continue
		}
		if !cookieDomainMatches(cookie.Domain, host) {
			continue
		}
		if cookie.Path != "" && cookie.Path != "/" {
			continue
		}
		if cookie.Name == SessionCookieJSESSIONID || cookie.Name == SessionCookieWLAuthSession {
			values[cookie.Name] = cookie.Value
		}
	}

	missing := make([]string, 0, 1)
	if values[SessionCookieJSESSIONID] == "" {
		missing = append(missing, SessionCookieJSESSIONID)
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("missing required session cookies for host %q: %s", host, strings.Join(missing, ", "))
	}

	header := fmt.Sprintf(
		"%s=%s",
		SessionCookieJSESSIONID,
		values[SessionCookieJSESSIONID],
	)
	if values[SessionCookieWLAuthSession] != "" {
		header += fmt.Sprintf("; %s=%s", SessionCookieWLAuthSession, values[SessionCookieWLAuthSession])
	}
	return header, nil
}

func cookieDomainMatches(cookieDomain, targetHost string) bool {
	domain := normalizeHost(cookieDomain)
	host := normalizeHost(targetHost)
	if domain == "" || host == "" {
		return false
	}
	return domain == host || strings.HasSuffix(host, "."+domain)
}

func normalizeHost(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, ".")
	value = strings.TrimSuffix(value, "/")
	return value
}
