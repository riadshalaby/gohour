package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/riadshalaby/gohour/storage"
)

func (s *Server) handlePartialMonth(w http.ResponseWriter, r *http.Request) {
	monthRaw := strings.TrimSpace(r.PathValue("month"))
	monthStart, err := parseMonth(monthRaw)
	if err != nil {
		http.Error(w, "invalid month format (expected YYYY-MM)", http.StatusBadRequest)
		return
	}
	monthEnd := endOfMonth(monthStart)
	refresh := strings.TrimSpace(r.URL.Query().Get("refresh")) == "1"

	localEntries, err := s.cache.LoadLocalRange(monthStart, monthEnd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	authErrorMsg := ""
	remoteEntries, refreshedAt, err := s.cache.LoadRemoteRange(r.Context(), monthStart, monthEnd, refresh)
	if err != nil {
		if refresh {
			writePartialTableError(w, http.StatusBadGateway, 6, fmt.Sprintf("load remote worklogs: %v", err))
			return
		}
		authErrorMsg = fmt.Sprintf(
			"OnePoint session may have expired (%v). In a new terminal run: gohour auth login",
			err,
		)
		remoteEntries = nil
	}

	rows, summary := buildMonthRows(monthStart, localEntries, remoteEntries)
	view := monthPageView{
		CurrentMonth:       monthRaw,
		Rows:               rows,
		TotalLocal:         summary.TotalLocalHours,
		TotalRemote:        summary.TotalRemoteHours,
		TotalLocalWorked:   summary.TotalLocalWorkedHours,
		TotalRemoteWorked:  summary.TotalRemoteWorkedHours,
		TotalWorkedDelta:   summary.TotalLocalWorkedHours - summary.TotalRemoteWorkedHours,
		TotalBillableDelta: summary.TotalDeltaHours,
		AuthErrorMsg:       authErrorMsg,
		RemoteRefreshedAt:  formatRefreshTime(refreshedAt),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderPartialTemplate(w, "partials/month_tbody.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handlePartialDay returns the day entry rows + OOB stat swaps as HTML
// (HTMX partial, Phase 2.2).
func (s *Server) handlePartialDay(w http.ResponseWriter, r *http.Request) {
	dayRaw := strings.TrimSpace(r.PathValue("date"))
	day, err := parseISODate(dayRaw)
	if err != nil {
		http.Error(w, "invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}
	refresh := strings.TrimSpace(r.URL.Query().Get("refresh")) == "1"
	if err := s.renderDayPartial(w, r, day, refresh, refresh); err != nil {
		http.Error(w, fmt.Sprintf("load remote worklogs: %v", err), http.StatusBadGateway)
		return
	}
}

func (s *Server) handlePartialWorklogCreate(w http.ResponseWriter, r *http.Request) {
	dayRaw := strings.TrimSpace(r.PathValue("date"))
	day, err := parseISODate(dayRaw)
	if err != nil {
		http.Error(w, "invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	body, err := parseMutationFromForm(r, dayRaw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if parseBoolFormValue(r.FormValue("force_overlap")) {
		r.Header.Set("X-Force-Overlap", "1")
	}
	entry, err := buildEntryFromMutation(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	entry.SourceFormat = "manual"
	entry.SourceMapper = "manual"
	entry.SourceFile = "web-ui"

	s.createMu.Lock()
	defer s.createMu.Unlock()

	existingEntries, err := s.cache.LoadLocalRange(day, day)
	if err != nil {
		http.Error(w, fmt.Sprintf("load local worklogs: %v", err), http.StatusInternalServerError)
		return
	}
	if s.writeMutationConflictIfAny(w, r, entry, existingEntries, 0) {
		return
	}

	id, inserted, err := s.store.InsertWorklog(entry)
	if err != nil {
		http.Error(w, fmt.Sprintf("insert worklog: %v", err), http.StatusInternalServerError)
		return
	}
	if !inserted {
		http.Error(w, "worklog already exists", http.StatusConflict)
		return
	}

	s.cache.InvalidateLocal()
	w.Header().Set(
		"HX-Trigger",
		fmt.Sprintf(`{"day-worklog-changed":{"day":"%s","action":"created","id":%d}}`, dayRaw, id),
	)
	if err := s.renderDayPartial(w, r, day, false, false); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
}

func (s *Server) handlePartialWorklogUpdate(w http.ResponseWriter, r *http.Request) {
	dayRaw := strings.TrimSpace(r.PathValue("date"))
	day, err := parseISODate(dayRaw)
	if err != nil {
		http.Error(w, "invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}
	id, err := parsePositiveInt64(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid worklog id", http.StatusBadRequest)
		return
	}

	existing, found, err := s.store.GetWorklogByID(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("get worklog by id: %v", err), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "worklog not found", http.StatusNotFound)
		return
	}

	body, err := parseMutationFromForm(r, dayRaw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if parseBoolFormValue(r.FormValue("force_overlap")) {
		r.Header.Set("X-Force-Overlap", "1")
	}
	entry, err := buildEntryFromMutation(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	entry.ID = existing.ID
	entry.SourceFormat = existing.SourceFormat
	entry.SourceMapper = existing.SourceMapper
	entry.SourceFile = existing.SourceFile

	s.createMu.Lock()
	defer s.createMu.Unlock()

	existingEntries, err := s.cache.LoadLocalRange(day, day)
	if err != nil {
		http.Error(w, fmt.Sprintf("load local worklogs: %v", err), http.StatusInternalServerError)
		return
	}
	if s.writeMutationConflictIfAny(w, r, entry, existingEntries, entry.ID) {
		return
	}

	if err := s.store.UpdateWorklog(entry); err != nil {
		if errors.Is(err, storage.ErrWorklogNotFound) {
			http.Error(w, "worklog not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("update worklog: %v", err), http.StatusInternalServerError)
		return
	}

	s.cache.InvalidateLocal()
	w.Header().Set(
		"HX-Trigger",
		fmt.Sprintf(`{"day-worklog-changed":{"day":"%s","action":"updated","id":%d}}`, dayRaw, id),
	)
	if err := s.renderDayPartial(w, r, day, false, false); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
}

func (s *Server) handlePartialWorklogDelete(w http.ResponseWriter, r *http.Request) {
	dayRaw := strings.TrimSpace(r.PathValue("date"))
	day, err := parseISODate(dayRaw)
	if err != nil {
		http.Error(w, "invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}
	id, err := parsePositiveInt64(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid worklog id", http.StatusBadRequest)
		return
	}

	deleted, err := s.store.DeleteWorklog(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("delete worklog: %v", err), http.StatusInternalServerError)
		return
	}
	if !deleted {
		http.Error(w, "worklog not found", http.StatusNotFound)
		return
	}

	s.cache.InvalidateLocal()
	w.Header().Set(
		"HX-Trigger",
		fmt.Sprintf(`{"day-worklog-changed":{"day":"%s","action":"deleted","id":%d}}`, dayRaw, id),
	)
	if err := s.renderDayPartial(w, r, day, false, false); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
}

func (s *Server) handlePartialSubmitDay(w http.ResponseWriter, r *http.Request) {
	dayRaw := strings.TrimSpace(r.PathValue("date"))
	day, err := parseISODate(dayRaw)
	if err != nil {
		http.Error(w, "invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}
	s.handlePartialSubmit(w, r, "day", dayRaw, day, day)
}

func (s *Server) handlePartialSubmitMonth(w http.ResponseWriter, r *http.Request) {
	monthRaw := strings.TrimSpace(r.PathValue("month"))
	monthStart, err := parseMonth(monthRaw)
	if err != nil {
		http.Error(w, "invalid month format (expected YYYY-MM)", http.StatusBadRequest)
		return
	}
	s.handlePartialSubmit(w, r, "month", monthRaw, monthStart, endOfMonth(monthStart))
}

func (s *Server) handlePartialSubmit(w http.ResponseWriter, r *http.Request, scope, target string, from, to time.Time) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
		return
	}
	dryRun := parseBoolFormValue(r.FormValue("dry_run"))
	if !dryRun {
		dryRun = strings.TrimSpace(r.URL.Query().Get("dry_run")) == "1"
	}

	s.logAudit(auditRecord{
		Operation: "submit",
		Scope:     scope,
		Target:    target,
		DryRun:    dryRun,
		Outcome:   "attempt",
	})

	view := submitPartialView{
		Scope:  scope,
		Target: target,
		DryRun: dryRun,
		Result: submitResponse{
			DryRun:     dryRun,
			LockedDays: []string{},
			Days:       []submitDayResult{},
		},
	}
	result, err := s.submitRange(r.Context(), from, to, dryRun)
	if err != nil {
		s.logAudit(auditRecord{
			Operation: "submit",
			Scope:     scope,
			Target:    target,
			DryRun:    dryRun,
			Outcome:   "error",
			Error:     err.Error(),
		})
		view.Error = err.Error()
		view.IsError = true
	} else {
		s.logAudit(auditRecord{
			Operation:  "submit",
			Scope:      scope,
			Target:     target,
			DryRun:     dryRun,
			Submitted:  result.Submitted,
			Duplicates: result.Duplicates,
			Overlaps:   result.Overlaps,
			LockedDays: append([]string(nil), result.LockedDays...),
			Outcome:    "success",
		})
		view.Result = result
		if !dryRun {
			if scope == "day" {
				w.Header().Set("HX-Trigger", fmt.Sprintf(`{"refresh-day":{"day":"%s"}}`, target))
			} else if scope == "month" {
				w.Header().Set("HX-Trigger", fmt.Sprintf(`{"refresh-month":{"month":"%s"}}`, target))
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderPartialTemplate(w, "partials/submit_result.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) renderDayPartial(w http.ResponseWriter, r *http.Request, day time.Time, refresh bool, failOnRemoteErr bool) error {
	view, err := s.buildDayPartialView(r.Context(), day, refresh, failOnRemoteErr)
	if err != nil {
		if failOnRemoteErr {
			writePartialTableError(w, http.StatusBadGateway, 11, fmt.Sprintf("load remote worklogs: %v", err))
			return nil
		}
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return renderPartialTemplate(w, "partials/day_tbody.html", view)
}

func (s *Server) buildDayPartialView(ctx context.Context, day time.Time, refresh bool, failOnRemoteErr bool) (dayPageView, error) {
	localEntries, err := s.cache.LoadLocalRange(day, day)
	if err != nil {
		return dayPageView{}, err
	}
	remoteEntries, refreshedAt, err := s.cache.LoadRemoteRange(ctx, day, day, refresh)
	if err != nil {
		if failOnRemoteErr {
			return dayPageView{}, err
		}
		remoteEntries = nil
		refreshedAt = time.Time{}
	}
	dayRows := BuildDailyView(localEntries, remoteEntries)
	row := DayRow{Date: day}
	if len(dayRows) > 0 {
		row = dayRows[0]
	}
	return dayPageView{
		Day:               day.Format("2006-01-02"),
		DayRow:            row,
		RemoteRefreshedAt: formatRefreshTime(refreshedAt),
	}, nil
}
