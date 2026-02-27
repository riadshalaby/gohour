package cmd

import "github.com/spf13/cobra"

var configRuleCmd = &cobra.Command{
	Use:   "rule",
	Short: "Manage import mapping rules in config.",
	Long: `Manage import rules stored under config key rules.

Rules map imported files (via mapper + file template) to target project/activity/skill.`,
}

func init() {
	configCmd.AddCommand(configRuleCmd)
}
