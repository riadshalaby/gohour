package output

import (
	"fmt"
	"gohour/worklog"
	"strings"
)

type Writer interface {
	Write(path string, entries []worklog.Entry) error
}

func WriterForFormat(format string) (Writer, error) {
	switch normalizeFormat(format) {
	case "csv":
		return &CSVWriter{}, nil
	case "excel", "xlsx":
		return &ExcelWriter{}, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %s", format)
	}
}

func normalizeFormat(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}
