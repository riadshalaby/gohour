package cmd

import (
	"fmt"
	"github.com/spf13/viper"

	"github.com/spf13/cobra"
	"gohour/config"
)

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show active configuration values.",
	Long: `Display the currently loaded configuration and the resolved config file path.

This command validates the configuration before printing values.`,
	Example: `
  # Show active configuration
  gohour config show
`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadAndValidate()
		if err != nil {
			fmt.Println("Invalid config:", err)
			return
		}

		if configPath := viper.ConfigFileUsed(); configPath != "" {
			fmt.Println("Config file loaded from:", viper.ConfigFileUsed())
			fmt.Println("Configuration:")
			fmt.Printf("onepoint.url: %s\n", cfg.OnePoint.URL)
			fmt.Printf("import.auto_reconcile_after_import: %t\n", cfg.Import.AutoReconcileAfterImport)
			fmt.Printf("rules: %d\n", len(cfg.Rules))
			for i, rule := range cfg.Rules {
				fmt.Printf("rules[%d].name: %s\n", i, rule.Name)
				fmt.Printf("rules[%d].mapper: %s\n", i, rule.Mapper)
				fmt.Printf("rules[%d].file_template: %s\n", i, rule.FileTemplate)
				fmt.Printf("rules[%d].project_id: %d\n", i, rule.ProjectID)
				fmt.Printf("rules[%d].project: %s\n", i, rule.Project)
				fmt.Printf("rules[%d].activity_id: %d\n", i, rule.ActivityID)
				fmt.Printf("rules[%d].activity: %s\n", i, rule.Activity)
				fmt.Printf("rules[%d].skill_id: %d\n", i, rule.SkillID)
				fmt.Printf("rules[%d].skill: %s\n", i, rule.Skill)
				billableStr := "true (default)"
				if rule.Billable != nil {
					billableStr = fmt.Sprintf("%t", *rule.Billable)
				}
				fmt.Printf("rules[%d].billable: %s\n", i, billableStr)
			}
		}

	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
}
