package web

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/riadshalaby/gohour/internal/timeutil"
	"github.com/riadshalaby/gohour/storage"
	"github.com/riadshalaby/gohour/worklog"
)

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
