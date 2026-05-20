package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/riadshalaby/gohour/internal/timeutil"
	"github.com/riadshalaby/gohour/onepoint"
	"github.com/riadshalaby/gohour/submitter"
	"github.com/riadshalaby/gohour/worklog"
)

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
