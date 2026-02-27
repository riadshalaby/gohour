package cmd

import (
	"fmt"
	"gohour/config"
	"gohour/importer"
	"gohour/reconcile"
	"gohour/storage"
	"gohour/worklog"
	"strings"

	"github.com/spf13/cobra"
)

var (
	importInputs        []string
	importFormat        string
	importMapper        string
	importDBPath        string
	importProject       string
	importActivity      string
	importSkill         string
	importReconcileMode string
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import CSV/Excel worklogs into a local SQLite database",
	Long: `Read source files, normalize each row via the selected mapper, and persist results in SQLite.

Use mapper "epm" for EPM-style Excel exports and mapper "generic" for structured CSV/Excel inputs.
When --format is omitted, format is inferred from each input file extension.

Mapper selection per input file:
- if a rule matches by file_template, that rule's mapper is used
- otherwise the CLI fallback mapper (--mapper) is used.

For EPM-mapped files, project/activity/skill must be provided by either:
- matching rules in configuration via file_template, or
- explicit --project/--activity/--skill flags.
If neither provides all values, import fails.`,
	Example: `
  # Import multiple EPM Excel files
  gohour import -i EPMExportRZ202601.xlsx -i EPMExportSZ202601.xlsx --mapper epm --db ./gohour.db

  # Import generic CSV file
  gohour import -i examples/generic_import_example.csv --format csv --mapper generic --db ./gohour.db

  # Override EPM project/activity/skill explicitly
  gohour import -i EPMExportRZ202601.xlsx --mapper epm --project "My RZ Project" --activity "Delivery" --skill "Go" --db ./gohour.db

  # Explicitly enable reconcile after import
  gohour import -i ./source.csv --format csv --mapper generic --reconcile on --db ./gohour.db

  # Explicitly disable reconcile for this run
  gohour import -i ./source.xlsx --mapper epm --reconcile off --db ./gohour.db

  # Import with custom config file
  gohour --configFile ./custom-gohour.yaml import -i ./source.xlsx --mapper epm --db ./gohour.db
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadAndValidate()
		if err != nil {
			return err
		}

		result := &importer.Result{Entries: make([]worklog.Entry, 0, 256)}
		runOptions := importer.RunOptions{
			EPMProject:  importProject,
			EPMActivity: importActivity,
			EPMSkill:    importSkill,
		}
		defaultMapper := strings.TrimSpace(importMapper)
		for _, path := range importInputs {
			mapperName := resolveMapperNameForFile(path, defaultMapper, cfg.Rules)
			mapper, mapErr := importer.MapperByName(mapperName)
			if mapErr != nil {
				return mapErr
			}

			fileResult, runErr := importer.Run([]string{path}, importFormat, mapper, *cfg, runOptions)
			if runErr != nil {
				return runErr
			}

			result.FilesProcessed += fileResult.FilesProcessed
			result.RowsRead += fileResult.RowsRead
			result.RowsMapped += fileResult.RowsMapped
			result.RowsSkipped += fileResult.RowsSkipped
			result.Entries = append(result.Entries, fileResult.Entries...)
		}

		store, err := storage.OpenSQLite(importDBPath)
		if err != nil {
			return err
		}
		defer store.Close()

		inserted, err := store.InsertWorklogs(result.Entries)
		if err != nil {
			return err
		}

		fmt.Printf("Import completed. Files: %d, Rows read: %d, Rows mapped: %d, Rows skipped: %d, Rows persisted: %d\n",
			result.FilesProcessed,
			result.RowsRead,
			result.RowsMapped,
			result.RowsSkipped,
			inserted,
		)

		shouldReconcile, err := resolveReconcileMode(importReconcileMode, cfg.Import.AutoReconcileAfterImport)
		if err != nil {
			return err
		}
		if shouldReconcile {
			reconcileResult, err := reconcile.Run(store)
			if err != nil {
				return err
			}
			fmt.Printf(
				"Auto-reconcile completed. Days processed: %d, Overlaps before: %d, Overlaps after: %d, EPM entries adjusted: %d, Rows updated: %d\n",
				reconcileResult.DaysProcessed,
				reconcileResult.OverlapsBefore,
				reconcileResult.OverlapsAfter,
				reconcileResult.EPMEntriesAdjusted,
				reconcileResult.RowsUpdated,
			)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().StringArrayVarP(&importInputs, "input", "i", nil, "Input file path (repeatable)")
	importCmd.Flags().StringVarP(&importFormat, "format", "f", "", "Input format: csv|excel (optional, inferred from extension when omitted)")
	importCmd.Flags().StringVarP(&importMapper, "mapper", "m", "epm", "Fallback mapper when no rule matches a file: epm|generic")
	importCmd.Flags().StringVar(&importProject, "project", "", "Explicit project value for EPM imports (overrides matching config rule)")
	importCmd.Flags().StringVar(&importActivity, "activity", "", "Explicit activity value for EPM imports (overrides matching config rule)")
	importCmd.Flags().StringVar(&importSkill, "skill", "", "Explicit skill value for EPM imports (overrides matching config rule)")
	importCmd.Flags().StringVar(&importDBPath, "db", "./gohour.db", "Path to local SQLite database")
	importCmd.Flags().StringVar(&importReconcileMode, "reconcile", "auto", "Reconcile mode after import: auto|on|off")

	_ = importCmd.MarkFlagRequired("input")
}

func resolveReconcileMode(mode string, configDefault bool) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "auto":
		return configDefault, nil
	case "on", "true", "yes":
		return true, nil
	case "off", "false", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid reconcile mode %q (supported: auto|on|off)", mode)
	}
}

func resolveMapperNameForFile(path, fallbackMapper string, rules []config.Rule) string {
	rule := importer.MatchRuleByTemplate(path, rules)
	if mapper := strings.TrimSpace(rule.Mapper); mapper != "" {
		return mapper
	}
	return strings.TrimSpace(fallbackMapper)
}
