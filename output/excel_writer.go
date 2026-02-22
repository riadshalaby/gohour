package output

import (
	"fmt"
	"gohour/worklog"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"
)

type ExcelWriter struct{}

func (w *ExcelWriter) Write(path string, entries []worklog.Entry) error {
	file := excelize.NewFile()
	defer file.Close()

	sheet := file.GetSheetName(0)
	headers := []string{"StartDateTime", "EndDateTime", "Billable", "Description", "Project", "Activity", "Skill", "SourceFormat", "SourceMapper", "SourceFile"}

	for col, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			return fmt.Errorf("set excel header %s: %w", cell, err)
		}
	}

	for i, entry := range entries {
		row := i + 2
		values := []string{
			entry.StartDateTime.Format(time.RFC3339),
			entry.EndDateTime.Format(time.RFC3339),
			strconv.Itoa(entry.Billable),
			entry.Description,
			entry.Project,
			entry.Activity,
			entry.Skill,
			entry.SourceFormat,
			entry.SourceMapper,
			entry.SourceFile,
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
