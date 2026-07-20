package service

import (
	"context"
	"sort"
	"time"

	"github.com/dashboard/bff/internal/model"
	"github.com/dashboard/bff/internal/store"
	"github.com/dashboard/bff/internal/upstream"
)

// StatsService serves the dashboard overview data.
type StatsService struct {
	client    *upstream.Client
	statsRepo *store.StatsRepo
	userRepo  *store.UserRepo
}

func NewStatsService(client *upstream.Client, statsRepo *store.StatsRepo, userRepo *store.UserRepo) *StatsService {
	return &StatsService{client: client, statsRepo: statsRepo, userRepo: userRepo}
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
	var cur model.StatsSnapshot
	degraded := false

	if fresh {
		if snap, err := s.client.GetCurrentJSFInfo(ctx); err == nil {
			cur = *snap
		} else {
			// 上游失败：降级到 DB 最新快照；DB 也无数据则返回空统计。
			degraded = true
			latest, lerr := s.statsRepo.Latest()
			if lerr == nil {
				cur = latest
			} else {
				cur = model.StatsSnapshot{Ts: time.Now().Unix()}
			}
		}
	} else {
		latest, err := s.statsRepo.Latest()
		if err == nil {
			cur = latest
		} else {
			// DB 无快照：尝试上游一次；上游也失败则返回空统计 + 降级。
			if snap, e2 := s.client.GetCurrentJSFInfo(ctx); e2 == nil {
				cur = *snap
			} else {
				degraded = true
				cur = model.StatsSnapshot{Ts: time.Now().Unix()}
			}
		}
	}

	// Previous snapshot: latest strictly older than current ts.
	var prev model.StatsSnapshot
	if p, err := s.statsRepo.PreviousBefore(cur.Ts); err == nil {
		prev = p
	}

	groups := buildGroups(cur, prev)
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
	snaps, err := s.statsRepo.Range(from, now.Unix())
	if err != nil {
		return nil, err
	}
	points := aggregate(snaps, r)
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
	if limit <= 0 {
		limit = 10
	}
	ts, stats, err := s.userRepo.TopAtLatest(limit)
	if err != nil {
		return nil, err
	}
	total, _ := s.userRepo.TotalAtLatest()
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
	return result, nil
}

func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}
