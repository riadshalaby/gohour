package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"gohour/config"
	"gohour/onepoint"
	"gohour/storage"
	"gohour/web"

	"github.com/spf13/cobra"
)

var (
	servePort      int
	serveDBPath    string
	serveURL       string
	serveStateFile string
	serveFromMonth string
	serveToMonth   string
	serveNoOpen    bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start local read-only web UI for OnePoint/local worklog comparison",
	Long: `Start a local HTTP server with monthly and daily overview pages.

The UI is read-only and compares local SQLite entries against current OnePoint entries.`,
	Example: `
  # Start local server on default port
  gohour serve

  # Start with explicit db/url/auth-state and custom port
  gohour serve --port 9090 --db ./gohour.db --url https://onepoint.virtual7.io/onepoint/faces/home --state-file ~/.gohour/onepoint-auth-state.json
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadAndValidate()
		if err != nil {
			return err
		}

		bounds, err := parseServeMonthBounds(serveFromMonth, serveToMonth)
		if err != nil {
			return err
		}

		baseURL, homeURL, host, err := resolveOnePointURLs(serveURL)
		if err != nil {
			return err
		}

		stateFile, err := resolveDefaultAuthStatePath(serveStateFile)
		if err != nil {
			return err
		}
		cookieHeader, err := onepoint.SessionCookieHeaderFromStateFile(stateFile, host)
		if err != nil {
			return fmt.Errorf("extract session cookies: %w", err)
		}

		store, err := storage.OpenSQLite(serveDBPath)
		if err != nil {
			return err
		}
		defer store.Close()

		client, err := onepoint.NewClient(onepoint.ClientConfig{
			BaseURL:        baseURL,
			RefererURL:     homeURL,
			SessionCookies: cookieHeader,
			UserAgent:      "gohour-serve/1.0",
		})
		if err != nil {
			return err
		}

		addr := fmt.Sprintf(":%d", servePort)
		server := &http.Server{
			Addr:    addr,
			Handler: withServeMonthRedirect(web.NewServer(store, client, *cfg), bounds),
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- server.ListenAndServe()
		}()

		listenURL := fmt.Sprintf("http://localhost:%d", servePort)
		fmt.Printf("Listening on %s\n", listenURL)
		if !serveNoOpen {
			target := listenURL
			if bounds.defaultMonth != "" {
				target = target + "/month/" + bounds.defaultMonth
			}
			if openErr := openURLInBrowser(target); openErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to open browser: %v\n", openErr)
			}
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigCh)

		select {
		case err := <-errCh:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		case <-sigCh:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				return fmt.Errorf("shutdown server: %w", err)
			}
			err := <-errCh
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		}
	},
}

type serveMonthBounds struct {
	from         *time.Time
	to           *time.Time
	defaultMonth string
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().IntVar(&servePort, "port", 8080, "HTTP port for the local web server")
	serveCmd.Flags().StringVar(&serveDBPath, "db", "./gohour.db", "Path to local SQLite database")
	serveCmd.Flags().StringVar(&serveURL, "url", "", "Override OnePoint URL from config (full home URL)")
	serveCmd.Flags().StringVar(&serveStateFile, "state-file", "", "Path to auth state JSON (default: $HOME/.gohour/onepoint-auth-state.json)")
	serveCmd.Flags().StringVar(&serveFromMonth, "from", "", "Preferred start month for initial view, format YYYY-MM")
	serveCmd.Flags().StringVar(&serveToMonth, "to", "", "Preferred end month for initial view, format YYYY-MM")
	serveCmd.Flags().BoolVar(&serveNoOpen, "no-open", false, "Do not open browser automatically")
}

func parseServeMonthBounds(fromValue, toValue string) (serveMonthBounds, error) {
	var out serveMonthBounds

	parse := func(raw string) (*time.Time, error) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, nil
		}
		parsed, err := time.ParseInLocation("2006-01", raw, time.Local)
		if err != nil {
			return nil, fmt.Errorf("invalid month %q (expected YYYY-MM)", raw)
		}
		value := time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.Local)
		return &value, nil
	}

	from, err := parse(fromValue)
	if err != nil {
		return out, fmt.Errorf("invalid --from value: %w", err)
	}
	to, err := parse(toValue)
	if err != nil {
		return out, fmt.Errorf("invalid --to value: %w", err)
	}
	if from != nil && to != nil && from.After(*to) {
		return out, fmt.Errorf("invalid range: --from must be <= --to")
	}

	out.from = from
	out.to = to

	nowMonth := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local)
	switch {
	case from != nil && nowMonth.Before(*from):
		out.defaultMonth = from.Format("2006-01")
	case to != nil && nowMonth.After(*to):
		out.defaultMonth = to.Format("2006-01")
	default:
		out.defaultMonth = nowMonth.Format("2006-01")
	}

	return out, nil
}

func withServeMonthRedirect(next http.Handler, bounds serveMonthBounds) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/" && bounds.defaultMonth != "" {
			http.Redirect(w, r, "/month/"+bounds.defaultMonth, http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func openURLInBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}
