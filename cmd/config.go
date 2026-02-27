package cmd

import "github.com/spf13/cobra"

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage gohour configuration file values.",
	Long: `Create, edit, display, and delete the gohour configuration file.

The configuration stores application-wide values and EPM import rules:
- onepoint.url
- import.auto_reconcile_after_import
- epm.rules[].file_template / project_id+project / activity_id+activity / skill_id+skill`,
	Example: `
  # Create default config in $HOME/.gohour.yaml
  gohour config create

  # Show active config and source file
  gohour config show

  # Open active config in editor (creates example if missing)
  gohour config edit

  # Add one EPM rule interactively from OnePoint lookups
  gohour config rule add

  # Delete active config file
  gohour config delete
`,
}

func init() {
	rootCmd.AddCommand(configCmd)
}
