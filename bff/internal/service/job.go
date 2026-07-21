package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dashboard/bff/internal/cache"
	"github.com/dashboard/bff/internal/model"
	"github.com/dashboard/bff/internal/upstream"
)

// JobService serves the job list (multi-value filtering on an in-memory cache)
// and proxies control actions to the upstream service.
type JobService struct {
	client    *upstream.Client
	cache     *cache.JobCache
	pageSize  int
	pageSleep time.Duration
	maxAge    time.Duration // cache freshness threshold for list/filter reads
}

func NewJobService(client *upstream.Client, jobCache *cache.JobCache, pageSize, pageSleepMs, samplerIntervalSec int) *JobService {
	if pageSize <= 0 {
		pageSize = 500
	}
	interval := time.Duration(samplerIntervalSec) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	return &JobService{
		client:    client,
		cache:     jobCache,
		pageSize:  pageSize,
		pageSleep: time.Duration(pageSleepMs) * time.Millisecond,
		// Allow up to 2 sampling intervals of staleness before a read refreshes.
		maxAge: 2 * interval,
	}
}

// JobListQuery holds the multi-value filter/pagination parameters for listing jobs.
// All slice fields are optional; an empty slice means "no filter on this field".
type JobListQuery struct {
	JobStatus       []int
	UserName        []string
	Project         []string
	Survey          []string
	Database        []string
	JobDesc         string // case-insensitive contains match on jobDesc
	CommitTimeStart int64  // unix seconds, inclusive; 0 = no lower bound
	CommitTimeEnd   int64  // unix seconds, inclusive; 0 = no upper bound
	Page            int
	PageSize        int
}

// JobListItem is a flattened job for the frontend table.
type JobListItem struct {
	JobName        string `json:"jobName"`
	JobDesc        string `json:"jobDesc"`
	JobStatus      int    `json:"jobStatus"`
	JobStatusLabel string `json:"jobStatusLabel"`
	UserName       string `json:"userName"`
	JobProcess     int    `json:"jobProcess"`
	Project        string `json:"project"`
	Survey         string `json:"survey"`
	Line           string `json:"line"`
	Database       string `json:"database"`
	CtrlNode       string `json:"ctrlNode"`
	Department     string `json:"department"`
	Application    string `json:"application"`
	ExecTime       uint   `json:"execTime"`
	WaitTime       uint   `json:"waitTime"`
	CommitTime     uint64 `json:"commitTime"`
	StartTime      uint64 `json:"startTime"`
	EndTime        uint64 `json:"endTime"`
	ExitCode       uint   `json:"exitCode"`
	Summary        string `json:"summary"`
}

// JobListResult is the paginated list response.
type JobListResult struct {
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
	List     []JobListItem `json:"list"`
	Cached   bool          `json:"cached"`
	CacheTs  int64         `json:"cacheTs"`
}

// ensureFresh refreshes the cache synchronously when it is empty or stale.
func (s *JobService) ensureFresh(ctx context.Context) error {
	if s.cache == nil {
		return fmt.Errorf("job cache not configured")
	}
	if !s.cache.Empty() && s.cache.Age() <= s.maxAge {
		return nil
	}
	jobs, _, err := s.client.FetchAllJobs(ctx, s.pageSize, int(s.pageSleep/time.Millisecond))
	if err != nil && len(jobs) == 0 {
		return err
	}
	s.cache.Set(jobs, time.Now().Unix())
	return nil
}

