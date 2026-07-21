package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// JobState 枚举（与 BFF model.JobState 下标一致）
const (
	jsFrozen = iota // 0
	jsQueue         // 1
	jsScheduled     // 2
	jsReady         // 3
	jsReleased      // 4
	jsActive        // 5
	jsSuspending    // 6
	jsSuspended     // 7
	jsResuming      // 8
	jsCanceling     // 9
	jsFinished      // 10
	jsFailed        // 11
	jsCanceled      // 12
)

// Job 镜像上游 JobInfo 结构。BFF 通过 Attributes（"key=value" 形式）
// 抽取 database/project/survey/line，故 mock 在此字段填充。
type Job struct {
	JobName       string   `json:"jobName"`
	JobDesc       string   `json:"jobDesc"`
	Attributes    []string `json:"attributes"`
	CtrlNode      string   `json:"ctrlNode"`
	RunNodes      []string `json:"runNodes"`
	JobStatus     string   `json:"jobStatus"`
	UserName      string   `json:"userName"`
	JobProcess    int      `json:"jobProcess"`
	CommitTime    uint64   `json:"commitTime"`
	ScheduleTime  uint64   `json:"scheduleTime"`
	StartTime     uint64   `json:"startTime"`
	EndTime       uint64   `json:"endTime"`
	EstimatedTime uint     `json:"estimatedTime"`
	WaitTime      uint     `json:"waitTime"`
	ExecTime      uint     `json:"execTime"`
	JobQueue      string   `json:"jobQueue"`
	Application   string   `json:"application"`
	Character     uint     `json:"character"`
	Department    string   `json:"department"`
	ExitCode      uint     `json:"exitCode"`
	ReportMessage string   `json:"reportMessage"`
	CrossGroup    uint     `json:"crossGroup"`
}

// ProjEntry 对应 GetJob 请求中的 projList 元素
type ProjEntry struct {
	Database string `json:"database"`
	Project  string `json:"project"`
	Survey   string `json:"survey"`
	Line     string `json:"line"`
}

// Store 内存作业存储，线程安全
type Store struct {
	mu      sync.RWMutex
	jobs    map[string]*Job // jobName -> job
	counter int             // 作业编号自增
	rng     *rand.Rand
	logRoot string // 非空时为作业生成对应的 list/LOG 日志文件（用于测试新命名方案）
}

func NewStore(logRoot string) *Store {
	s := &Store{
		jobs:    make(map[string]*Job),
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
		logRoot: logRoot,
	}
	s.seed()
	return s
}

// 候选值（与 projects.conf 对齐）
var (
	projects    = []string{"qqqq", "BGPCUP2026"}
	databases   = []string{"ndp", "ndp_check"}
	surveys     = []string{"survey1", "survey2", "survey3"}
	lines       = []string{"line1", "line2", "line3", ""}
	users       = []string{"user1", "user2", "user3", "user4", "user5"}
	applications = []string{"Migration", "Velocity", "Inversion", "Decon", "Stack", "NMO"}
	departments  = []string{"dept1", "dept2"}
	nodes        = []string{"node01", "node02", "node03", "node04"}
)

// seed 生成初始作业集合
func (s *Store) seed() {
	now := time.Now().Unix()
	for i := 0; i < 200; i++ {
		// 状态分布：约 30% finished, 15% active, 20% queue, 10% failed,
		// 10% canceled, 其余散落其他态
		var status int
		r := s.rng.Intn(100)
		switch {
		case r < 30:
			status = jsFinished
		case r < 45:
			status = jsActive
		case r < 65:
			status = jsQueue
		case r < 75:
			status = jsFailed
		case r < 85:
			status = jsCanceled
		case r < 90:
			status = jsScheduled
		case r < 95:
			status = jsReady
		default:
			status = jsSuspended
		}
		s.newJobLocked(status, now-int64(s.rng.Intn(48*3600))) // 最近48h内随机提交
	}
}

