package importer

import "fmt"

type Reader interface {
	Read(path string) ([]Record, error)
}

func ReaderForFormat(format string) (Reader, error) {
	switch normalizeHeader(format) {
	case "csv":
		return &CSVReader{}, nil
	case "excel", "xlsx", "xlsm", "xls":
		return &ExcelReader{}, nil
	default:
		return nil, fmt.Errorf("unsupported input format: %s", format)
	}
}
