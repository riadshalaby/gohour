package output

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

func writeDailySummariesExcel(path string, summaries []DailySummary) error {
	file := excelize.NewFile()
	defer file.Close()

	sheet := file.GetSheetName(0)
	headers := []string{"Date", "StartTime", "EndTime", "WorkedHours", "BillableHours", "BreakHours", "WorklogCount"}

	for col, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			return fmt.Errorf("set excel header %s: %w", cell, err)
		}
	}

	for i, summary := range summaries {
		row := i + 2
		values := []string{
			summary.Date,
			summary.StartDateTime.Format("15:04"),
			summary.EndDateTime.Format("15:04"),
			fmt.Sprintf("%.2f", summary.WorkedHours),
			fmt.Sprintf("%.2f", summary.BillableHours),
			fmt.Sprintf("%.2f", summary.BreakHours),
			fmt.Sprintf("%d", summary.WorklogCount),
		}

		for col, value := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			if err := file.SetCellValue(sheet, cell, value); err != nil {
				return fmt.Errorf("set excel value %s: %w", cell, err)
			}
		}
	}

	if err := file.SaveAs(path); err != nil {
		return fmt.Errorf("save excel output %s: %w", path, err)
	}

	return nil
}
