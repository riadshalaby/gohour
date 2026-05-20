package web

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/riadshalaby/gohour/config"
	"github.com/riadshalaby/gohour/importer"
	"github.com/riadshalaby/gohour/internal/timeutil"
	"github.com/riadshalaby/gohour/storage"
	"github.com/riadshalaby/gohour/worklog"
)

type importService struct {
	store  *storage.SQLiteStore
	config *configStore
}

func newImportService(store *storage.SQLiteStore, cfg *configStore) *importService {
	return &importService{
		store:  store,
		config: cfg,
	}
}

func (s *importService) ParseAndRunForm(r *http.Request) (importFormResult, error) {
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

func (s *importService) PersistRuleUpdate(result importFormResult) error {
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

func importMapperNames() []string {
	return []string{"epm", "generic", "atwork"}
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
