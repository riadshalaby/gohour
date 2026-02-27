package cmd

import "github.com/spf13/cobra"

var configRuleCmd = &cobra.Command{
	Use:   "rule",
	Short: "Manage EPM mapping rules in config.",
	Long: `Manage EPM import rules stored under config key epm.rules.

Rules map imported EPM files (via file template) to target project/activity/skill.`,
}

func init() {
	configCmd.AddCommand(configRuleCmd)
}
