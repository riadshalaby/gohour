package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var configDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the active configuration file.",
	Long: `Delete the configuration file currently selected by gohour.

If no configuration file is active, the command returns an error.`,
	Example: `
  # Delete active config
  gohour config delete

  # Delete config at a custom path
  gohour --configFile ./custom-gohour.yaml config delete
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := viper.ConfigFileUsed()
		if configPath == "" {
			return fmt.Errorf("no configuration file found")
		}

		if err := os.Remove(configPath); err != nil {
			return fmt.Errorf("error deleting configuration file: %w", err)
		}

		fmt.Printf("Configuration file successfully deleted: %s\n", configPath)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configDeleteCmd)
}
