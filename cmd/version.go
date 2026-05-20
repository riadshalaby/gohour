package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is updated by release-please when a release PR is merged.
// The line below carries the release-please annotation; do not edit by hand
// outside of release-please PRs. Local development builds keep the source-tree
// version literal.
const Version = "0.4.2" // x-release-please-version

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the gohour version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gohour %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
