package web

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/riadshalaby/gohour/onepoint"
)

var errOnePointUpstream = errors.New("onepoint upstream error")

type upstreamErrorClient struct {
	base onepoint.Client
}

func wrapUpstreamError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", errOnePointUpstream, err)
}

func (c upstreamErrorClient) ListProjects(ctx context.Context) ([]onepoint.Project, error) {
	values, err := c.base.ListProjects(ctx)
	return values, wrapUpstreamError(err)
}

func (c upstreamErrorClient) ListActivities(ctx context.Context) ([]onepoint.Activity, error) {
	values, err := c.base.ListActivities(ctx)
	return values, wrapUpstreamError(err)
}

func (c upstreamErrorClient) ListSkills(ctx context.Context) ([]onepoint.Skill, error) {
	values, err := c.base.ListSkills(ctx)
	return values, wrapUpstreamError(err)
}

func (c upstreamErrorClient) GetFilteredWorklogs(ctx context.Context, from, to time.Time) ([]onepoint.DayWorklog, error) {
	values, err := c.base.GetFilteredWorklogs(ctx, from, to)
	return values, wrapUpstreamError(err)
}

func (c upstreamErrorClient) GetDayWorklogs(ctx context.Context, day time.Time) ([]onepoint.DayWorklog, error) {
	values, err := c.base.GetDayWorklogs(ctx, day)
	return values, wrapUpstreamError(err)
}

func (c upstreamErrorClient) PersistWorklogs(ctx context.Context, day time.Time, worklogs []onepoint.PersistWorklog) ([]onepoint.PersistResult, error) {
	values, err := c.base.PersistWorklogs(ctx, day, worklogs)
	return values, wrapUpstreamError(err)
}

func (c upstreamErrorClient) FetchLookupSnapshot(ctx context.Context) (onepoint.LookupSnapshot, error) {
	value, err := c.base.FetchLookupSnapshot(ctx)
	return value, wrapUpstreamError(err)
}

func (c upstreamErrorClient) ResolveIDs(ctx context.Context, projectName, activityName, skillName string, options onepoint.ResolveOptions) (onepoint.ResolvedIDs, error) {
	value, err := c.base.ResolveIDs(ctx, projectName, activityName, skillName, options)
	return value, wrapUpstreamError(err)
}
