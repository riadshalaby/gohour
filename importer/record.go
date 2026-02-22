package importer

import (
	"strings"
)

type Record struct {
	RowNumber int
	Values    map[string]string
}

func (r Record) Get(keys ...string) string {
	for _, key := range keys {
		normalized := normalizeHeader(key)
		if value, ok := r.Values[normalized]; ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeHeader(input string) string {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	trimmed = strings.ReplaceAll(trimmed, "_", "")
	trimmed = strings.ReplaceAll(trimmed, "-", "")
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	return trimmed
}
