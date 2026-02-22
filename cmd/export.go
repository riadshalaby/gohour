package cmd

import (
	"fmt"
	"gohour/output"
	"gohour/storage"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	exportFormat string
	exportMode   string
	exportOutput string
	exportDBPath string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export normalized worklogs from SQLite to CSV/Excel",
	Long: `Export normalized worklogs from SQLite.

Modes:
- raw: export each normalized worklog row
- daily: export per-day aggregates (start/end, worked hours, billable hours, break hours)

Output format can be selected explicitly via --format or inferred from --output extension.`,
	Example: `
  # Export raw rows to CSV
  gohour export --mode raw --db ./gohour.db --output ./worklogs.csv

  # Export raw rows to Excel
  gohour export --mode raw --db ./gohour.db --output ./worklogs.xlsx

  # Export daily summary to CSV
  gohour export --mode daily --db ./gohour.db --output ./daily-summary.csv

  # Force Excel format independent of extension
  gohour export --mode daily --format excel --db ./gohour.db --output ./daily-summary.out
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format := exportFormat
		if strings.TrimSpace(format) == "" {
			format = detectExportFormat(exportOutput)
		}

		store, err := storage.OpenSQLite(exportDBPath)
		if err != nil {
			return err
		}
		defer store.Close()

		entries, err := store.ListWorklogs()
		if err != nil {
			return err
		}

		mode := strings.TrimSpace(strings.ToLower(exportMode))
		switch mode {
		case "", "raw":
			writer, writerErr := output.WriterForFormat(format)
			if writerErr != nil {
				return writerErr
			}
			if err := writer.Write(exportOutput, entries); err != nil {
				return err
			}
			fmt.Printf("Export completed. Rows: %d, Mode: raw, Format: %s, File: %s\n", len(entries), format, exportOutput)
		case "daily":
			summaries := output.BuildDailySummaries(entries)
			if err := output.WriteDailySummaries(exportOutput, format, summaries); err != nil {
				return err
			}
			fmt.Printf("Export completed. Days: %d, Mode: daily, Format: %s, File: %s\n", len(summaries), format, exportOutput)
		default:
			return fmt.Errorf("unsupported export mode: %s (supported: raw, daily)", exportMode)
		}
		return nil
	},
}

func detectExportFormat(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	switch ext {
	case "csv":
		return "csv"
	case "xlsx", "xlsm", "xls":
		return "excel"
	default:
		return "csv"
	}
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.Flags().StringVar(&exportMode, "mode", "raw", "Export mode: raw|daily")
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "", "Output format: csv|excel (optional, inferred from output extension)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file path")
	exportCmd.Flags().StringVar(&exportDBPath, "db", "./gohour.db", "Path to local SQLite database")

	_ = exportCmd.MarkFlagRequired("output")
}
