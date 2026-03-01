package cmd

import (
	"fmt"

	"gohour/onepoint"

	"github.com/spf13/cobra"
)

var (
	authShowCookiesStateFile string
	authShowCookiesURL       string
)

var authShowCookiesCmd = &cobra.Command{
	Use:   "show-cookies",
	Short: "Print session cookies as HTTP Cookie header.",
	Long: `Read auth state JSON and print the cookie header required by OnePoint REST endpoints.

Output format:
JSESSIONID=<...>; _WL_AUTHCOOKIE_JSESSIONID=<...>`,
	Example: `
  # Print cookie header from default auth state file
  gohour auth show-cookies
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stateFile, err := resolveDefaultAuthStatePath(authShowCookiesStateFile)
		if err != nil {
			return err
		}

		_, _, host, err := resolveOnePointURLs(authShowCookiesURL)
		if err != nil {
			return err
		}

		header, err := onepoint.SessionCookieHeaderFromStateFile(stateFile, host)
		if err != nil {
			return err
		}
		fmt.Println(header)
		return nil
	},
}

func init() {
	authCmd.AddCommand(authShowCookiesCmd)

	authShowCookiesCmd.Flags().StringVar(&authShowCookiesStateFile, "state-file", "", "Path to auth state JSON (default: $HOME/.gohour/onepoint-auth-state.json)")
	authShowCookiesCmd.Flags().StringVar(&authShowCookiesURL, "url", "", "Override OnePoint URL from config (full home URL)")
}
