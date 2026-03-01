package cmd

import (
	"bufio"
	"context"
	"fmt"
	"gohour/config"
	"gohour/onepoint"
	"gohour/storage"
	"gohour/worklog"
	"os"
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

The command groups local rows by day and validates each day against existing remote entries.
For each day it:
- skips the full day if any remote entry is locked
- skips duplicates (same time + project/activity/skill)
- detects overlaps with existing entries
- prompts how to handle overlaps (write/skip/write-all/skip-all/abort), unless --dry-run is used

In --dry-run mode, remote day worklogs are still loaded to report locked days and overlaps,
but no persist call is made.
Authentication uses session cookies from auth state JSON (created by "gohour auth login").`,
	Example: `
  # Submit all local worklogs from the default DB
  gohour submit

  # Submit only a date range (inclusive)
  gohour submit --from 2026-03-01 --to 2026-03-31

  # Dry-run: validate against remote entries without writing
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
		idMap, err := resolveIDsForEntries(resolveCtx, client, cfg.Rules, entries, onepoint.ResolveOptions{
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

		totalResponses := 0
		totalDuplicates := 0
		totalOverlaps := 0
		lockedDays := make([]string, 0)
		globalSkipAllOverlaps := false
		globalWriteAllOverlaps := false

		if submitDryRun {
			fmt.Println("Submit dry-run mode: validating against existing OnePoint entries without persisting changes.")
		}

		for _, batch := range dayBatches {
			dayLabel := onepoint.FormatDay(batch.Day)

			dayCtx, cancelDay := context.WithTimeout(context.Background(), submitTimeout)
			existing, submitErr := client.GetDayWorklogs(dayCtx, batch.Day)
			cancelDay()
			if submitErr != nil {
				return fmt.Errorf("load existing day %s failed: %w", dayLabel, submitErr)
			}

			lockedCount := countLockedDayWorklogs(existing)
			if lockedCount > 0 {
				lockedDays = append(lockedDays, dayLabel)
				fmt.Printf("Warning: skipping day %s: %d locked entry/entries found - no changes made\n", dayLabel, lockedCount)
				continue
			}

			existingPayload := dayWorklogsToPersistPayload(existing)
			toAdd, overlaps, duplicates := classifySubmitWorklogs(batch.Worklogs, existingPayload)
			totalDuplicates += duplicates
			totalOverlaps += len(overlaps)

			approvedOverlaps, err := handleOverlaps(overlaps, submitDryRun, &globalSkipAllOverlaps, &globalWriteAllOverlaps)
			if err != nil {
				return err
			}
			toAdd = append(toAdd, approvedOverlaps...)

			if submitDryRun {
				fmt.Printf(
					"Dry-run day %s: local=%d duplicates=%d overlaps=%d ready=%d\n",
					dayLabel,
					len(batch.Worklogs),
					duplicates,
					len(overlaps),
					len(toAdd),
				)
				continue
			}

			if len(toAdd) == 0 {
				fmt.Printf("No new entries for day %s. Skipping persist.\n", dayLabel)
				continue
			}

			payload := make([]onepoint.PersistWorklog, 0, len(existingPayload)+len(toAdd))
			payload = append(payload, existingPayload...)
			payload = append(payload, toAdd...)

			dayCtx, cancelDay = context.WithTimeout(context.Background(), submitTimeout)
			results, err := client.PersistWorklogs(dayCtx, batch.Day, payload)
			cancelDay()
			if err != nil {
				return fmt.Errorf("submit day %s failed: %w", dayLabel, err)
			}

			totalResponses += len(results)
			fmt.Printf(
				"Submitted day %s. Local entries: %d, Added entries: %d, Persist responses: %d\n",
				dayLabel,
				len(batch.Worklogs),
				len(toAdd),
				len(results),
			)
		}

		if submitDryRun {
			fmt.Println("Dry-run summary:")
			fmt.Printf("  Days to submit:               %d\n", len(dayBatches))
			if len(lockedDays) > 0 {
				fmt.Printf("  Days skipped (locked):        %d  [%s]\n", len(lockedDays), strings.Join(lockedDays, ", "))
			} else {
				fmt.Printf("  Days skipped (locked):        %d\n", 0)
			}
			fmt.Printf("  Local entries prepared:       %d\n", totalLocal)
			fmt.Printf("  Duplicates (skipped):         %d\n", totalDuplicates)
			fmt.Printf("  Overlapping entries (warned): %d\n", totalOverlaps)
			return nil
		}

		fmt.Printf(
			"Submit completed. Days: %d, Local entries submitted: %d, Duplicates skipped: %d, Overlaps seen: %d, Persist responses: %d\n",
			len(dayBatches),
			totalLocal,
			totalDuplicates,
			totalOverlaps,
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
	Mapper   string
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
	submitCmd.Flags().BoolVar(&submitDryRun, "dry-run", false, "Validate against remote day worklogs without persisting (warns for locked days/overlaps)")
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
	rules []config.Rule,
	entries []worklog.Entry,
	options onepoint.ResolveOptions,
) (map[submitNameTuple]submitResolvedIDs, error) {
	requiredTuples, err := collectRequiredNameTuples(entries)
	if err != nil {
		return nil, err
	}
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
				"resolve ids for mapper=%q project=%q activity=%q skill=%q: %w",
				tuple.Mapper,
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

func collectRequiredNameTuples(entries []worklog.Entry) ([]submitNameTuple, error) {
	unique := make(map[submitNameTuple]struct{}, len(entries))
	for _, entry := range entries {
		tuple := submitNameTuple{
			Mapper:   normalizeSubmitMapper(entry.SourceMapper),
			Project:  normalizeSubmitName(entry.Project),
			Activity: normalizeSubmitName(entry.Activity),
			Skill:    normalizeSubmitName(entry.Skill),
		}
		if tuple.Project == "" || tuple.Activity == "" || tuple.Skill == "" {
			return nil, fmt.Errorf(
				"worklog id=%d has empty project/activity/skill values and cannot resolve IDs",
				entry.ID,
			)
		}
		unique[tuple] = struct{}{}
	}

	out := make([]submitNameTuple, 0, len(unique))
	for tuple := range unique {
		out = append(out, tuple)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Mapper != out[j].Mapper {
			return out[i].Mapper < out[j].Mapper
		}
		if out[i].Project != out[j].Project {
			return out[i].Project < out[j].Project
		}
		if out[i].Activity != out[j].Activity {
			return out[i].Activity < out[j].Activity
		}
		return out[i].Skill < out[j].Skill
	})
	return out, nil
}

func buildRuleIDMap(rules []config.Rule) map[submitNameTuple]submitResolvedIDs {
	out := make(map[submitNameTuple]submitResolvedIDs, len(rules))
	for _, rule := range rules {
		tuple := submitNameTuple{
			Mapper:   normalizeSubmitMapper(rule.Mapper),
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
			Mapper:   normalizeSubmitMapper(entry.SourceMapper),
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
				"no resolved ids for worklog id=%d (mapper=%q, project=%q, activity=%q, skill=%q)",
				entry.ID,
				tuple.Mapper,
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
		if billable < 0 {
			return nil, fmt.Errorf("worklog id=%d has negative billable value (%d)", entry.ID, billable)
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

func countLockedDayWorklogs(existing []onepoint.DayWorklog) int {
	count := 0
	for _, item := range existing {
		if item.Locked != 0 {
			count++
		}
	}
	return count
}

func dayWorklogsToPersistPayload(existing []onepoint.DayWorklog) []onepoint.PersistWorklog {
	payload := make([]onepoint.PersistWorklog, 0, len(existing))
	for _, item := range existing {
		if item.Locked != 0 {
			continue
		}
		payload = append(payload, item.ToPersistWorklog())
	}
	return payload
}

func classifySubmitWorklogs(local, existing []onepoint.PersistWorklog) ([]onepoint.PersistWorklog, []onepoint.OverlapInfo, int) {
	toAdd := make([]onepoint.PersistWorklog, 0, len(local))
	overlaps := make([]onepoint.OverlapInfo, 0)
	duplicates := 0

	for _, candidate := range local {
		isDuplicate := false
		for _, existingEntry := range existing {
			if onepoint.PersistWorklogsEquivalent(existingEntry, candidate) {
				isDuplicate = true
				break
			}
		}
		if isDuplicate {
			duplicates++
			continue
		}

		hasOverlap := false
		for _, existingEntry := range existing {
			if onepoint.WorklogTimeOverlaps(candidate, existingEntry) {
				overlaps = append(overlaps, onepoint.OverlapInfo{
					Local:    candidate,
					Existing: existingEntry,
				})
				hasOverlap = true
				break
			}
		}
		if hasOverlap {
			continue
		}

		toAdd = append(toAdd, candidate)
	}

	return toAdd, overlaps, duplicates
}

func handleOverlaps(
	overlaps []onepoint.OverlapInfo,
	dryRun bool,
	globalSkipAll *bool,
	globalWriteAll *bool,
) ([]onepoint.PersistWorklog, error) {
	if len(overlaps) == 0 {
		return nil, nil
	}

	if dryRun {
		for _, overlap := range overlaps {
			fmt.Printf(
				"Warning: local entry %s (ProjectID=%s) overlaps with existing %s\n",
				formatPersistWorklogRange(overlap.Local),
				formatFlexibleIDForDryRun(overlap.Local.ProjectID),
				formatPersistWorklogRange(overlap.Existing),
			)
		}
		return nil, nil
	}

	if globalSkipAll != nil && *globalSkipAll {
		return nil, nil
	}
	if globalWriteAll != nil && *globalWriteAll {
		return collectOverlapLocals(overlaps), nil
	}

	dayLabel := strings.TrimSpace(overlaps[0].Local.WorklogDate)
	if dayLabel == "" {
		dayLabel = "unknown day"
	}

	fmt.Printf("Warning: %d local entries overlap with existing OnePoint entries for %s:\n", len(overlaps), dayLabel)
	for i, overlap := range overlaps {
		fmt.Printf(
			"  [%d] %s %q overlaps with existing %s %q\n",
			i+1,
			formatPersistWorklogRange(overlap.Local),
			strings.TrimSpace(overlap.Local.Comment),
			formatPersistWorklogRange(overlap.Existing),
			strings.TrimSpace(overlap.Existing.Comment),
		)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("How to handle overlapping entries?")
		fmt.Println("  (w) Write overlapping entries anyway")
		fmt.Println("  (s) Skip overlapping entries")
		fmt.Println("  (W) Write ALL overlapping entries for all remaining days")
		fmt.Println("  (S) Skip ALL overlapping entries for all remaining days")
		fmt.Println("  (a) Abort submit")
		fmt.Print("Enter choice: ")

		input, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(input) == "" {
			return nil, fmt.Errorf("read overlap choice: %w", err)
		}

		switch strings.TrimSpace(input) {
		case "w":
			return collectOverlapLocals(overlaps), nil
		case "s":
			return nil, nil
		case "W":
			if globalWriteAll != nil {
				*globalWriteAll = true
			}
			return collectOverlapLocals(overlaps), nil
		case "S":
			if globalSkipAll != nil {
				*globalSkipAll = true
			}
			return nil, nil
		case "a":
			return nil, fmt.Errorf("submit aborted by user")
		default:
			fmt.Println("Invalid choice. Please enter one of: w, s, W, S, a")
		}
	}
}

func collectOverlapLocals(overlaps []onepoint.OverlapInfo) []onepoint.PersistWorklog {
	locals := make([]onepoint.PersistWorklog, 0, len(overlaps))
	for _, overlap := range overlaps {
		locals = append(locals, overlap.Local)
	}
	return locals
}

func normalizeSubmitName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func normalizeSubmitMapper(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
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

func formatDryRunWorklog(value onepoint.PersistWorklog) string {
	return fmt.Sprintf(
		"time=%s-%s duration=%d billable=%d projectId=%s activityId=%s skillId=%s comment=%q",
		formatMinutesForDryRun(value.StartTime),
		formatMinutesForDryRun(value.FinishTime),
		value.Duration,
		value.Billable,
		formatFlexibleIDForDryRun(value.ProjectID),
		formatFlexibleIDForDryRun(value.ActivityID),
		formatFlexibleIDForDryRun(value.SkillID),
		strings.TrimSpace(value.Comment),
	)
}

func formatPersistWorklogRange(value onepoint.PersistWorklog) string {
	return fmt.Sprintf("%s-%s", formatMinutesForDryRun(value.StartTime), formatMinutesForDryRun(value.FinishTime))
}

func formatMinutesForDryRun(value *int) string {
	if value == nil {
		return "?"
	}
	if *value < 0 {
		return fmt.Sprintf("%d", *value)
	}

	hour := *value / 60
	minute := *value % 60
	return fmt.Sprintf("%02d:%02d", hour, minute)
}

func formatFlexibleIDForDryRun(value onepoint.FlexibleInt64) string {
	if !value.Valid {
		return "<empty>"
	}
	return fmt.Sprintf("%d", value.Value)
}
