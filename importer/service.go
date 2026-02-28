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
		reader, err := readerForMapper(mapperName, sourceFormat)
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
			if !cfgForFile.ImportBillable {
				entry.Billable = 0
			}
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

// mapperNeedsRuleConfig returns true for mappers that require project/activity/skill
// to be supplied via rule config or CLI flags (rather than from CSV columns).
func mapperNeedsRuleConfig(mapperName string) bool {
	switch strings.ToLower(mapperName) {
	case "epm", "atwork":
		return true
	default:
		return false
	}
}

func resolveConfigForFile(path, mapperName string, cfg config.Config, options RunOptions) (config.Config, error) {
	resolved := cfg
	resolved.ImportBillable = true // default

	rule := MatchRuleByTemplate(path, cfg.Rules)
	resolved.ImportBillable = rule.IsBillable()

	if !mapperNeedsRuleConfig(mapperName) {
		return resolved, nil
	}

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
			"no matching rule for file %s; missing explicit values for: %s (set --project/--activity/--skill or add rules in config)",
			path,
			strings.Join(missing, ", "),
		)
	}

	return resolved, fmt.Errorf(
		"rule %q matched file %s but is missing values for: %s",
		rule.FileTemplate,
		path,
		strings.Join(missing, ", "),
	)
}

func MatchRuleByTemplate(path string, rules []config.Rule) config.Rule {
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
	return config.Rule{}
}

// readerForMapper returns a specialized reader when the mapper requires a
// non-standard file format (e.g. atwork uses UTF-16 TSV). For all other
// mappers it falls back to the format-based reader selection.
func readerForMapper(mapperName, sourceFormat string) (Reader, error) {
	if strings.EqualFold(mapperName, "atwork") {
		return &ATWorkReader{}, nil
	}
	return ReaderForFormat(sourceFormat)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
