package sampler

import (
	"context"
	"log/slog"
	"time"

	"github.com/dashboard/bff/internal/model"
	"github.com/dashboard/bff/internal/store"
	"github.com/dashboard/bff/internal/upstream"
)

// JSFSampler polls GetCurrentJSFInfo and writes snapshots.
type JSFSampler struct {
	client *upstream.Client
	repo   *store.StatsRepo
	logger *slog.Logger
}

func NewJSFSampler(client *upstream.Client, repo *store.StatsRepo, logger *slog.Logger) *JSFSampler {
	if logger == nil {
		logger = slog.Default()
	}
	return &JSFSampler{client: client, repo: repo, logger: logger}
}

// Sample performs one sampling tick.
func (s *JSFSampler) Sample(ctx context.Context) {
	snap, err := s.client.GetCurrentJSFInfo(ctx)
	if err != nil {
		s.logger.Warn("jsf sampler: GetCurrentJSFInfo failed", "err", err)
		return
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
