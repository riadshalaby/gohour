package web

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/riadshalaby/gohour/internal/timeutil"
	"github.com/riadshalaby/gohour/onepoint"
	"github.com/riadshalaby/gohour/storage"
	"github.com/riadshalaby/gohour/worklog"
)

type dataCache struct {
	store  *storage.SQLiteStore
	client onepoint.Client

	mu          sync.RWMutex
	dayCache    map[string][]onepoint.DayWorklog
	dayFetched  map[string]bool
	dayRefresh  map[string]time.Time
	localByDay  map[string][]worklog.Entry
	localLoaded bool

	remoteFetchMu sync.Mutex
	localLoadMu   sync.Mutex

	lookupMu      sync.Mutex
	lookupSnap    *onepoint.LookupSnapshot
	lookupFetched bool
}

func newDataCache(store *storage.SQLiteStore, client onepoint.Client) *dataCache {
	return &dataCache{
		store:      store,
		client:     client,
		dayCache:   make(map[string][]onepoint.DayWorklog),
		dayFetched: make(map[string]bool),
		dayRefresh: make(map[string]time.Time),
		localByDay: make(map[string][]worklog.Entry),
	}
}

func (c *dataCache) LoadLocalRange(from, to time.Time) ([]worklog.Entry, error) {
	if err := c.EnsureLocalCache(); err != nil {
		return nil, err
	}

	filtered := make([]worklog.Entry, 0, 64)
	c.mu.RLock()
	for _, day := range rangeDays(from, to) {
		key := day.Format("2006-01-02")
		filtered = append(filtered, c.localByDay[key]...)
	}
	c.mu.RUnlock()
	return filtered, nil
}

func (c *dataCache) LoadRemoteRange(ctx context.Context, from, to time.Time, refresh bool) ([]onepoint.DayWorklog, time.Time, error) {
	days := rangeDays(from, to)
	if refresh {
		c.InvalidateRemoteDays(days)
	}
	if c.HasRemoteCacheMiss(days) {
		// Serialize miss handling so concurrent requests don't trigger duplicate fetches.
		c.remoteFetchMu.Lock()
		if c.HasRemoteCacheMiss(days) {
			loaded, err := c.client.GetFilteredWorklogs(ctx, from, to)
			if err != nil {
				c.remoteFetchMu.Unlock()
				return nil, time.Time{}, err
			}
			byKey := make(map[string][]onepoint.DayWorklog, len(days))
			for _, day := range days {
				byKey[day.Format("2006-01-02")] = nil
			}
			for _, item := range loaded {
				parsed, err := onepoint.ParseDay(item.WorklogDate)
				if err != nil {
					continue
				}
				key := timeutil.StartOfDay(parsed).Format("2006-01-02")
				if _, ok := byKey[key]; !ok {
					continue
				}
				byKey[key] = append(byKey[key], item)
			}
			for key := range byKey {
				sortDayWorklogs(byKey[key])
			}

			refreshedAt := time.Now().UTC()
			c.mu.Lock()
			for _, day := range days {
				key := day.Format("2006-01-02")
				c.dayCache[key] = append([]onepoint.DayWorklog(nil), byKey[key]...)
				c.dayFetched[key] = true
				c.dayRefresh[key] = refreshedAt
			}
			c.mu.Unlock()
		}
		c.remoteFetchMu.Unlock()
	}

	out := make([]onepoint.DayWorklog, 0, 64)
	c.mu.RLock()
	for _, day := range days {
		key := day.Format("2006-01-02")
		out = append(out, c.dayCache[key]...)
	}
	c.mu.RUnlock()
	refreshedAt, _ := c.RemoteRangeRefreshTime(days)
	return out, refreshedAt, nil
}

func (c *dataCache) EnsureLocalCache() error {
	c.mu.RLock()
	loaded := c.localLoaded
	c.mu.RUnlock()
	if loaded {
		return nil
	}

	c.localLoadMu.Lock()
	defer c.localLoadMu.Unlock()

	c.mu.RLock()
	loaded = c.localLoaded
	c.mu.RUnlock()
	if loaded {
		return nil
	}

	allEntries, err := c.store.ListWorklogs()
	if err != nil {
		return fmt.Errorf("list local worklogs: %w", err)
	}

	index := make(map[string][]worklog.Entry, len(allEntries))
	for _, entry := range allEntries {
		key := timeutil.StartOfDay(entry.StartDateTime).Format("2006-01-02")
		index[key] = append(index[key], entry)
	}

	c.mu.Lock()
	c.localByDay = index
	c.localLoaded = true
	c.mu.Unlock()
	return nil
}

func (c *dataCache) HasRemoteCacheMiss(days []time.Time) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, day := range days {
		key := day.Format("2006-01-02")
		if !c.dayFetched[key] {
			return true
		}
	}
	return false
}

func (c *dataCache) InvalidateLocal() {
	c.mu.Lock()
	c.localByDay = make(map[string][]worklog.Entry)
	c.localLoaded = false
	c.mu.Unlock()
}

func (c *dataCache) InvalidateRemoteDays(days []time.Time) {
	if len(days) == 0 {
		return
	}

	c.mu.Lock()
	for _, day := range days {
		key := timeutil.StartOfDay(day).Format("2006-01-02")
		delete(c.dayCache, key)
		delete(c.dayFetched, key)
		delete(c.dayRefresh, key)
	}
	c.mu.Unlock()
}

func (c *dataCache) RemoteRangeRefreshTime(days []time.Time) (time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var latest time.Time
	for _, day := range days {
		key := day.Format("2006-01-02")
		ts, ok := c.dayRefresh[key]
		if !ok {
			continue
		}
		if latest.IsZero() || ts.After(latest) {
			latest = ts
		}
	}
	if latest.IsZero() {
		return time.Time{}, false
	}
	return latest, true
}

func (c *dataCache) LookupSnapshot(ctx context.Context, refresh bool) (onepoint.LookupSnapshot, error) {
	if !refresh {
		c.lookupMu.Lock()
		if c.lookupFetched && c.lookupSnap != nil {
			snapshot := *c.lookupSnap
			c.lookupMu.Unlock()
			return snapshot, nil
		}
		c.lookupMu.Unlock()
	}

	snapshot, err := c.client.FetchLookupSnapshot(ctx)
	if err != nil {
		return onepoint.LookupSnapshot{}, err
	}

	c.lookupMu.Lock()
	c.lookupSnap = &snapshot
	c.lookupFetched = true
	c.lookupMu.Unlock()

	return snapshot, nil
}

func localEntryIsSynced(entry worklog.Entry, remote []onepoint.PersistWorklog) bool {
	candidate := localEntryToPersistWorklog(entry)
	for _, item := range remote {
		if hasSameTimeRange(candidate, item) {
			return true
		}
	}
	return false
}
