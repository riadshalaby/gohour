// Package web serves a localhost-only single-user UI; it intentionally has no
// auth/CSRF protection in this mode.
package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/riadshalaby/gohour/config"
	"github.com/riadshalaby/gohour/importer"
	"github.com/riadshalaby/gohour/internal/timeutil"
	"github.com/riadshalaby/gohour/onepoint"
	"github.com/riadshalaby/gohour/reconcile"
	"github.com/riadshalaby/gohour/storage"
	"github.com/riadshalaby/gohour/submitter"
	"github.com/riadshalaby/gohour/worklog"
)

type Server struct {
	store  *storage.SQLiteStore
	client onepoint.Client

	submitOptions onepoint.ResolveOptions
	audit         auditLogger
	cache         *dataCache
	config        *configStore
	mux           *http.ServeMux

	createMu sync.Mutex
}

var (
	errRuleDuplicate = errors.New("rule already exists")
	errRuleNotFound  = errors.New("rule not found")
)

func NewServer(store *storage.SQLiteStore, client onepoint.Client, cfg config.Config) http.Handler {
	server := &Server{
		store:  store,
		client: client,
		audit:  newFileAuditLogger(defaultAuditLogPath()),
		cache:  newDataCache(store, client),
		config: newConfigStore(cfg),
	}

	mux := http.NewServeMux()

	// Static file serving (embedded; served at /static/)
	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// Page routes
	mux.HandleFunc("GET /month", server.handleMonthPicker)
	mux.HandleFunc("GET /month/{month}", server.handleMonth)
	mux.HandleFunc("GET /day/{date}", server.handleDay)
	mux.HandleFunc("GET /config", server.handleConfig)

	// HTMX partial routes (Phase 2)
	mux.HandleFunc("GET /partials/month/{month}", server.handlePartialMonth)
	mux.HandleFunc("GET /partials/day/{date}", server.handlePartialDay)
	mux.HandleFunc("POST /partials/day/{date}/worklog", server.handlePartialWorklogCreate)
	mux.HandleFunc("POST /partials/day/{date}/worklog/{id}", server.handlePartialWorklogUpdate)
	mux.HandleFunc("POST /partials/day/{date}/worklog/{id}/delete", server.handlePartialWorklogDelete)
	mux.HandleFunc("POST /partials/submit/day/{date}", server.handlePartialSubmitDay)
	mux.HandleFunc("POST /partials/submit/month/{month}", server.handlePartialSubmitMonth)

	// JSON API routes
	mux.HandleFunc("GET /api/month/{month}", server.handleAPIMonth)
	mux.HandleFunc("GET /api/day/{date}", server.handleAPIDay)
	mux.HandleFunc("GET /api/lookup", server.handleAPILookup)
	mux.HandleFunc("GET /api/config", server.handleAPIConfig)
	mux.HandleFunc("PATCH /api/config", server.handleAPIConfigPatch)
	mux.HandleFunc("GET /api/rules", server.handleAPIRules)
	mux.HandleFunc("POST /api/rules", server.handleAPIRuleCreate)
	mux.HandleFunc("PATCH /api/rules/{name}", server.handleAPIRulePatch)
	mux.HandleFunc("DELETE /api/rules/{name}", server.handleAPIRuleDelete)
	mux.HandleFunc("POST /api/worklog", server.handleAPIWorklogCreate)
	mux.HandleFunc("PATCH /api/worklog/{id}", server.handleAPIWorklogPatch)
	mux.HandleFunc("DELETE /api/worklog/{id}", server.handleAPIWorklogDelete)
	mux.HandleFunc("POST /api/import", server.handleAPIImport)
	mux.HandleFunc("GET /api/import/rule-match", server.handleAPIImportRuleMatch)
	mux.HandleFunc("POST /api/import-preview", server.handleAPIImportPreview)
	mux.HandleFunc("POST /api/submit/day/{date}", server.handleAPISubmitDay)
	mux.HandleFunc("POST /api/submit/month/{month}", server.handleAPISubmitMonth)
	mux.HandleFunc("DELETE /api/month/{month}/worklogs", server.handleAPIDeleteMonthWorklogs)
	mux.HandleFunc("DELETE /api/month/{month}/remote-worklogs", server.handleAPIDeleteMonthRemoteWorklogs)
	mux.HandleFunc("POST /api/month/{month}/copy-from-remote", server.handleAPICopyMonthRemote)
	mux.HandleFunc("POST /api/month/{month}/sync", server.handleAPISyncMonthRemote)
	server.mux = mux

	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleMonthPicker(w http.ResponseWriter, r *http.Request) {
	month := strings.TrimSpace(r.URL.Query().Get("month"))
	if month == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if _, err := parseMonth(month); err != nil {
		http.Error(w, "invalid month format (expected YYYY-MM)", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/month/"+month, http.StatusFound)
}

func (s *Server) handleMonth(w http.ResponseWriter, r *http.Request) {
	monthRaw := strings.TrimSpace(r.PathValue("month"))
	monthStart, err := parseMonth(monthRaw)
	if err != nil {
		http.Error(w, "invalid month format (expected YYYY-MM)", http.StatusBadRequest)
		return
	}
	monthEnd := endOfMonth(monthStart)

	localEntries, err := s.cache.LoadLocalRange(monthStart, monthEnd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	authErrorMsg := ""
	remoteEntries, refreshedAt, err := s.cache.LoadRemoteRange(r.Context(), monthStart, monthEnd, false)
	if err != nil {
		authErrorMsg = fmt.Sprintf(
			"OnePoint session may have expired (%v). In a new terminal run: gohour auth login",
			err,
		)
		remoteEntries = nil
	}

	rows, summary := buildMonthRows(monthStart, localEntries, remoteEntries)

	view := monthPageView{
		Title:              "gohour - month " + monthRaw,
		CurrentMonth:       monthRaw,
		PreviousMonth:      monthStart.AddDate(0, -1, 0).Format("2006-01"),
		NextMonth:          monthStart.AddDate(0, 1, 0).Format("2006-01"),
		AuthErrorMsg:       authErrorMsg,
		Rows:               rows,
		TotalLocal:         summary.TotalLocalHours,
		TotalRemote:        summary.TotalRemoteHours,
		TotalLocalWorked:   summary.TotalLocalWorkedHours,
		TotalRemoteWorked:  summary.TotalRemoteWorkedHours,
		TotalWorkedDelta:   summary.TotalLocalWorkedHours - summary.TotalRemoteWorkedHours,
		TotalBillableDelta: summary.TotalDeltaHours,
		RemoteRefreshedAt:  formatRefreshTime(refreshedAt),
	}
	if err := renderTemplate(w, "month.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleDay(w http.ResponseWriter, r *http.Request) {
	dayRaw := strings.TrimSpace(r.PathValue("date"))
	day, err := parseISODate(dayRaw)
	if err != nil {
		http.Error(w, "invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	localEntries, err := s.cache.LoadLocalRange(day, day)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	authErrorMsg := ""
	remoteEntries, refreshedAt, err := s.cache.LoadRemoteRange(r.Context(), day, day, false)
	if err != nil {
		authErrorMsg = fmt.Sprintf(
			"OnePoint session may have expired (%v). In a new terminal run: gohour auth login",
			err,
		)
		remoteEntries = nil
	}
	dayRows := BuildDailyView(localEntries, remoteEntries)
	row := DayRow{Date: day}
	if len(dayRows) > 0 {
		row = dayRows[0]
	}

	view := dayPageView{
		Title:             "gohour - day " + dayRaw,
		CurrentMonth:      day.Format("2006-01"),
		Day:               dayRaw,
		AuthErrorMsg:      authErrorMsg,
		DayRow:            row,
		RemoteRefreshedAt: formatRefreshTime(refreshedAt),
	}
	if err := renderTemplate(w, "day.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.config.Snapshot()
	view := configPageView{
		Title:        "gohour - config",
		CurrentMonth: time.Now().Format("2006-01"),
		Config:       cfg,
		Rules:        cfg.Rules,
	}
	if err := renderTemplate(w, "config.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handlePartialMonth returns just the month table rows as an HTML fragment
// (HTMX partial, Phase 2.1). The response includes OOB swaps for stats.
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

func (s *Server) handleAPIMonth(w http.ResponseWriter, r *http.Request) {
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
		// Local-only month refreshes should still succeed when remote auth is
		// unavailable, mirroring page rendering behavior.
		if refresh {
			http.Error(w, fmt.Sprintf("load remote worklogs: %v", err), http.StatusBadGateway)
			return
		}
		authErrorMsg = fmt.Sprintf(
			"OnePoint session may have expired (%v). In a new terminal run: gohour auth login",
			err,
		)
		remoteEntries = nil
	}

	rows, summary := buildMonthRows(monthStart, localEntries, remoteEntries)
	writeJSON(w, http.StatusOK, monthAPIResponse{
		Month:              monthRaw,
		Rows:               rows,
		TotalLocal:         summary.TotalLocalHours,
		TotalRemote:        summary.TotalRemoteHours,
		TotalLocalWorked:   summary.TotalLocalWorkedHours,
		TotalRemoteWorked:  summary.TotalRemoteWorkedHours,
		TotalWorkedDelta:   summary.TotalLocalWorkedHours - summary.TotalRemoteWorkedHours,
		TotalBillableDelta: summary.TotalDeltaHours,
		AuthErrorMsg:       authErrorMsg,
		RemoteRefreshedAt:  formatRefreshTime(refreshedAt),
	})
}

func (s *Server) handleAPIDay(w http.ResponseWriter, r *http.Request) {
	dayRaw := strings.TrimSpace(r.PathValue("date"))
	day, err := parseISODate(dayRaw)
	if err != nil {
		http.Error(w, "invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	localEntries, err := s.cache.LoadLocalRange(day, day)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	refresh := strings.TrimSpace(r.URL.Query().Get("refresh")) == "1"
	remoteEntries, refreshedAt, err := s.cache.LoadRemoteRange(r.Context(), day, day, refresh)
	if err != nil {
		http.Error(w, fmt.Sprintf("load remote worklogs: %v", err), http.StatusBadGateway)
		return
	}
	dayRows := BuildDailyView(localEntries, remoteEntries)
	row := DayRow{Date: day}
	if len(dayRows) > 0 {
		row = dayRows[0]
	}

	writeJSON(w, http.StatusOK, dayAPIResponse{
		Date:              row.Date.Format("2006-01-02"),
		LocalHours:        row.LocalHours,
		RemoteHours:       row.RemoteHours,
		LocalWorkedHours:  row.LocalWorkedHours,
		RemoteWorkedHours: row.RemoteWorkedHours,
		Entries:           row.Entries,
		RemoteRefreshedAt: formatRefreshTime(refreshedAt),
	})
}

func (s *Server) handleAPILookup(w http.ResponseWriter, r *http.Request) {
	refresh := strings.TrimSpace(r.URL.Query().Get("refresh")) == "1"

	snapshot, err := s.cache.LookupSnapshot(r.Context(), refresh)
	if err != nil {
		http.Error(w, fmt.Sprintf("load lookup snapshot: %v", err), http.StatusBadGateway)
		return
	}

	resp := lookupResponseFromSnapshot(snapshot)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, configResponseFromConfig(s.config.Snapshot()))
}

func (s *Server) handleAPIConfigPatch(w http.ResponseWriter, r *http.Request) {
	var body configPatchRequest
	if err := decodeJSON(r, &body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.OnePointURL == nil {
		http.Error(w, "onepointUrl is required", http.StatusBadRequest)
		return
	}
	nextURL := strings.TrimSpace(*body.OnePointURL)
	if err := validateOnePointURL(nextURL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := s.config.Update(func(next *config.Config) error {
		next.OnePoint.URL = nextURL
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, configResponseFromConfig(cfg))
}

func (s *Server) handleAPIRules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, rulePayloadsFromRules(s.config.Snapshot().Rules))
}

func (s *Server) handleAPIRuleCreate(w http.ResponseWriter, r *http.Request) {
	var body rulePayload
	if err := decodeJSON(r, &body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rule, err := ruleFromPayload(body, "", true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := s.config.Update(func(next *config.Config) error {
		if findRuleIndex(next.Rules, rule.Name) >= 0 {
			return errRuleDuplicate
		}
		next.Rules = append(next.Rules, rule)
		return nil
	})
	if err != nil {
		if errors.Is(err, errRuleDuplicate) {
			http.Error(w, "rule already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	index := findRuleIndex(cfg.Rules, rule.Name)
	writeJSON(w, http.StatusCreated, rulePayloadFromRule(cfg.Rules[index]))
}

func (s *Server) handleAPIRulePatch(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		http.Error(w, "rule name is required", http.StatusBadRequest)
		return
	}
	var body rulePayload
	if err := decodeJSON(r, &body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rule, err := ruleFromPayload(body, name, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := s.config.Update(func(next *config.Config) error {
		index := findRuleIndex(next.Rules, name)
		if index < 0 {
			return errRuleNotFound
		}
		if !sameRuleName(name, rule.Name) && findRuleIndex(next.Rules, rule.Name) >= 0 {
			return errRuleDuplicate
		}
		next.Rules[index] = rule
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, errRuleNotFound):
			http.Error(w, "rule not found", http.StatusNotFound)
		case errors.Is(err, errRuleDuplicate):
			http.Error(w, "rule already exists", http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	index := findRuleIndex(cfg.Rules, rule.Name)
	writeJSON(w, http.StatusOK, rulePayloadFromRule(cfg.Rules[index]))
}

func (s *Server) handleAPIRuleDelete(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		http.Error(w, "rule name is required", http.StatusBadRequest)
		return
	}

	_, err := s.config.Update(func(next *config.Config) error {
		index := findRuleIndex(next.Rules, name)
		if index < 0 {
			return errRuleNotFound
		}
		next.Rules = append(next.Rules[:index], next.Rules[index+1:]...)
		return nil
	})
	if err != nil {
		if errors.Is(err, errRuleNotFound) {
			http.Error(w, "rule not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAPIWorklogCreate(w http.ResponseWriter, r *http.Request) {
	var body worklogMutationRequest
	if err := decodeJSON(r, &body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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

	day := timeutil.StartOfDay(entry.StartDateTime)
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
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) handleAPIWorklogPatch(w http.ResponseWriter, r *http.Request) {
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

	var body worklogMutationRequest
	if err := decodeJSON(r, &body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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

	day := timeutil.StartOfDay(entry.StartDateTime)
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
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAPIWorklogDelete(w http.ResponseWriter, r *http.Request) {
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
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAPIImport(w http.ResponseWriter, r *http.Request) {
	formResult, err := s.parseAndRunImportForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer os.Remove(formResult.tmpPath)

	result := formResult.result

	skipSet := parseSkipIndicesSet(r.FormValue("skipIndices"))
	if len(skipSet) > 0 {
		filtered := make([]worklog.Entry, 0, len(result.Entries))
		for i, entry := range result.Entries {
			if !skipSet[i] {
				filtered = append(filtered, entry)
			}
		}
		result.Entries = filtered
	}

	skipOverlapping := parseBoolFormValue(r.FormValue("skipOverlapping"))
	forceOverlapping := parseBoolFormValue(r.FormValue("forceOverlapping"))
	if skipOverlapping && forceOverlapping {
		http.Error(w, "skipOverlapping and forceOverlapping cannot both be true", http.StatusBadRequest)
		return
	}

	s.createMu.Lock()
	defer s.createMu.Unlock()

	toInsert := result.Entries
	overlapsSkipped := 0
	duplicateCount := 0
	if len(result.Entries) > 0 {
		minDay := timeutil.StartOfDay(result.Entries[0].StartDateTime)
		maxDay := minDay
		for _, entry := range result.Entries[1:] {
			day := timeutil.StartOfDay(entry.StartDateTime)
			if day.Before(minDay) {
				minDay = day
			}
			if day.After(maxDay) {
				maxDay = day
			}
		}

		existingEntries, err := s.cache.LoadLocalRange(minDay, maxDay)
		if err != nil {
			http.Error(w, fmt.Sprintf("load local worklogs: %v", err), http.StatusInternalServerError)
			return
		}
		accepted := append([]worklog.Entry(nil), existingEntries...)
		clean := make([]worklog.Entry, 0, len(result.Entries))
		overlapEntries := make([]worklog.Entry, 0)
		overlapItems := make([]importOverlapItem, 0)

		for _, entry := range result.Entries {
			conflictType, existingID, hasConflict := detectLocalConflict(entry, accepted)
			if !hasConflict {
				clean = append(clean, entry)
				accepted = append(accepted, entry)
				continue
			}

			if conflictType == "duplicate" {
				duplicateCount++
				continue
			}
			if conflictType == "overlap" {
				overlapEntries = append(overlapEntries, entry)
				overlapItems = append(overlapItems, importOverlapItem{
					Date:       timeutil.StartOfDay(entry.StartDateTime).Format("2006-01-02"),
					Start:      entry.StartDateTime.Format("15:04"),
					End:        entry.EndDateTime.Format("15:04"),
					Project:    entry.Project,
					Activity:   entry.Activity,
					Skill:      entry.Skill,
					ExistingID: existingID,
				})
				if forceOverlapping {
					accepted = append(accepted, entry)
				}
				continue
			}
		}

		if len(overlapEntries) > 0 && !skipOverlapping && !forceOverlapping {
			writeJSON(w, http.StatusConflict, importConflictResponse{
				Error:      "overlapping entries detected",
				Overlaps:   overlapItems,
				CleanCount: len(clean),
				Duplicates: duplicateCount,
			})
			return
		}

		toInsert = clean
		if forceOverlapping {
			toInsert = append(toInsert, overlapEntries...)
		} else {
			overlapsSkipped = len(overlapEntries)
		}
	}

	inserted, err := s.store.InsertWorklogs(toInsert)
	if err != nil {
		http.Error(w, fmt.Sprintf("insert imported worklogs: %v", err), http.StatusInternalServerError)
		return
	}
	if err := s.persistImportRuleUpdate(formResult); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	reconcileWarning := ""
	if shouldAutoReconcileImport(formResult) && inserted > 0 {
		if from, to, ok := worklogRange(toInsert); ok {
			if _, err := s.autoReconcileImportedRange(r.Context(), from, to); err != nil {
				reconcileWarning = err.Error()
			}
		}
	}

	s.cache.InvalidateLocal()
	writeJSON(w, http.StatusOK, importResponse{
		FilesProcessed:   result.FilesProcessed,
		RowsRead:         result.RowsRead,
		RowsMapped:       result.RowsMapped,
		RowsSkipped:      result.RowsSkipped + duplicateCount + overlapsSkipped,
		RowsPersisted:    inserted,
		ReconcileWarning: reconcileWarning,
		OverlapsSkipped:  overlapsSkipped,
	})
}

func (s *Server) handleAPIImportPreview(w http.ResponseWriter, r *http.Request) {
	formResult, err := s.parseAndRunImportForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer os.Remove(formResult.tmpPath)

	result := formResult.result
	response := importPreviewResponse{
		RowsMapped:  result.RowsMapped,
		RowsSkipped: result.RowsSkipped,
		Entries:     make([]importPreviewEntry, 0, len(result.Entries)),
		Selection:   formResult.selection,
		Mappers:     importMapperNames(),
	}
	if formResult.matchedRule.FileTemplate != "" {
		matched := rulePayloadFromRule(formResult.matchedRule)
		response.MatchedRule = &matched
	}
	if snapshot, err := s.cache.LookupSnapshot(r.Context(), false); err == nil {
		lookup := lookupResponseFromSnapshot(snapshot)
		response.Lookup = &lookup
	}

	if len(result.Entries) == 0 {
		writeJSON(w, http.StatusOK, response)
		return
	}

	minDay := timeutil.StartOfDay(result.Entries[0].StartDateTime)
	maxDay := minDay
	for _, entry := range result.Entries[1:] {
		day := timeutil.StartOfDay(entry.StartDateTime)
		if day.Before(minDay) {
			minDay = day
		}
		if day.After(maxDay) {
			maxDay = day
		}
	}

	existingEntries, err := s.cache.LoadLocalRange(minDay, maxDay)
	if err != nil {
		http.Error(w, fmt.Sprintf("load local worklogs: %v", err), http.StatusInternalServerError)
		return
	}

	accepted := append([]worklog.Entry(nil), existingEntries...)
	for i, entry := range result.Entries {
		preview := importPreviewEntry{
			Index:        i,
			Date:         timeutil.StartOfDay(entry.StartDateTime).Format("2006-01-02"),
			Start:        entry.StartDateTime.Format("15:04"),
			End:          entry.EndDateTime.Format("15:04"),
			Project:      entry.Project,
			Activity:     entry.Activity,
			Skill:        entry.Skill,
			BillableMins: entry.Billable,
			DurationMins: max(0, int(entry.EndDateTime.Sub(entry.StartDateTime).Minutes())),
			Description:  entry.Description,
			Status:       "clean",
		}

		conflictType, existingID, hasConflict := detectLocalConflict(entry, accepted)
		if hasConflict {
			if conflictType == "duplicate" {
				preview.Status = "duplicate"
				preview.ConflictID = existingID
			} else if conflictType == "overlap" {
				preview.Status = "overlap"
				preview.ConflictID = existingID
			}
		}
		if preview.Status == "clean" {
			accepted = append(accepted, entry)
		}
		response.Entries = append(response.Entries, preview)
	}

	writeJSON(w, http.StatusOK, response)
}

// handleAPIImportRuleMatch resolves filename-only import defaults for file-pick prefill.
func (s *Server) handleAPIImportRuleMatch(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimSpace(r.URL.Query().Get("filename"))
	if filename == "" {
		http.Error(w, "filename query parameter is required", http.StatusBadRequest)
		return
	}

	cfg := s.config.Snapshot()
	matched := importer.MatchRuleByTemplate(filename, cfg.Rules)
	selection := rulePayloadFromRule(matched)
	if strings.TrimSpace(selection.Mapper) == "" {
		selection.Mapper = "epm"
	}

	var matchedPayload *rulePayload
	if matched.FileTemplate != "" {
		mp := rulePayloadFromRule(matched)
		matchedPayload = &mp
	}

	lookup := &lookupResponse{}
	if snapshot, err := s.cache.LookupSnapshot(r.Context(), false); err == nil {
		resp := lookupResponseFromSnapshot(snapshot)
		lookup = &resp
	}

	writeJSON(w, http.StatusOK, importRuleMatchResponse{
		MatchedRule: matchedPayload,
		Selection:   selection,
		Mappers:     importMapperNames(),
		Lookup:      lookup,
	})
}

func (s *Server) handleAPIDeleteMonthWorklogs(w http.ResponseWriter, r *http.Request) {
	monthRaw := strings.TrimSpace(r.PathValue("month"))
	if _, err := parseMonth(monthRaw); err != nil {
		http.Error(w, "invalid month format (expected YYYY-MM)", http.StatusBadRequest)
		return
	}

	deleted, err := s.store.DeleteWorklogsByMonth(monthRaw)
	if err != nil {
		http.Error(w, fmt.Sprintf("delete month worklogs: %v", err), http.StatusInternalServerError)
		return
	}

	s.cache.InvalidateLocal()
	writeJSON(w, http.StatusOK, map[string]int{"deleted": deleted})
}

func (s *Server) handleAPIDeleteMonthRemoteWorklogs(w http.ResponseWriter, r *http.Request) {
	monthRaw := strings.TrimSpace(r.PathValue("month"))
	monthStart, err := parseMonth(monthRaw)
	if err != nil {
		http.Error(w, "invalid month format (expected YYYY-MM)", http.StatusBadRequest)
		return
	}
	s.logAudit(auditRecord{
		Operation: "delete_remote_month",
		Scope:     "month",
		Target:    monthRaw,
		Outcome:   "attempt",
	})
	monthEnd := endOfMonth(monthStart)
	// Clear every calendar day in the month, not only days returned by
	// getFilteredWorklogs, because OnePoint can retain stale month totals for
	// days that no longer expose timerecord entries.
	days := rangeDays(monthStart, monthEnd)

	client := upstreamErrorClient{base: s.client}
	deleted := 0
	lockedDays := make([]string, 0)
	clearedDays := make([]time.Time, 0)
	for _, day := range days {
		dayKey := day.Format("2006-01-02")
		existing, err := client.GetDayWorklogs(r.Context(), day)
		if err != nil {
			s.logAudit(auditRecord{
				Operation:     "delete_remote_month",
				Scope:         "month",
				Target:        monthRaw,
				Deleted:       deleted,
				SkippedLocked: len(lockedDays),
				LockedDays:    append([]string(nil), lockedDays...),
				Outcome:       "error",
				Error:         fmt.Sprintf("load day %s: %v", dayKey, err),
			})
			http.Error(w, fmt.Sprintf("load existing day %s failed: %v", dayKey, err), http.StatusBadGateway)
			return
		}
		if submitter.CountLockedDayWorklogs(existing) > 0 {
			lockedDays = append(lockedDays, dayKey)
			continue
		}
		if _, err := client.PersistWorklogs(r.Context(), day, []onepoint.PersistWorklog{}); err != nil {
			s.logAudit(auditRecord{
				Operation:     "delete_remote_month",
				Scope:         "month",
				Target:        monthRaw,
				Deleted:       deleted,
				SkippedLocked: len(lockedDays),
				LockedDays:    append([]string(nil), lockedDays...),
				Outcome:       "error",
				Error:         fmt.Sprintf("clear day %s: %v", dayKey, err),
			})
			http.Error(w, fmt.Sprintf("clear remote day %s failed: %v", dayKey, err), http.StatusBadGateway)
			return
		}
		deleted += len(existing)
		clearedDays = append(clearedDays, day)
	}

	s.cache.InvalidateRemoteDays(clearedDays)
	s.logAudit(auditRecord{
		Operation:     "delete_remote_month",
		Scope:         "month",
		Target:        monthRaw,
		Deleted:       deleted,
		SkippedLocked: len(lockedDays),
		LockedDays:    append([]string(nil), lockedDays...),
		Outcome:       "success",
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted":       deleted,
		"skippedLocked": len(lockedDays),
		"lockedDays":    lockedDays,
	})
}

func (s *Server) handleAPICopyMonthRemote(w http.ResponseWriter, r *http.Request) {
	monthRaw := strings.TrimSpace(r.PathValue("month"))
	monthStart, err := parseMonth(monthRaw)
	if err != nil {
		http.Error(w, "invalid month format (expected YYYY-MM)", http.StatusBadRequest)
		return
	}
	monthEnd := endOfMonth(monthStart)

	snapshot, err := s.cache.LookupSnapshot(r.Context(), false)
	if err != nil {
		http.Error(w, fmt.Sprintf("load lookup snapshot: %v", err), http.StatusBadGateway)
		return
	}

	remoteEntries, _, err := s.cache.LoadRemoteRange(r.Context(), monthStart, monthEnd, false)
	if err != nil {
		http.Error(w, fmt.Sprintf("load remote worklogs: %v", err), http.StatusBadGateway)
		return
	}

	entries := make([]worklog.Entry, 0, len(remoteEntries))
	for _, item := range remoteEntries {
		day, err := onepoint.ParseDay(item.WorklogDate)
		if err != nil {
			continue
		}
		day = timeutil.StartOfDay(day)
		start := day.Add(time.Duration(item.StartTime) * time.Minute)
		end := day.Add(time.Duration(item.FinishTime) * time.Minute)
		if !end.After(start) {
			continue
		}

		entries = append(entries, worklog.Entry{
			StartDateTime: start,
			EndDateTime:   end,
			Billable:      item.Billable,
			Description:   strings.TrimSpace(item.Comment),
			Project:       lookupProjectName(snapshot, item.ProjectID),
			Activity:      lookupActivityName(snapshot, item.ActivityID),
			Skill:         lookupSkillName(snapshot, item.SkillID),
			SourceFormat:  "remote",
			SourceMapper:  "onepoint",
			SourceFile:    "onepoint-sync-" + monthRaw,
		})
	}

	existingLocal, err := s.cache.LoadLocalRange(monthStart, monthEnd)
	if err != nil {
		http.Error(w, fmt.Sprintf("load local worklogs: %v", err), http.StatusInternalServerError)
		return
	}

	filtered := make([]worklog.Entry, 0, len(entries))
	accepted := append([]worklog.Entry(nil), existingLocal...)
	for _, entry := range entries {
		if containsSameLocalWorklogKey(entry, accepted) {
			continue
		}
		filtered = append(filtered, entry)
		accepted = append(accepted, entry)
	}

	inserted, err := s.store.InsertWorklogs(filtered)
	if err != nil {
		http.Error(w, fmt.Sprintf("insert copied worklogs: %v", err), http.StatusInternalServerError)
		return
	}

	s.cache.InvalidateLocal()
	writeJSON(w, http.StatusOK, map[string]int{
		"copied": inserted,
		"total":  len(entries),
	})
}

// Backward-compatible alias for older "sync" endpoint name.
func (s *Server) handleAPISyncMonthRemote(w http.ResponseWriter, r *http.Request) {
	s.handleAPICopyMonthRemote(w, r)
}

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

func (s *Server) autoReconcileImportedRange(ctx context.Context, from, to time.Time) (*reconcile.Result, error) {
	allEntries, err := s.store.ListWorklogs()
	if err != nil {
		return nil, fmt.Errorf("list local worklogs: %w", err)
	}
	localEntries := make([]worklog.Entry, 0, len(allEntries))
	for _, entry := range allEntries {
		day := timeutil.StartOfDay(entry.StartDateTime)
		if day.Before(timeutil.StartOfDay(from)) || day.After(timeutil.StartOfDay(to)) {
			continue
		}
		localEntries = append(localEntries, entry)
	}
	if len(localEntries) == 0 {
		return &reconcile.Result{}, nil
	}

	remoteEntries, _, err := s.cache.LoadRemoteRange(ctx, from, to, true)
	if err != nil {
		return nil, fmt.Errorf("load remote range: %w", err)
	}

	remoteByDay := make(map[string][]onepoint.PersistWorklog)
	for _, item := range remoteEntries {
		day, parseErr := onepoint.ParseDay(item.WorklogDate)
		if parseErr != nil {
			continue
		}
		key := timeutil.StartOfDay(day).Format("2006-01-02")
		remoteByDay[key] = append(remoteByDay[key], item.ToPersistWorklog())
	}

	eligibleIDs := make(map[int64]struct{})
	for _, entry := range localEntries {
		if entry.ID <= 0 {
			continue
		}
		dayKey := timeutil.StartOfDay(entry.StartDateTime).Format("2006-01-02")
		if localEntryIsSynced(entry, remoteByDay[dayKey]) {
			continue
		}
		eligibleIDs[entry.ID] = struct{}{}
	}
	if len(eligibleIDs) == 0 {
		return &reconcile.Result{}, nil
	}

	return reconcile.RunForEligibleIDs(s.store, eligibleIDs)
}

func (s *Server) parseAndRunImportForm(r *http.Request) (importFormResult, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return importFormResult{}, fmt.Errorf("parse multipart form: %w", err)
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return importFormResult{}, fmt.Errorf("missing file upload")
	}
	defer file.Close()

	cfg := s.config.Snapshot()
	matchedRule := importer.MatchRuleByTemplate(header.Filename, cfg.Rules)
	mapperName := strings.TrimSpace(r.FormValue("mapper"))
	if mapperName == "" {
		mapperName = strings.TrimSpace(matchedRule.Mapper)
	}
	if mapperName == "" {
		mapperName = "epm"
	}
	mapper, err := importer.MapperByName(mapperName)
	if err != nil {
		return importFormResult{}, err
	}
	mapperName = mapper.Name()
	selection, err := importSelectionFromForm(r, mapperName, matchedRule)
	if err != nil {
		return importFormResult{}, err
	}

	tmp, err := os.CreateTemp("", tempUploadPattern(header.Filename))
	if err != nil {
		return importFormResult{}, fmt.Errorf("create temp upload: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := io.Copy(tmp, file); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return importFormResult{}, fmt.Errorf("save upload: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return importFormResult{}, fmt.Errorf("close upload temp file: %w", err)
	}

	result, err := importer.Run(
		[]string{tmpPath},
		"",
		mapper,
		cfg,
		importer.RunOptions{
			EPMProject:  selection.Project,
			EPMActivity: selection.Activity,
			EPMSkill:    selection.Skill,
		},
	)
	if err != nil {
		_ = os.Remove(tmpPath)
		return importFormResult{}, err
	}

	applyImportSelection(result.Entries, selection)

	return importFormResult{
		tmpPath:     tmpPath,
		result:      result,
		mapperName:  mapperName,
		matchedRule: matchedRule,
		selection:   selection,
		updateRule:  parseBoolFormValue(r.FormValue("updateRule")),
	}, nil
}

func shouldAutoReconcileImport(result importFormResult) bool {
	return strings.EqualFold(strings.TrimSpace(result.mapperName), "epm")
}

func worklogRange(entries []worklog.Entry) (time.Time, time.Time, bool) {
	if len(entries) == 0 {
		return time.Time{}, time.Time{}, false
	}
	minDay := timeutil.StartOfDay(entries[0].StartDateTime)
	maxDay := minDay
	for _, entry := range entries[1:] {
		day := timeutil.StartOfDay(entry.StartDateTime)
		if day.Before(minDay) {
			minDay = day
		}
		if day.After(maxDay) {
			maxDay = day
		}
	}
	return minDay, maxDay, true
}

func importSelectionFromForm(r *http.Request, mapperName string, matchedRule config.Rule) (rulePayload, error) {
	selection := rulePayloadFromRule(matchedRule)
	selection.Mapper = firstNonEmptyString(strings.TrimSpace(r.FormValue("mapper")), selection.Mapper, mapperName)
	selection.Mapper = strings.ToLower(strings.TrimSpace(selection.Mapper))
	selection.Project = firstNonEmptyString(strings.TrimSpace(r.FormValue("project")), selection.Project)
	selection.Activity = firstNonEmptyString(strings.TrimSpace(r.FormValue("activity")), selection.Activity)
	selection.Skill = firstNonEmptyString(strings.TrimSpace(r.FormValue("skill")), selection.Skill)
	selection.ProjectID = firstNonZeroInt64(parseInt64FormValue(r.FormValue("projectId")), selection.ProjectID)
	selection.ActivityID = firstNonZeroInt64(parseInt64FormValue(r.FormValue("activityId")), selection.ActivityID)
	selection.SkillID = firstNonZeroInt64(parseInt64FormValue(r.FormValue("skillId")), selection.SkillID)
	billableValue := strings.ToLower(strings.TrimSpace(r.FormValue("billable")))
	switch billableValue {
	case "billable":
		selection.Billable = boolValuePtr(true)
	case "non-billable":
		selection.Billable = boolValuePtr(false)
	case "":
		if selection.Billable == nil {
			selection.Billable = boolValuePtr(true)
		}
	default:
		return rulePayload{}, fmt.Errorf("invalid billable value: %q (expected billable or non-billable)", billableValue)
	}
	return selection, nil
}

func applyImportSelection(entries []worklog.Entry, selection rulePayload) {
	for i := range entries {
		if strings.TrimSpace(selection.Project) != "" {
			entries[i].Project = selection.Project
		}
		if strings.TrimSpace(selection.Activity) != "" {
			entries[i].Activity = selection.Activity
		}
		if strings.TrimSpace(selection.Skill) != "" {
			entries[i].Skill = selection.Skill
		}
		if selection.Billable != nil {
			if *selection.Billable {
				entries[i].Billable = max(0, int(entries[i].EndDateTime.Sub(entries[i].StartDateTime).Minutes()))
			} else {
				entries[i].Billable = 0
			}
		}
	}
}

func (s *Server) persistImportRuleUpdate(result importFormResult) error {
	if !result.updateRule {
		return nil
	}
	if result.matchedRule.FileTemplate == "" {
		return fmt.Errorf("updateRule requires a matched rule")
	}
	rule, err := ruleFromPayload(result.selection, result.matchedRule.Name, true)
	if err != nil {
		return fmt.Errorf("update import rule: %w", err)
	}
	if rule.FileTemplate == "" {
		rule.FileTemplate = result.matchedRule.FileTemplate
	}

	_, err = s.config.Update(func(next *config.Config) error {
		index := findRuleIndex(next.Rules, result.matchedRule.Name)
		if index < 0 {
			return errRuleNotFound
		}
		next.Rules[index] = rule
		return nil
	})
	if err != nil {
		return fmt.Errorf("update import rule: %w", err)
	}
	return nil
}

func (s *Server) writeMutationConflictIfAny(w http.ResponseWriter, r *http.Request, entry worklog.Entry, existingEntries []worklog.Entry, ignoreID int64) bool {
	filtered := make([]worklog.Entry, 0, len(existingEntries))
	for _, item := range existingEntries {
		if ignoreID > 0 && item.ID == ignoreID {
			continue
		}
		filtered = append(filtered, item)
	}

	conflictType, conflictID, hasConflict := detectLocalConflict(entry, filtered)
	if !hasConflict {
		return false
	}
	if conflictType == "duplicate" {
		writeJSON(w, http.StatusConflict, worklogConflictResponse{
			Error:      "worklog duplicate with existing local entry",
			Type:       "duplicate",
			ExistingID: conflictID,
		})
		return true
	}
	if conflictType == "overlap" && r.Header.Get("X-Force-Overlap") != "1" {
		writeJSON(w, http.StatusConflict, worklogConflictResponse{
			Error:      "worklog overlaps existing local entry",
			Type:       "overlap",
			ExistingID: conflictID,
		})
		return true
	}
	return false
}

func importMapperNames() []string {
	return []string{"epm", "generic", "atwork"}
}

func submitErrorStatus(err error) int {
	if errors.Is(err, errOnePointUpstream) {
		return http.StatusBadGateway
	}
	return http.StatusInternalServerError
}

func tempUploadPattern(filename string) string {
	base := filepath.Base(strings.TrimSpace(filename))
	if base == "" || base == "." {
		return "upload-*"
	}

	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		stem = "upload"
	}
	if ext == "" {
		return stem + "-*"
	}
	return stem + "-*" + ext
}
