package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

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
