package sampler

import (
	"context"
	"log/slog"
	"time"

	"github.com/dashboard/bff/internal/cache"
	"github.com/dashboard/bff/internal/model"
	"github.com/dashboard/bff/internal/store"
	"github.com/dashboard/bff/internal/upstream"
)

// JobSampler polls GetJob (paginated, full) and:
//   - aggregates per-user counts into the user_job_stat table (for Top10);
//   - replaces the in-memory full-job cache (for multi-value filtering and
//     candidate-value extraction on the job list page).
type JobSampler struct {
	client    *upstream.Client
	userRepo  *store.UserRepo
	cache     *cache.JobCache
	pageSize  int
	pageSleep time.Duration
	logger    *slog.Logger
}

func NewJobSampler(client *upstream.Client, userRepo *store.UserRepo, jobCache *cache.JobCache, pageSize int, pageSleepMs int, logger *slog.Logger) *JobSampler {
	if pageSize <= 0 {
		pageSize = 500
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &JobSampler{
		client:    client,
		userRepo:  userRepo,
		cache:     jobCache,
		pageSize:  pageSize,
		pageSleep: time.Duration(pageSleepMs) * time.Millisecond,
		logger:    logger,
	}
}

// Sample performs one full sampling tick.
func (s *JobSampler) Sample(ctx context.Context) {
	jobs, _, err := s.client.FetchAllJobs(ctx, s.pageSize, int(s.pageSleep/time.Millisecond))
	if err != nil {
		s.logger.Warn("job sampler: FetchAllJobs failed", "err", err)
		// Still try to keep whatever partial data was collected, if any.
		if len(jobs) == 0 {
			return
		}
	}

	ts := time.Now().Unix()

	// Update in-memory full-job cache (used by list/filter endpoints).
	if s.cache != nil {
		s.cache.Set(jobs, ts)
	}

	// Aggregate per-user counts.
	countByUser := make(map[string]int, len(jobs))
	for i := range jobs {
		if u := jobs[i].UserName; u != "" {
			countByUser[u]++
		}
	}
	stats := make([]model.UserJobStat, 0, len(countByUser))
	for u, c := range countByUser {
		stats = append(stats, model.UserJobStat{Ts: ts, UserName: u, JobCount: c})
	}
	if err := s.userRepo.ReplaceAllForTS(ts, stats); err != nil {
		s.logger.Warn("job sampler: write user stats failed", "err", err)
		return
	}
	s.logger.Debug("job sampler ok", "jobs", len(jobs), "users", len(stats))
}
