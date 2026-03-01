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

	lookupMu      sync.Mutex
	lookupSnap    *onepoint.LookupSnapshot
	lookupFetched bool
}

type monthRowView struct {
	Date        string
	LocalHours  float64
	RemoteHours float64
	DeltaHours  float64
	DayLink     string
}

type monthPageView struct {
	Title          string
	CurrentMonth   string
	PreviousMonth  string
	NextMonth      string
	Rows           []monthRowView
	TotalLocal     float64
	TotalRemote    float64
	TotalDelta     float64
	HasActiveRange bool
}

type dayPageView struct {
	Title        string
	CurrentMonth string
	Day          string
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
	FilesProcessed int `json:"filesProcessed"`
	RowsRead       int `json:"rowsRead"`
	RowsMapped     int `json:"rowsMapped"`
	RowsSkipped    int `json:"rowsSkipped"`
	RowsPersisted  int `json:"rowsPersisted"`
}

type submitDayResult struct {
	Date       string `json:"date"`
	Added      int    `json:"added"`
	Duplicates int    `json:"duplicates"`
	Overlaps   int    `json:"overlaps"`
	Locked     bool   `json:"locked"`
}

type submitResponse struct {
	Submitted  int               `json:"submitted"`
	Duplicates int               `json:"duplicates"`
	Overlaps   int               `json:"overlaps"`
	LockedDays []string          `json:"lockedDays"`
	Days       []submitDayResult `json:"days"`
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
	mux.HandleFunc("POST /api/submit/day/{date}", server.handleAPISubmitDay)
	mux.HandleFunc("POST /api/submit/month/{month}", server.handleAPISubmitMonth)
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
	remoteEntries, err := s.loadRemoteRange(r.Context(), monthStart, monthEnd)
	if err != nil {
		http.Error(w, fmt.Sprintf("load remote worklogs: %v", err), http.StatusBadGateway)
		return
	}

	dayRows := BuildDailyView(localEntries, remoteEntries)
	dayRows = fillMonthDays(monthStart, dayRows)
	summary := BuildMonthlyView(dayRows)

	rows := make([]monthRowView, 0, len(summary.Days))
	for _, day := range summary.Days {
		dayISO := day.Date.Format("2006-01-02")
		rows = append(rows, monthRowView{
			Date:        dayISO,
			LocalHours:  day.LocalHours,
			RemoteHours: day.RemoteHours,
			DeltaHours:  day.DeltaHours,
			DayLink:     "/day/" + dayISO,
		})
	}

	view := monthPageView{
		Title:         "gohour - month " + monthRaw,
		CurrentMonth:  monthRaw,
		PreviousMonth: monthStart.AddDate(0, -1, 0).Format("2006-01"),
		NextMonth:     monthStart.AddDate(0, 1, 0).Format("2006-01"),
		Rows:          rows,
		TotalLocal:    summary.TotalLocalHours,
		TotalRemote:   summary.TotalRemoteHours,
		TotalDelta:    summary.TotalDeltaHours,
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

	view := dayPageView{
		Title:        "gohour - day " + dayRaw,
		CurrentMonth: day.Format("2006-01"),
		Day:          dayRaw,
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
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, fmt.Sprintf("parse multipart form: %v", err), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	mapperName := strings.TrimSpace(r.FormValue("mapper"))
	if mapperName == "" {
		mapperName = "epm"
	}
	mapper, err := importer.MapperByName(mapperName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tmp, err := os.CreateTemp("", tempUploadPattern(header.Filename))
	if err != nil {
		http.Error(w, fmt.Sprintf("create temp upload: %v", err), http.StatusInternalServerError)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, file); err != nil {
		_ = tmp.Close()
		http.Error(w, fmt.Sprintf("save upload: %v", err), http.StatusInternalServerError)
		return
	}
	if err := tmp.Close(); err != nil {
		http.Error(w, fmt.Sprintf("close upload temp file: %v", err), http.StatusInternalServerError)
		return
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	inserted, err := s.store.InsertWorklogs(result.Entries)
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
		FilesProcessed: result.FilesProcessed,
		RowsRead:       result.RowsRead,
		RowsMapped:     result.RowsMapped,
		RowsSkipped:    result.RowsSkipped,
		RowsPersisted:  inserted,
	})
}

func (s *Server) handleAPISubmitDay(w http.ResponseWriter, r *http.Request) {
	dayRaw := strings.TrimSpace(r.PathValue("date"))
	day, err := parseISODate(dayRaw)
	if err != nil {
		http.Error(w, "invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	resp, err := s.submitRange(r.Context(), day, day)
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

	resp, err := s.submitRange(r.Context(), monthStart, endOfMonth(monthStart))
	if err != nil {
		http.Error(w, err.Error(), submitErrorStatus(err))
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) submitRange(ctx context.Context, from, to time.Time) (submitResponse, error) {
	response := submitResponse{
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
		dayResult.Duplicates = duplicates
		dayResult.Overlaps = len(overlaps)
		response.Duplicates += duplicates
		response.Overlaps += len(overlaps)

		if len(toAdd) > 0 {
			payload := make([]onepoint.PersistWorklog, 0, len(existingPayload)+len(toAdd))
			payload = append(payload, existingPayload...)
			payload = append(payload, toAdd...)

			if _, err := client.PersistWorklogs(ctx, batch.Day, payload); err != nil {
				return response, fmt.Errorf("submit day %s failed: %w", dayLabel, err)
			}
			dayResult.Added = len(toAdd)
			response.Submitted += len(toAdd)
			submittedDays = append(submittedDays, batch.Day)
		}

		response.Days = append(response.Days, dayResult)
	}

	s.invalidateRemoteDays(submittedDays)
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

	sort.Slice(out, func(i, j int) bool {
		left, leftErr := onepoint.ParseDay(out[i].WorklogDate)
		right, rightErr := onepoint.ParseDay(out[j].WorklogDate)
		if leftErr == nil && rightErr == nil && !left.Equal(right) {
			return left.Before(right)
		}
		if out[i].StartTime == out[j].StartTime {
			return out[i].FinishTime < out[j].FinishTime
		}
		return out[i].StartTime < out[j].StartTime
	})
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

	start := day.Add(time.Duration(startMinutes) * time.Minute)
	end := day.Add(time.Duration(endMinutes) * time.Minute)

	return worklog.Entry{
		StartDateTime: start,
		EndDateTime:   end,
		Billable:      body.Billable,
		Description:   strings.TrimSpace(body.Description),
		Project:       strings.TrimSpace(body.Project),
		Activity:      strings.TrimSpace(body.Activity),
		Skill:         strings.TrimSpace(body.Skill),
	}, nil
}

func parseClockMinutes(value string) (int, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
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
