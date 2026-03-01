package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"gohour/onepoint"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"
)

var (
	authLoginURL          string
	authLoginStateFile    string
	authLoginProfileDir   string
	authLoginSkipVerify   bool
	authLoginBrowserBin   string
	authLoginTimeout      time.Duration
	authLoginDebugCookies bool
)

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Start interactive browser login and save authenticated state.",
	Long: `Open a visible browser for Microsoft SSO login and save auth state as JSON.

The command validates that OnePoint session cookies are present. By default, it also verifies
the session with a test API call (list projects).`,
	Example: `
  # Open browser, log in manually, save auth state, verify API access
  gohour auth login
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stateFile, err := resolveDefaultAuthStatePath(authLoginStateFile)
		if err != nil {
			return err
		}
		profileDir, isTempProfile, err := resolveProfileDir(authLoginProfileDir)
		if err != nil {
			return err
		}
		if isTempProfile {
			defer os.RemoveAll(profileDir)
		}

		baseURL, homeURL, host, err := resolveOnePointURLs(authLoginURL)
		if err != nil {
			return err
		}

		if err := ensureParentDir(stateFile, 0o700); err != nil {
			return err
		}
		if err := os.MkdirAll(profileDir, 0o700); err != nil {
			return fmt.Errorf("create profile directory %q: %w", profileDir, err)
		}

		allocOptions := []chromedp.ExecAllocatorOption{
			chromedp.Flag("headless", false),
			chromedp.UserDataDir(profileDir),
			chromedp.Flag("disable-infobars", true),
			chromedp.Flag("new-window", true),
			chromedp.Flag("restore-last-session", false),
			chromedp.NoDefaultBrowserCheck,
			chromedp.NoFirstRun,
		}
		if strings.TrimSpace(authLoginBrowserBin) != "" {
			allocOptions = append(allocOptions, chromedp.ExecPath(strings.TrimSpace(authLoginBrowserBin)))
		}

		allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOptions...)
		defer allocCancel()

		ctx, cancel := chromedp.NewContext(allocCtx)
		defer cancel()

		if err := chromedp.Run(ctx,
			network.Enable(),
			chromedp.Navigate(homeURL),
		); err != nil {
			return fmt.Errorf("open browser and navigate failed: %w", err)
		}

		fmt.Println("Complete Microsoft login in the opened browser.")
		fmt.Printf("Waiting for OnePoint session cookies (timeout: %s)...\n", authLoginTimeout)
		waitCtx, waitCancel := context.WithTimeout(ctx, authLoginTimeout)
		defer waitCancel()
		waitResult, err := waitForSessionCookies(waitCtx, homeURL, baseURL, host, authLoginDebugCookies)
		if err != nil {
			return err
		}
		host = waitResult.Host
		if waitResult.BaseURL != "" {
			baseURL = waitResult.BaseURL
		}
		if waitResult.HomeURL != "" {
			homeURL = waitResult.HomeURL
		}

		allCookies, err := getBrowserCookies(ctx)
		if err != nil {
			return fmt.Errorf("read browser cookies failed: %w", err)
		}

		state := authStateFile{
			Cookies: filterCookiesForHost(allCookies, host),
			Origins: []any{},
		}

		content, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return fmt.Errorf("encode auth state: %w", err)
		}
		if err := os.WriteFile(stateFile, content, 0o600); err != nil {
			return fmt.Errorf("write auth state file: %w", err)
		}

		cookieHeader, err := onepoint.SessionCookieHeaderFromStateFile(stateFile, host)
		if err != nil {
			return fmt.Errorf("extract session cookies from %q: %w", stateFile, err)
		}

		if authLoginSkipVerify {
			fmt.Printf("Auth state saved: %s\n", stateFile)
			fmt.Println("Session cookies are present and ready for REST calls.")
			return nil
		}

		client, err := onepoint.NewClient(onepoint.ClientConfig{
			BaseURL:        baseURL,
			RefererURL:     homeURL,
			SessionCookies: cookieHeader,
			UserAgent:      "gohour-auth/1.0",
		})
		if err != nil {
			return err
		}

		verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer verifyCancel()

		projects, err := client.ListProjects(verifyCtx)
		if err != nil {
			return fmt.Errorf("auth verification failed (ListProjects): %w", err)
		}

		fmt.Printf("Auth state saved: %s\n", stateFile)
		fmt.Printf("Auth verification successful. Projects visible: %d\n", len(projects))
		return nil
	},
}

type authStateFile struct {
	Cookies []authStateCookie `json:"cookies"`
	Origins []any             `json:"origins"`
}

type authStateCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite"`
}

type sessionWaitResult struct {
	Host    string
	BaseURL string
	HomeURL string
}

