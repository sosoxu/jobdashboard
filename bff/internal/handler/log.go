package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/dashboard/bff/internal/service"
)

type LogHandler struct {
	logs     *service.LogService
	analyzer service.Analyzer
}

func NewLogHandler(logs *service.LogService, analyzer service.Analyzer) *LogHandler {
	return &LogHandler{logs: logs, analyzer: analyzer}
}

// GET /api/v1/jobs/:jobName/logs?type=list|log&level=all|info|warn|error&keyword=&project=&survey=
func (h *LogHandler) Logs(c *gin.Context) {
	jobName := c.Param("jobName")
	logType := c.DefaultQuery("type", "list")
	level := c.DefaultQuery("level", "all")
	keyword := c.Query("keyword")
	project := c.Query("project")
	survey := c.Query("survey")

	if project == "" || survey == "" {
		failBadRequest(c, "project 和 survey 为必填参数")
		return
	}

	res, err := h.logs.Read(c.Request.Context(), jobName, project, survey, logType, level, keyword)
	if err != nil {
		failBadRequest(c, err.Error())
		return
	}
	ok(c, res)
}

// POST /api/v1/jobs/:jobName/logs/analyze
type analyzeReq struct {
	Type    string `json:"type"`
	Level   string `json:"level"`
	Keyword string `json:"keyword"`
	Project string `json:"project"`
	Survey  string `json:"survey"`
}

func (h *LogHandler) Analyze(c *gin.Context) {
	jobName := c.Param("jobName")
	var req analyzeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		// allow empty body; fall back to query params
		req.Type = c.DefaultQuery("type", "log")
		req.Level = c.DefaultQuery("level", "all")
		req.Project = c.Query("project")
		req.Survey = c.Query("survey")
	}
	if req.Type == "" {
		req.Type = "log"
	}
	if req.Project == "" || req.Survey == "" {
		failBadRequest(c, "project 和 survey 为必填参数")
		return
	}

	logRes, err := h.logs.Read(c.Request.Context(), jobName, req.Project, req.Survey, req.Type, "all", req.Keyword)
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
