package web

import (
	"fmt"
	"sort"
	"time"

	"gohour/internal/timeutil"
	"gohour/onepoint"
	"gohour/worklog"
)

type DayRow struct {
	Date        time.Time
	LocalHours  float64
	RemoteHours float64
	Entries     []EntryRow
}

type EntryRow struct {
	ID           int64
	Source       string
	Start        string
	End          string
	DurationMins int
	Project      string
	Activity     string
	Skill        string
	BillableMins int
	Description  string
}

type MonthDayRow struct {
	Date        time.Time
	LocalHours  float64
	RemoteHours float64
	DeltaHours  float64
}

type MonthSummary struct {
	Days             []MonthDayRow
	TotalLocalHours  float64
	TotalRemoteHours float64
	TotalDeltaHours  float64
}

func BuildDailyView(local []worklog.Entry, remote []onepoint.DayWorklog) []DayRow {
	localByDay := make(map[string][]worklog.Entry)
	remoteByDay := make(map[string][]onepoint.DayWorklog)
	days := make(map[string]time.Time)

	for _, entry := range local {
		day := timeutil.StartOfDay(entry.StartDateTime)
		key := day.Format("2006-01-02")
		localByDay[key] = append(localByDay[key], entry)
		days[key] = day
	}
	for _, item := range remote {
		day, err := onepoint.ParseDay(item.WorklogDate)
		if err != nil {
			continue
		}
		day = timeutil.StartOfDay(day)
		key := day.Format("2006-01-02")
		remoteByDay[key] = append(remoteByDay[key], item)
		days[key] = day
	}

	keys := make([]string, 0, len(days))
	for key := range days {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return days[keys[i]].Before(days[keys[j]])
	})

	out := make([]DayRow, 0, len(keys))
	for _, key := range keys {
		localEntries := append([]worklog.Entry(nil), localByDay[key]...)
		remoteEntries := append([]onepoint.DayWorklog(nil), remoteByDay[key]...)

		sort.Slice(localEntries, func(i, j int) bool {
			if localEntries[i].StartDateTime.Equal(localEntries[j].StartDateTime) {
				return localEntries[i].EndDateTime.Before(localEntries[j].EndDateTime)
			}
			return localEntries[i].StartDateTime.Before(localEntries[j].StartDateTime)
		})
		sort.Slice(remoteEntries, func(i, j int) bool {
			if remoteEntries[i].StartTime == remoteEntries[j].StartTime {
				return remoteEntries[i].FinishTime < remoteEntries[j].FinishTime
			}
			return remoteEntries[i].StartTime < remoteEntries[j].StartTime
		})

		remotePayload := remotePayloadFor(remoteEntries)
		localPayload := make([]onepoint.PersistWorklog, 0, len(localEntries))
		rows := make([]EntryRow, 0, len(localEntries)+len(remoteEntries))

		localHours := 0.0
		for _, entry := range localEntries {
			payload := localEntryToPersistWorklog(entry)
			localPayload = append(localPayload, payload)

			rows = append(rows, EntryRow{
				ID:           entry.ID,
				Source:       classifyLocalEntry(payload, remotePayload),
				Start:        entry.StartDateTime.Format("15:04"),
				End:          entry.EndDateTime.Format("15:04"),
				DurationMins: max(0, timeutil.MinutesFromMidnight(entry.EndDateTime)-timeutil.MinutesFromMidnight(entry.StartDateTime)),
				Project:      entry.Project,
				Activity:     entry.Activity,
				Skill:        entry.Skill,
				BillableMins: entry.Billable,
				Description:  entry.Description,
			})
			localHours += hoursFromMinutes(entry.Billable)
		}

		remoteHours := 0.0
		for _, item := range remoteEntries {
			remoteHours += hoursFromMinutes(item.Billable)
		}

		for _, item := range remoteEntries {
			payload := item.ToPersistWorklog()
			if hasEquivalentLocal(localPayload, payload) {
				continue
			}
			rows = append(rows, EntryRow{
				Source:       "remote",
				Start:        minutesToClock(item.StartTime),
				End:          minutesToClock(item.FinishTime),
				DurationMins: max(0, item.FinishTime-item.StartTime),
				Project:      fmt.Sprintf("%d", item.ProjectID),
				Activity:     fmt.Sprintf("%d", item.ActivityID),
				Skill:        fmt.Sprintf("%d", item.SkillID),
				BillableMins: item.Billable,
				Description:  item.Comment,
			})
		}

		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].Start == rows[j].Start {
				return rows[i].Source < rows[j].Source
			}
			return rows[i].Start < rows[j].Start
		})

		out = append(out, DayRow{
			Date:        days[key],
			LocalHours:  localHours,
			RemoteHours: remoteHours,
			Entries:     rows,
		})
	}

	return out
}

