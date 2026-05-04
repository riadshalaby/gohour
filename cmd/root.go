/*
Copyright © 2025 riad@rsworld.eu

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"github.com/spf13/viper"
	"os"

	"github.com/riadshalaby/gohour/config"
	"github.com/spf13/cobra"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gohour",
	Short: "Import, reconcile, submit, and export worklogs from multiple source formats.",
	Long: `
**********************************************
*              GO HOUR GO                    *
**********************************************

This CLI imports source files (Excel, CSV), normalizes records into a local SQLite database,
exports normalized worklogs to CSV or Excel, and can submit local worklogs to OnePoint.

Supported input formats:
- Excel: .xlsx, .xlsm, .xls
- CSV: .csv
`,
	Example: `
  # Create configuration file
  gohour config create

  # Import source files
  gohour import -i EPMExportRZ202601.xlsx -i EPMExportSZ202601.xlsx

  # Reconcile simulated EPM timings against all other sources
  gohour reconcile

  # Submit local worklogs to OnePoint
  gohour submit

  # Export rows
  gohour export --output ./worklogs.csv
`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	config.SetDefaults()

	// Optional: Validate configuration
	rootCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if !requiresConfig(cmd) {
			return nil
		}

		_, err := config.LoadAndValidate()
		return err
	}
}

func requiresConfig(cmd *cobra.Command) bool {
	return cmd != nil && cmd.Name() == "import"
}

// initConfig migrates legacy paths once, then reads the fixed config file.
func initConfig() {
	cobra.CheckErr(config.RunMigration())

	viper.SetConfigFile(config.ConfigPath())
	viper.SetConfigType("yaml")
	viper.AutomaticEnv() // read in environment variables that match

	if err := viper.ReadInConfig(); err != nil {
		cobra.CheckErr(err)
	}
}
