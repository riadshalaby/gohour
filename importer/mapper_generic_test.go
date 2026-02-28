package importer

import (
	"gohour/config"
	"testing"
	"time"
)

func TestGenericMapper_BillableOverrideUsesMinutes(t *testing.T) {
	t.Parallel()

	mapper := &GenericMapper{}
	record := Record{
		RowNumber: 2,
		Values: map[string]string{
			normalizeHeader("description"):   "Task",
			normalizeHeader("startdatetime"): "2026-03-05 09:00",
			normalizeHeader("enddatetime"):   "2026-03-05 17:00",
			normalizeHeader("billable"):      "8",
		},
	}

	entry, ok, err := mapper.Map(record, config.Config{}, "csv", "source.csv")
	if err != nil {
		t.Fatalf("map record: %v", err)
	}
	if !ok {
		t.Fatalf("expected mapped entry")
	}

	wantEnd := entry.StartDateTime.Add(8 * time.Minute)
	if !entry.EndDateTime.Equal(wantEnd) {
		t.Fatalf("unexpected end time: want %s, got %s", wantEnd, entry.EndDateTime)
	}
	if entry.Billable != 8 {
		t.Fatalf("unexpected billable: want 8, got %d", entry.Billable)
	}
}

func TestGenericMapper_BillableOverrideParsesDecimalMinutes(t *testing.T) {
	t.Parallel()

	mapper := &GenericMapper{}
	record := Record{
		RowNumber: 2,
		Values: map[string]string{
			normalizeHeader("description"):   "Task",
			normalizeHeader("startdatetime"): "2026-03-05 09:00",
			normalizeHeader("enddatetime"):   "2026-03-05 17:00",
			normalizeHeader("billable"):      "7.5",
		},
	}

	entry, ok, err := mapper.Map(record, config.Config{}, "csv", "source.csv")
	if err != nil {
		t.Fatalf("map record: %v", err)
	}
	if !ok {
		t.Fatalf("expected mapped entry")
	}

	if entry.Billable != 8 {
		t.Fatalf("unexpected rounded billable: want 8, got %d", entry.Billable)
	}
}
