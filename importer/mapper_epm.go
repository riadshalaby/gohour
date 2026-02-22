package importer

import (
	"fmt"
	"gohour/config"
	"gohour/worklog"
	"math"
	"strconv"
	"strings"
	"time"
)

type EPMMapper struct {
	dayStateByKey    map[string]*epmDayState
	sourceRunByFile  map[string]int
	sourceSeenByFile map[string]bool
}

type epmDayState struct {
	dayStart             time.Time
	dayEndOriginal       time.Time
	previousEnd          time.Time
	expectedBillableMins int
	consumedBillableMins int
	breakMins            int
	pauseInserted        bool
}

func (m *EPMMapper) Name() string {
	return "epm"
}

func (m *EPMMapper) Map(record Record, cfg config.Config, sourceFormat, sourceFile string) (*worklog.Entry, bool, error) {
	m.ensureInitialized()
	run := m.detectSourceRun(sourceFile, record.RowNumber)
	description := strings.TrimSpace(record.Get("Durchgeführte Arbeiten", "Beschreibung", "description"))
	dayValue := strings.TrimSpace(record.Get("Datum", "date"))
	if dayValue == "" {
		if description == "" {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("row %d: missing date", record.RowNumber)
	}
	dayKey := m.buildDayKey(sourceFile, run, dayValue)

	state, err := m.ensureDayState(dayKey, record)
	if err != nil {
		return nil, false, fmt.Errorf("row %d: %w", record.RowNumber, err)
	}

	if description == "" {
		return nil, false, nil
	}

	billable, err := parseGermanDecimalHoursToMinutes(record.Get("Stunden", "hours", "duration", "billable"))
	if err != nil {
		return nil, false, fmt.Errorf("row %d: %w", record.RowNumber, err)
	}
	if billable <= 0 {
		return nil, false, nil
	}

	start := state.dayStart
	if !state.previousEnd.IsZero() {
		start = state.previousEnd
	}

	if m.shouldInsertPauseBeforeCurrent(state, billable) {
		start = start.Add(time.Duration(state.breakMins) * time.Minute)
		state.pauseInserted = true
	}

	end := start.Add(time.Duration(billable) * time.Minute)
	state.previousEnd = end
	state.consumedBillableMins += billable

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

func (m *EPMMapper) ensureInitialized() {
	if m.dayStateByKey == nil {
		m.dayStateByKey = make(map[string]*epmDayState)
	}
	if m.sourceRunByFile == nil {
		m.sourceRunByFile = make(map[string]int)
	}
	if m.sourceSeenByFile == nil {
		m.sourceSeenByFile = make(map[string]bool)
	}
}

func (m *EPMMapper) detectSourceRun(sourceFile string, rowNumber int) int {
	if rowNumber == 2 {
		if m.sourceSeenByFile[sourceFile] {
			m.sourceRunByFile[sourceFile]++
		}
		m.sourceSeenByFile[sourceFile] = true
	}
	return m.sourceRunByFile[sourceFile]
}

func (m *EPMMapper) buildDayKey(sourceFile string, run int, day string) string {
	day = normalizeDayKey(day)
	return sourceFile + "|" + strconv.Itoa(run) + "|" + day
}

func normalizeDayKey(day string) string {
	return strings.TrimSpace(day)
}

func (m *EPMMapper) ensureDayState(dayKey string, record Record) (*epmDayState, error) {
	state, ok := m.dayStateByKey[dayKey]
	if !ok {
		state = &epmDayState{}
		m.dayStateByKey[dayKey] = state
	}

	date := record.Get("Datum", "date")
	startParsed, startErr := parseDateAndTime(date, record.Get("Von", "start", "starttime"))
	endParsed, endErr := parseDateAndTime(date, record.Get("Bis", "end", "endtime"))
	if startErr != nil || endErr != nil {
		if state.dayStart.IsZero() || state.dayEndOriginal.IsZero() {
			if startErr != nil {
				return nil, fmt.Errorf("parse day start datetime: %w", startErr)
			}
			return nil, fmt.Errorf("parse day end datetime: %w", endErr)
		}
	} else {
		if state.dayStart.IsZero() {
			state.dayStart = startParsed
		}
		if state.dayEndOriginal.IsZero() {
			state.dayEndOriginal = endParsed
		}
	}

	if state.dayStart.IsZero() || state.dayEndOriginal.IsZero() {
		return nil, fmt.Errorf("missing day start/end information")
	}

	if rawDayTotal := strings.TrimSpace(record.Get("Tagessumme", "daytotal", "daysum")); rawDayTotal != "" {
		expectedBillableMins, err := parseGermanDecimalHoursToMinutes(rawDayTotal)
		if err != nil {
			return nil, fmt.Errorf("parse day total: %w", err)
		}
		if expectedBillableMins > 0 {
			state.expectedBillableMins = expectedBillableMins
			state.breakMins = m.computeBreakMinutes(state.dayStart, state.dayEndOriginal, state.expectedBillableMins)
		}
	}

	return state, nil
}

func (m *EPMMapper) computeBreakMinutes(dayStart, dayEnd time.Time, expectedBillableMins int) int {
	spanMins := int(dayEnd.Sub(dayStart).Minutes())
	if spanMins <= 0 {
		return 0
	}
	if expectedBillableMins <= 0 || expectedBillableMins >= spanMins {
		return 0
	}
	return spanMins - expectedBillableMins
}

func (m *EPMMapper) shouldInsertPauseBeforeCurrent(state *epmDayState, currentBillable int) bool {
	if state == nil || state.pauseInserted || state.breakMins <= 0 || state.expectedBillableMins <= 0 {
		return false
	}
	if state.consumedBillableMins <= 0 {
		return false
	}

	consumed := state.consumedBillableMins
	nextBoundary := consumed + currentBillable
	half := float64(state.expectedBillableMins) / 2.0

	if float64(consumed) >= half {
		return true
	}
	if float64(nextBoundary) >= half {
		distNow := half - float64(consumed)
		distNext := float64(nextBoundary) - half
		return distNow <= distNext || almostEqual(distNow, distNext)
	}

	return false
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.0000001
}
