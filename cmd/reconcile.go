package cmd

import (
	"fmt"
	"gohour/reconcile"
	"gohour/storage"

	"github.com/spf13/cobra"
)

var reconcileDBPath string

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Verify and correct overlapping EPM worklogs after mixed-source imports",
	Long: `Verify and correct time overlaps for EPM-derived worklogs.

Why this exists:
- EPM imports simulate per-task times within a day window.
- Additional imports from other sources can introduce overlaps.

This command adjusts EPM rows only, so one resource is not assigned to overlapping work at the same time.`,
	Example: `
  # Reconcile overlaps
  gohour reconcile

  # Typical workflow: import, reconcile, export
  gohour import -i EPMExportRZ202601.xlsx
  gohour reconcile
  gohour export --output ./worklogs.csv
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storage.OpenSQLite(reconcileDBPath)
		if err != nil {
			return err
		}
		defer store.Close()

		result, err := reconcile.Run(store)
		if err != nil {
			return err
		}

		fmt.Printf(
			"Reconcile completed. Days processed: %d, Overlaps before: %d, Overlaps after: %d, EPM entries adjusted: %d, Rows updated: %d\n",
			result.DaysProcessed,
			result.OverlapsBefore,
			result.OverlapsAfter,
			result.EPMEntriesAdjusted,
			result.RowsUpdated,
		)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(reconcileCmd)

	reconcileCmd.Flags().StringVar(&reconcileDBPath, "db", "./gohour.db", "Path to local SQLite database")
}