// newJobLocked 创建一个作业并加入 store（调用方持锁）
func (s *Store) newJobLocked(status int, commitTs int64) *Job {
	s.counter++
	project := projects[s.rng.Intn(len(projects))]
	database := databases[s.rng.Intn(len(databases))]
	// project=qqqq 对应 ndp，BGPCUP2026 对应 ndp_check（模拟真实映射）
	if project == "qqqq" {
		database = "ndp"
	} else {
		database = "ndp_check"
	}
	survey := surveys[s.rng.Intn(len(surveys))]
	line := lines[s.rng.Intn(len(lines))]
	user := users[s.rng.Intn(len(users))]
	app := applications[s.rng.Intn(len(applications))]
	dept := departments[s.rng.Intn(len(departments))]

	idx := s.counter
	jobName := fmt.Sprintf("J%d%07d", commitTs, idx)
	jobDesc := fmt.Sprintf("%s_%s_%s_%d.job", app, project, survey, idx)

	j := &Job{
		JobName:    jobName,
		JobDesc:    jobDesc,
		Attributes: []string{
			fmt.Sprintf("database=%s", database),
			fmt.Sprintf("project=%s", project),
			fmt.Sprintf("survey=%s", survey),
			fmt.Sprintf("line=%s", line),
		},
		CtrlNode:    nodes[s.rng.Intn(len(nodes))],
		RunNodes:    []string{nodes[s.rng.Intn(len(nodes))], nodes[s.rng.Intn(len(nodes))]},
		JobStatus:   fmt.Sprintf("%d", status),
		UserName:    user,
		JobQueue:    "default",
		Application: app,
		Department:  dept,
		CommitTime:  uint64(commitTs),
	}

	now := uint64(time.Now().Unix())
	// 按状态填充时间与进度字段
	switch status {
	case jsQueue, jsScheduled, jsReady, jsFrozen:
		j.JobProcess = 0
		j.WaitTime = uint(now - uint64(commitTs))
	case jsActive:
		j.ScheduleTime = uint64(commitTs) + uint64(s.rng.Intn(60))
		j.StartTime = j.ScheduleTime + uint64(s.rng.Intn(10))
		j.JobProcess = s.rng.Intn(90) + 5
		j.ExecTime = uint(now - j.StartTime)
	case jsFinished:
		j.ScheduleTime = uint64(commitTs) + uint64(s.rng.Intn(60))
		j.StartTime = j.ScheduleTime + uint64(s.rng.Intn(10))
		dur := uint64(s.rng.Intn(1800) + 10)
		j.EndTime = j.StartTime + dur
		j.ExecTime = uint(dur)
		j.JobProcess = 100
		// 约 20% 概率产出 jsFinished + exitCode!=0 的"执行失败"样本，
		// 用于验证 BFF 端将其视为 jsFailed 的逻辑。
		if s.rng.Intn(100) < 20 {
			j.ExitCode = 1
			j.ReportMessage = "execution failed: non-zero exit code"
		} else {
			j.ExitCode = 0
		}
	case jsFailed:
		j.ScheduleTime = uint64(commitTs) + uint64(s.rng.Intn(60))
		j.StartTime = j.ScheduleTime + uint64(s.rng.Intn(10))
		dur := uint64(s.rng.Intn(600) + 5)
		j.EndTime = j.StartTime + dur
		j.ExecTime = uint(dur)
		j.JobProcess = s.rng.Intn(80)
		j.ExitCode = 1
		j.ReportMessage = "execution failed: see LOG for details"
	case jsCanceled:
		j.JobProcess = s.rng.Intn(50)
		j.ExitCode = 2
		j.ReportMessage = "canceled by user"
	case jsSuspended:
		j.JobProcess = s.rng.Intn(60)
	}
	s.jobs[jobName] = j
	// 若启用 logRoot，按新命名方案为作业生成对应的 list/LOG 文件。
	// 文件名：{jobDesc}.{四位数编号}.{jobName}.list|.log
	if s.logRoot != "" {
		s.writeLogFiles(j)
	}
	return j
}

