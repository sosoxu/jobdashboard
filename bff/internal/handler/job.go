package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/dashboard/bff/internal/service"
)

type JobHandler struct {
	jobs *service.JobService
}

func NewJobHandler(jobs *service.JobService) *JobHandler {
	return &JobHandler{jobs: jobs}
}

// GET /api/v1/jobs
// Multi-value params are comma-separated, e.g. ?jobStatus=5,11&userName=u1,u2
func (h *JobHandler) List(c *gin.Context) {
	q := service.JobListQuery{
		JobStatus:       parseInts(c.Query("jobStatus")),
		UserName:        splitCSV(c.Query("userName")),
		Project:         splitCSV(c.Query("project")),
		Survey:          splitCSV(c.Query("survey")),
		Database:        splitCSV(c.Query("database")),
		JobDesc:         c.Query("jobDesc"),
		CommitTimeStart: parseInt64(c.Query("commitTimeStart")),
		CommitTimeEnd:   parseInt64(c.Query("commitTimeEnd")),
	}
	q.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	q.PageSize, _ = strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	res, err := h.jobs.List(c.Request.Context(), q)
	if err != nil {
		failInternal(c, "查询作业列表失败: "+err.Error())
		return
	}
	ok(c, res)
}

// GET /api/v1/jobs/filters
func (h *JobHandler) Filters(c *gin.Context) {
	res, err := h.jobs.Filters(c.Request.Context())
	if err != nil {
		failInternal(c, "获取过滤候选值失败: "+err.Error())
		return
	}
	ok(c, res)
}

type controlReq struct {
	Action string   `json:"action" binding:"required"`
	Names  []string `json:"names" binding:"required"`
}

// POST /api/v1/jobs/control  （需登录；只能控制自己提交的作业）
func (h *JobHandler) Control(c *gin.Context) {
	var req controlReq
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadRequest(c, "参数错误: "+err.Error())
		return
	}
	username, _ := c.Get("username")
	user, _ := username.(string)
	res, err := h.jobs.Control(c.Request.Context(), service.ControlAction(req.Action), req.Names, user)
	if err != nil {
		failBadRequest(c, err.Error())
		return
	}
	ok(c, res)
}

// RegisterJobRoutes mounts job routes.
func RegisterJobRoutes(rg *gin.RouterGroup, h *JobHandler) {
	rg.GET("/jobs", h.List)
	rg.GET("/jobs/filters", h.Filters)
	rg.POST("/jobs/control", h.Control)
}

// --- param helpers ---

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseInts(s string) []int {
	parts := splitCSV(s)
	if parts == nil {
		return nil
	}
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err == nil {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseInt64(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
