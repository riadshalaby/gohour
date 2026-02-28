package importer

import (
	"fmt"
	"gohour/config"
	"gohour/worklog"
	"strings"
)

// ATWorkMapper maps records from the atwork time-tracking app CSV export.
// It is stateless. Project, Activity and Skill are taken from the resolved
// rule config (like the EPM mapper). The CSV columns Projekt/Aufgabe provide
// source context and are folded into the description.
type ATWorkMapper struct{}

func (m *ATWorkMapper) Name() string {
	return "atwork"
}

func (m *ATWorkMapper) Map(record Record, cfg config.Config, sourceFormat, sourceFile string) (*worklog.Entry, bool, error) {
	startRaw := record.Get("Beginn", "beginn", "start")
	endRaw := record.Get("Ende", "ende", "end")
	durationRaw := record.Get("Dauer", "dauer", "duration")

	start, err := parseDateTime(startRaw)
	if err != nil {
		return nil, false, fmt.Errorf("row %d: parse start datetime: %w", record.RowNumber, err)
	}

	end, err := parseDateTime(endRaw)
	if err != nil {
		return nil, false, fmt.Errorf("row %d: parse end datetime: %w", record.RowNumber, err)
	}

	if !end.After(start) {
		return nil, false, fmt.Errorf("row %d: end datetime must be after start datetime", record.RowNumber)
	}

	billable, err := parseGermanDecimalHoursToMinutes(durationRaw)
	if err != nil {
		return nil, false, fmt.Errorf("row %d: parse duration: %w", record.RowNumber, err)
	}
	if billable <= 0 {
		return nil, false, nil // skip zero-duration rows
	}

	description := buildATWorkDescription(
		record.Get("Notiz", "notiz", "note"),
		record.Get("Aufgabe", "aufgabe", "task"),
		record.Get("Projekt", "projekt", "project"),
	)

	entry := &worklog.Entry{
		StartDateTime: start,
		EndDateTime:   end,
		Billable:      billable,
		Description:   description,
		Project:       cfg.ImportProject,
		Activity:      cfg.ImportActivity,
		Skill:         cfg.ImportSkill,
		SourceFormat:  sourceFormat,
		SourceFile:    sourceFile,
	}

	return entry, true, nil
}

// buildATWorkDescription builds a description from the atwork CSV fields.
// Priority: Notiz (main text). If empty, falls back to "Aufgabe".
// The Projekt and Aufgabe are prepended as context when Notiz is present.
func buildATWorkDescription(notiz, aufgabe, projekt string) string {
	notiz = strings.TrimSpace(notiz)
	aufgabe = strings.TrimSpace(aufgabe)
	projekt = strings.TrimSpace(projekt)

	if notiz == "" {
		if aufgabe != "" {
			return aufgabe
		}
		return projekt
	}

	// Prepend context: "[Projekt/Aufgabe] Notiz"
	var prefix string
	switch {
	case projekt != "" && aufgabe != "":
		prefix = projekt + "/" + aufgabe
	case projekt != "":
		prefix = projekt
	case aufgabe != "":
		prefix = aufgabe
	}

	if prefix != "" {
		return "[" + prefix + "] " + notiz
	}
	return notiz
}
