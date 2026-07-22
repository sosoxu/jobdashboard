package service

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/dashboard/bff/internal/cache"
	"github.com/dashboard/bff/internal/model"
	"github.com/dashboard/bff/internal/store"
	"github.com/dashboard/bff/internal/upstream"
)

// StatsService serves the dashboard overview data.
type StatsService struct {
	client    *upstream.Client
	statsRepo *store.StatsRepo
	userRepo  *store.UserRepo
	jobCache  *cache.JobCache
	logger    *slog.Logger
}

func NewStatsService(client *upstream.Client, statsRepo *store.StatsRepo, userRepo *store.UserRepo, jobCache *cache.JobCache) *StatsService {
	return &StatsService{client: client, statsRepo: statsRepo, userRepo: userRepo, jobCache: jobCache, logger: slog.Default()}
}

// correctByExitCode 按 exitCode 修正 snapshot：jsFinished + exitCode!=0 的作业
// 从 Finish 移到 Failed。jobCache 为空或不可用时原样返回。
func (s *StatsService) correctByExitCode(snap *model.StatsSnapshot) {
	if s.jobCache == nil || s.jobCache.Empty() {
		return
	}
	jobs, _ := s.jobCache.Snapshot()
	bad := model.CountBadFinish(jobs)
	if bad <= 0 {
		return
	}
	snap.Finish -= bad
	if snap.Finish < 0 {
		snap.Finish = 0
	}
	snap.Failed += bad
}

// GroupStat is one status group with current value and change vs previous.
type GroupStat struct {
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	Count     int     `json:"count"`
	PrevCount int     `json:"prevCount"`
	Delta     int     `json:"delta"`
	DeltaPct  float64 `json:"deltaPct"`
}

// StatsResult is the response of the dashboard stats endpoint.
type StatsResult struct {
	UpdatedAt int64       `json:"updatedAt"`
	Groups    []GroupStat `json:"groups"`
	Degraded  bool        `json:"degraded"`
}

func (s *StatsService) Stats(ctx context.Context, fresh bool) (*StatsResult, error) {
	t0 := time.Now()
	var cur model.StatsSnapshot
	degraded := false

	if fresh {
		t1 := time.Now()
		if snap, err := s.client.GetCurrentJSFInfo(ctx); err == nil {
			cur = *snap
			s.correctByExitCode(&cur)
			s.logger.Info("stats.timing", "step", "upstream GetCurrentJSFInfo", "dur", time.Since(t1).String())
		} else {
			degraded = true
			latest, lerr := s.statsRepo.Latest()
			if lerr == nil {
				cur = latest
			} else {
				cur = model.StatsSnapshot{Ts: time.Now().Unix()}
			}
			s.logger.Info("stats.timing", "step", "upstream_failed_fallback_db", "dur", time.Since(t1).String(), "upstream_err", err.Error())
		}
	} else {
		t1 := time.Now()
		latest, err := s.statsRepo.Latest()
		dbDur := time.Since(t1)
		if err == nil {
			cur = latest
			s.logger.Info("stats.timing", "step", "db Latest", "dur", dbDur.String())
		} else {
			// DB 无快照：尝试上游一次；上游也失败则返回空统计 + 降级。
			t2 := time.Now()
			if snap, e2 := s.client.GetCurrentJSFInfo(ctx); e2 == nil {
				cur = *snap
				s.correctByExitCode(&cur)
				s.logger.Info("stats.timing", "step", "db_empty_upstream", "dur", time.Since(t2).String())
			} else {
				degraded = true
				cur = model.StatsSnapshot{Ts: time.Now().Unix()}
				s.logger.Info("stats.timing", "step", "db_empty_upstream_failed", "dur", time.Since(t2).String(), "err", e2.Error())
			}
		}
	}

	// Previous snapshot: latest strictly older than current ts.
	var prev model.StatsSnapshot
	t1 := time.Now()
	if p, err := s.statsRepo.PreviousBefore(cur.Ts); err == nil {
		prev = p
	}
	s.logger.Info("stats.timing", "step", "db PreviousBefore", "dur", time.Since(t1).String())

	groups := buildGroups(cur, prev)
	s.logger.Info("stats.timing", "step", "total", "dur", time.Since(t0).String(), "degraded", degraded)
	return &StatsResult{UpdatedAt: cur.Ts, Groups: groups, Degraded: degraded}, nil
}

func buildGroups(cur, prev model.StatsSnapshot) []GroupStat {
	defs := []struct {
		key   string
		label string
		cur   int
		prev  int
	}{
		{"active", "运行中", cur.Active, prev.Active},
		{"queue", "排队中", cur.Queue, prev.Queue},
		{"finish", "已完成", cur.Finish, prev.Finish},
		{"failed", "失败", cur.Failed, prev.Failed},
		{"canceled", "已取消", cur.Canceled, prev.Canceled},
		{"othercount", "其他", cur.OtherCount, prev.OtherCount},
	}
	out := make([]GroupStat, 0, len(defs))
	for _, d := range defs {
		g := GroupStat{Key: d.key, Label: d.label, Count: d.cur, PrevCount: d.prev, Delta: d.cur - d.prev}
		if d.prev > 0 {
			g.DeltaPct = float64(g.Delta) / float64(d.prev) * 100
		} else if g.Delta > 0 {
			g.DeltaPct = 100
		}
		out = append(out, g)
	}
	return out
}

