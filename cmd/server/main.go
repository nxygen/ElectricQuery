// ElectricQuery - 宿舍电量查询与通知系统
// Go + Gin + GORM 多用户架构
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"electricquery/internal/config"
	"electricquery/internal/handler"
	"electricquery/internal/logger"
	"electricquery/internal/middleware"
	"electricquery/internal/model"
	"electricquery/internal/notifier"
	"electricquery/internal/scheduler"
	dormsyncer "electricquery/internal/dormsyncer"

	"github.com/gin-gonic/gin"
)

func main() {
	// ---- 1. 读取配置文件（仅取日志配置，用于尽早初始化日志模块）----
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "application.conf"
	}
	absPath, _ := filepath.Abs(cfgPath)
	raw, err := os.ReadFile(cfgPath)
	if err == nil {
		// 尝试解析 log 配置段（启动早期使用标准 log）
		var rawCfg map[string]any
		if json.Unmarshal(raw, &rawCfg) == nil {
			if logCfg, ok := rawCfg["log"].(map[string]any); ok {
				cfgLog := &struct {
					Level      string `json:"level"`
					Path       string `json:"path"`
					MaxSizeMB  int    `json:"max_size_mb"`
					MaxBackups int    `json:"max_backups"`
					MaxAgeDays int    `json:"max_age_days"`
					Compress   bool   `json:"compress"`
					Console    bool   `json:"console"`
				}{}
				if data, _ := json.Marshal(logCfg); json.Unmarshal(data, cfgLog) == nil {
					os.MkdirAll(filepath.Dir(absPath), 0750)
					workDir, _ := os.Getwd()
					_ = workDir
					log.Printf("[boot] 配置文件: %s (level=%s)", absPath, cfgLog.Level)
				}
			}
		}
	}

	// ---- 2. 加载配置 ----
	cfg := config.Load()
	// Gin 在包加载时就读取了 GIN_MODE 环境变量，需要先清掉让代码生效
	os.Unsetenv("GIN_MODE")
	// Gin 只支持 debug/release/test 三种模式，根据日志级别映射
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// ---- 3. 初始化日志模块（基于配置文件，支持文件轮转）----
	// 注意：启动早期的标准 log 输出会丢失，由后续的 logger 接管
	// 重新以标准 log 打印 banner（logger 接管后不再需要）
	log.Printf("[boot] ElectricQuery v1.0 启动中...")

	logger.Init(
		cfg.Log.Level,
		cfg.Log.Path,
		cfg.Log.MaxSizeMB,
		cfg.Log.MaxBackups,
		cfg.Log.MaxAgeDays,
		cfg.Log.Compress,
		cfg.Log.Console,
	)
	// 重新打印 banner 到结构化日志
	logger.Info("ElectricQuery 启动", "host", cfg.App.Host, "port", cfg.App.Port, "config", absPath)

	// ---- 4. 初始化数据库 ----
	model.InitDB(cfg)

	// ---- 5. 初始化通知器 ----
	notifier.Init(cfg)

	// ---- 6. 初始化下拉选项同步器 ----
	syncer := dormsyncer.NewSyncer(model.DB, cfg)

	// 启动检查：若无数据则阻塞同步一次；有数据则直接跳过
	// 注意：热更新（air/reflex 等）期间若不希望同步，可设环境变量 SKIP_SYNC=1
	if os.Getenv("SKIP_SYNC") != "1" {
		syncer.EnsureInitialized()
	}

	// 启动 30 天定时后台同步
	syncer.StartPeriodicSync()

	// ---- 7. 启动定时调度器 ----
	sched := scheduler.New(cfg)
	sched.Start()

	// ---- 8. 初始化 Gin 路由 ----
	r := gin.New()
	// 自定义 Recovery：panic 堆栈写入日志，响应体返回安全错误信息
	r.Use(gin.Logger(), customRecovery())
	r.Use(middleware.CORS())

	// 初始化速率限制器
	middleware.InitRateLimiter()

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().Format(time.RFC3339)})
	})

	// 静态前端（./frontend/dist 存在时挂载，支持 SPA 路由兜底）
	// 生产部署时确保 dist 目录与二进制文件同目录
	serveFrontend(r)

	api := r.Group("/api")

	// 认证接口（无需 JWT，但受速率限制保护）
	auth := api.Group("/auth")
	{
		auth.POST("/register", middleware.RateLimitRegister(), handler.Register)
		auth.POST("/login", middleware.RateLimitLogin(), handler.Login)
	}

	// 需要 JWT 的用户接口
	user := api.Group("/user")
	user.Use(middleware.JWTAuth())
	{
		user.GET("/profile",      handler.GetProfile)
		user.PATCH("/profile",   handler.UpdateProfile)
		user.POST("/student-id", handler.BindStudentID) // 独立绑定学号（唯一性校验）
		user.POST("/validate-dorm", handler.ValidateDorm) // 实时校验宿舍号
		user.POST("/change-password", handler.ChangePassword) // 修改密码
		user.GET("/totp/setup",   handler.SetupTOTP)   // 生成 TOTP 密钥
		user.POST("/totp/enable",  handler.EnableTOTP)  // 激活 TOTP
		user.POST("/totp/disable", handler.DisableTOTP) // 关闭 TOTP
		user.GET("/channel",      handler.GetChannel)
		user.PUT("/channel",      handler.UpdateChannel)
	}

	// 系统配置（公开接口，无需认证）
	api.GET("/system/config", handler.GetSystemConfig)

	// 需要 JWT 的电量接口
	power := api.Group("/power")
	power.Use(middleware.JWTAuth())
	power.POST("/current", handler.QueryPower)

	// 历史记录
	api.GET("/records", middleware.JWTAuth(), handler.GetPowerHistory)

	// 需要 JWT 的水量接口
	water := api.Group("/water")
	water.Use(middleware.JWTAuth())
	water.POST("/balance", handler.QueryWaterPower)

	// 下拉选项同步（POST 需要 Internal Token，GET 需要 JWT）
	syncHandler := handler.NewSyncHandler(syncer)
	{
		api.POST("/sync/dorm-options", middleware.InternalAuth(), syncHandler.SyncDormOptions) // 触发同步
		api.GET("/sync/dorm-options", middleware.JWTAuth(), syncHandler.GetDormOptions)       // 获取选项（用户）
	}

	// 管理后台接口（仅需 Admin Token 鉴权，无需普通用户 JWT）
	adminHandler := handler.NewAdminHandler(syncer)
	admin := api.Group("/admin")
	admin.Use(middleware.AdminAuth()) // 仅需 AdminToken，无需 JWT
	{
		admin.GET("/users",            adminHandler.ListUsers)           // 用户列表
		admin.DELETE("/users/:id",     adminHandler.DeleteUser)          // 删除用户
		admin.POST("/users/:id/reset-password", adminHandler.ResetPassword)   // 重置密码
		admin.POST("/users/:id/disable-totp",  adminHandler.ForceDisableTOTP) // 强制关闭两步验证
		admin.GET("/sync/status",      adminHandler.GetSyncStatus)       // 同步状态
		admin.POST("/sync/trigger",    adminHandler.TriggerSync)         // 手动触发同步
		admin.POST("/power/query",     adminHandler.QueryPower)          // 管理员手动查询宿舍电量
	}

	// 内部接口（Internal Token 鉴权）
	internal := api.Group("/internal")
	internal.Use(middleware.InternalAuth())
	{
		internal.GET("/power/:dorm", handler.InternalQueryPower)
	}

	// ---- 8. 启动 HTTP 服务（优雅关闭）----
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

	// 等待系统信号（Ctrl+C / SIGTERM）优雅关闭
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

// customRecovery provides a panic-safe recovery middleware that logs the
// stack trace and returns a generic error to the client.
func customRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// 记录堆栈信息（不返回给客户端）
				logger.Error("panic recovered in request", "panic", r, "path", c.Request.URL.Path)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"msg": "服务器内部错误，请稍后重试",
				})
			}
		}()
		c.Next()
	}
}