func BuildMonthlyView(days []DayRow) MonthSummary {
	sorted := append([]DayRow(nil), days...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date.Before(sorted[j].Date)
	})

	summary := MonthSummary{
		Days: make([]MonthDayRow, 0, len(sorted)),
	}
	for _, day := range sorted {
		delta := day.LocalHours - day.RemoteHours
		summary.Days = append(summary.Days, MonthDayRow{
			Date:        timeutil.StartOfDay(day.Date),
			LocalHours:  day.LocalHours,
			RemoteHours: day.RemoteHours,
			DeltaHours:  delta,
		})
		summary.TotalLocalHours += day.LocalHours
		summary.TotalRemoteHours += day.RemoteHours
		summary.TotalDeltaHours += delta
	}
	return summary
}

func classifyLocalEntry(candidate onepoint.PersistWorklog, remote []onepoint.PersistWorklog) string {
	for _, item := range remote {
		if hasSameTimeRange(candidate, item) {
			return "duplicate"
		}
	}
	for _, item := range remote {
		if onepoint.WorklogTimeOverlaps(candidate, item) {
			return "overlap"
		}
	}
	return "new"
}

func hasEquivalentLocal(local []onepoint.PersistWorklog, candidate onepoint.PersistWorklog) bool {
	for _, item := range local {
		if hasSameTimeRange(item, candidate) {
			return true
		}
	}
	return false
}

func hasSameTimeRange(a, b onepoint.PersistWorklog) bool {
	if a.StartTime == nil || a.FinishTime == nil || b.StartTime == nil || b.FinishTime == nil {
		return false
	}
	return *a.StartTime == *b.StartTime && *a.FinishTime == *b.FinishTime
}

func localEntryToPersistWorklog(entry worklog.Entry) onepoint.PersistWorklog {
	start := timeutil.MinutesFromMidnight(entry.StartDateTime)
	finish := timeutil.MinutesFromMidnight(entry.EndDateTime)
	duration := int(entry.EndDateTime.Sub(entry.StartDateTime).Minutes())
	if duration < 0 {
		duration = 0
	}
	return onepoint.PersistWorklog{
		TimeRecordID: -1,
		WorkSlipID:   -1,
		WorkRecordID: -1,
		WorklogDate:  onepoint.FormatDay(timeutil.StartOfDay(entry.StartDateTime)),
		StartTime:    &start,
		FinishTime:   &finish,
		Duration:     duration,
		Billable:     entry.Billable,
		Valuable:     0,
		ProjectID:    onepoint.ID(0),
		ActivityID:   onepoint.ID(0),
		SkillID:      onepoint.ID(0),
		Comment:      entry.Description,
	}
}

func remotePayloadFor(values []onepoint.DayWorklog) []onepoint.PersistWorklog {
	out := make([]onepoint.PersistWorklog, 0, len(values))
	for _, item := range values {
		out = append(out, item.ToPersistWorklog())
	}
	return out
}

func hoursFromMinutes(minutes int) float64 {
	return float64(minutes) / 60.0
}

func minutesToClock(total int) string {
	if total < 0 {
		total = 0
	}
	hours := total / 60
	minutes := total % 60
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}
