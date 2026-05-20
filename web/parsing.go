package web

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/riadshalaby/gohour/internal/timeutil"
	"github.com/riadshalaby/gohour/worklog"
)

func parseMonth(value string) (time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01", strings.TrimSpace(value), time.Local)
	if err != nil {
		return time.Time{}, err
	}
	return timeutil.StartOfDay(parsed), nil
}

func parseISODate(value string) (time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.Local)
	if err != nil {
		return time.Time{}, err
	}
	return timeutil.StartOfDay(parsed), nil
}

func parsePositiveInt64(value string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, err
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("value must be > 0")
	}
	return parsed, nil
}

func parseMutationFromForm(r *http.Request, fallbackDate string) (worklogMutationRequest, error) {
	if err := r.ParseForm(); err != nil {
		return worklogMutationRequest{}, fmt.Errorf("parse form: %w", err)
	}

	date := strings.TrimSpace(r.FormValue("date"))
	if date == "" {
		date = strings.TrimSpace(fallbackDate)
	}

	billable := 0
	if rawMins := strings.TrimSpace(r.FormValue("billable")); rawMins != "" {
		parsed, err := strconv.Atoi(rawMins)
		if err != nil {
			return worklogMutationRequest{}, fmt.Errorf("invalid billable minutes")
		}
		billable = parsed
	} else {
		rawHours := strings.TrimSpace(r.FormValue("billableHours"))
		if rawHours == "" {
			return worklogMutationRequest{}, fmt.Errorf("missing billable hours")
		}
		hours, err := strconv.ParseFloat(rawHours, 64)
		if err != nil {
			return worklogMutationRequest{}, fmt.Errorf("invalid billable hours")
		}
		billable = int(math.Round(hours * 60))
	}

	return worklogMutationRequest{
		Start:       strings.TrimSpace(r.FormValue("start")),
		End:         strings.TrimSpace(r.FormValue("end")),
		Project:     strings.TrimSpace(r.FormValue("project")),
		Activity:    strings.TrimSpace(r.FormValue("activity")),
		Skill:       strings.TrimSpace(r.FormValue("skill")),
		Billable:    billable,
		Description: strings.TrimSpace(r.FormValue("description")),
		Date:        date,
	}, nil
}

func buildEntryFromMutation(body worklogMutationRequest) (worklog.Entry, error) {
	day, err := parseISODate(body.Date)
	if err != nil {
		return worklog.Entry{}, fmt.Errorf("invalid date format (expected YYYY-MM-DD)")
	}

	startMinutes, err := parseClockMinutes(body.Start)
	if err != nil {
		return worklog.Entry{}, fmt.Errorf("invalid start time (expected HH:MM)")
	}
	endMinutes, err := parseClockMinutes(body.End)
	if err != nil {
		return worklog.Entry{}, fmt.Errorf("invalid end time (expected HH:MM)")
	}
	if endMinutes <= startMinutes {
		return worklog.Entry{}, fmt.Errorf("end time must be after start time")
	}
	if body.Billable < 0 {
		return worklog.Entry{}, fmt.Errorf("billable must be >= 0")
	}
	project := strings.TrimSpace(body.Project)
	activity := strings.TrimSpace(body.Activity)
	skill := strings.TrimSpace(body.Skill)
	if project == "" {
		return worklog.Entry{}, fmt.Errorf("project must not be empty")
	}
	if activity == "" {
		return worklog.Entry{}, fmt.Errorf("activity must not be empty")
	}
	if skill == "" {
		return worklog.Entry{}, fmt.Errorf("skill must not be empty")
	}

	start := day.Add(time.Duration(startMinutes) * time.Minute)
	end := day.Add(time.Duration(endMinutes) * time.Minute)

	return worklog.Entry{
		StartDateTime: start,
		EndDateTime:   end,
		Billable:      body.Billable,
		Description:   strings.TrimSpace(body.Description),
		Project:       project,
		Activity:      activity,
		Skill:         skill,
	}, nil
}

func parseSkipIndicesSet(value string) map[int]bool {
	out := make(map[int]bool)
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return out
	}

	for _, part := range strings.Split(trimmed, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		index, err := strconv.Atoi(part)
		if err != nil || index < 0 {
			continue
		}
		out[index] = true
	}
	return out
}

func parseClockMinutes(value string) (int, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func parseBoolFormValue(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseInt64FormValue(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func boolValuePtr(value bool) *bool {
	return &value
}
