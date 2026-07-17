package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/dashboard/bff/internal/service"
)

// AuthHandler 处理注册/登录/当前用户接口。
type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

type authReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req authReq
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadRequest(c, "参数错误: 用户名和密码不能为空")
		return
	}
	res, err := h.svc.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		failBadRequest(c, err.Error())
		return
	}
	ok(c, res)
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req authReq
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadRequest(c, "参数错误: 用户名和密码不能为空")
		return
	}
	res, err := h.svc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		fail(c, http.StatusUnauthorized, err.Error())
		return
	}
	ok(c, res)
}

// GET /api/v1/auth/me  （需登录，路由注册时挂 AuthMiddleware）
func (h *AuthHandler) Me(c *gin.Context) {
	username, exists := c.Get("username")
	if !exists {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	ok(c, service.AuthUser{Username: username.(string)})
}

// AuthMiddleware 校验 Authorization: Bearer <token>，将用户名注入 gin.Context。
func AuthMiddleware(svc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c.GetHeader("Authorization"))
		if token == "" {
			fail(c, http.StatusUnauthorized, "未登录")
			c.Abort()
			return
		}
		username, err := svc.ParseToken(token)
		if err != nil {
			fail(c, http.StatusUnauthorized, "登录已过期，请重新登录")
			c.Abort()
			return
		}
		c.Set("username", username)
		c.Next()
	}
}

func extractBearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) > 7 && strings.EqualFold(header[:7], "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

// RegisterAuthRoutes 注册认证路由。register/login 公开；me 需登录。
func RegisterAuthRoutes(rg *gin.RouterGroup, h *AuthHandler) {
	rg.POST("/auth/register", h.Register)
	rg.POST("/auth/login", h.Login)
	rg.GET("/auth/me", AuthMiddleware(h.svc), h.Me)
}
