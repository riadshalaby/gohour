package importer

import (
	"fmt"
	"gohour/config"
	"gohour/worklog"
	"strings"
	"time"
)

type GenericMapper struct{}

func (m *GenericMapper) Name() string {
	return "generic"
}

func (m *GenericMapper) Map(record Record, _ config.Config, sourceFormat, sourceFile string) (*worklog.Entry, bool, error) {
	description := strings.TrimSpace(record.Get("description", "beschreibung"))
	if description == "" {
		return nil, false, nil
	}

	start, err := parseDateTime(record.Get("startdatetime", "start", "von"))
	if err != nil {
		return nil, false, fmt.Errorf("row %d: parse start datetime: %w", record.RowNumber, err)
	}

	end, err := parseDateTime(record.Get("enddatetime", "end", "bis"))
	if err != nil {
		return nil, false, fmt.Errorf("row %d: parse end datetime: %w", record.RowNumber, err)
	}
	if !end.After(start) {
		return nil, false, fmt.Errorf("row %d: end datetime must be after start datetime", record.RowNumber)
	}

	billable := int(end.Sub(start).Minutes())
	if value := strings.TrimSpace(record.Get("billable", "minutes", "arbeitszeit", "duration")); value != "" {
		parsed, parseErr := parseMinutesOrHours(value)
		if parseErr != nil {
			return nil, false, fmt.Errorf("row %d: parse billable value: %w", record.RowNumber, parseErr)
		}
		if parsed > 0 {
			billable = parsed
			end = start.Add(time.Duration(billable) * time.Minute)
		}
	}

	entry := &worklog.Entry{
		StartDateTime: start,
		EndDateTime:   end,
		Billable:      billable,
		Description:   description,
		Project:       fallback(record.Get("project", "projekt"), ""),
		Activity:      fallback(record.Get("activity", "aktivitaet", "aktivität"), ""),
		Skill:         fallback(record.Get("skill"), ""),
		SourceFormat:  sourceFormat,
		SourceFile:    sourceFile,
	}

	return entry, true, nil
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return strings.TrimSpace(value)
}
