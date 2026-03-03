// Package web serves a localhost-only single-user UI; it intentionally has no
// auth/CSRF protection in this mode.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gohour/config"
	"gohour/importer"
	"gohour/internal/timeutil"
	"gohour/onepoint"
	"gohour/reconcile"
	"gohour/storage"
	"gohour/submitter"
	"gohour/worklog"
)

//go:embed templates/*.html
var templateFS embed.FS

type Server struct {
	store  *storage.SQLiteStore
	client onepoint.Client
	cfg    config.Config

	submitOptions onepoint.ResolveOptions
	mux           *http.ServeMux

	mu          sync.RWMutex
	dayCache    map[string][]onepoint.DayWorklog
	dayFetched  map[string]bool
	localByDay  map[string][]worklog.Entry
	localLoaded bool

	remoteFetchMu sync.Mutex
	localLoadMu   sync.Mutex
	createMu      sync.Mutex

	lookupMu      sync.Mutex
	lookupSnap    *onepoint.LookupSnapshot
	lookupFetched bool
}

type monthRowView struct {
	Date         string
	IsWeekend    bool
	IsToday      bool
	LocalHours   float64
	RemoteHours  float64
	LocalWorked  float64
	RemoteWorked float64
	DeltaHours   float64
	DayLink      string
}

type monthPageView struct {
	Title             string
	CurrentMonth      string
	PreviousMonth     string
	NextMonth         string
	AuthErrorMsg      string
	Rows              []monthRowView
	TotalLocal        float64
	TotalRemote       float64
	TotalLocalWorked  float64
	TotalRemoteWorked float64
	TotalDelta        float64
}

type dayPageView struct {
	Title        string
	CurrentMonth string
	Day          string
	AuthErrorMsg string
	DayRow       DayRow
}

type worklogMutationRequest struct {
	Start       string `json:"start"`
	End         string `json:"end"`
	Project     string `json:"project"`
	Activity    string `json:"activity"`
	Skill       string `json:"skill"`
	Billable    int    `json:"billable"`
	Description string `json:"description"`
	Date        string `json:"date"`
}

type importResponse struct {
	FilesProcessed  int `json:"filesProcessed"`
	RowsRead        int `json:"rowsRead"`
	RowsMapped      int `json:"rowsMapped"`
	RowsSkipped     int `json:"rowsSkipped"`
	RowsPersisted   int `json:"rowsPersisted"`
	OverlapsSkipped int `json:"overlapsSkipped,omitempty"`
}

