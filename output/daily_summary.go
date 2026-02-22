package output

import (
	"fmt"
	"gohour/worklog"
	"math"
	"sort"
	"time"
)

type DailySummary struct {
	Date          string
	StartDateTime time.Time
	EndDateTime   time.Time
	WorkedHours   float64
	BillableHours float64
	BreakHours    float64
	WorklogCount  int
}

type interval struct {
	start time.Time
	end   time.Time
}

func BuildDailySummaries(entries []worklog.Entry) []DailySummary {
	if len(entries) == 0 {
		return []DailySummary{}
	}

	byDay := make(map[string][]worklog.Entry)
	for _, entry := range entries {
		day := entry.StartDateTime.In(time.Local).Format("2006-01-02")
		byDay[day] = append(byDay[day], entry)
	}

	days := make([]string, 0, len(byDay))
	for day := range byDay {
		days = append(days, day)
	}
	sort.Strings(days)

	summaries := make([]DailySummary, 0, len(days))
	for _, day := range days {
		dayEntries := byDay[day]
		summary := summarizeDay(day, dayEntries)
		summaries = append(summaries, summary)
	}

	return summaries
}

func summarizeDay(day string, entries []worklog.Entry) DailySummary {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].StartDateTime.Equal(entries[j].StartDateTime) {
			return entries[i].EndDateTime.Before(entries[j].EndDateTime)
		}
		return entries[i].StartDateTime.Before(entries[j].StartDateTime)
	})

	start := entries[0].StartDateTime
	end := entries[len(entries)-1].EndDateTime
	if end.Before(start) {
		end = start
	}

	billableMinutes := 0
	workedDuration := time.Duration(0)
	intervals := make([]interval, 0, len(entries))

	for _, entry := range entries {
		billableMinutes += entry.Billable
		if entry.EndDateTime.After(entry.StartDateTime) {
			workedDuration += entry.EndDateTime.Sub(entry.StartDateTime)
		}
		intervals = append(intervals, interval{
			start: entry.StartDateTime,
			end:   entry.EndDateTime,
		})
	}

	span := end.Sub(start)
	covered := mergedCoverageWithinWindow(intervals, start, end)
	breakDuration := span - covered
	if breakDuration < 0 {
		breakDuration = 0
	}

	return DailySummary{
		Date:          day,
		StartDateTime: start,
		EndDateTime:   end,
		WorkedHours:   roundHours(workedDuration.Hours()),
		BillableHours: roundHours(float64(billableMinutes) / 60.0),
		BreakHours:    roundHours(breakDuration.Hours()),
		WorklogCount:  len(entries),
	}
}

func mergedCoverageWithinWindow(intervals []interval, windowStart, windowEnd time.Time) time.Duration {
	if len(intervals) == 0 {
		return 0
	}
	if !windowEnd.After(windowStart) {
		return 0
	}

	clipped := make([]interval, 0, len(intervals))
	for _, candidate := range intervals {
		start := maxTime(candidate.start, windowStart)
		end := minTime(candidate.end, windowEnd)
		if end.After(start) {
			clipped = append(clipped, interval{start: start, end: end})
		}
	}
	if len(clipped) == 0 {
		return 0
	}

	sort.Slice(clipped, func(i, j int) bool {
		return clipped[i].start.Before(clipped[j].start)
	})

	currentStart := clipped[0].start
	currentEnd := clipped[0].end
	covered := time.Duration(0)

	for _, candidate := range clipped[1:] {
		if candidate.start.After(currentEnd) {
			covered += currentEnd.Sub(currentStart)
			currentStart = candidate.start
			currentEnd = candidate.end
			continue
		}

		if candidate.end.After(currentEnd) {
			currentEnd = candidate.end
		}
	}

	covered += currentEnd.Sub(currentStart)
	return covered
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func roundHours(value float64) float64 {
	return math.Round(value*100) / 100
}

func WriteDailySummaries(path, format string, summaries []DailySummary) error {
	switch normalizeFormat(format) {
	case "csv":
		return writeDailySummariesCSV(path, summaries)
	case "excel", "xlsx":
		return writeDailySummariesExcel(path, summaries)
	default:
		return fmt.Errorf("unsupported output format for daily summaries: %s", format)
	}
}