// writeLogFiles 在 logRoot 下为作业生成 list 和 LOG 文件，使用新命名方案。
// 文件名：{jobDesc}.{四位数编号}.{jobName}.list|.log
// 路径：{logRoot}/{project}/{survey}/list|LOG/...
// 内容是简化的占位文本，主要用于验证 BFF 端的文件定位与读取逻辑。
func (s *Store) writeLogFiles(j *Job) {
	attrs := make(map[string]string)
	for _, a := range j.Attributes {
		if idx := strings.Index(a, "="); idx > 0 {
			attrs[a[:idx]] = a[idx+1:]
		}
	}
	project := attrs["project"]
	survey := attrs["survey"]
	if project == "" || survey == "" {
		return
	}
	seq := s.counter % 10000 // 0..9999，四位数编号
	body := fmt.Sprintf("==== mock log for job %s ====\nJobDesc: %s\nProject: %s\nSurvey: %s\nStatus: %s\nExitCode: %d\nTime: %s\n",
		j.JobName, j.JobDesc, project, survey, j.JobStatus, j.ExitCode,
		time.Now().Format("2006-01-02 15:04:05"))
	for _, item := range []struct {
		subDir, ext string
		extra       string
	}{
		{"list", "list", "\n===  Start of Job Code  === \n-------------- Job Structure --------------\nModule Run Time Information\n..........  Job Done Successful .........."},
		{"LOG", "log", "\nPrepare Module GeoDiskIn\nLoad Module From : /testdata/geodiskin.so\nModule Execute Phase Complete!"},
	} {
		dir := filepath.Join(s.logRoot, project, survey, item.subDir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		name := fmt.Sprintf("%s.%04d.%s.%s", j.JobDesc, seq, j.JobName, item.ext)
		path := filepath.Join(dir, name)
		_ = os.WriteFile(path, []byte(body+item.extra+"\n"), 0o644)
	}
}

// Evolve 推进作业状态，模拟真实调度行为
func (s *Store) Evolve() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()

	for _, j := range s.jobs {
		status := atoiSafe(j.JobStatus)
		switch status {
		case jsActive:
			// 推进进度
			j.JobProcess += s.rng.Intn(8) + 1
			j.ExecTime += 5
			if j.JobProcess >= 100 {
				j.JobProcess = 100
				if s.rng.Intn(100) < 5 { // 5% 概率失败
					j.JobStatus = fmt.Sprintf("%d", jsFailed)
					j.ExitCode = 1
					j.EndTime = uint64(now)
					j.ReportMessage = "execution failed: timeout"
				} else {
					// 即使达到 100% 也可能以非零退出码结束（约 20%），
					// 此时上游 jobStatus 仍为 jsFinished，但 exitCode!=0。
					j.JobStatus = fmt.Sprintf("%d", jsFinished)
					j.EndTime = uint64(now)
					if s.rng.Intn(100) < 20 {
						j.ExitCode = 1
						j.ReportMessage = "execution failed: non-zero exit code"
					} else {
						j.ExitCode = 0
					}
				}
			}
		case jsQueue:
			// 20% 概率被调度执行
			if s.rng.Intn(100) < 20 {
				j.JobStatus = fmt.Sprintf("%d", jsActive)
				j.ScheduleTime = uint64(now)
				j.StartTime = uint64(now)
				j.JobProcess = s.rng.Intn(10)
			}
		case jsScheduled, jsReady:
			// 30% 概率进入排队或激活
			if s.rng.Intn(100) < 30 {
				j.JobStatus = fmt.Sprintf("%d", jsActive)
				j.StartTime = uint64(now)
			}
		}
	}

	// 偶尔新增作业（模拟新提交）
	if s.rng.Intn(100) < 40 {
		n := s.rng.Intn(3) + 1
		for i := 0; i < n; i++ {
			s.newJobLocked(jsQueue, now)
		}
	}
}

// JSFStats 聚合统计
type JSFStats struct {
	Active     int `json:"active"`
	Queue      int `json:"queue"`
	Finish     int `json:"finish"`
	Failed     int `json:"failed"`
	Canceled   int `json:"canceled"`
	OtherCount int `json:"othercount"`
}

