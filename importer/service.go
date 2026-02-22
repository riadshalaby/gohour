package importer

import (
	"fmt"
	"gohour/config"
	"gohour/worklog"
	"path/filepath"
	"strings"
)

type Result struct {
	FilesProcessed int
	RowsRead       int
	RowsMapped     int
	RowsSkipped    int
	Entries        []worklog.Entry
}

type RunOptions struct {
	EPMProject  string
	EPMActivity string
	EPMSkill    string
}

func Run(paths []string, format string, mapper Mapper, cfg config.Config, options RunOptions) (*Result, error) {
	result := &Result{Entries: make([]worklog.Entry, 0, 256)}
	mapperName := mapper.Name()
	for _, path := range paths {
		sourceFormat, err := inferFormat(path, format)
		if err != nil {
			return nil, err
		}
		reader, err := ReaderForFormat(sourceFormat)
		if err != nil {
			return nil, err
		}

		records, err := reader.Read(path)
		if err != nil {
			return nil, err
		}

		cfgForFile, err := resolveConfigForFile(path, mapperName, cfg, options)
		if err != nil {
			return nil, err
		}

		result.FilesProcessed++
		result.RowsRead += len(records)
		for _, record := range records {
			entry, ok, mapErr := mapper.Map(record, cfgForFile, sourceFormat, path)
			if mapErr != nil {
				return nil, mapErr
			}
			if !ok || entry == nil {
				result.RowsSkipped++
				continue
			}

			result.RowsMapped++
			entry.SourceMapper = mapperName
			result.Entries = append(result.Entries, *entry)
		}
	}

	return result, nil
}

func inferFormat(path string, format string) (string, error) {
	if strings.TrimSpace(format) != "" {
		return format, nil
	}

	extension := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch extension {
	case "csv":
		return "csv", nil
	case "xlsx", "xlsm", "xls":
		return "excel", nil
	default:
		return "", fmt.Errorf("unsupported file extension for %s", path)
	}
}

func resolveConfigForFile(path, mapperName string, cfg config.Config, options RunOptions) (config.Config, error) {
	resolved := cfg
	if !strings.EqualFold(mapperName, "epm") {
		return resolved, nil
	}

	rule := matchEPMRule(path, cfg.EPM.Rules)
	resolved.ImportProject = firstNonEmpty(options.EPMProject, rule.Project)
	resolved.ImportActivity = firstNonEmpty(options.EPMActivity, rule.Activity)
	resolved.ImportSkill = firstNonEmpty(options.EPMSkill, rule.Skill)

	missing := make([]string, 0, 3)
	if strings.TrimSpace(resolved.ImportProject) == "" {
		missing = append(missing, "project")
	}
	if strings.TrimSpace(resolved.ImportActivity) == "" {
		missing = append(missing, "activity")
	}
	if strings.TrimSpace(resolved.ImportSkill) == "" {
		missing = append(missing, "skill")
	}
	if len(missing) == 0 {
		return resolved, nil
	}

	if rule.FileTemplate == "" {
		return resolved, fmt.Errorf(
			"no matching EPM rule for file %s and missing explicit values for: %s (set --project/--activity/--skill or add epm.rules in config)",
			path,
			strings.Join(missing, ", "),
		)
	}

	return resolved, fmt.Errorf(
		"EPM rule %q matched file %s but is missing values for: %s",
		rule.FileTemplate,
		path,
		strings.Join(missing, ", "),
	)
}

func matchEPMRule(path string, rules []config.EPMRule) config.EPMRule {
	baseName := filepath.Base(path)
	for _, rule := range rules {
		template := strings.TrimSpace(rule.FileTemplate)
		if template == "" {
			continue
		}
		matchesBase, err := filepath.Match(template, baseName)
		if err == nil && matchesBase {
			return rule
		}
		matchesFull, err := filepath.Match(template, path)
		if err == nil && matchesFull {
			return rule
		}
	}
	return config.EPMRule{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
