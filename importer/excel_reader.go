package importer

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

type ExcelReader struct{}

func (r *ExcelReader) Read(path string) ([]Record, error) {
	file, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open excel file %s: %w", path, err)
	}
	defer file.Close()

	sheetName := file.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("excel file has no sheets: %s", path)
	}

	rows, err := file.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("read rows from sheet %s: %w", sheetName, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("sheet %s is empty", sheetName)
	}

	headers := rows[0]
	normalizedHeaders := make([]string, len(headers))
	for i, header := range headers {
		normalizedHeaders[i] = normalizeHeader(header)
	}

	records := make([]Record, 0, len(rows)-1)
	for i, row := range rows[1:] {
		values := make(map[string]string, len(normalizedHeaders))
		for col := range normalizedHeaders {
			if col < len(row) {
				values[normalizedHeaders[col]] = row[col]
			} else {
				values[normalizedHeaders[col]] = ""
			}
		}

		records = append(records, Record{RowNumber: i + 2, Values: values})
	}

	return records, nil
}