func waitForSessionCookies(ctx context.Context, homeURL, baseURL, preferredHost string, debug bool) (sessionWaitResult, error) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastURL := homeURL

	for {
		var currentURL string
		if err := chromedp.Run(ctx, chromedp.Location(&currentURL)); err == nil && strings.TrimSpace(currentURL) != "" {
			lastURL = currentURL
		}

		cookies, err := getBrowserCookies(ctx)
		if debug {
			if err != nil {
				fmt.Printf("[auth-debug] url=%s cookie-read-error=%v\n", lastURL, err)
			} else {
				fmt.Printf("[auth-debug] url=%s %s\n", lastURL, summarizeCookieInventory(cookies))
			}
		}
		if err == nil && hasRequiredSessionCookies(cookies, "") {
			detectedHost := findSessionCookieHost(cookies)
			if detectedHost == "" {
				detectedHost = preferredHost
			}
			detectedBase := baseURL
			detectedHome := homeURL
			if parsed, parseErr := url.Parse(strings.TrimSpace(lastURL)); parseErr == nil && parsed.Scheme != "" && parsed.Host != "" {
				detectedBase = fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
				path := "/" + strings.Trim(strings.TrimSpace(parsed.Path), "/")
				if path != "/" {
					detectedHome = detectedBase + path
				}
			}
			return sessionWaitResult{
				Host:    detectedHost,
				BaseURL: detectedBase,
				HomeURL: detectedHome,
			}, nil
		}

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return sessionWaitResult{}, fmt.Errorf(
					"timed out waiting for OnePoint session cookies; finish login in browser and retry (or increase --timeout). last URL: %s",
					lastURL,
				)
			}
			return sessionWaitResult{}, fmt.Errorf("waiting for OnePoint login interrupted: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func hasRequiredSessionCookies(cookies []*network.Cookie, host string) bool {
	var hasJSessionID bool
	filterByHost := strings.TrimSpace(host) != ""
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		if filterByHost && !onepointCookieDomainMatches(cookie.Domain, host) {
			continue
		}
		if cookie.Name == onepoint.SessionCookieJSESSIONID && strings.TrimSpace(cookie.Value) != "" {
			hasJSessionID = true
		}
	}
	return hasJSessionID
}

func findSessionCookieHost(cookies []*network.Cookie) string {
	jsByHost := map[string]struct{}{}
	wlByHost := map[string]struct{}{}

	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		host := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(cookie.Domain)), ".")
		if host == "" {
			continue
		}
		switch cookie.Name {
		case onepoint.SessionCookieJSESSIONID:
			if strings.TrimSpace(cookie.Value) != "" {
				jsByHost[host] = struct{}{}
			}
		case onepoint.SessionCookieWLAuthSession:
			if strings.TrimSpace(cookie.Value) != "" {
				wlByHost[host] = struct{}{}
			}
		}
	}

	for host := range jsByHost {
		if _, ok := wlByHost[host]; ok {
			return host
		}
	}
	for host := range jsByHost {
		return host
	}
	return ""
}

func getBrowserCookies(ctx context.Context) ([]*network.Cookie, error) {
	chromeCtx := chromedp.FromContext(ctx)
	if chromeCtx == nil || chromeCtx.Browser == nil {
		return nil, errors.New("browser context not available for cookie read")
	}
	browserExecutorCtx := cdp.WithExecutor(ctx, chromeCtx.Browser)
	return storage.GetCookies().Do(browserExecutorCtx)
}

func summarizeCookieInventory(cookies []*network.Cookie) string {
	if len(cookies) == 0 {
		return "cookies=0"
	}

	byDomain := make(map[string][]string)
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		domain := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(cookie.Domain)), ".")
		if domain == "" {
			domain = "<empty-domain>"
		}
		name := strings.TrimSpace(cookie.Name)
		if name == "" {
			continue
		}
		byDomain[domain] = append(byDomain[domain], name)
	}

	domains := make([]string, 0, len(byDomain))
	for domain := range byDomain {
		domains = append(domains, domain)
	}
	sort.Strings(domains)

	parts := make([]string, 0, len(domains))
	for _, domain := range domains {
		names := byDomain[domain]
		sort.Strings(names)
		names = uniqueStrings(names)
		parts = append(parts, fmt.Sprintf("%s=[%s]", domain, strings.Join(names, ",")))
	}

	return fmt.Sprintf("cookies=%d domains{%s}", len(cookies), strings.Join(parts, "; "))
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := make([]string, 0, len(values))
	last := ""
	for i, value := range values {
		if i == 0 || value != last {
			out = append(out, value)
		}
		last = value
	}
	return out
}

func filterCookiesForHost(cookies []*network.Cookie, host string) []authStateCookie {
	out := make([]authStateCookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		if !onepointCookieDomainMatches(cookie.Domain, host) {
			continue
		}
		out = append(out, authStateCookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Expires:  float64(cookie.Expires),
			HTTPOnly: cookie.HTTPOnly,
			Secure:   cookie.Secure,
			SameSite: cookie.SameSite.String(),
		})
	}
	return out
}

func onepointCookieDomainMatches(cookieDomain, targetHost string) bool {
	cookieDomain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(cookieDomain)), ".")
	targetHost = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(targetHost)), ".")
	return cookieDomain == targetHost || strings.HasSuffix(targetHost, "."+cookieDomain)
}

func init() {
	authCmd.AddCommand(authLoginCmd)

	authLoginCmd.Flags().StringVar(&authLoginURL, "url", "", "Override OnePoint URL from config (full home URL)")
	authLoginCmd.Flags().StringVar(&authLoginStateFile, "state-file", "", "Path to save auth state JSON (default: $HOME/.gohour/onepoint-auth-state.json)")
	authLoginCmd.Flags().StringVar(&authLoginProfileDir, "profile-dir", "", "Browser profile directory (optional; default is a fresh temporary profile per run)")
	authLoginCmd.Flags().StringVar(&authLoginBrowserBin, "browser-bin", "", "Optional browser binary path (Chrome/Chromium)")
	authLoginCmd.Flags().DurationVar(&authLoginTimeout, "timeout", 10*time.Minute, "Maximum wait time for successful browser login")
	authLoginCmd.Flags().BoolVar(&authLoginDebugCookies, "debug-cookies", false, "Print cookie names/domains while waiting for login detection")
	authLoginCmd.Flags().BoolVar(&authLoginSkipVerify, "skip-verify", false, "Skip OnePoint API verification after saving auth state")
}
