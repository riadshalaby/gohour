package importer

import (
	"gohour/config"
	"strings"
	"testing"
	"time"
)

func TestEPMMapper_SequentiallyAssignsTimesWithinDay(t *testing.T) {
	mapper := &EPMMapper{}
	cfg := baseConfig()

	records := []Record{
		newEPMRecord(2, "05.01.2026", "08:00 AM", "05:00 PM", "8,00", "", ""),
		newEPMRecord(3, "05.01.2026", "08:00 AM", "05:00 PM", "", "2,00", "Task A"),
		newEPMRecord(4, "05.01.2026", "08:00 AM", "05:00 PM", "", "1,50", "Task B"),
		newEPMRecord(5, "05.01.2026", "08:00 AM", "05:00 PM", "", "0,50", "Task C"),
	}

	_, ok, err := mapper.Map(records[0], cfg, "excel", "source.xlsx")
	if err != nil {
		t.Fatalf("unexpected header row error: %v", err)
	}
	if ok {
		t.Fatalf("expected summary row to be skipped")
	}

	entryA, ok, err := mapper.Map(records[1], cfg, "excel", "source.xlsx")
	assertMapped(t, ok, err)
	entryB, ok, err := mapper.Map(records[2], cfg, "excel", "source.xlsx")
	assertMapped(t, ok, err)
	entryC, ok, err := mapper.Map(records[3], cfg, "excel", "source.xlsx")
	assertMapped(t, ok, err)

	dayStart, _ := parseDateAndTime("05.01.2026", "08:00 AM")
	assertTime(t, dayStart, entryA.StartDateTime, "entryA start")
	assertTime(t, dayStart.Add(2*time.Hour), entryA.EndDateTime, "entryA end")

	assertTime(t, dayStart.Add(2*time.Hour), entryB.StartDateTime, "entryB start")
	assertTime(t, dayStart.Add(3*time.Hour+30*time.Minute), entryB.EndDateTime, "entryB end")

	assertTime(t, dayStart.Add(3*time.Hour+30*time.Minute), entryC.StartDateTime, "entryC start")
	assertTime(t, dayStart.Add(4*time.Hour), entryC.EndDateTime, "entryC end")
}

func TestEPMMapper_InsertsMiddayBreakBasedOnOriginalDayEnd(t *testing.T) {
	mapper := &EPMMapper{}
	cfg := baseConfig()

	records := []Record{
		newEPMRecord(2, "05.01.2026", "08:00 AM", "05:00 PM", "8,00", "", ""),
		newEPMRecord(3, "05.01.2026", "08:00 AM", "05:00 PM", "", "4,00", "Task A"),
		newEPMRecord(4, "05.01.2026", "08:00 AM", "05:00 PM", "", "4,00", "Task B"),
	}

	_, _, _ = mapper.Map(records[0], cfg, "excel", "source.xlsx")
	entryA, ok, err := mapper.Map(records[1], cfg, "excel", "source.xlsx")
	assertMapped(t, ok, err)
	entryB, ok, err := mapper.Map(records[2], cfg, "excel", "source.xlsx")
	assertMapped(t, ok, err)

	assertTime(t, mustParseDateTime(t, "05.01.2026", "08:00 AM"), entryA.StartDateTime, "entryA start")
	assertTime(t, mustParseDateTime(t, "05.01.2026", "12:00 PM"), entryA.EndDateTime, "entryA end")
	assertTime(t, mustParseDateTime(t, "05.01.2026", "01:00 PM"), entryB.StartDateTime, "entryB start with break")
	assertTime(t, mustParseDateTime(t, "05.01.2026", "05:00 PM"), entryB.EndDateTime, "entryB end against original day end")
}

func TestEPMMapper_PauseInsertedAtNearestBoundaryToMiddle(t *testing.T) {
	mapper := &EPMMapper{}
	cfg := baseConfig()

	records := []Record{
		newEPMRecord(2, "05.01.2026", "08:00 AM", "05:00 PM", "8,00", "", ""),
		newEPMRecord(3, "05.01.2026", "08:00 AM", "05:00 PM", "", "2,00", "Task A"),
		newEPMRecord(4, "05.01.2026", "08:00 AM", "05:00 PM", "", "3,00", "Task B"),
		newEPMRecord(5, "05.01.2026", "08:00 AM", "05:00 PM", "", "3,00", "Task C"),
	}

	_, _, _ = mapper.Map(records[0], cfg, "excel", "source.xlsx")
	_, ok, err := mapper.Map(records[1], cfg, "excel", "source.xlsx")
	assertMapped(t, ok, err)
	_, ok, err = mapper.Map(records[2], cfg, "excel", "source.xlsx")
	assertMapped(t, ok, err)
	entryC, ok, err := mapper.Map(records[3], cfg, "excel", "source.xlsx")
	assertMapped(t, ok, err)

	assertTime(t, mustParseDateTime(t, "05.01.2026", "02:00 PM"), entryC.StartDateTime, "entryC start after inserted break")
	assertTime(t, mustParseDateTime(t, "05.01.2026", "05:00 PM"), entryC.EndDateTime, "entryC end")
}

