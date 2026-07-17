package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/dashboard/bff/internal/cache"
	"github.com/dashboard/bff/internal/config"
	"github.com/dashboard/bff/internal/handler"
	"github.com/dashboard/bff/internal/logpath"
	"github.com/dashboard/bff/internal/sampler"
	"github.com/dashboard/bff/internal/service"
	"github.com/dashboard/bff/internal/store"
	"github.com/dashboard/bff/internal/upstream"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfgPath := ""
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Error("load config failed", "err", err)
		os.Exit(1)
	}

	// Persistence.
	db, err := store.Open(cfg.Storage.SqlitePath)
	if err != nil {
		logger.Error("open db failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	statsRepo := store.NewStatsRepo(db)
	userRepo := store.NewUserRepo(db)
	authRepo := store.NewAuthRepo(db)

	// Upstream client.
	client := upstream.New(cfg.Upstream.JobServiceURL, cfg.Upstream.TimeoutSec)

	// In-memory full-job cache (shared by sampler and job list/filter service).
	jobCache := cache.NewJobCache()

	// Services.
	statsSvc := service.NewStatsService(client, statsRepo, userRepo)
	jobSvc := service.NewJobService(client, jobCache, cfg.Sampler.JobPageSize, cfg.Sampler.JobPageSleepMs, cfg.Sampler.JobIntervalSec)
	resolver := logpath.New(cfg.Log.NgpEnv, cfg.Log.ProjectsConfRel, logger)
	logSvc := service.NewLogService(resolver, cfg.Log.MaxLogLines)
	analyzer := service.NewRuleAnalyzer()
	authSvc := service.NewAuthService(authRepo, cfg.Auth.JWTSecret, cfg.Auth.TokenTTLHour)

	// Sampler scheduler.
	sched := sampler.New(logger)
	if cfg.Sampler.Enabled {
		jsf := sampler.NewJSFSampler(client, statsRepo, logger)
		job := sampler.NewJobSampler(client, userRepo, jobCache, cfg.Sampler.JobPageSize, cfg.Sampler.JobPageSleepMs, logger)
		cleanup := sampler.NewCleanup(statsRepo, userRepo, cfg.Sampler.RetainDays, logger)

		// Bootstrap one immediate sample so the dashboard isn't empty on boot.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			jsf.Sample(ctx)
			job.Sample(ctx)
		}()

		sched.Register("jsf", time.Duration(cfg.Sampler.JsfIntervalSec)*time.Second, jsf.Sample)
		sched.Register("job", time.Duration(cfg.Sampler.JobIntervalSec)*time.Second, job.Sample)
		sched.Register("cleanup", 24*time.Hour, cleanup.Run)
		sched.Start()
		logger.Info("sampler started",
			"jsfInterval", cfg.Sampler.JsfIntervalSec,
			"jobInterval", cfg.Sampler.JobIntervalSec)
	}
	defer sched.Stop()

	// HTTP server.
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginLogger(logger))

	api := r.Group("/api/v1")

	// 公开路由：注册/登录
	authH := handler.NewAuthHandler(authSvc)
	handler.RegisterAuthRoutes(api, authH)

	// 受保护路由：需登录（携带 Authorization Bearer token）
	protected := api.Group("")
	protected.Use(handler.AuthMiddleware(authSvc))
	dashH := handler.NewDashboardHandler(statsSvc)
	jobH := handler.NewJobHandler(jobSvc)
	logH := handler.NewLogHandler(logSvc, analyzer)
	handler.RegisterDashboardRoutes(protected, dashH)
	handler.RegisterJobRoutes(protected, jobH)
	handler.RegisterLogRoutes(protected, logH)

	// Health check.
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// Static frontend (served from ./web/dist if present).
	frontendDir := "web/dist"
	if _, err := os.Stat(frontendDir); err == nil {
		registerStatic(r, frontendDir)
		logger.Info("serving frontend", "dir", frontendDir)
	} else {
		r.GET("/", func(c *gin.Context) {
			c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("BFF running. Frontend not built (web/dist missing). API at /api/v1.\n"))
		})
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown.
	go func() {
		logger.Info("server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// registerStatic serves the SPA: static assets + SPA fallback to index.html.
func registerStatic(r *gin.Engine, dir string) {
	abs, _ := filepath.Abs(dir)
	r.Use(func(c *gin.Context) {
		// Let API routes take precedence (they're registered before this middleware).
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.Next()
			return
		}
		p := filepath.Join(abs, filepath.Clean(c.Request.URL.Path))
		if !isWithin(p, abs) {
			c.Next()
			return
		}
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			c.File(p)
			c.Abort()
			return
		}
		// SPA fallback.
		c.File(filepath.Join(abs, "index.html"))
		c.Abort()
	})
}

func isWithin(target, root string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == ".." || len(rel) >= 3 && rel[:3] == "../" {
		return false
	}
	return true
}

func ginLogger(l *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		l.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency", time.Since(start).String(),
		)
	}
}
