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
	"fmt"
	"github.com/spf13/viper"
	"os"

	"github.com/spf13/cobra"
	"gohour/config"
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

  # Import EPM Excel exports
  gohour import -i EPMExportRZ202601.xlsx -i EPMExportSZ202601.xlsx --mapper epm

  # Import generic CSV source
  gohour import -i examples/generic_import_example.csv --format csv --mapper generic

  # Reconcile simulated EPM timings against all other sources
  gohour reconcile

  # Preview submit against remote OnePoint entries (no writes)
  gohour submit --dry-run

  # Submit local worklogs to OnePoint
  gohour submit

  # Export raw rows
  gohour export --mode raw --output ./worklogs.csv

  # Export daily summary
  gohour export --mode daily --output ./daily-summary.csv
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

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "configFile", "", "Config file override (default discovery: $HOME/.gohour.yaml, then ./.gohour.yaml)")

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

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".gohour" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".gohour")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Fprintln(os.Stderr, "No config file found. Create one first with: gohour config create")
	}
}
