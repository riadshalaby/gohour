package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/riadshalaby/gohour/config"
	"github.com/riadshalaby/gohour/onepoint"

	"github.com/spf13/viper"
)

func resolveDefaultAuthStatePath(explicitPath string) (string, error) {
	if strings.TrimSpace(explicitPath) != "" {
		return explicitPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".gohour", "onepoint-auth-state.json"), nil
}

func resolveProfileDir(explicitDir string) (string, bool, error) {
	if strings.TrimSpace(explicitDir) != "" {
		return explicitDir, false, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false, fmt.Errorf("resolve home directory: %w", err)
	}
	base := filepath.Join(home, ".gohour")
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", false, fmt.Errorf("create directory %q: %w", base, err)
	}
	profileDir, err := os.MkdirTemp(base, "chrome-profile-*")
	if err != nil {
		return "", false, fmt.Errorf("create temporary profile dir: %w", err)
	}
	return profileDir, true, nil
}

func ensureParentDir(path string, mode os.FileMode) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, mode); err != nil {
		return fmt.Errorf("create directory %q: %w", parent, err)
	}
	return nil
}

func resolveOnePointURLs(urlOverride string) (string, string, string, error) {
	rawURL := strings.TrimSpace(urlOverride)
	if rawURL == "" {
		if strings.TrimSpace(viper.ConfigFileUsed()) == "" {
			return "", "", "", errors.New("no config file loaded; set `onepoint.url` in config or pass --url")
		}
		cfg, err := config.LoadAndValidate()
		if err != nil {
			return "", "", "", fmt.Errorf("load config: %w", err)
		}
		rawURL = strings.TrimSpace(cfg.OnePoint.URL)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("parse url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", "", "", fmt.Errorf("invalid url %q", rawURL)
	}
	path := "/" + strings.Trim(strings.TrimSpace(parsed.Path), "/")
	if path == "/" {
		return "", "", "", fmt.Errorf("invalid onepoint url %q: path is required (expected full home URL)", rawURL)
	}

	apiBaseURL := parsed.Scheme + "://" + parsed.Host
	homeURL := apiBaseURL + path
	return apiBaseURL, homeURL, parsed.Hostname(), nil
}

func ensureAuthenticatedWithStateFile(urlOverride, stateFilePath string) (cookieHeader, baseURL, homeURL, host, stateFile string, err error) {
	baseURL, homeURL, host, err = resolveOnePointURLs(urlOverride)
	if err != nil {
		return
	}

	stateFile, err = resolveDefaultAuthStatePath(stateFilePath)
	if err != nil {
		return
	}

	cookieHeader, err = onepoint.SessionCookieHeaderFromStateFile(stateFile, host)
	if err == nil {
		return
	}

	if !errors.Is(err, onepoint.ErrAuthStateNotFound) && !errors.Is(err, onepoint.ErrMissingSessionCookies) {
		err = fmt.Errorf("read auth state: %w", err)
		return
	}

	fmt.Println("Not logged in to OnePoint. Opening browser for login...")
	cookieHeader, err = runBrowserLogin(baseURL, homeURL, host, stateFile, 10*time.Minute, false)
	return
}

// ensureAuthenticated returns a valid session cookie header, triggering an
// interactive browser login automatically if the auth state is missing or
// incomplete.
func ensureAuthenticated(urlOverride, stateFilePath string) (cookieHeader, baseURL, homeURL, host string, err error) {
	cookieHeader, baseURL, homeURL, host, _, err = ensureAuthenticatedWithStateFile(urlOverride, stateFilePath)
	return
}

func retryWithRelogin[T any](
	baseURL, homeURL, host, stateFile, userAgent string,
	cookieHeader *string,
	operation func(client onepoint.Client) (T, error),
) (T, error) {
	var zero T
	if cookieHeader == nil {
		return zero, errors.New("cookie header pointer is required")
	}

	newClient := func(header string) (onepoint.Client, error) {
		return onepoint.NewClient(onepoint.ClientConfig{
			BaseURL:        baseURL,
			RefererURL:     homeURL,
			SessionCookies: header,
			UserAgent:      userAgent,
		})
	}

	client, err := newClient(*cookieHeader)
	if err != nil {
		return zero, err
	}

	result, err := operation(client)
	if err == nil {
		return result, nil
	}
	if !errors.Is(err, onepoint.ErrAuthUnauthorized) {
		return zero, err
	}

	fmt.Println("OnePoint session expired. Opening browser for login...")
	refreshedHeader, loginErr := runBrowserLogin(baseURL, homeURL, host, stateFile, 10*time.Minute, false)
	if loginErr != nil {
		return zero, loginErr
	}
	*cookieHeader = refreshedHeader

	client, err = newClient(*cookieHeader)
	if err != nil {
		return zero, err
	}
	return operation(client)
}
