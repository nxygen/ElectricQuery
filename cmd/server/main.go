// ElectricQuery - 宿舍电量查询与通知系统
// Go + Gin + GORM 多用户架构
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"electricquery/internal/config"
	"electricquery/internal/handler"
	"electricquery/internal/middleware"
	"electricquery/internal/model"
	"electricquery/internal/notifier"
	"electricquery/internal/scheduler"

	"github.com/gin-gonic/gin"
)

func main() {
	// ---- 1. 加载配置 ----
	cfg := config.Load()
	gin.SetMode(cfg.App.Mode)

	// ---- 2. 初始化数据库 ----
	model.InitDB(cfg)

	// ---- 3. 初始化通知器 ----
	notifier.Init(cfg)

	// ---- 4. 启动定时调度器 ----
	sched := scheduler.New(cfg)
	sched.Start()

	// ---- 5. 初始化 Gin 路由 ----
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(middleware.CORS())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().Format(time.RFC3339)})
	})

	api := r.Group("/api")

	// ---- 认证接口（无需 JWT）----
	auth := api.Group("/auth")
	{
		auth.POST("/register", handler.Register)
		auth.POST("/login", handler.Login)
	}

	// ---- 需要 JWT 的用户接口 ----
	user := api.Group("/user")
	user.Use(middleware.JWTAuth())
	{
		user.GET("/profile",      handler.GetProfile)
		user.PATCH("/profile",   handler.UpdateProfile)
		user.POST("/student-id", handler.BindStudentID) // 独立绑定学号（唯一性校验）
		user.POST("/validate-dorm", handler.ValidateDorm) // 实时校验宿舍号
		user.GET("/channel",      handler.GetChannel)
		user.PUT("/channel",      handler.UpdateChannel)
	}

	// ---- 需要 JWT 的电量接口 ----
	power := api.Group("/power")
	power.Use(middleware.JWTAuth())
	{
		power.POST("/query",  handler.QueryPower)
		power.POST("/water",  handler.QueryWaterPower) // 水费查询
		power.GET("/history", handler.GetPowerHistory)
	}

	// ---- 内部接口（Internal Token 鉴权）----
	internal := api.Group("/internal")
	internal.Use(middleware.InternalAuth())
	{
		internal.GET("/power/:dorm", handler.InternalQueryPower)
	}

	// ---- 6. 启动 HTTP 服务（优雅关闭）----
	addr := fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		log.Printf("[server] ElectricQuery 启动，监听 %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[server] 启动失败: %v", err)
		}
	}()

	// 等待系统信号（Ctrl+C / SIGTERM）优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[server] 收到关闭信号，正在优雅关闭...")
	sched.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("[server] 强制关闭: %v", err)
	}
	log.Println("[server] 服务已关闭")
}
