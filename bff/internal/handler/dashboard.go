package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/dashboard/bff/internal/service"
)

type DashboardHandler struct {
	stats *service.StatsService
}

func NewDashboardHandler(stats *service.StatsService) *DashboardHandler {
	return &DashboardHandler{stats: stats}
}

// GET /api/v1/dashboard/stats?fresh=0|1
func (h *DashboardHandler) Stats(c *gin.Context) {
	fresh := c.Query("fresh") == "1"
	res, err := h.stats.Stats(c.Request.Context(), fresh)
	if err != nil {
		failInternal(c, "查询统计失败: "+err.Error())
		return
	}
	ok(c, res)
}

// GET /api/v1/dashboard/trend?range=24h|7d|30d
func (h *DashboardHandler) Trend(c *gin.Context) {
	r := c.DefaultQuery("range", "24h")
	res, err := h.stats.Trend(c.Request.Context(), r)
	if err != nil {
		failInternal(c, "查询趋势失败: "+err.Error())
		return
	}
	ok(c, res)
}

// GET /api/v1/dashboard/top-users?limit=10
func (h *DashboardHandler) TopUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	res, err := h.stats.TopUsers(c.Request.Context(), limit)
	if err != nil {
		failInternal(c, "查询Top用户失败: "+err.Error())
		return
	}
	ok(c, res)
}

// RegisterDashboardRoutes mounts dashboard routes.
func RegisterDashboardRoutes(rg *gin.RouterGroup, h *DashboardHandler) {
	rg.GET("/dashboard/stats", h.Stats)
	rg.GET("/dashboard/trend", h.Trend)
	rg.GET("/dashboard/top-users", h.TopUsers)
}
