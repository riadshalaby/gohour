package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gohour/config"

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
			return "", "", "", errors.New("no config file loaded; set `url` in config or pass --url")
		}
		cfg, err := config.LoadAndValidate()
		if err != nil {
			return "", "", "", fmt.Errorf("load config: %w", err)
		}
		rawURL = strings.TrimSpace(cfg.URL)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("parse url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", "", "", fmt.Errorf("invalid url %q", rawURL)
	}

	apiBaseURL := parsed.Scheme + "://" + parsed.Host
	homeURL := apiBaseURL + "/onepoint/faces/home"
	return apiBaseURL, homeURL, parsed.Hostname(), nil
}
