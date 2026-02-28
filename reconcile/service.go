package reconcile

import (
	"fmt"
	"gohour/storage"
	"gohour/worklog"
	"sort"
	"strings"
	"time"
)

type Result struct {
	DaysProcessed      int
	OverlapsBefore     int
	OverlapsAfter      int
	EPMEntriesAdjusted int
	RowsUpdated        int
}

type interval struct {
	start time.Time
	end   time.Time
}

func Run(store *storage.SQLiteStore) (*Result, error) {
	entries, err := store.ListWorklogs()
	if err != nil {
		return nil, err
	}

	result := &Result{}
	if len(entries) == 0 {
		return result, nil
	}

	byDay := groupByDay(entries)
	updates := make([]worklog.Entry, 0, 64)
	days := sortedKeys(byDay)
	result.DaysProcessed = len(days)

	for _, day := range days {
		dayEntries := byDay[day]
		result.OverlapsBefore += countConflicts(dayEntries)

		dayUpdates, adjusted := reconcileDay(dayEntries)
		result.EPMEntriesAdjusted += adjusted
		if len(dayUpdates) > 0 {
			updates = append(updates, dayUpdates...)
		}

		updatedDay := applyUpdates(dayEntries, dayUpdates)
		result.OverlapsAfter += countConflicts(updatedDay)
	}

	updatedRows, err := store.UpdateWorklogTimes(updates)
	if err != nil {
		return nil, fmt.Errorf("persist reconciled worklogs: %w", err)
	}
	result.RowsUpdated = updatedRows

	return result, nil
}

func groupByDay(entries []worklog.Entry) map[string][]worklog.Entry {
	byDay := make(map[string][]worklog.Entry)
	for _, entry := range entries {
		day := entry.StartDateTime.In(time.Local).Format("2006-01-02")
		byDay[day] = append(byDay[day], entry)
	}
	return byDay
}

func sortedKeys(values map[string][]worklog.Entry) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func reconcileDay(entries []worklog.Entry) ([]worklog.Entry, int) {
	if len(entries) < 2 {
		return nil, 0
	}

	dayEntries := append([]worklog.Entry(nil), entries...)
	sort.Slice(dayEntries, func(i, j int) bool {
		if dayEntries[i].StartDateTime.Equal(dayEntries[j].StartDateTime) {
			return dayEntries[i].ID < dayEntries[j].ID
		}
		return dayEntries[i].StartDateTime.Before(dayEntries[j].StartDateTime)
	})

	busy := make([]interval, 0, len(dayEntries))
	epmEntries := make([]worklog.Entry, 0, len(dayEntries))

	for _, entry := range dayEntries {
		if isEPMEntry(entry) {
			epmEntries = append(epmEntries, entry)
			continue
		}
		busy = addInterval(busy, interval{start: entry.StartDateTime, end: entry.EndDateTime})
	}

	updates := make([]worklog.Entry, 0, len(epmEntries))
	adjusted := 0
	for _, entry := range epmEntries {
		duration := entry.EndDateTime.Sub(entry.StartDateTime)
		if duration <= 0 {
			duration = time.Duration(entry.Billable) * time.Minute
		}
		if duration <= 0 {
			continue
		}

		newStart := findNextAvailableStart(busy, entry.StartDateTime, duration)
		newEnd := newStart.Add(duration)
		if !sameCalendarDay(entry.StartDateTime, newStart) || !sameCalendarDay(entry.StartDateTime, newEnd) {
			busy = addInterval(busy, interval{start: entry.StartDateTime, end: entry.EndDateTime})
			continue
		}
		if !newStart.Equal(entry.StartDateTime) || !newEnd.Equal(entry.EndDateTime) {
			entry.StartDateTime = newStart
			entry.EndDateTime = newEnd
			updates = append(updates, entry)
			adjusted++
		}

		busy = addInterval(busy, interval{start: newStart, end: newEnd})
	}

	return updates, adjusted
}

func findNextAvailableStart(busy []interval, desiredStart time.Time, duration time.Duration) time.Time {
	candidate := desiredStart
	for _, slot := range busy {
		candidateEnd := candidate.Add(duration)
		if !candidateEnd.After(slot.start) {
			return candidate
		}
		if !candidate.Before(slot.end) {
			continue
		}
		candidate = slot.end
	}
	return candidate
}

func addInterval(busy []interval, in interval) []interval {
	if !in.end.After(in.start) {
		return busy
	}

	all := append(append([]interval(nil), busy...), in)
	sort.Slice(all, func(i, j int) bool {
		return all[i].start.Before(all[j].start)
	})

	merged := make([]interval, 0, len(all))
	current := all[0]
	for _, next := range all[1:] {
		if next.start.After(current.end) {
			merged = append(merged, current)
			current = next
			continue
		}
		if next.end.After(current.end) {
			current.end = next.end
		}
	}
	merged = append(merged, current)

	return merged
}

func countConflicts(entries []worklog.Entry) int {
	if len(entries) < 2 {
		return 0
	}

	sorted := append([]worklog.Entry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].StartDateTime.Equal(sorted[j].StartDateTime) {
			return sorted[i].EndDateTime.Before(sorted[j].EndDateTime)
		}
		return sorted[i].StartDateTime.Before(sorted[j].StartDateTime)
	})

	conflicts := 0
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if !sorted[j].StartDateTime.Before(sorted[i].EndDateTime) {
				break
			}
			conflicts++
		}
	}

	return conflicts
}

func applyUpdates(dayEntries []worklog.Entry, updates []worklog.Entry) []worklog.Entry {
	if len(updates) == 0 {
		return dayEntries
	}

	byID := make(map[int64]worklog.Entry, len(updates))
	for _, update := range updates {
		byID[update.ID] = update
	}

	result := make([]worklog.Entry, 0, len(dayEntries))
	for _, entry := range dayEntries {
		if update, ok := byID[entry.ID]; ok {
			result = append(result, update)
			continue
		}
		result = append(result, entry)
	}

	return result
}

func isEPMEntry(entry worklog.Entry) bool {
	if strings.EqualFold(strings.TrimSpace(entry.SourceMapper), "epm") {
		return true
	}
	return strings.Contains(strings.ToLower(entry.SourceFile), "epmexport")
}

func sameCalendarDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}
