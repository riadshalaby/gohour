package output

import (
	"encoding/csv"
	"fmt"
	"gohour/worklog"
	"os"
	"strconv"
	"time"
)

type CSVWriter struct{}

func (w *CSVWriter) Write(path string, entries []worklog.Entry) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv output %s: %w", path, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	headers := []string{"StartDateTime", "EndDateTime", "Billable", "Description", "Project", "Activity", "Skill", "SourceFormat", "SourceMapper", "SourceFile"}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("write csv headers: %w", err)
	}

	for _, entry := range entries {
		row := []string{
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
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}

	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush csv output: %w", err)
	}

	return nil
}
