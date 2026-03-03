package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via:
// go build -ldflags "-X gohour/cmd.Version=v0.2.2"
var Version = "dev"

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