// CurrentJSFInfo 计算当前聚合统计。
// 模拟生产上游行为：按 jobStatus 原始值分组，不感知 exitCode。
// jsFinished + exitCode!=0 的作业仍会被计入 Finish（由 BFF 端做修正）。
func (s *Store) CurrentJSFInfo() JSFStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var st JSFStats
	for _, j := range s.jobs {
		switch atoiSafe(j.JobStatus) {
		case jsActive:
			st.Active++
		case jsQueue:
			st.Queue++
		case jsFinished:
			st.Finish++
		case jsFailed:
			st.Failed++
		case jsCanceled:
			st.Canceled++
		default:
			st.OtherCount++
		}
	}
	return st
}

// GetJobQuery 解析后的查询条件
type GetJobQuery struct {
	Offset     int
	Size       int
	JobStatus  string // 逗号分隔状态码
	UserName   string
	DescFilter string
	ProjList   []ProjEntry
}

// GetJob 返回过滤+分页后的作业列表及总数
func (s *Store) GetJob(q GetJobQuery) ([]*Job, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 收集快照
	all := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		all = append(all, j)
	}

	// 过滤
	filtered := make([]*Job, 0, len(all))
	statusSet := parseStatusSet(q.JobStatus)
	descLower := strings.ToLower(strings.TrimSpace(q.DescFilter))
	for _, j := range all {
		if len(statusSet) > 0 && !statusSet[atoiSafe(j.JobStatus)] {
			continue
		}
		if q.UserName != "" && j.UserName != q.UserName {
			continue
		}
		if descLower != "" && !strings.Contains(strings.ToLower(j.JobDesc), descLower) {
			continue
		}
		if len(q.ProjList) > 0 && !matchProjList(j, q.ProjList) {
			continue
		}
		filtered = append(filtered, j)
	}

	// 按提交时间倒序
	sort.Slice(filtered, func(a, b int) bool {
		return filtered[a].CommitTime > filtered[b].CommitTime
	})

	total := len(filtered)
	// 分页
	if q.Offset < 0 {
		q.Offset = 0
	}
	if q.Offset > total {
		q.Offset = total
	}
	end := q.Offset + q.Size
	if end > total {
		end = total
	}
	if q.Size <= 0 {
		end = total
	}
	return filtered[q.Offset:end], total
}

// matchProjList 检查作业 attributes 是否匹配 projList 任一条目（非空字段需一致）
func matchProjList(j *Job, pl []ProjEntry) bool {
	am := attrMap(j.Attributes)
	for _, e := range pl {
		if e.Database != "" && e.Database != am["database"] {
			continue
		}
		if e.Project != "" && e.Project != am["project"] {
			continue
		}
		if e.Survey != "" && e.Survey != am["survey"] {
			continue
		}
		if e.Line != "" && e.Line != am["line"] {
			continue
		}
		return true
	}
	return false
}

func attrMap(attrs []string) map[string]string {
	m := make(map[string]string, len(attrs))
	for _, a := range attrs {
		if idx := strings.Index(a, "="); idx > 0 {
			m[a[:idx]] = a[idx+1:]
		}
	}
	return m
}

func parseStatusSet(s string) map[int]bool {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	m := make(map[int]bool)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		m[atoiSafe(part)] = true
	}
	return m
}

// Delete 删除指定作业
func (s *Store) Delete(names []string) (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ok, fail := 0, 0
	for _, n := range names {
		if _, exists := s.jobs[n]; exists {
			delete(s.jobs, n)
			ok++
		} else {
			fail++
		}
	}
	return ok, fail
}

// Rerunmulti 将失败/已取消作业重置为排队
func (s *Store) Rerunmulti(names []string) (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ok, fail := 0, 0
	now := time.Now().Unix()
	for _, n := range names {
		j, exists := s.jobs[n]
		if !exists {
			fail++
			continue
		}
		status := atoiSafe(j.JobStatus)
		if status == jsFailed || status == jsCanceled {
			j.JobStatus = fmt.Sprintf("%d", jsQueue)
			j.JobProcess = 0
			j.CommitTime = uint64(now)
			j.StartTime = 0
			j.EndTime = 0
			j.ExitCode = 0
			j.ReportMessage = ""
			j.ExecTime = 0
			j.WaitTime = 0
			ok++
		} else {
			fail++
		}
	}
	return ok, fail
}

func atoiSafe(s string) int {
	var n int
	for _, r := range s {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	return n
}
