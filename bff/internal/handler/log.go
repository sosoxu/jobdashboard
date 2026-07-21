package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/dashboard/bff/internal/cache"
	"github.com/dashboard/bff/internal/service"
)

type LogHandler struct {
	logs     *service.LogService
	analyzer service.Analyzer
	jobCache *cache.JobCache
}

func NewLogHandler(logs *service.LogService, analyzer service.Analyzer, jobCache *cache.JobCache) *LogHandler {
	return &LogHandler{logs: logs, analyzer: analyzer, jobCache: jobCache}
}

// resolveJobLocation 通过 jobName 在作业缓存中查 project/survey/line。
// jobName 全局唯一，前端不再需要传 project/survey；line 默认为空（无测线）。
// 返回 (project, survey, line, ok)。
func (h *LogHandler) resolveJobLocation(jobName string) (string, string, string, bool) {
	if h.jobCache == nil {
		return "", "", "", false
	}
	jobs, _ := h.jobCache.Snapshot()
	for i := range jobs {
		if jobs[i].JobName == jobName {
			return jobs[i].Project(), jobs[i].Survey(), jobs[i].Line(), true
		}
	}
	return "", "", "", false
}

// GET /api/v1/jobs/:jobName/logs?type=list|log&keyword=&page=1&pageSize=200
// project/survey/line 由 BFF 通过 jobName 查作业缓存获取（jobName 全局唯一），前端无需传。
func (h *LogHandler) Logs(c *gin.Context) {
	jobName := c.Param("jobName")
	logType := c.DefaultQuery("type", "list")
	keyword := c.Query("keyword")
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 0) // 0 → service default

	project, survey, line, found := h.resolveJobLocation(jobName)
	if !found {
		failBadRequest(c, "未在作业缓存中找到作业 "+jobName+"，无法解析 project/survey")
		return
	}

	res, err := h.logs.Read(c.Request.Context(), jobName, project, survey, line, logType, keyword, page, pageSize)
	if err != nil {
		failBadRequest(c, err.Error())
		return
	}
	ok(c, res)
}

// POST /api/v1/jobs/:jobName/logs/analyze
type analyzeReq struct {
	Type     string `json:"type"`
	Keyword  string `json:"keyword"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// Analyze 基于当前过滤条件拉取日志（最多分析 maxScanLines 行），再做规则诊断。
// 为避免对大文件做全量分析，前端可指定 page/pageSize 限定分析范围；未指定时
// 默认拉取前 2000 行（pageSize=2000, page=1）。
func (h *LogHandler) Analyze(c *gin.Context) {
	jobName := c.Param("jobName")
	var req analyzeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Type = c.DefaultQuery("type", "log")
		req.Keyword = c.Query("keyword")
	}
	if req.Type == "" {
		req.Type = "log"
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 2000
	}

	project, survey, line, found := h.resolveJobLocation(jobName)
	if !found {
		failBadRequest(c, "未在作业缓存中找到作业 "+jobName+"，无法解析 project/survey")
		return
	}

	logRes, err := h.logs.Read(c.Request.Context(), jobName, project, survey, line, req.Type, req.Keyword, req.Page, req.PageSize)
	if err != nil {
		failBadRequest(c, err.Error())
		return
	}
	ana, err := h.analyzer.Analyze(c.Request.Context(), logRes)
	if err != nil {
		failInternal(c, "分析失败: "+err.Error())
		return
	}
	ok(c, ana)
}

// RegisterLogRoutes mounts log routes.
func RegisterLogRoutes(rg *gin.RouterGroup, h *LogHandler) {
	rg.GET("/jobs/:jobName/logs", h.Logs)
	rg.POST("/jobs/:jobName/logs/analyze", h.Analyze)
}

func parsePositiveInt(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}
