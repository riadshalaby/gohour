package output

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
)

func writeDailySummariesCSV(path string, summaries []DailySummary) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv output %s: %w", path, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	headers := []string{"Date", "StartTime", "EndTime", "WorkedHours", "BillableHours", "BreakHours", "WorklogCount"}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("write csv headers: %w", err)
	}

	for _, summary := range summaries {
		row := []string{
			summary.Date,
			summary.StartDateTime.Format("15:04"),
			summary.EndDateTime.Format("15:04"),
			fmt.Sprintf("%.2f", summary.WorkedHours),
			fmt.Sprintf("%.2f", summary.BillableHours),
			fmt.Sprintf("%.2f", summary.BreakHours),
			strconv.Itoa(summary.WorklogCount),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}

	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush csv output: %w", err)
	}

	return nil
}
