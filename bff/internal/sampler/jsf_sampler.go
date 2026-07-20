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

// JSFSampler polls GetCurrentJSFInfo and writes snapshots.
// 若注入了 jobCache，会在写入前按 exitCode 修正 Finish/Failed：
// jsFinished + exitCode!=0 的作业从 Finish 移到 Failed。
type JSFSampler struct {
	client *upstream.Client
	repo   *store.StatsRepo
	cache  *cache.JobCache
	logger *slog.Logger
}

func NewJSFSampler(client *upstream.Client, repo *store.StatsRepo, jobCache *cache.JobCache, logger *slog.Logger) *JSFSampler {
	if logger == nil {
		logger = slog.Default()
	}
	return &JSFSampler{client: client, repo: repo, cache: jobCache, logger: logger}
}

// Sample performs one sampling tick.
func (s *JSFSampler) Sample(ctx context.Context) {
	snap, err := s.client.GetCurrentJSFInfo(ctx)
	if err != nil {
		s.logger.Warn("jsf sampler: GetCurrentJSFInfo failed", "err", err)
		return
	}
	// 按 exitCode 修正：jsFinished + exitCode!=0 的作业应计入 Failed 而非 Finish。
	// 上游聚合不感知 exitCode，BFF 用全量作业缓存做一次重算修正。
	if s.cache != nil && !s.cache.Empty() {
		jobs, _ := s.cache.Snapshot()
		bad := model.CountBadFinish(jobs)
		if bad > 0 {
			snap.Finish -= bad
			if snap.Finish < 0 {
				snap.Finish = 0
			}
			snap.Failed += bad
			s.logger.Debug("jsf sampler: corrected finish/failed by exitCode", "bad", bad)
		}
	}
	// Align ts to the second; keep the upstream-derived freshness.
	snap.Ts = time.Now().Unix()
	if err := s.repo.Insert(*snap); err != nil {
		s.logger.Warn("jsf sampler: insert failed", "err", err)
		return
	}
	if err := s.repo.MetaSet("latest_snapshot_ts", itoa(snap.Ts)); err != nil {
		s.logger.Warn("jsf sampler: meta set failed", "err", err)
	}
	s.logger.Debug("jsf sampler ok", "snapshot", model.StatsSnapshot(*snap))
}

func itoa(n int64) string {
	// avoid strconv import cycle concerns; small helper
	return time.Unix(n, 0).Format("2006-01-02 15:04:05")
}
