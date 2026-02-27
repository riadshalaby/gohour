package cmd

import (
	"context"
	"fmt"
	"gohour/config"
	"gohour/onepoint"
	"gohour/storage"
	"gohour/worklog"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	submitDBPath                  string
	submitURL                     string
	submitStateFile               string
	submitTimeout                 time.Duration
	submitFromDay                 string
	submitToDay                   string
	submitDryRun                  bool
	submitIncludeArchived         bool
	submitIncludeLockedActivities bool
)

var submitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit worklogs from SQLite to OnePoint",
	Long: `Read normalized worklogs from SQLite, resolve OnePoint IDs, and submit them via OnePoint REST API.

The command groups local rows by day, merges them with existing remote day entries, and persists the result.
Authentication uses session cookies from auth state JSON (created by "gohour auth login").`,
	Example: `
  # Submit all local worklogs from the default DB
  gohour submit

  # Submit only a date range (inclusive)
  gohour submit --from 2026-03-01 --to 2026-03-31

  # Dry-run: resolve and build payloads without sending
  gohour submit --dry-run

  # Override OnePoint URL and auth state location
  gohour submit --url https://onepoint.virtual7.io/onepoint/faces/home --state-file ~/.gohour/onepoint-auth-state.json
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadAndValidate()
		if err != nil {
			return err
		}

		baseURL, homeURL, host, err := resolveOnePointURLs(submitURL)
		if err != nil {
			return err
		}

		stateFile, err := resolveDefaultAuthStatePath(submitStateFile)
		if err != nil {
			return err
		}
		cookieHeader, err := onepoint.SessionCookieHeaderFromStateFile(stateFile, host)
		if err != nil {
			return fmt.Errorf("extract session cookies: %w", err)
		}

		store, err := storage.OpenSQLite(submitDBPath)
		if err != nil {
			return err
		}
		defer store.Close()

		allEntries, err := store.ListWorklogs()
		if err != nil {
			return err
		}
		if len(allEntries) == 0 {
			return fmt.Errorf("no worklogs found in %s", submitDBPath)
		}

		from, to, err := parseSubmitRange(submitFromDay, submitToDay)
		if err != nil {
			return err
		}
		entries := filterEntriesByDayRange(allEntries, from, to)
		if len(entries) == 0 {
			return fmt.Errorf("no worklogs matched the selected date range")
		}

		client, err := onepoint.NewClient(onepoint.ClientConfig{
			BaseURL:        baseURL,
			RefererURL:     homeURL,
			SessionCookies: cookieHeader,
			UserAgent:      "gohour-submit/1.0",
		})
		if err != nil {
			return err
		}

		resolveCtx, cancelResolve := context.WithTimeout(context.Background(), submitTimeout)
		defer cancelResolve()
		idMap, err := resolveIDsForEntries(resolveCtx, client, cfg.EPM.Rules, entries, onepoint.ResolveOptions{
			IncludeArchivedProjects: submitIncludeArchived,
			IncludeLockedActivities: submitIncludeLockedActivities,
		})
		if err != nil {
			return err
		}

		dayBatches, err := buildSubmitDayBatches(entries, idMap)
		if err != nil {
			return err
		}
		if len(dayBatches) == 0 {
			return fmt.Errorf("no valid day batches to submit")
		}

		totalLocal := 0
		for _, batch := range dayBatches {
			totalLocal += len(batch.Worklogs)
		}

		if submitDryRun {
			fmt.Printf("Submit dry-run completed. Days: %d, Local entries prepared: %d\n", len(dayBatches), totalLocal)
			for _, batch := range dayBatches {
				fmt.Printf("  %s -> %d local entries\n", onepoint.FormatDay(batch.Day), len(batch.Worklogs))
			}
			return nil
		}

		totalResponses := 0
		for _, batch := range dayBatches {
			dayCtx, cancelDay := context.WithTimeout(context.Background(), submitTimeout)
			results, submitErr := client.MergeAndPersistWorklogs(dayCtx, batch.Day, batch.Worklogs)
			cancelDay()
			if submitErr != nil {
				return fmt.Errorf("submit day %s failed: %w", onepoint.FormatDay(batch.Day), submitErr)
			}

			totalResponses += len(results)
			fmt.Printf(
				"Submitted day %s. Local entries: %d, Persist responses: %d\n",
				onepoint.FormatDay(batch.Day),
				len(batch.Worklogs),
				len(results),
			)
		}

		fmt.Printf(
			"Submit completed. Days: %d, Local entries submitted: %d, Persist responses: %d\n",
			len(dayBatches),
			totalLocal,
			totalResponses,
		)
		return nil
	},
}

type submitDayBatch struct {
	Day      time.Time
	Worklogs []onepoint.PersistWorklog
}

type submitNameTuple struct {
	Project  string
	Activity string
	Skill    string
}

type submitResolvedIDs struct {
	ProjectID  int64
	ActivityID int64
	SkillID    int64
}

func init() {
	rootCmd.AddCommand(submitCmd)

	submitCmd.Flags().StringVar(&submitDBPath, "db", "./gohour.db", "Path to local SQLite database")
	submitCmd.Flags().StringVar(&submitURL, "url", "", "Override OnePoint URL from config (full home URL)")
	submitCmd.Flags().StringVar(&submitStateFile, "state-file", "", "Path to auth state JSON (default: $HOME/.gohour/onepoint-auth-state.json)")
	submitCmd.Flags().DurationVar(&submitTimeout, "timeout", 60*time.Second, "Timeout per OnePoint API operation")
	submitCmd.Flags().StringVar(&submitFromDay, "from", "", "Filter start day (inclusive), format YYYY-MM-DD")
	submitCmd.Flags().StringVar(&submitToDay, "to", "", "Filter end day (inclusive), format YYYY-MM-DD")
	submitCmd.Flags().BoolVar(&submitDryRun, "dry-run", false, "Build and validate submit payloads without sending")
	submitCmd.Flags().BoolVar(&submitIncludeArchived, "include-archived-projects", false, "Allow archived projects during name->ID lookup fallback")
	submitCmd.Flags().BoolVar(&submitIncludeLockedActivities, "include-locked-activities", false, "Allow locked activities during name->ID lookup fallback")
}

func parseSubmitRange(fromValue, toValue string) (*time.Time, *time.Time, error) {
	var from *time.Time
	var to *time.Time
	if strings.TrimSpace(fromValue) != "" {
		day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(fromValue), time.Local)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid --from value %q (expected YYYY-MM-DD)", fromValue)
		}
		normalized := startOfDay(day)
		from = &normalized
	}
	if strings.TrimSpace(toValue) != "" {
		day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(toValue), time.Local)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid --to value %q (expected YYYY-MM-DD)", toValue)
		}
		normalized := startOfDay(day)
		to = &normalized
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, nil, fmt.Errorf("invalid range: --from must be <= --to")
	}
	return from, to, nil
}

func filterEntriesByDayRange(entries []worklog.Entry, from, to *time.Time) []worklog.Entry {
	if from == nil && to == nil {
		return append([]worklog.Entry(nil), entries...)
	}

	out := make([]worklog.Entry, 0, len(entries))
	for _, entry := range entries {
		day := startOfDay(entry.StartDateTime)
		if from != nil && day.Before(*from) {
			continue
		}
		if to != nil && day.After(*to) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func resolveIDsForEntries(
	ctx context.Context,
	client *onepoint.HTTPClient,
	rules []config.EPMRule,
	entries []worklog.Entry,
	options onepoint.ResolveOptions,
) (map[submitNameTuple]submitResolvedIDs, error) {
	requiredTuples := collectRequiredNameTuples(entries)
	if len(requiredTuples) == 0 {
		return map[submitNameTuple]submitResolvedIDs{}, nil
	}

	ruleIDs := buildRuleIDMap(rules)
	resolved := make(map[submitNameTuple]submitResolvedIDs, len(requiredTuples))
	missing := make([]submitNameTuple, 0)

	for _, tuple := range requiredTuples {
		if ids, ok := ruleIDs[tuple]; ok {
			resolved[tuple] = ids
			continue
		}
		missing = append(missing, tuple)
	}

	if len(missing) == 0 {
		return resolved, nil
	}

	snapshot, err := client.FetchLookupSnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch onepoint lookup snapshot: %w", err)
	}

	for _, tuple := range missing {
		ids, err := onepoint.ResolveIDsFromSnapshot(snapshot, tuple.Project, tuple.Activity, tuple.Skill, options)
		if err != nil {
			return nil, fmt.Errorf(
				"resolve ids for project=%q activity=%q skill=%q: %w",
				tuple.Project,
				tuple.Activity,
				tuple.Skill,
				err,
			)
		}
		resolved[tuple] = submitResolvedIDs{
			ProjectID:  ids.ProjectID,
			ActivityID: ids.ActivityID,
			SkillID:    ids.SkillID,
		}
	}

	return resolved, nil
}

func collectRequiredNameTuples(entries []worklog.Entry) []submitNameTuple {
	unique := make(map[submitNameTuple]struct{}, len(entries))
	for _, entry := range entries {
		tuple := submitNameTuple{
			Project:  normalizeSubmitName(entry.Project),
			Activity: normalizeSubmitName(entry.Activity),
			Skill:    normalizeSubmitName(entry.Skill),
		}
		if tuple.Project == "" || tuple.Activity == "" || tuple.Skill == "" {
			continue
		}
		unique[tuple] = struct{}{}
	}

	out := make([]submitNameTuple, 0, len(unique))
	for tuple := range unique {
		out = append(out, tuple)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Project != out[j].Project {
			return out[i].Project < out[j].Project
		}
		if out[i].Activity != out[j].Activity {
			return out[i].Activity < out[j].Activity
		}
		return out[i].Skill < out[j].Skill
	})
	return out
}

func buildRuleIDMap(rules []config.EPMRule) map[submitNameTuple]submitResolvedIDs {
	out := make(map[submitNameTuple]submitResolvedIDs, len(rules))
	for _, rule := range rules {
		tuple := submitNameTuple{
			Project:  normalizeSubmitName(rule.Project),
			Activity: normalizeSubmitName(rule.Activity),
			Skill:    normalizeSubmitName(rule.Skill),
		}
		if tuple.Project == "" || tuple.Activity == "" || tuple.Skill == "" {
			continue
		}
		if rule.ProjectID <= 0 || rule.ActivityID <= 0 || rule.SkillID <= 0 {
			continue
		}
		if _, exists := out[tuple]; exists {
			continue
		}
		out[tuple] = submitResolvedIDs{
			ProjectID:  rule.ProjectID,
			ActivityID: rule.ActivityID,
			SkillID:    rule.SkillID,
		}
	}
	return out
}

func buildSubmitDayBatches(entries []worklog.Entry, idsByTuple map[submitNameTuple]submitResolvedIDs) ([]submitDayBatch, error) {
	sortedEntries := append([]worklog.Entry(nil), entries...)
	sort.Slice(sortedEntries, func(i, j int) bool {
		if sortedEntries[i].StartDateTime.Equal(sortedEntries[j].StartDateTime) {
			return sortedEntries[i].ID < sortedEntries[j].ID
		}
		return sortedEntries[i].StartDateTime.Before(sortedEntries[j].StartDateTime)
	})

	byDay := make(map[string]*submitDayBatch)
	dayKeys := make([]string, 0, 64)
	nextTempID := int64(-1)

	for _, entry := range sortedEntries {
		tuple := submitNameTuple{
			Project:  normalizeSubmitName(entry.Project),
			Activity: normalizeSubmitName(entry.Activity),
			Skill:    normalizeSubmitName(entry.Skill),
		}
		if tuple.Project == "" || tuple.Activity == "" || tuple.Skill == "" {
			return nil, fmt.Errorf("worklog id=%d has empty project/activity/skill values", entry.ID)
		}
		ids, ok := idsByTuple[tuple]
		if !ok {
			return nil, fmt.Errorf(
				"no resolved ids for worklog id=%d (project=%q, activity=%q, skill=%q)",
				entry.ID,
				tuple.Project,
				tuple.Activity,
				tuple.Skill,
			)
		}
		if ids.ProjectID <= 0 || ids.ActivityID <= 0 || ids.SkillID <= 0 {
			return nil, fmt.Errorf(
				"resolved ids must be > 0 for worklog id=%d (project=%d, activity=%d, skill=%d)",
				entry.ID,
				ids.ProjectID,
				ids.ActivityID,
				ids.SkillID,
			)
		}

		day := startOfDay(entry.StartDateTime)
		if !sameDay(entry.StartDateTime, entry.EndDateTime) {
			return nil, fmt.Errorf("worklog id=%d crosses day boundaries and cannot be submitted", entry.ID)
		}

		startMins := minutesFromMidnight(entry.StartDateTime)
		finishMins := minutesFromMidnight(entry.EndDateTime)
		duration := int(entry.EndDateTime.Sub(entry.StartDateTime).Minutes())
		if duration <= 0 || finishMins <= startMins {
			return nil, fmt.Errorf("worklog id=%d has invalid time range", entry.ID)
		}

		billable := entry.Billable
		if billable <= 0 {
			billable = duration
		}
		if billable <= 0 {
			return nil, fmt.Errorf("worklog id=%d has invalid billable value", entry.ID)
		}

		dayKey := onepoint.FormatDay(day)
		batch, exists := byDay[dayKey]
		if !exists {
			batch = &submitDayBatch{Day: day, Worklogs: make([]onepoint.PersistWorklog, 0, 32)}
			byDay[dayKey] = batch
			dayKeys = append(dayKeys, dayKey)
		}

		start := startMins
		finish := finishMins
		batch.Worklogs = append(batch.Worklogs, onepoint.PersistWorklog{
			TimeRecordID: nextTempID,
			WorkSlipID:   -1,
			WorkRecordID: -1,
			WorklogDate:  onepoint.FormatDay(day),
			StartTime:    &start,
			FinishTime:   &finish,
			Duration:     duration,
			Billable:     billable,
			Valuable:     0,
			ProjectID:    onepoint.ID(ids.ProjectID),
			ActivityID:   onepoint.ID(ids.ActivityID),
			SkillID:      onepoint.ID(ids.SkillID),
			Comment:      strings.TrimSpace(entry.Description),
		})
		nextTempID--
	}

	sort.Slice(dayKeys, func(i, j int) bool {
		left, _ := onepoint.ParseDay(dayKeys[i])
		right, _ := onepoint.ParseDay(dayKeys[j])
		return left.Before(right)
	})

	out := make([]submitDayBatch, 0, len(dayKeys))
	for _, key := range dayKeys {
		out = append(out, *byDay[key])
	}
	return out, nil
}

func normalizeSubmitName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func startOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func minutesFromMidnight(value time.Time) int {
	return value.Hour()*60 + value.Minute()
}
