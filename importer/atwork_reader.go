package importer

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// ATWorkReader reads UTF-16LE tab-separated exports from the atwork time-tracking app.
// The file contains multiple sections; only the "Einträge" (entries) section is parsed.
// Parsing stops when the first column reads "Gesamt" or is empty.
type ATWorkReader struct{}

func (r *ATWorkReader) Read(path string) ([]Record, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open atwork file %s: %w", path, err)
	}
	defer file.Close()

	// Decode UTF-16 (with BOM detection) into UTF-8.
	decoder := unicode.BOMOverride(unicode.UTF8.NewDecoder())
	utf8Reader := transform.NewReader(file, decoder)

	csvReader := csv.NewReader(utf8Reader)
	csvReader.Comma = '\t'
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true

	// Skip the first row — section title (e.g. "Einträge").
	if _, err := csvReader.Read(); err != nil {
		return nil, fmt.Errorf("read atwork section header: %w", err)
	}

	// Second row is the column header row.
	headers, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read atwork column headers: %w", err)
	}

	normalizedHeaders := make([]string, len(headers))
	for i, header := range headers {
		normalizedHeaders[i] = normalizeHeader(header)
	}

	records := make([]Record, 0, 64)
	rowNumber := 2 // headers are row 2 in the file
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read atwork row %d: %w", rowNumber+1, err)
		}
		rowNumber++

		// Stop at summary rows: first column is "Gesamt" or empty.
		if len(row) == 0 {
			break
		}
		first := strings.TrimSpace(row[0])
		if first == "" || strings.EqualFold(first, "Gesamt") {
			break
		}

		values := make(map[string]string, len(normalizedHeaders))
		for i := range normalizedHeaders {
			if i < len(row) {
				values[normalizedHeaders[i]] = row[i]
			} else {
				values[normalizedHeaders[i]] = ""
			}
		}

		records = append(records, Record{RowNumber: rowNumber, Values: values})
	}

	return records, nil
}
