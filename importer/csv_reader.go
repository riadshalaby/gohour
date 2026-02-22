package importer

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
)

type CSVReader struct{}

func (r *CSVReader) Read(path string) ([]Record, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv file %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}

	normalizedHeaders := make([]string, len(headers))
	for i, header := range headers {
		normalizedHeaders[i] = normalizeHeader(header)
	}

	records := make([]Record, 0, 128)
	rowNumber := 1
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv row %d: %w", rowNumber+1, err)
		}

		values := make(map[string]string, len(normalizedHeaders))
		for i := range normalizedHeaders {
			if i < len(row) {
				values[normalizedHeaders[i]] = row[i]
			} else {
				values[normalizedHeaders[i]] = ""
			}
		}

		records = append(records, Record{RowNumber: rowNumber + 1, Values: values})
		rowNumber++
	}

	return records, nil
}
