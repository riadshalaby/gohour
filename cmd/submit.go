package cmd

import (
	"bufio"
	"context"
	"fmt"
	"gohour/config"
	"gohour/internal/timeutil"
	"gohour/onepoint"
	"gohour/storage"
	"gohour/submitter"
	"gohour/worklog"
	"os"
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
  # Submit all local worklogs
  gohour submit
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

			lockedCount := submitter.CountLockedDayWorklogs(existing)
			if lockedCount > 0 {
				lockedDays = append(lockedDays, dayLabel)
				fmt.Printf("Warning: skipping day %s: %d locked entry/entries found - no changes made\n", dayLabel, lockedCount)
				continue
			}

			existingPayload := submitter.DayWorklogsToPersistPayload(existing)
			toAdd, overlaps, duplicates := submitter.ClassifyWorklogs(batch.Worklogs, existingPayload)
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

type submitDayBatch = submitter.DayBatch
type submitNameTuple = submitter.NameTuple
type submitResolvedIDs = submitter.ResolvedIDs

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
		normalized := timeutil.StartOfDay(day)
		from = &normalized
	}
	if strings.TrimSpace(toValue) != "" {
		day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(toValue), time.Local)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid --to value %q (expected YYYY-MM-DD)", toValue)
		}
		normalized := timeutil.StartOfDay(day)
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
		day := timeutil.StartOfDay(entry.StartDateTime)
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
	client onepoint.Client,
	rules []config.Rule,
	entries []worklog.Entry,
	options onepoint.ResolveOptions,
) (map[submitNameTuple]submitResolvedIDs, error) {
	return submitter.ResolveIDsForEntries(ctx, client, rules, entries, options)
}

func buildSubmitDayBatches(entries []worklog.Entry, idsByTuple map[submitNameTuple]submitResolvedIDs) ([]submitDayBatch, error) {
	return submitter.BuildDayBatches(entries, idsByTuple)
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
