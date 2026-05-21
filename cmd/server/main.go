package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"electricquery/internal/cache"
	"electricquery/internal/config"
	dormsyncer "electricquery/internal/dormsyncer"
	"electricquery/internal/handler"
	"electricquery/internal/logger"
	"electricquery/internal/middleware"
	"electricquery/internal/migrations"
	"electricquery/internal/model"
	"electricquery/internal/notifier"
	"electricquery/internal/scheduler"

	"github.com/gin-gonic/gin"
)

func main() {
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "application.conf"
	}
	absPath, _ := filepath.Abs(cfgPath)
	if cfgLog, err := config.ParseLogConfigFile(cfgPath); err == nil {
		os.MkdirAll(filepath.Dir(cfgLog.Path), 0750)
		log.Printf("[boot] 配置文件: %s (level=%s)", absPath, cfgLog.Level)
	}

	cfg := config.Load()
	os.Unsetenv("GIN_MODE")
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	logger.Init(
		cfg.Log.Level,
		cfg.Log.Path,
		cfg.Log.MaxSizeMB,
		cfg.Log.MaxBackups,
		cfg.Log.MaxAgeDays,
		cfg.Log.Compress,
		cfg.Log.Console,
	)
	logger.Info("ElectricQuery 启动", "host", cfg.App.Host, "port", cfg.App.Port, "config", absPath)

	model.InitDB(cfg)
	cache.InitCache()

	if err := migrations.RunAll(); err != nil {
		logger.Error("数据库迁移失败", "err", err)
		os.Exit(1)
	}

	notifier.Init(cfg)

	syncer := dormsyncer.NewSyncer(model.DB, cfg)
	if os.Getenv("SKIP_SYNC") != "1" {
		syncer.EnsureInitialized()
	}
	syncer.StartPeriodicSync()

	sched := scheduler.New(cfg)
	sched.Start()

	r := gin.New()
	r.Use(gin.Logger(), customRecovery())
	r.Use(middleware.CORS())

	middleware.InitRateLimiter()
	serveFrontend(r)

	r.GET("/health", func(c *gin.Context) {
		checks := gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		}
		healthy := true

		sqlDB, err := model.DB.DB()
		if err != nil || sqlDB.Ping() != nil {
			checks["db"] = "fail"
			healthy = false
		} else {
			checks["db"] = "ok"
		}

		checks["target"] = "unknown"
		if cfg.PowerChecker.LoginURL != "" {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, http.MethodHead, cfg.PowerChecker.LoginURL, nil)
			req.Header.Set("User-Agent", cfg.PowerChecker.UserAgent)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				checks["target"] = "unreachable"
			} else {
				resp.Body.Close()
				checks["target"] = "ok"
			}
		}

		if healthy {
			c.JSON(http.StatusOK, checks)
		} else {
			c.JSON(http.StatusServiceUnavailable, checks)
		}
	})

	api := r.Group("/api")

	auth := api.Group("/auth")
	{
		auth.POST("/register", middleware.RateLimitRegister(), handler.Register)
		auth.POST("/login", middleware.RateLimitLogin(), handler.Login)
	}

	user := api.Group("/user")
	user.Use(middleware.JWTAuth())
	{
		user.GET("/profile", handler.GetProfile)
		user.PATCH("/profile", handler.UpdateProfile)
		user.POST("/student-id", handler.BindStudentID)
		user.POST("/validate-dorm", handler.ValidateDorm)
		user.POST("/change-password", handler.ChangePassword)
		user.GET("/totp/setup", handler.SetupTOTP)
		user.POST("/totp/enable", handler.EnableTOTP)
		user.POST("/totp/disable", handler.DisableTOTP)
		user.GET("/channel", handler.GetChannel)
		user.PUT("/channel", handler.UpdateChannel)
	}

	api.GET("/system/config", handler.GetSystemConfig)

	power := api.Group("/power")
	power.Use(middleware.JWTAuth())
	power.POST("/current", handler.QueryPower)

	api.GET("/records", middleware.JWTAuth(), handler.GetPowerHistory)

	water := api.Group("/water")
	water.Use(middleware.JWTAuth())
	water.POST("/balance", handler.QueryWaterPower)

	syncHandler := handler.NewSyncHandler(syncer)
	{
		api.POST("/sync/dorm-options", middleware.InternalAuth(), syncHandler.SyncDormOptions)
		api.GET("/sync/dorm-options", middleware.JWTAuth(), syncHandler.GetDormOptions)
	}

	adminHandler := handler.NewAdminHandler(syncer)
	admin := api.Group("/admin")
	admin.Use(middleware.AdminAuth())
	{
		admin.GET("/users", adminHandler.ListUsers)
		admin.DELETE("/users/:id", adminHandler.DeleteUser)
		admin.POST("/users/:id/reset-password", adminHandler.ResetPassword)
		admin.POST("/users/:id/disable-totp", adminHandler.ForceDisableTOTP)
		admin.GET("/sync/status", adminHandler.GetSyncStatus)
		admin.POST("/sync/trigger", adminHandler.TriggerSync)
		admin.POST("/power/query", adminHandler.QueryPower)
	}

	internal := api.Group("/internal")
	internal.Use(middleware.InternalAuth())
	{
		internal.GET("/power/:dorm", handler.InternalQueryPower)
	}

	addr := fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		logger.Info("HTTP 服务启动", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP 服务启动失败", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("收到关闭信号，正在优雅关闭...")
	sched.Stop()
	syncer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("强制关闭", "err", err)
	}
	logger.Info("服务已关闭")
}

func customRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recovered in request", "panic", r, "path", c.Request.URL.Path)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"msg": "服务器内部错误，请稍后重试",
				})
			}
		}()
		c.Next()
	}
}