// TrendPoint is one point in the trend series.
type TrendPoint struct {
	Ts      int64 `json:"ts"`
	Finish  int   `json:"finish"`
	Active  int   `json:"active"`
	Queue   int   `json:"queue"`
	Failed  int   `json:"failed"`
	Canceled int  `json:"canceled"`
}

// TrendResult is the response of the trend endpoint.
type TrendResult struct {
	Range  string       `json:"range"`
	Points []TrendPoint `json:"points"`
}

func (s *StatsService) Trend(ctx context.Context, r string) (*TrendResult, error) {
	t0 := time.Now()
	now := time.Now()
	var from int64
	switch r {
	case "24h":
		from = now.Add(-24 * time.Hour).Unix()
	case "7d":
		from = now.Add(-7 * 24 * time.Hour).Unix()
	case "30d":
		from = now.Add(-30 * 24 * time.Hour).Unix()
	default:
		r = "24h"
		from = now.Add(-24 * time.Hour).Unix()
	}
	t1 := time.Now()
	snaps, err := s.statsRepo.Range(from, now.Unix())
	dbDur := time.Since(t1)
	if err != nil {
		s.logger.Info("trend.timing", "step", "db Range FAILED", "dur", dbDur.String(), "err", err.Error())
		return nil, err
	}
	s.logger.Info("trend.timing", "step", "db Range", "dur", dbDur.String(), "rows", len(snaps))
	points := aggregate(snaps, r)
	s.logger.Info("trend.timing", "step", "total", "dur", time.Since(t0).String())
	return &TrendResult{Range: r, Points: points}, nil
}

// aggregate downsamples snapshots to the right granularity for the range.
func aggregate(snaps []model.StatsSnapshot, r string) []TrendPoint {
	if len(snaps) == 0 {
		return []TrendPoint{}
	}
	var bucketSec int64
	switch r {
	case "24h":
		bucketSec = 60 // 1 minute
	case "7d":
		bucketSec = 3600 // 1 hour
	case "30d":
		bucketSec = 86400 // 1 day
	default:
		bucketSec = 60
	}
	// Pick the last snapshot within each bucket.
	lastInBucket := make(map[int64]model.StatsSnapshot)
	var keys []int64
	for _, s := range snaps {
		k := s.Ts / bucketSec
		if _, ok := lastInBucket[k]; !ok {
			keys = append(keys, k)
		}
		if s.Ts >= lastInBucket[k].Ts {
			lastInBucket[k] = s
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([]TrendPoint, 0, len(keys))
	for _, k := range keys {
		s := lastInBucket[k]
		out = append(out, TrendPoint{
			Ts:       s.Ts,
			Finish:   s.Finish,
			Active:   s.Active,
			Queue:    s.Queue,
			Failed:   s.Failed,
			Canceled: s.Canceled,
		})
	}
	return out
}

// TopUser is one user in the Top10 list.
type TopUser struct {
	UserName string  `json:"userName"`
	Count    int     `json:"count"`
	Pct      float64 `json:"pct"`
}

// TopUsersResult is the response of the top-users endpoint.
type TopUsersResult struct {
	Window string    `json:"window"`
	Ts     int64     `json:"ts"`
	Total  int       `json:"total"`
	Users  []TopUser `json:"users"`
	Others TopUser   `json:"others"`
}

func (s *StatsService) TopUsers(ctx context.Context, limit int) (*TopUsersResult, error) {
	t0 := time.Now()
	if limit <= 0 {
		limit = 10
	}
	t1 := time.Now()
	ts, stats, err := s.userRepo.TopAtLatest(limit)
	dbDur := time.Since(t1)
	if err != nil {
		s.logger.Info("topusers.timing", "step", "db TopAtLatest FAILED", "dur", dbDur.String(), "err", err.Error())
		return nil, err
	}
	s.logger.Info("topusers.timing", "step", "db TopAtLatest", "dur", dbDur.String(), "rows", len(stats))

	t2 := time.Now()
	total, _ := s.userRepo.TotalAtLatest()
	s.logger.Info("topusers.timing", "step", "db TotalAtLatest", "dur", time.Since(t2).String(), "total", total)

	if total == 0 && len(stats) > 0 {
		for _, s := range stats {
			total += s.JobCount
		}
	}
	result := &TopUsersResult{Window: "current", Ts: ts, Total: total}
	topSum := 0
	for _, s := range stats {
		topSum += s.JobCount
		var pct float64
		if total > 0 {
			pct = float64(s.JobCount) / float64(total) * 100
		}
		result.Users = append(result.Users, TopUser{UserName: s.UserName, Count: s.JobCount, Pct: round1(pct)})
	}
	othersCount := total - topSum
	var othersPct float64
	if total > 0 {
		othersPct = float64(othersCount) / float64(total) * 100
	}
	result.Others = TopUser{UserName: "其他", Count: othersCount, Pct: round1(othersPct)}
	s.logger.Info("topusers.timing", "step", "total", "dur", time.Since(t0).String())
	return result, nil
}

func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}
