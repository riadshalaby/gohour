package web

import (
	"fmt"
	"net/http"
	"strings"
)

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
