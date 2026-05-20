package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/riadshalaby/gohour/onepoint"
	"github.com/riadshalaby/gohour/submitter"
)

func (s *Server) handleAPISubmitDay(w http.ResponseWriter, r *http.Request) {
	dayRaw := strings.TrimSpace(r.PathValue("date"))
	day, err := parseISODate(dayRaw)
	if err != nil {
		http.Error(w, "invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	dryRun := strings.TrimSpace(r.URL.Query().Get("dry_run")) == "1"
	s.logAudit(auditRecord{
		Operation: "submit",
		Scope:     "day",
		Target:    dayRaw,
		DryRun:    dryRun,
		Outcome:   "attempt",
	})
	resp, err := s.submitRange(r.Context(), day, day, dryRun)
	if err != nil {
		s.logAudit(auditRecord{
			Operation: "submit",
			Scope:     "day",
			Target:    dayRaw,
			DryRun:    dryRun,
			Outcome:   "error",
			Error:     err.Error(),
		})
		http.Error(w, err.Error(), submitErrorStatus(err))
		return
	}
	s.logAudit(auditRecord{
		Operation:  "submit",
		Scope:      "day",
		Target:     dayRaw,
		DryRun:     dryRun,
		Submitted:  resp.Submitted,
		Duplicates: resp.Duplicates,
		Overlaps:   resp.Overlaps,
		LockedDays: append([]string(nil), resp.LockedDays...),
		Outcome:    "success",
	})
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAPISubmitMonth(w http.ResponseWriter, r *http.Request) {
	monthRaw := strings.TrimSpace(r.PathValue("month"))
	monthStart, err := parseMonth(monthRaw)
	if err != nil {
		http.Error(w, "invalid month format (expected YYYY-MM)", http.StatusBadRequest)
		return
	}

	dryRun := strings.TrimSpace(r.URL.Query().Get("dry_run")) == "1"
	s.logAudit(auditRecord{
		Operation: "submit",
		Scope:     "month",
		Target:    monthRaw,
		DryRun:    dryRun,
		Outcome:   "attempt",
	})
	resp, err := s.submitRange(r.Context(), monthStart, endOfMonth(monthStart), dryRun)
	if err != nil {
		s.logAudit(auditRecord{
			Operation: "submit",
			Scope:     "month",
			Target:    monthRaw,
			DryRun:    dryRun,
			Outcome:   "error",
			Error:     err.Error(),
		})
		http.Error(w, err.Error(), submitErrorStatus(err))
		return
	}
	s.logAudit(auditRecord{
		Operation:  "submit",
		Scope:      "month",
		Target:     monthRaw,
		DryRun:     dryRun,
		Submitted:  resp.Submitted,
		Duplicates: resp.Duplicates,
		Overlaps:   resp.Overlaps,
		LockedDays: append([]string(nil), resp.LockedDays...),
		Outcome:    "success",
	})
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) submitRange(ctx context.Context, from, to time.Time, dryRun bool) (submitResponse, error) {
	response := submitResponse{
		DryRun:     dryRun,
		LockedDays: make([]string, 0),
		Days:       make([]submitDayResult, 0),
	}
	client := upstreamErrorClient{base: s.client}

	entries, err := s.cache.LoadLocalRange(from, to)
	if err != nil {
		return response, err
	}
	if len(entries) == 0 {
		return response, nil
	}

	idMap, err := submitter.ResolveIDsForEntries(ctx, client, s.config.Snapshot().Rules, entries, s.submitOptions)
	if err != nil {
		return response, err
	}

	dayBatches, err := submitter.BuildDayBatches(entries, idMap)
	if err != nil {
		return response, err
	}

	submittedDays := make([]time.Time, 0)
	for _, batch := range dayBatches {
		dayLabel := onepoint.FormatDay(batch.Day)
		dayResult := submitDayResult{Date: batch.Day.Format("2006-01-02")}

		existing, err := client.GetDayWorklogs(ctx, batch.Day)
		if err != nil {
			return response, fmt.Errorf("load existing day %s failed: %w", dayLabel, err)
		}

		if submitter.CountLockedDayWorklogs(existing) > 0 {
			dayResult.Locked = true
			response.LockedDays = append(response.LockedDays, dayResult.Date)
			response.Days = append(response.Days, dayResult)
			continue
		}

		existingPayload := submitter.DayWorklogsToPersistPayload(existing)
		toAdd, overlaps, duplicates := submitter.ClassifyWorklogs(batch.Worklogs, existingPayload)
		dayResult.Added = len(toAdd)
		dayResult.Duplicates = len(duplicates)
		dayResult.Overlaps = len(overlaps)
		response.Duplicates += len(duplicates)
		response.Overlaps += len(overlaps)

		if !dryRun && len(toAdd) > 0 {
			payload := submitter.BuildPersistPayload(existingPayload, toAdd)

			if _, err := client.PersistWorklogs(ctx, batch.Day, payload); err != nil {
				return response, fmt.Errorf("submit day %s failed: %w", dayLabel, err)
			}
			response.Submitted += len(toAdd)
			submittedDays = append(submittedDays, batch.Day)
		}

		response.Days = append(response.Days, dayResult)
	}

	if !dryRun {
		s.cache.InvalidateRemoteDays(submittedDays)
	}
	return response, nil
}

func submitErrorStatus(err error) int {
	if errors.Is(err, errOnePointUpstream) {
		return http.StatusBadGateway
	}
	return http.StatusInternalServerError
}
