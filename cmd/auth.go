package cmd

import "github.com/spf13/cobra"

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate against OnePoint via interactive browser login.",
	Long: `Authentication helpers for Microsoft SSO + OnePoint session cookies.

Use "auth login" to perform an interactive browser login and save auth state.
Use "auth show-cookies" to print the Cookie header for direct REST calls.`,
}

func init() {
	rootCmd.AddCommand(authCmd)
}
