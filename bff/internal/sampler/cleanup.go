package sampler

import (
	"context"
	"log/slog"
	"time"

	"github.com/dashboard/bff/internal/store"
)

// Cleanup removes snapshots older than retainDays. Intended to run daily.
type Cleanup struct {
	statsRepo *store.StatsRepo
	userRepo  *store.UserRepo
	retain    time.Duration
	logger    *slog.Logger
}

func NewCleanup(statsRepo *store.StatsRepo, userRepo *store.UserRepo, retainDays int, logger *slog.Logger) *Cleanup {
	if logger == nil {
		logger = slog.Default()
	}
	if retainDays <= 0 {
		retainDays = 30
	}
	return &Cleanup{
		statsRepo: statsRepo,
		userRepo:  userRepo,
		retain:    time.Duration(retainDays) * 24 * time.Hour,
		logger:    logger,
	}
}

func (c *Cleanup) Run(ctx context.Context) {
	cutoff := time.Now().Add(-c.retain).Unix()
	if n, err := c.statsRepo.DeleteOlderThan(cutoff); err != nil {
		c.logger.Warn("cleanup stats failed", "err", err)
	} else {
		c.logger.Info("cleanup stats ok", "deleted", n)
	}
	if n, err := c.userRepo.DeleteOlderThan(cutoff); err != nil {
		c.logger.Warn("cleanup user stats failed", "err", err)
	} else {
		c.logger.Info("cleanup user stats ok", "deleted", n)
	}
}
