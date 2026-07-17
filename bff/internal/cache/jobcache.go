package cache

import (
	"sync"
	"time"

	"github.com/dashboard/bff/internal/model"
)

// JobCache holds the latest full snapshot of jobs in memory.
// It is written by the sampler each tick and read by the job list/filter
// endpoints to support multi-value filtering and candidate-value extraction
// (the upstream service exposes no REST API for distinct projects/surveys/users).
type JobCache struct {
	mu   sync.RWMutex
	jobs []model.JobInfo
	ts   int64 // unix seconds of last refresh
}

func NewJobCache() *JobCache {
	return &JobCache{}
}

// Set replaces the cached jobs.
func (c *JobCache) Set(jobs []model.JobInfo, ts int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Copy to avoid external mutation aliasing.
	cp := make([]model.JobInfo, len(jobs))
	copy(cp, jobs)
	c.jobs = cp
	c.ts = ts
}

// Snapshot returns the cached jobs and their refresh timestamp.
// Callers must not mutate the returned slice.
func (c *JobCache) Snapshot() ([]model.JobInfo, int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.jobs, c.ts
}

// Age returns seconds since last refresh; returns a large value when empty.
func (c *JobCache) Age() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.ts == 0 {
		return time.Hour
	}
	return time.Since(time.Unix(c.ts, 0))
}

// Empty reports whether the cache holds no jobs.
func (c *JobCache) Empty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.jobs) == 0
}