func TestEPMMapper_ResetsSequenceOnNewDay(t *testing.T) {
	mapper := &EPMMapper{}
	cfg := baseConfig()

	summaryDay1 := newEPMRecord(2, "05.01.2026", "08:00 AM", "05:00 PM", "8,00", "", "")
	recordDay1 := newEPMRecord(3, "05.01.2026", "08:00 AM", "05:00 PM", "", "2,00", "Task A")
	summaryDay2 := newEPMRecord(4, "06.01.2026", "09:00 AM", "06:00 PM", "8,00", "", "")
	recordDay2 := newEPMRecord(5, "06.01.2026", "09:00 AM", "06:00 PM", "", "1,00", "Task B")

	_, _, _ = mapper.Map(summaryDay1, cfg, "excel", "source.xlsx")
	_, ok, err := mapper.Map(recordDay1, cfg, "excel", "source.xlsx")
	assertMapped(t, ok, err)

	_, _, _ = mapper.Map(summaryDay2, cfg, "excel", "source.xlsx")
	entryDay2, ok, err := mapper.Map(recordDay2, cfg, "excel", "source.xlsx")
	assertMapped(t, ok, err)

	assertTime(t, mustParseDateTime(t, "06.01.2026", "09:00 AM"), entryDay2.StartDateTime, "day2 start")
	assertTime(t, mustParseDateTime(t, "06.01.2026", "10:00 AM"), entryDay2.EndDateTime, "day2 end")
}

func TestEPMMapper_RestartsWhenSameFileStartsAgain(t *testing.T) {
	mapper := &EPMMapper{}
	cfg := baseConfig()

	summary := newEPMRecord(2, "05.01.2026", "08:00 AM", "05:00 PM", "8,00", "", "")
	firstRun := newEPMRecord(3, "05.01.2026", "08:00 AM", "05:00 PM", "", "1,00", "Task A")
	secondInFirstRun := newEPMRecord(4, "05.01.2026", "08:00 AM", "05:00 PM", "", "1,00", "Task B")
	summarySecondRun := newEPMRecord(2, "05.01.2026", "08:00 AM", "05:00 PM", "8,00", "", "")
	firstOfSecondRun := newEPMRecord(3, "05.01.2026", "08:00 AM", "05:00 PM", "", "1,00", "Task C")

	_, _, _ = mapper.Map(summary, cfg, "excel", "same.xlsx")
	_, ok, err := mapper.Map(firstRun, cfg, "excel", "same.xlsx")
	assertMapped(t, ok, err)
	_, ok, err = mapper.Map(secondInFirstRun, cfg, "excel", "same.xlsx")
	assertMapped(t, ok, err)

	_, _, _ = mapper.Map(summarySecondRun, cfg, "excel", "same.xlsx")
	entryRestarted, ok, err := mapper.Map(firstOfSecondRun, cfg, "excel", "same.xlsx")
	assertMapped(t, ok, err)

	assertTime(t, mustParseDateTime(t, "05.01.2026", "08:00 AM"), entryRestarted.StartDateTime, "restarted run start")
	assertTime(t, mustParseDateTime(t, "05.01.2026", "09:00 AM"), entryRestarted.EndDateTime, "restarted run end")
}

func TestEPMMapper_ReturnsErrorWhenComputedEndCrossesDayBoundary(t *testing.T) {
	mapper := &EPMMapper{}
	cfg := baseConfig()

	summary := newEPMRecord(2, "05.01.2026", "05:00 PM", "06:00 PM", "8,00", "", "")
	first := newEPMRecord(3, "05.01.2026", "05:00 PM", "06:00 PM", "", "3,00", "Task A")
	second := newEPMRecord(4, "05.01.2026", "05:00 PM", "06:00 PM", "", "5,00", "Task B")

	_, _, _ = mapper.Map(summary, cfg, "excel", "source.xlsx")
	_, ok, err := mapper.Map(first, cfg, "excel", "source.xlsx")
	assertMapped(t, ok, err)

	_, ok, err = mapper.Map(second, cfg, "excel", "source.xlsx")
	if err == nil {
		t.Fatalf("expected day-boundary error")
	}
	if ok {
		t.Fatalf("expected mapping to fail")
	}
	if !strings.Contains(err.Error(), "exceeds day boundary") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newEPMRecord(row int, date, from, bis, tagessumme, hours, description string) Record {
	return Record{
		RowNumber: row,
		Values: map[string]string{
			normalizeHeader("Datum"):                  date,
			normalizeHeader("Von"):                    from,
			normalizeHeader("Bis"):                    bis,
			normalizeHeader("Tagessumme"):             tagessumme,
			normalizeHeader("Stunden"):                hours,
			normalizeHeader("Durchgeführte Arbeiten"): description,
		},
	}
}

func baseConfig() config.Config {
	return config.Config{
		ImportProject:  "test-project",
		ImportActivity: "test-activity",
		ImportSkill:    "test-skill",
	}
}

func mustParseDateTime(t *testing.T, date, clock string) time.Time {
	t.Helper()
	parsed, err := parseDateAndTime(date, clock)
	if err != nil {
		t.Fatalf("parse datetime %q %q: %v", date, clock, err)
	}
	return parsed
}

func assertMapped(t *testing.T, ok bool, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected record to map")
	}
}

func assertTime(t *testing.T, expected, actual time.Time, field string) {
	t.Helper()
	if !expected.Equal(actual) {
		t.Fatalf("unexpected %s: expected %s, got %s", field, expected.Format(time.RFC3339), actual.Format(time.RFC3339))
	}
}
