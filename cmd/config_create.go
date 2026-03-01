package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a configuration file from the example template.",
	Long: `Create a new configuration file from the same example template used by "config edit".

If a configuration file is already in use, no new file is written.`,
	Example: `
  # Create default config at $HOME/.gohour.yaml
  gohour config create
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return saveDefaultConfig()
	},
}

func saveDefaultConfig() error {
	configPath, err := resolveConfigEditPath(cfgFile, viper.ConfigFileUsed())
	if err != nil {
		return err
	}

	created, err := ensureConfigFileWithTemplate(configPath)
	if err != nil {
		return err
	}

	if created {
		fmt.Printf("New config file created at: %s\n", configPath)
		return nil
	}

	fmt.Printf("Config file already exists at: %s\n", configPath)
	return nil
}

func init() {
	configCmd.AddCommand(configCreateCmd)
}
