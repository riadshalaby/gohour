package web

import (
	"sort"
	"strings"
	"time"

	"github.com/riadshalaby/gohour/onepoint"
	"github.com/riadshalaby/gohour/worklog"
)

func detectLocalConflict(candidate worklog.Entry, existing []worklog.Entry) (conflictType string, existingID int64, ok bool) {
	for _, entry := range existing {
		if sameLocalWorklogKey(candidate, entry) {
			return "duplicate", entry.ID, true
		}
	}
	for _, entry := range existing {
		if timesOverlap(candidate.StartDateTime, candidate.EndDateTime, entry.StartDateTime, entry.EndDateTime) {
			return "overlap", entry.ID, true
		}
	}
	return "", 0, false
}

func sameLocalWorklogKey(left, right worklog.Entry) bool {
	return left.StartDateTime.Equal(right.StartDateTime) &&
		left.EndDateTime.Equal(right.EndDateTime) &&
		normalizeConflictName(left.Project) == normalizeConflictName(right.Project) &&
		normalizeConflictName(left.Activity) == normalizeConflictName(right.Activity) &&
		normalizeConflictName(left.Skill) == normalizeConflictName(right.Skill)
}

func containsSameLocalWorklogKey(candidate worklog.Entry, existing []worklog.Entry) bool {
	for _, item := range existing {
		if sameLocalWorklogKey(candidate, item) {
			return true
		}
	}
	return false
}

func timesOverlap(leftStart, leftEnd, rightStart, rightEnd time.Time) bool {
	return leftStart.Before(rightEnd) && leftEnd.After(rightStart)
}

func sortDayWorklogs(values []onepoint.DayWorklog) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].StartTime == values[j].StartTime {
			return values[i].FinishTime < values[j].FinishTime
		}
		return values[i].StartTime < values[j].StartTime
	})
}

func normalizeConflictName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}
