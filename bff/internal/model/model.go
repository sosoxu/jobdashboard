package model

import "strings"

// JobState enum (index-based mapping, confirmed).
const (
	JsFrozen = iota // 0
	JsQueue         // 1
	JsScheduled     // 2
	JsReady         // 3
	JsReleased      // 4
	JsActive        // 5
	JsSuspending    // 6
	JsSuspended     // 7
	JsResuming      // 8
	JsCanceling     // 9
	JsFinished      // 10
	JsFailed        // 11
	JsCanceled      // 12
)

// StateLabel returns a Chinese label for a status code.
func StateLabel(code int) string {
	switch code {
	case JsActive:
		return "运行中"
	case JsQueue:
		return "排队中"
	case JsFrozen:
		return "冻结"
	case JsScheduled:
		return "已调度"
	case JsReady:
		return "就绪"
	case JsReleased:
		return "已释放"
	case JsSuspending:
		return "挂起中"
	case JsSuspended:
		return "已挂起"
	case JsResuming:
		return "恢复中"
	case JsCanceling:
		return "取消中"
	case JsFinished:
		return "已完成"
	case JsFailed:
		return "失败"
	case JsCanceled:
		return "已取消"
	default:
		return "未知"
	}
}

// GroupKey maps a status code to a GetCurrentJSFInfo aggregate field.
func GroupKey(code int) string {
	switch code {
	case JsActive:
		return "active"
	case JsQueue:
		return "queue"
	case JsFinished:
		return "finish"
	case JsFailed:
		return "failed"
	case JsCanceled:
		return "canceled"
	case JsFrozen, JsScheduled, JsReady, JsReleased,
		JsSuspending, JsSuspended, JsResuming, JsCanceling:
		return "othercount"
	default:
		return "othercount"
	}
}

// JobInfo mirrors the upstream JobInfo structure.
type JobInfo struct {
	JobName          string   `json:"jobName"`
	JobDesc          string   `json:"jobDesc"`
	Attributes       []string `json:"attributes"`
	CtrlNode         string   `json:"ctrlNode"`
	RunNodes         []string `json:"runNodes"`
	JobStatus        string   `json:"jobStatus"`
	UserName         string   `json:"userName"`
	JobProcess       int      `json:"jobProcess"`
	CommitTime       uint64   `json:"commitTime"`
	ScheduleTime     uint64   `json:"scheduleTime"`
	StartTime        uint64   `json:"startTime"`
	EndTime          uint64   `json:"endTime"`
	EstimatedTime    uint     `json:"estimatedTime"`
	EstimatedWaitTime uint    `json:"estimatedWaitTime"`
	WaitTime         uint     `json:"waitTime"`
	ExecTime         uint     `json:"execTime"`
	JobQueue         string   `json:"jobQueue"`
	Application      string   `json:"application"`
	Character        uint     `json:"character"`
	Department       string   `json:"department"`
	ExitCode         uint     `json:"exitCode"`
	ReportMessage    string   `json:"reportMessage"`
	CrossGroup       uint     `json:"crossGroup"`
}

// AttrMap parses attributes like "database=ndp_check" into a map.
func (j *JobInfo) AttrMap() map[string]string {
	m := make(map[string]string, len(j.Attributes))
	for _, a := range j.Attributes {
		if idx := strings.Index(a, "="); idx > 0 {
			m[a[:idx]] = a[idx+1:]
		}
	}
	return m
}

// Project / Survey / Database extractors (from attributes).
func (j *JobInfo) Project() string  { return j.AttrMap()["project"] }
func (j *JobInfo) Survey() string   { return j.AttrMap()["survey"] }
func (j *JobInfo) Database() string { return j.AttrMap()["database"] }

// StatusCode parses the jobStatus string into an int.
func (j *JobInfo) StatusCode() int {
	var n int
	for _, r := range j.JobStatus {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// StatsSnapshot is a point-in-time aggregate from GetCurrentJSFInfo.
type StatsSnapshot struct {
	Ts         int64
	Active     int
	Queue      int
	Finish     int
	Failed     int
	Canceled   int
	OtherCount int
}

// UserJobStat is a per-user job count at a point in time.
type UserJobStat struct {
	Ts       int64
	UserName string
	JobCount int
}
