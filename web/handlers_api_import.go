package web

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/riadshalaby/gohour/importer"
	"github.com/riadshalaby/gohour/internal/timeutil"
	"github.com/riadshalaby/gohour/worklog"
)

func (s *Server) handleAPIImport(w http.ResponseWriter, r *http.Request) {
	formResult, err := s.imports.ParseAndRunForm(r)
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
	if err := s.imports.PersistRuleUpdate(formResult); err != nil {
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
	formResult, err := s.imports.ParseAndRunForm(r)
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
