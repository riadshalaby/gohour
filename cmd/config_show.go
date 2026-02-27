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

  # Show configuration from a specific file
  gohour --configFile ./custom-gohour.yaml config show
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
			fmt.Printf("epm.rules: %d\n", len(cfg.EPM.Rules))
			for i, rule := range cfg.EPM.Rules {
				fmt.Printf("epm.rules[%d].name: %s\n", i, rule.Name)
				fmt.Printf("epm.rules[%d].file_template: %s\n", i, rule.FileTemplate)
				fmt.Printf("epm.rules[%d].project_id: %d\n", i, rule.ProjectID)
				fmt.Printf("epm.rules[%d].project: %s\n", i, rule.Project)
				fmt.Printf("epm.rules[%d].activity_id: %d\n", i, rule.ActivityID)
				fmt.Printf("epm.rules[%d].activity: %s\n", i, rule.Activity)
				fmt.Printf("epm.rules[%d].skill_id: %d\n", i, rule.SkillID)
				fmt.Printf("epm.rules[%d].skill: %s\n", i, rule.Skill)
			}
		}

	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
}
