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
	"os"

	"github.com/riadshalaby/gohour/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gohour",
	Short: "Run the local gohour web UI for time-tracking review and submission.",
	Long: `
**********************************************
*              GO HOUR GO                    *
**********************************************

gohour stores its local data under ~/.gohour and provides a browser UI for importing,
editing, comparing, and submitting worklogs.
`,
	Example: `
  # Start the local web UI
  gohour serve

  # Print the installed version
  gohour version
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
	config.SetDefaults()
}

// initConfig migrates legacy paths once, then reads the fixed config file.
func initConfig() error {
	if err := config.RunMigration(); err != nil {
		return err
	}

	viper.SetConfigFile(config.ConfigPath())
	viper.SetConfigType("yaml")
	viper.AutomaticEnv() // read in environment variables that match

	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	return nil
}
