package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"gohour/onepoint"
	"gohour/storage"
	"gohour/worklog"
)

//go:embed templates/*.html
var templateFS embed.FS

type Server struct {
	store  *storage.SQLiteStore
	client onepoint.Client
	mux    *http.ServeMux

	mu          sync.RWMutex
	dayCache    map[string][]onepoint.DayWorklog
	dayFetched  map[string]bool
	localByDay  map[string][]worklog.Entry
	localLoaded bool

	remoteFetchMu sync.Mutex
	localLoadMu   sync.Mutex
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

func NewServer(store *storage.SQLiteStore, client onepoint.Client) http.Handler {
	server := &Server{
		store:      store,
		client:     client,
		dayCache:   make(map[string][]onepoint.DayWorklog),
		dayFetched: make(map[string]bool),
		localByDay: make(map[string][]worklog.Entry),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /month", server.handleMonthPicker)
	mux.HandleFunc("GET /month/{month}", server.handleMonth)
	mux.HandleFunc("GET /day/{date}", server.handleDay)
	mux.HandleFunc("GET /api/day/{date}", server.handleAPIDay)
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

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(row); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
				key := startOfDay(parsed).Format("2006-01-02")
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
		key := startOfDay(entry.StartDateTime).Format("2006-01-02")
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

func renderTemplate(w http.ResponseWriter, pageTemplate string, data any) error {
	tmpl, err := template.New("base.html").Funcs(template.FuncMap{
		"fmtHours": func(value float64) string {
			return fmt.Sprintf("%.2f", value)
		},
		"fmtDelta": func(value float64) string {
			return fmt.Sprintf("%+.2f", value)
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
	return startOfDay(parsed), nil
}

func parseISODate(value string) (time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.Local)
	if err != nil {
		return time.Time{}, err
	}
	return startOfDay(parsed), nil
}

func endOfMonth(monthStart time.Time) time.Time {
	return monthStart.AddDate(0, 1, -1)
}

func rangeDays(from, to time.Time) []time.Time {
	out := make([]time.Time, 0, 32)
	for day := startOfDay(from); !day.After(to); day = day.AddDate(0, 0, 1) {
		out = append(out, day)
	}
	return out
}

func fillMonthDays(monthStart time.Time, rows []DayRow) []DayRow {
	index := make(map[string]DayRow, len(rows))
	for _, row := range rows {
		index[startOfDay(row.Date).Format("2006-01-02")] = row
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