type importPreviewEntry struct {
	Index        int    `json:"index"`
	Date         string `json:"date"`
	Start        string `json:"start"`
	End          string `json:"end"`
	Project      string `json:"project"`
	Activity     string `json:"activity"`
	Skill        string `json:"skill"`
	BillableMins int    `json:"billableMins"`
	DurationMins int    `json:"durationMins"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	ConflictID   int64  `json:"conflictId,omitempty"`
}

type importPreviewResponse struct {
	RowsMapped  int                  `json:"rowsMapped"`
	RowsSkipped int                  `json:"rowsSkipped"`
	Entries     []importPreviewEntry `json:"entries"`
}

type importFormResult struct {
	tmpPath string
	result  *importer.Result
}

type importOverlapItem struct {
	Date       string `json:"date"`
	Start      string `json:"start"`
	End        string `json:"end"`
	Project    string `json:"project"`
	Activity   string `json:"activity"`
	Skill      string `json:"skill"`
	ExistingID int64  `json:"existingId"`
}

type importConflictResponse struct {
	Error      string              `json:"error"`
	Overlaps   []importOverlapItem `json:"overlaps"`
	CleanCount int                 `json:"cleanCount"`
	Duplicates int                 `json:"duplicates"`
}

type submitDayResult struct {
	Date       string `json:"date"`
	Added      int    `json:"added"`
	Duplicates int    `json:"duplicates"`
	Overlaps   int    `json:"overlaps"`
	Locked     bool   `json:"locked"`
}

type submitResponse struct {
	DryRun     bool              `json:"dryRun,omitempty"`
	Submitted  int               `json:"submitted"`
	Duplicates int               `json:"duplicates"`
	Overlaps   int               `json:"overlaps"`
	LockedDays []string          `json:"lockedDays"`
	Days       []submitDayResult `json:"days"`
}

type worklogConflictResponse struct {
	Error      string `json:"error"`
	Type       string `json:"type"`
	ExistingID int64  `json:"existingId"`
}

type lookupProject struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}

type lookupActivity struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	ProjectID int64  `json:"projectId"`
	Locked    bool   `json:"locked"`
}

type lookupSkill struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	ActivityID int64  `json:"activityId"`
}

type lookupResponse struct {
	Projects   []lookupProject  `json:"projects"`
	Activities []lookupActivity `json:"activities"`
	Skills     []lookupSkill    `json:"skills"`
}

var errOnePointUpstream = errors.New("onepoint upstream error")

type upstreamErrorClient struct {
	base onepoint.Client
}

func NewServer(store *storage.SQLiteStore, client onepoint.Client, cfg config.Config) http.Handler {
	server := &Server{
		store:      store,
		client:     client,
		cfg:        cfg,
		dayCache:   make(map[string][]onepoint.DayWorklog),
		dayFetched: make(map[string]bool),
		localByDay: make(map[string][]worklog.Entry),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /month", server.handleMonthPicker)
	mux.HandleFunc("GET /month/{month}", server.handleMonth)
	mux.HandleFunc("GET /day/{date}", server.handleDay)
	mux.HandleFunc("GET /api/day/{date}", server.handleAPIDay)
	mux.HandleFunc("GET /api/lookup", server.handleAPILookup)
	mux.HandleFunc("POST /api/worklog", server.handleAPIWorklogCreate)
	mux.HandleFunc("PATCH /api/worklog/{id}", server.handleAPIWorklogPatch)
	mux.HandleFunc("DELETE /api/worklog/{id}", server.handleAPIWorklogDelete)
	mux.HandleFunc("POST /api/import", server.handleAPIImport)
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

	localEntries, err := s.loadLocalRange(monthStart, monthEnd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	authErrorMsg := ""
	remoteEntries, err := s.loadRemoteRange(r.Context(), monthStart, monthEnd)
	if err != nil {
		authErrorMsg = fmt.Sprintf(
			"OnePoint session may have expired (%v). In a new terminal run: gohour auth login",
			err,
		)
		remoteEntries = nil
	}

	dayRows := BuildDailyView(localEntries, remoteEntries)
	dayRows = fillMonthDays(monthStart, dayRows)
	summary := BuildMonthlyView(dayRows)

	now := timeutil.StartOfDay(time.Now())
	rows := make([]monthRowView, 0, len(summary.Days))
	for _, day := range summary.Days {
		dayDate := timeutil.StartOfDay(day.Date)
		dayISO := dayDate.Format("2006-01-02")
		wd := dayDate.Weekday()
		rows = append(rows, monthRowView{
			Date:         dayISO,
			IsWeekend:    wd == time.Saturday || wd == time.Sunday,
			IsToday:      dayDate.Equal(now),
			LocalHours:   day.LocalHours,
			RemoteHours:  day.RemoteHours,
			LocalWorked:  day.LocalWorkedHours,
			RemoteWorked: day.RemoteWorkedHours,
			DeltaHours:   day.DeltaHours,
			DayLink:      "/day/" + dayISO,
		})
	}

	view := monthPageView{
		Title:             "gohour - month " + monthRaw,
		CurrentMonth:      monthRaw,
		PreviousMonth:     monthStart.AddDate(0, -1, 0).Format("2006-01"),
		NextMonth:         monthStart.AddDate(0, 1, 0).Format("2006-01"),
		AuthErrorMsg:      authErrorMsg,
		Rows:              rows,
		TotalLocal:        summary.TotalLocalHours,
		TotalRemote:       summary.TotalRemoteHours,
		TotalLocalWorked:  summary.TotalLocalWorkedHours,
		TotalRemoteWorked: summary.TotalRemoteWorkedHours,
		TotalDelta:        summary.TotalDeltaHours,
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

	localEntries, err := s.loadLocalRange(day, day)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	authErrorMsg := ""
	remoteEntries, err := s.loadRemoteRange(r.Context(), day, day)
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
		Title:        "gohour - day " + dayRaw,
		CurrentMonth: day.Format("2006-01"),
		Day:          dayRaw,
		AuthErrorMsg: authErrorMsg,
		DayRow:       row,
	}
	if err := renderTemplate(w, "day.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleAPIDay(w http.ResponseWriter, r *http.Request) {
	dayRaw := strings.TrimSpace(r.PathValue("date"))
	day, err := parseISODate(dayRaw)
	if err != nil {
		http.Error(w, "invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	localEntries, err := s.loadLocalRange(day, day)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	remoteEntries, err := s.loadRemoteRange(r.Context(), day, day)
	if err != nil {
		http.Error(w, fmt.Sprintf("load remote worklogs: %v", err), http.StatusBadGateway)
		return
	}
	dayRows := BuildDailyView(localEntries, remoteEntries)
	row := DayRow{Date: day}
	if len(dayRows) > 0 {
		row = dayRows[0]
	}

	writeJSON(w, http.StatusOK, row)
}

func (s *Server) handleAPILookup(w http.ResponseWriter, r *http.Request) {
	refresh := strings.TrimSpace(r.URL.Query().Get("refresh")) == "1"

	snapshot, err := s.loadLookupSnapshot(r.Context(), refresh)
	if err != nil {
		http.Error(w, fmt.Sprintf("load lookup snapshot: %v", err), http.StatusBadGateway)
		return
	}

	resp := lookupResponse{
		Projects:   make([]lookupProject, 0, len(snapshot.Projects)),
		Activities: make([]lookupActivity, 0, len(snapshot.Activities)),
		Skills:     make([]lookupSkill, 0, len(snapshot.Skills)),
	}
	for _, p := range snapshot.Projects {
		resp.Projects = append(resp.Projects, lookupProject{
			ID:       p.ID,
			Name:     p.Name,
			Archived: p.IsArchived(),
		})
	}
	for _, a := range snapshot.Activities {
		resp.Activities = append(resp.Activities, lookupActivity{
			ID:        a.ID,
			Name:      a.Name,
			ProjectID: a.ProjectNodeID,
			Locked:    a.Locked,
		})
	}
	for _, sk := range snapshot.Skills {
		resp.Skills = append(resp.Skills, lookupSkill{
			ID:         sk.SkillID,
			Name:       sk.Name,
			ActivityID: sk.ActivityID,
		})
	}

	writeJSON(w, http.StatusOK, resp)
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
	existingEntries, err := s.loadLocalRange(day, day)
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

	s.invalidateLocalCache()
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
	existingEntries, err := s.loadLocalRange(day, day)
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

	s.invalidateLocalCache()
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

	s.invalidateLocalCache()
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

		existingEntries, err := s.loadLocalRange(minDay, maxDay)
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

	if s.cfg.Import.AutoReconcileAfterImport {
		if _, err := reconcile.Run(s.store); err != nil {
			http.Error(w, fmt.Sprintf("reconcile imported worklogs: %v", err), http.StatusInternalServerError)
			return
		}
	}

	s.invalidateLocalCache()
	writeJSON(w, http.StatusOK, importResponse{
		FilesProcessed:  result.FilesProcessed,
		RowsRead:        result.RowsRead,
		RowsMapped:      result.RowsMapped,
		RowsSkipped:     result.RowsSkipped + duplicateCount + overlapsSkipped,
		RowsPersisted:   inserted,
		OverlapsSkipped: overlapsSkipped,
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

	existingEntries, err := s.loadLocalRange(minDay, maxDay)
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

	s.invalidateLocalCache()
	writeJSON(w, http.StatusOK, map[string]int{"deleted": deleted})
}

func (s *Server) handleAPIDeleteMonthRemoteWorklogs(w http.ResponseWriter, r *http.Request) {
	monthRaw := strings.TrimSpace(r.PathValue("month"))
	monthStart, err := parseMonth(monthRaw)
	if err != nil {
		http.Error(w, "invalid month format (expected YYYY-MM)", http.StatusBadRequest)
		return
	}
	monthEnd := endOfMonth(monthStart)

	remoteEntries, err := s.loadRemoteRange(r.Context(), monthStart, monthEnd)
	if err != nil {
		http.Error(w, fmt.Sprintf("load remote worklogs: %v", err), http.StatusBadGateway)
		return
	}

	dayCounts := make(map[string]int)
	dayByKey := make(map[string]time.Time)
	for _, item := range remoteEntries {
		day, err := onepoint.ParseDay(item.WorklogDate)
		if err != nil {
			continue
		}
		day = timeutil.StartOfDay(day)
		key := day.Format("2006-01-02")
		dayCounts[key]++
		dayByKey[key] = day
	}

	dayKeys := make([]string, 0, len(dayCounts))
	for key := range dayCounts {
		dayKeys = append(dayKeys, key)
	}
	sort.Strings(dayKeys)

	client := upstreamErrorClient{base: s.client}
	deleted := 0
	lockedDays := make([]string, 0)
	clearedDays := make([]time.Time, 0)
	for _, dayKey := range dayKeys {
		day := dayByKey[dayKey]
		existing, err := client.GetDayWorklogs(r.Context(), day)
		if err != nil {
			http.Error(w, fmt.Sprintf("load existing day %s failed: %v", dayKey, err), http.StatusBadGateway)
			return
		}
		if submitter.CountLockedDayWorklogs(existing) > 0 {
			lockedDays = append(lockedDays, dayKey)
			continue
		}
		if _, err := client.PersistWorklogs(r.Context(), day, []onepoint.PersistWorklog{}); err != nil {
			http.Error(w, fmt.Sprintf("clear remote day %s failed: %v", dayKey, err), http.StatusBadGateway)
			return
		}
		deleted += len(existing)
		clearedDays = append(clearedDays, day)
	}

	s.invalidateRemoteDays(clearedDays)
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

	snapshot, err := s.loadLookupSnapshot(r.Context(), false)
	if err != nil {
		http.Error(w, fmt.Sprintf("load lookup snapshot: %v", err), http.StatusBadGateway)
		return
	}

	remoteEntries, err := s.loadRemoteRange(r.Context(), monthStart, monthEnd)
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

	existingLocal, err := s.loadLocalRange(monthStart, monthEnd)
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

	s.invalidateLocalCache()
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
	resp, err := s.submitRange(r.Context(), day, day, dryRun)
	if err != nil {
		http.Error(w, err.Error(), submitErrorStatus(err))
		return
	}
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
	resp, err := s.submitRange(r.Context(), monthStart, endOfMonth(monthStart), dryRun)
	if err != nil {
		http.Error(w, err.Error(), submitErrorStatus(err))
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) submitRange(ctx context.Context, from, to time.Time, dryRun bool) (submitResponse, error) {
	response := submitResponse{
		DryRun:     dryRun,
		LockedDays: make([]string, 0),
		Days:       make([]submitDayResult, 0),
	}
	client := upstreamErrorClient{base: s.client}

	entries, err := s.loadLocalRange(from, to)
	if err != nil {
		return response, err
	}
	if len(entries) == 0 {
		return response, nil
	}

	idMap, err := submitter.ResolveIDsForEntries(ctx, client, s.cfg.Rules, entries, s.submitOptions)
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
		s.invalidateRemoteDays(submittedDays)
	}
	return response, nil
}

func (s *Server) loadLocalRange(from, to time.Time) ([]worklog.Entry, error) {
	if err := s.ensureLocalCache(); err != nil {
		return nil, err
	}

	filtered := make([]worklog.Entry, 0, 64)
	s.mu.RLock()
	for _, day := range rangeDays(from, to) {
		key := day.Format("2006-01-02")
		filtered = append(filtered, s.localByDay[key]...)
	}
	s.mu.RUnlock()
	return filtered, nil
}

func (s *Server) loadRemoteRange(ctx context.Context, from, to time.Time) ([]onepoint.DayWorklog, error) {
	days := rangeDays(from, to)
	if s.hasRemoteCacheMiss(days) {
		// Serialize miss handling so concurrent requests don't trigger duplicate fetches.
		s.remoteFetchMu.Lock()
		if s.hasRemoteCacheMiss(days) {
			loaded, err := s.client.GetFilteredWorklogs(ctx, from, to)
			if err != nil {
				s.remoteFetchMu.Unlock()
				return nil, err
			}
			byKey := make(map[string][]onepoint.DayWorklog, len(days))
			for _, day := range days {
				byKey[day.Format("2006-01-02")] = nil
			}
			for _, item := range loaded {
				parsed, err := onepoint.ParseDay(item.WorklogDate)
				if err != nil {
					continue
				}
				key := timeutil.StartOfDay(parsed).Format("2006-01-02")
				if _, ok := byKey[key]; !ok {
					continue
				}
				byKey[key] = append(byKey[key], item)
			}
			for key := range byKey {
				sortDayWorklogs(byKey[key])
			}

			s.mu.Lock()
			for _, day := range days {
				key := day.Format("2006-01-02")
				s.dayCache[key] = append([]onepoint.DayWorklog(nil), byKey[key]...)
				s.dayFetched[key] = true
			}
			s.mu.Unlock()
		}
		s.remoteFetchMu.Unlock()
	}

	out := make([]onepoint.DayWorklog, 0, 64)
	s.mu.RLock()
	for _, day := range days {
		key := day.Format("2006-01-02")
		out = append(out, s.dayCache[key]...)
	}
	s.mu.RUnlock()
	return out, nil
}

func (s *Server) ensureLocalCache() error {
	s.mu.RLock()
	loaded := s.localLoaded
	s.mu.RUnlock()
	if loaded {
		return nil
	}

	s.localLoadMu.Lock()
	defer s.localLoadMu.Unlock()

	s.mu.RLock()
	loaded = s.localLoaded
	s.mu.RUnlock()
	if loaded {
		return nil
	}

	allEntries, err := s.store.ListWorklogs()
	if err != nil {
		return fmt.Errorf("list local worklogs: %w", err)
	}

	index := make(map[string][]worklog.Entry, len(allEntries))
	for _, entry := range allEntries {
		key := timeutil.StartOfDay(entry.StartDateTime).Format("2006-01-02")
		index[key] = append(index[key], entry)
	}

	s.mu.Lock()
	s.localByDay = index
	s.localLoaded = true
	s.mu.Unlock()
	return nil
}

func (s *Server) hasRemoteCacheMiss(days []time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, day := range days {
		key := day.Format("2006-01-02")
		if !s.dayFetched[key] {
			return true
		}
	}
	return false
}

func (s *Server) invalidateLocalCache() {
	s.mu.Lock()
	s.localByDay = make(map[string][]worklog.Entry)
	s.localLoaded = false
	s.mu.Unlock()
}

func (s *Server) invalidateRemoteDays(days []time.Time) {
	if len(days) == 0 {
		return
	}

	s.mu.Lock()
	for _, day := range days {
		key := timeutil.StartOfDay(day).Format("2006-01-02")
		delete(s.dayCache, key)
		delete(s.dayFetched, key)
	}
	s.mu.Unlock()
}

func (s *Server) loadLookupSnapshot(ctx context.Context, refresh bool) (onepoint.LookupSnapshot, error) {
	if !refresh {
		s.lookupMu.Lock()
		if s.lookupFetched && s.lookupSnap != nil {
			snapshot := *s.lookupSnap
			s.lookupMu.Unlock()
			return snapshot, nil
		}
		s.lookupMu.Unlock()
	}

	snapshot, err := s.client.FetchLookupSnapshot(ctx)
	if err != nil {
		return onepoint.LookupSnapshot{}, err
	}

	s.lookupMu.Lock()
	s.lookupSnap = &snapshot
	s.lookupFetched = true
	s.lookupMu.Unlock()

	return snapshot, nil
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

	mapperName := strings.TrimSpace(r.FormValue("mapper"))
	if mapperName == "" {
		mapperName = "epm"
	}
	mapper, err := importer.MapperByName(mapperName)
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
		s.cfg,
		importer.RunOptions{
			EPMProject:  strings.TrimSpace(r.FormValue("project")),
			EPMActivity: strings.TrimSpace(r.FormValue("activity")),
			EPMSkill:    strings.TrimSpace(r.FormValue("skill")),
		},
	)
	if err != nil {
		_ = os.Remove(tmpPath)
		return importFormResult{}, err
	}

	billableMode := strings.TrimSpace(r.FormValue("billable"))
	if billableMode == "non-billable" {
		for i := range result.Entries {
			result.Entries[i].Billable = 0
		}
	}

	return importFormResult{tmpPath: tmpPath, result: result}, nil
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

func renderTemplate(w http.ResponseWriter, pageTemplate string, data any) error {
	tmpl, err := template.New("base.html").Funcs(template.FuncMap{
		"fmtHours": func(value float64) string {
			return fmt.Sprintf("%.2f", value)
		},
		"fmtDelta": func(value float64) string {
			return fmt.Sprintf("%+.2f", value)
		},
		"toMins": func(hours float64) int {
			return int(math.Round(hours * 60))
		},
	}).ParseFS(templateFS, "templates/base.html", "templates/"+pageTemplate)
	if err != nil {
		return fmt.Errorf("parse template %s: %w", pageTemplate, err)
	}
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		return fmt.Errorf("render template %s: %w", pageTemplate, err)
	}
	return nil
}

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

func detectLocalConflict(candidate worklog.Entry, existing []worklog.Entry) (conflictType string, existingID int64, ok bool) {
	for _, entry := range existing {
		if sameLocalWorklogKey(candidate, entry) {
			return "duplicate", entry.ID, true
		}
	}
	for _, entry := range existing {
		if timesOverlap(candidate.StartDateTime, candidate.EndDateTime, entry.StartDateTime, entry.EndDateTime) {
			return "overlap", entry.ID, true
		}
	}
	return "", 0, false
}

func sameLocalWorklogKey(left, right worklog.Entry) bool {
	return left.StartDateTime.Equal(right.StartDateTime) &&
		left.EndDateTime.Equal(right.EndDateTime) &&
		normalizeConflictName(left.Project) == normalizeConflictName(right.Project) &&
		normalizeConflictName(left.Activity) == normalizeConflictName(right.Activity) &&
		normalizeConflictName(left.Skill) == normalizeConflictName(right.Skill)
}

func containsSameLocalWorklogKey(candidate worklog.Entry, existing []worklog.Entry) bool {
	for _, item := range existing {
		if sameLocalWorklogKey(candidate, item) {
			return true
		}
	}
	return false
}

func timesOverlap(leftStart, leftEnd, rightStart, rightEnd time.Time) bool {
	return leftStart.Before(rightEnd) && leftEnd.After(rightStart)
}

func sortDayWorklogs(values []onepoint.DayWorklog) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].StartTime == values[j].StartTime {
			return values[i].FinishTime < values[j].FinishTime
		}
		return values[i].StartTime < values[j].StartTime
	})
}

func normalizeConflictName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
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

func lookupProjectName(snap onepoint.LookupSnapshot, id int64) string {
	for _, project := range snap.Projects {
		if project.ID == id {
			return project.Name
		}
	}
	return fmt.Sprintf("id:%d", id)
}

func lookupActivityName(snap onepoint.LookupSnapshot, id int64) string {
	for _, activity := range snap.Activities {
		if activity.ID == id {
			return activity.Name
		}
	}
	return fmt.Sprintf("id:%d", id)
}

func lookupSkillName(snap onepoint.LookupSnapshot, id int64) string {
	for _, skill := range snap.Skills {
		if skill.SkillID == id {
			return skill.Name
		}
	}
	return fmt.Sprintf("id:%d", id)
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

func decodeJSON(r *http.Request, out any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("request body must contain a single JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func submitErrorStatus(err error) int {
	if errors.Is(err, errOnePointUpstream) {
		return http.StatusBadGateway
	}
	return http.StatusInternalServerError
}

func wrapUpstreamError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", errOnePointUpstream, err)
}

func (c upstreamErrorClient) ListProjects(ctx context.Context) ([]onepoint.Project, error) {
	values, err := c.base.ListProjects(ctx)
	return values, wrapUpstreamError(err)
}

func (c upstreamErrorClient) ListActivities(ctx context.Context) ([]onepoint.Activity, error) {
	values, err := c.base.ListActivities(ctx)
	return values, wrapUpstreamError(err)
}

func (c upstreamErrorClient) ListSkills(ctx context.Context) ([]onepoint.Skill, error) {
	values, err := c.base.ListSkills(ctx)
	return values, wrapUpstreamError(err)
}

func (c upstreamErrorClient) GetFilteredWorklogs(ctx context.Context, from, to time.Time) ([]onepoint.DayWorklog, error) {
	values, err := c.base.GetFilteredWorklogs(ctx, from, to)
	return values, wrapUpstreamError(err)
}

func (c upstreamErrorClient) GetDayWorklogs(ctx context.Context, day time.Time) ([]onepoint.DayWorklog, error) {
	values, err := c.base.GetDayWorklogs(ctx, day)
	return values, wrapUpstreamError(err)
}

func (c upstreamErrorClient) PersistWorklogs(ctx context.Context, day time.Time, worklogs []onepoint.PersistWorklog) ([]onepoint.PersistResult, error) {
	values, err := c.base.PersistWorklogs(ctx, day, worklogs)
	return values, wrapUpstreamError(err)
}

func (c upstreamErrorClient) FetchLookupSnapshot(ctx context.Context) (onepoint.LookupSnapshot, error) {
	value, err := c.base.FetchLookupSnapshot(ctx)
	return value, wrapUpstreamError(err)
}

func (c upstreamErrorClient) ResolveIDs(ctx context.Context, projectName, activityName, skillName string, options onepoint.ResolveOptions) (onepoint.ResolvedIDs, error) {
	value, err := c.base.ResolveIDs(ctx, projectName, activityName, skillName, options)
	return value, wrapUpstreamError(err)
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

func endOfMonth(monthStart time.Time) time.Time {
	return monthStart.AddDate(0, 1, -1)
}

func rangeDays(from, to time.Time) []time.Time {
	out := make([]time.Time, 0, 32)
	for day := timeutil.StartOfDay(from); !day.After(to); day = day.AddDate(0, 0, 1) {
		out = append(out, day)
	}
	return out
}

func fillMonthDays(monthStart time.Time, rows []DayRow) []DayRow {
	index := make(map[string]DayRow, len(rows))
	for _, row := range rows {
		index[timeutil.StartOfDay(row.Date).Format("2006-01-02")] = row
	}

	monthEnd := endOfMonth(monthStart)
	out := make([]DayRow, 0, monthEnd.Day())
	for day := monthStart; !day.After(monthEnd); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		if row, ok := index[key]; ok {
			out = append(out, row)
			continue
		}
		out = append(out, DayRow{Date: day})
	}
	return out
}