func (s *JobService) List(ctx context.Context, q JobListQuery) (*JobListResult, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if err := s.ensureFresh(ctx); err != nil {
		return nil, err
	}

	jobs, ts := s.cache.Snapshot()

	statusSet := toIntSet(q.JobStatus)
	userSet := toLowerSet(q.UserName)
	projSet := toLowerSet(q.Project)
	surveySet := toLowerSet(q.Survey)
	dbSet := toLowerSet(q.Database)
	descLower := strings.ToLower(strings.TrimSpace(q.JobDesc))

	filtered := make([]model.JobInfo, 0, len(jobs))
	for i := range jobs {
		j := &jobs[i]
		// 过滤按"有效状态"匹配：jsFinished + exitCode!=0 视为 jsFailed。
		if len(statusSet) > 0 && !statusSet[model.EffectiveStatusCode(j.StatusCode(), j.ExitCode)] {
			continue
		}
		if len(userSet) > 0 && !userSet[strings.ToLower(j.UserName)] {
			continue
		}
		if len(projSet) > 0 && !projSet[strings.ToLower(j.Project())] {
			continue
		}
		if len(surveySet) > 0 && !surveySet[strings.ToLower(j.Survey())] {
			continue
		}
		if len(dbSet) > 0 && !dbSet[strings.ToLower(j.Database())] {
			continue
		}
		if descLower != "" && !strings.Contains(strings.ToLower(j.JobDesc), descLower) {
			continue
		}
		if q.CommitTimeStart > 0 && int64(j.CommitTime) < q.CommitTimeStart {
			continue
		}
		if q.CommitTimeEnd > 0 && int64(j.CommitTime) > q.CommitTimeEnd {
			continue
		}
		filtered = append(filtered, *j)
	}

	// Sort by commit time descending (newest first).
	sort.Slice(filtered, func(a, b int) bool {
		return filtered[a].CommitTime > filtered[b].CommitTime
	})

	total := len(filtered)
	start := (q.Page - 1) * q.PageSize
	if start > total {
		start = total
	}
	end := start + q.PageSize
	if end > total {
		end = total
	}
	pageJobs := filtered[start:end]

	items := make([]JobListItem, 0, len(pageJobs))
	for i := range pageJobs {
		j := &pageJobs[i]
		// 使用有效状态：jsFinished + exitCode!=0 视为 jsFailed，避免把
		// 执行失败的作业误显示为"已完成"。
		code := model.EffectiveStatusCode(j.StatusCode(), j.ExitCode)
		items = append(items, JobListItem{
			JobName:        j.JobName,
			JobDesc:        j.JobDesc,
			JobStatus:      code,
			JobStatusLabel: model.StateLabel(code),
			UserName:       j.UserName,
			JobProcess:     j.JobProcess,
			Project:        j.Project(),
			Survey:         j.Survey(),
			Line:           j.Line(),
			Database:       j.Database(),
			CtrlNode:       j.CtrlNode,
			Department:     j.Department,
			Application:    j.Application,
			ExecTime:       j.ExecTime,
			WaitTime:       j.WaitTime,
			CommitTime:     j.CommitTime,
			StartTime:      j.StartTime,
			EndTime:        j.EndTime,
			ExitCode:       j.ExitCode,
			Summary:        buildSummary(j, code),
		})
	}
	return &JobListResult{
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
		List:     items,
		Cached:   true,
		CacheTs:  ts,
	}, nil
}

// JobFilters is the set of distinct candidate values for the filter dropdowns.
type JobFilters struct {
	CacheTs  int64    `json:"cacheTs"`
	Projects []string `json:"projects"`
	Surveys  []string `json:"surveys"`
	Users    []string `json:"users"`
	Databases []string `json:"databases"`
}

// Filters returns the distinct project/survey/user/database values extracted
// from the cached full job snapshot, for the frontend's multi-select dropdowns.
func (s *JobService) Filters(ctx context.Context) (*JobFilters, error) {
	if err := s.ensureFresh(ctx); err != nil {
		return nil, err
	}
	jobs, ts := s.cache.Snapshot()
	projSet := newOrderedSet()
	surveySet := newOrderedSet()
	userSet := newOrderedSet()
	dbSet := newOrderedSet()
	for i := range jobs {
		j := &jobs[i]
		if v := j.Project(); v != "" {
			projSet.add(v)
		}
		if v := j.Survey(); v != "" {
			surveySet.add(v)
		}
		if v := j.UserName; v != "" {
			userSet.add(v)
		}
		if v := j.Database(); v != "" {
			dbSet.add(v)
		}
	}
	return &JobFilters{
		CacheTs:   ts,
		Projects:  projSet.slice(),
		Surveys:   surveySet.slice(),
		Users:     userSet.slice(),
		Databases: dbSet.slice(),
	}, nil
}

