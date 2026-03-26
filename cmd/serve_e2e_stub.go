package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/riadshalaby/gohour/config"
	"github.com/riadshalaby/gohour/onepoint"
)

const e2eStubRemoteEnv = "GOHOUR_E2E_STUB_REMOTE"

func buildServeClient(cfg config.Config) (onepoint.Client, error) {
	if strings.TrimSpace(os.Getenv(e2eStubRemoteEnv)) == "1" {
		return newServeE2EStubClient(cfg), nil
	}

	cookieHeader, baseURL, homeURL, host, stateFile, err := ensureAuthenticatedWithStateFile(serveURL, serveStateFile)
	if err != nil {
		return nil, err
	}

	_, err = retryWithRelogin(
		baseURL,
		homeURL,
		host,
		stateFile,
		"gohour-serve/1.0",
		&cookieHeader,
		func(client onepoint.Client) (struct{}, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			projects, err := client.ListProjects(ctx)
			if err != nil {
				return struct{}{}, err
			}
			if len(projects) == 0 {
				return struct{}{}, fmt.Errorf(
					"%w: ListProjects returned empty result (session may have expired)",
					onepoint.ErrAuthUnauthorized,
				)
			}
			return struct{}{}, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("validate OnePoint session: %w", err)
	}

	client, err := onepoint.NewClient(onepoint.ClientConfig{
		BaseURL:        baseURL,
		RefererURL:     homeURL,
		SessionCookies: cookieHeader,
		UserAgent:      "gohour-serve/1.0",
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

type serveE2EStubClient struct {
	snapshot onepoint.LookupSnapshot
}

func newServeE2EStubClient(cfg config.Config) onepoint.Client {
	projects := make([]onepoint.Project, 0, len(cfg.Rules))
	activities := make([]onepoint.Activity, 0, len(cfg.Rules))
	skills := make([]onepoint.Skill, 0, len(cfg.Rules))

	seenProjects := make(map[int64]struct{}, len(cfg.Rules))
	seenActivities := make(map[int64]struct{}, len(cfg.Rules))
	seenSkills := make(map[int64]struct{}, len(cfg.Rules))

	for _, rule := range cfg.Rules {
		projectName := strings.TrimSpace(rule.Project)
		activityName := strings.TrimSpace(rule.Activity)
		skillName := strings.TrimSpace(rule.Skill)
		if rule.ProjectID <= 0 || rule.ActivityID <= 0 || rule.SkillID <= 0 {
			continue
		}
		if projectName == "" || activityName == "" || skillName == "" {
			continue
		}

		if _, ok := seenProjects[rule.ProjectID]; !ok {
			projects = append(projects, onepoint.Project{
				ID:       rule.ProjectID,
				Name:     projectName,
				Archived: "0",
			})
			seenProjects[rule.ProjectID] = struct{}{}
		}
		if _, ok := seenActivities[rule.ActivityID]; !ok {
			activities = append(activities, onepoint.Activity{
				ID:            rule.ActivityID,
				Name:          activityName,
				ProjectNodeID: rule.ProjectID,
				Locked:        false,
			})
			seenActivities[rule.ActivityID] = struct{}{}
		}
		if _, ok := seenSkills[rule.SkillID]; !ok {
			skills = append(skills, onepoint.Skill{
				SkillID:    rule.SkillID,
				Name:       skillName,
				ActivityID: rule.ActivityID,
			})
			seenSkills[rule.SkillID] = struct{}{}
		}
	}

	sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
	sort.Slice(activities, func(i, j int) bool { return activities[i].ID < activities[j].ID })
	sort.Slice(skills, func(i, j int) bool { return skills[i].SkillID < skills[j].SkillID })

	return serveE2EStubClient{
		snapshot: onepoint.LookupSnapshot{
			Projects:   projects,
			Activities: activities,
			Skills:     skills,
		},
	}
}

func (c serveE2EStubClient) ListProjects(context.Context) ([]onepoint.Project, error) {
	return append([]onepoint.Project(nil), c.snapshot.Projects...), nil
}

func (c serveE2EStubClient) ListActivities(context.Context) ([]onepoint.Activity, error) {
	return append([]onepoint.Activity(nil), c.snapshot.Activities...), nil
}

func (c serveE2EStubClient) ListSkills(context.Context) ([]onepoint.Skill, error) {
	return append([]onepoint.Skill(nil), c.snapshot.Skills...), nil
}

func (c serveE2EStubClient) GetFilteredWorklogs(context.Context, time.Time, time.Time) ([]onepoint.DayWorklog, error) {
	return nil, errors.New("remote refresh unavailable in e2e stub")
}

func (c serveE2EStubClient) GetDayWorklogs(context.Context, time.Time) ([]onepoint.DayWorklog, error) {
	return []onepoint.DayWorklog{}, nil
}

func (c serveE2EStubClient) PersistWorklogs(_ context.Context, day time.Time, worklogs []onepoint.PersistWorklog) ([]onepoint.PersistResult, error) {
	results := make([]onepoint.PersistResult, 0, len(worklogs))
	for i, worklog := range worklogs {
		results = append(results, onepoint.PersistResult{
			Message:         "stubbed",
			MessageType:     "info",
			NewTimeRecordID: int64(i + 1),
			OldTimeRecordID: worklog.TimeRecordID,
			WorkRecordID:    worklog.WorkRecordID,
			WorkSlipID:      worklog.WorkSlipID,
			WorklogDate:     day.Format("02-01-2006"),
		})
	}
	return results, nil
}

func (c serveE2EStubClient) FetchLookupSnapshot(context.Context) (onepoint.LookupSnapshot, error) {
	return c.snapshot, nil
}

func (c serveE2EStubClient) ResolveIDs(_ context.Context, projectName, activityName, skillName string, options onepoint.ResolveOptions) (onepoint.ResolvedIDs, error) {
	return onepoint.ResolveIDsFromSnapshot(c.snapshot, projectName, activityName, skillName, options)
}
