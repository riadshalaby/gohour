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
	"syscall"
	"time"

	"github.com/riadshalaby/gohour/config"
	"github.com/riadshalaby/gohour/storage"
	"github.com/riadshalaby/gohour/web"

	"github.com/spf13/cobra"
)

var (
	servePort   int
	serveNoOpen bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the local interactive web UI",
	Long: `Start a localhost HTTP server backed by the fixed gohour data directory.

The UI reads ~/.gohour/config.yaml, stores local worklogs in ~/.gohour/gohour.db, and
uses ~/.gohour/onepoint-auth-state.json for OnePoint browser-login state. It supports
month/day review, local import/edit/delete actions, remote refresh, and day/month submit.`,
	Example: `
  # Start the local web UI on the default port
  gohour serve

	# Start on a custom port
  gohour serve --port 9090
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initConfig(); err != nil {
			return err
		}

		cfg, err := config.LoadAndValidate()
		if err != nil {
			return err
		}

		bounds := defaultServeMonthBounds()

		store, err := storage.OpenSQLite(config.DBPath())
		if err != nil {
			return err
		}
		defer store.Close()

		client, err := buildServeClient(*cfg)
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
	defaultMonth string
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().IntVar(&servePort, "port", 8080, "HTTP port for the local web server")
	serveCmd.Flags().BoolVar(&serveNoOpen, "no-open", false, "Do not open browser automatically")
}

func defaultServeMonthBounds() serveMonthBounds {
	nowMonth := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local)
	return serveMonthBounds{defaultMonth: nowMonth.Format("2006-01")}
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