// ControlAction enumerates job control operations.
type ControlAction string

const (
	ActionDelete  ControlAction = "delete"
	ActionRerun   ControlAction = "rerun"
	ActionSuspend ControlAction = "suspend"
	ActionResume  ControlAction = "resume"
)

// ControlResult reports per-name outcomes.
type ControlResult struct {
	Success []string      `json:"success"`
	Failed  []NameFailure `json:"failed"`
}

type NameFailure struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// Control 执行作业控制操作。currentUser 为当前登录用户名；只能控制自己提交
// 的作业（作业 userName 与 currentUser 一致），他人作业会被拒绝并放入 Failed。
// 作业归属校验依赖内存缓存，缓存不可用时拒绝执行以防误操作。
func (s *JobService) Control(ctx context.Context, action ControlAction, names []string, currentUser string) (*ControlResult, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("names is empty")
	}
	if currentUser == "" {
		return nil, fmt.Errorf("未登录")
	}
	switch action {
	case ActionDelete, ActionRerun:
	case ActionSuspend, ActionResume:
		return nil, fmt.Errorf("接口待提供：暂停/恢复接口尚未由上游服务实现")
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}

	if err := s.ensureFresh(ctx); err != nil {
		return nil, fmt.Errorf("无法校验作业归属，请稍后重试: %w", err)
	}
	jobs, _ := s.cache.Snapshot()
	owner := make(map[string]string, len(jobs))
	for i := range jobs {
		owner[jobs[i].JobName] = jobs[i].UserName
	}

	var mine []string
	failed := make([]NameFailure, 0)
	for _, n := range names {
		u, ok := owner[n]
		if !ok {
			failed = append(failed, NameFailure{Name: n, Reason: "作业不存在"})
			continue
		}
		if u != currentUser {
			failed = append(failed, NameFailure{Name: n, Reason: "无权操作他人作业"})
			continue
		}
		mine = append(mine, n)
	}

	success := make([]string, 0)
	if len(mine) > 0 {
		switch action {
		case ActionDelete:
			if err := s.client.Delete(ctx, mine); err != nil {
				return nil, err
			}
		case ActionRerun:
			if err := s.client.Rerunmulti(ctx, mine); err != nil {
				return nil, err
			}
		}
		success = append(success, mine...)
	}
	return &ControlResult{Success: success, Failed: failed}, nil
}

// --- helpers ---

func toIntSet(xs []int) map[int]bool {
	if len(xs) == 0 {
		return nil
	}
	m := make(map[int]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func toLowerSet(xs []string) map[string]bool {
	if len(xs) == 0 {
		return nil
	}
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[strings.ToLower(x)] = true
	}
	return m
}

// orderedSet preserves insertion order and deduplicates (case-sensitive).
type orderedSet struct {
	m  map[string]bool
	sl []string
}

func newOrderedSet() *orderedSet { return &orderedSet{m: make(map[string]bool)} }
func (o *orderedSet) add(v string) {
	if !o.m[v] {
		o.m[v] = true
		o.sl = append(o.sl, v)
	}
}
func (o *orderedSet) slice() []string {
	if len(o.sl) == 0 {
		return []string{}
	}
	out := make([]string, len(o.sl))
	copy(out, o.sl)
	return out
}

func buildSummary(j *model.JobInfo, code int) string {
	commitStr := ""
	if j.CommitTime > 0 {
		commitStr = time.Unix(int64(j.CommitTime), 0).Format("2006-01-02 15:04:05")
	}
	return fmt.Sprintf("作业 %s 由 %s 于 %s 提交，已执行 %ds，进度 %d%%，状态 %s",
		j.JobDesc, j.UserName, commitStr, j.ExecTime, j.JobProcess, model.StateLabel(code))
}
