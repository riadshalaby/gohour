// Package web serves a localhost-only single-user UI; it intentionally has no
// auth/CSRF protection in this mode.
package web

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/riadshalaby/gohour/config"
	"github.com/riadshalaby/gohour/internal/timeutil"
	"github.com/riadshalaby/gohour/onepoint"
	"github.com/riadshalaby/gohour/reconcile"
	"github.com/riadshalaby/gohour/storage"
	"github.com/riadshalaby/gohour/worklog"
)

type Server struct {
	store  *storage.SQLiteStore
	client onepoint.Client

	submitOptions onepoint.ResolveOptions
	audit         auditLogger
	cache         *dataCache
	config        *configStore
	imports       *importService
	mux           *http.ServeMux

	createMu sync.Mutex
}

var (
	errRuleDuplicate = errors.New("rule already exists")
	errRuleNotFound  = errors.New("rule not found")
)

func NewServer(store *storage.SQLiteStore, client onepoint.Client, cfg config.Config) http.Handler {
	configStore := newConfigStore(cfg)
	server := &Server{
		store:   store,
		client:  client,
		audit:   newFileAuditLogger(defaultAuditLogPath()),
		cache:   newDataCache(store, client),
		config:  configStore,
		imports: newImportService(store, configStore),
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
