package submitter

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"gohour/config"
	"gohour/internal/timeutil"
	"gohour/onepoint"
	"gohour/worklog"
)

type DayBatch struct {
	Day      time.Time
	Worklogs []onepoint.PersistWorklog
}

type NameTuple struct {
	Mapper   string
	Project  string
	Activity string
	Skill    string
}

type ResolvedIDs struct {
	ProjectID  int64
	ActivityID int64
	SkillID    int64
}

func CollectRequiredNameTuples(entries []worklog.Entry) ([]NameTuple, error) {
	unique := make(map[NameTuple]struct{}, len(entries))
	for _, entry := range entries {
		tuple := NameTuple{
			Mapper:   normalizeMapper(entry.SourceMapper),
			Project:  normalizeName(entry.Project),
			Activity: normalizeName(entry.Activity),
			Skill:    normalizeName(entry.Skill),
		}
		if tuple.Project == "" || tuple.Activity == "" || tuple.Skill == "" {
			return nil, fmt.Errorf(
				"worklog id=%d has empty project/activity/skill values and cannot resolve IDs",
				entry.ID,
			)
		}
		unique[tuple] = struct{}{}
	}

	out := make([]NameTuple, 0, len(unique))
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

func BuildRuleIDMap(rules []config.Rule) map[NameTuple]ResolvedIDs {
	out := make(map[NameTuple]ResolvedIDs, len(rules))
	for _, rule := range rules {
		tuple := NameTuple{
			Mapper:   normalizeMapper(rule.Mapper),
			Project:  normalizeName(rule.Project),
			Activity: normalizeName(rule.Activity),
			Skill:    normalizeName(rule.Skill),
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
		out[tuple] = ResolvedIDs{
			ProjectID:  rule.ProjectID,
			ActivityID: rule.ActivityID,
			SkillID:    rule.SkillID,
		}
	}
	return out
}

func ResolveIDsForEntries(
	ctx context.Context,
	client onepoint.Client,
	rules []config.Rule,
	entries []worklog.Entry,
	options onepoint.ResolveOptions,
) (map[NameTuple]ResolvedIDs, error) {
	requiredTuples, err := CollectRequiredNameTuples(entries)
	if err != nil {
		return nil, err
	}
	if len(requiredTuples) == 0 {
		return map[NameTuple]ResolvedIDs{}, nil
	}

	ruleIDs := BuildRuleIDMap(rules)
	resolved := make(map[NameTuple]ResolvedIDs, len(requiredTuples))
	missing := make([]NameTuple, 0)

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
		resolved[tuple] = ResolvedIDs{
			ProjectID:  ids.ProjectID,
			ActivityID: ids.ActivityID,
			SkillID:    ids.SkillID,
		}
	}

	return resolved, nil
}

func BuildDayBatches(entries []worklog.Entry, idsByTuple map[NameTuple]ResolvedIDs) ([]DayBatch, error) {
	sortedEntries := append([]worklog.Entry(nil), entries...)
	sort.Slice(sortedEntries, func(i, j int) bool {
		if sortedEntries[i].StartDateTime.Equal(sortedEntries[j].StartDateTime) {
			return sortedEntries[i].ID < sortedEntries[j].ID
		}
		return sortedEntries[i].StartDateTime.Before(sortedEntries[j].StartDateTime)
	})

	byDay := make(map[string]*DayBatch)
	dayKeys := make([]string, 0, 64)
	nextTempID := int64(-1)

	for _, entry := range sortedEntries {
		tuple := NameTuple{
			Mapper:   normalizeMapper(entry.SourceMapper),
			Project:  normalizeName(entry.Project),
			Activity: normalizeName(entry.Activity),
			Skill:    normalizeName(entry.Skill),
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

		day := timeutil.StartOfDay(entry.StartDateTime)
		if !timeutil.SameDay(entry.StartDateTime, entry.EndDateTime) {
			return nil, fmt.Errorf("worklog id=%d crosses day boundaries and cannot be submitted", entry.ID)
		}

		startMins := timeutil.MinutesFromMidnight(entry.StartDateTime)
		finishMins := timeutil.MinutesFromMidnight(entry.EndDateTime)
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
			batch = &DayBatch{Day: day, Worklogs: make([]onepoint.PersistWorklog, 0, 32)}
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

	out := make([]DayBatch, 0, len(dayKeys))
	for _, key := range dayKeys {
		out = append(out, *byDay[key])
	}
	return out, nil
}

func ClassifyWorklogs(local, existing []onepoint.PersistWorklog) (toAdd []onepoint.PersistWorklog, overlaps []onepoint.OverlapInfo, duplicates int) {
	toAdd = make([]onepoint.PersistWorklog, 0, len(local))
	overlaps = make([]onepoint.OverlapInfo, 0)
	duplicates = 0

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

func CountLockedDayWorklogs(existing []onepoint.DayWorklog) int {
	count := 0
	for _, item := range existing {
		if item.Locked != 0 {
			count++
		}
	}
	return count
}

func DayWorklogsToPersistPayload(existing []onepoint.DayWorklog) []onepoint.PersistWorklog {
	payload := make([]onepoint.PersistWorklog, 0, len(existing))
	for _, item := range existing {
		if item.Locked != 0 {
			continue
		}
		payload = append(payload, item.ToPersistWorklog())
	}
	return payload
}

func normalizeName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func normalizeMapper(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
