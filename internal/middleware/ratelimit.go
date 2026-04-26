package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"electricquery/internal/config"
	"electricquery/internal/logger"
)

// RateLimitConfig 速率限制配置参数
type RateLimitConfig struct {
	MaxRequests int
	Window      time.Duration
}

// windowEntry 滑动窗口内请求计数器
type windowEntry struct {
	count   int
	tsStart time.Time
	mu      sync.Mutex
}

var (
	rlLogin    *windowEntry
	rlRegister *windowEntry
	rlOnce     sync.Once
)

// InitRateLimiter 初始化所有窗口条目（仅执行一次）
func InitRateLimiter() {
	rlOnce.Do(func() {
		now := time.Now()
		rlLogin    = &windowEntry{count: 0, tsStart: now}
		rlRegister = &windowEntry{count: 0, tsStart: now}
	})
}

// clientIP 从请求头提取真实客户端 IP
func clientIP(c *gin.Context) string {
	if fwd := c.GetHeader("X-Forwarded-For"); fwd != "" {
		for i := 0; i < len(fwd); i++ {
			if fwd[i] == ',' {
				return fwd[:i]
			}
		}
		return fwd
	}
	if fwd := c.GetHeader("X-Real-IP"); fwd != "" {
		return fwd
	}
	return c.ClientIP()
}

// checkAndRecord 原子性地检查并递增计数器，请求允许时返回 true
func (e *windowEntry) checkAndRecord(cfg RateLimitConfig) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	if now.Sub(e.tsStart) >= cfg.Window {
		// 窗口过期，重置计数器
		e.count = 0
		e.tsStart = now
	}

	if e.count >= cfg.MaxRequests {
		return false
	}
	e.count++
	return true
}

// getMaxLogin 从配置读取自定义登录速率限制，未配置时返回 0（由中间件使用默认值）
func getMaxLogin() int {
	cfg := config.Load()
	if cfg.App.MaxLoginPerWindow > 0 {
		return cfg.App.MaxLoginPerWindow
	}
	return 0 // signals "use default"
}

// getMaxRegister 从配置读取自定义注册速率限制
func getMaxRegister() int {
	cfg := config.Load()
	if cfg.App.MaxRegisterPerWindow > 0 {
		return cfg.App.MaxRegisterPerWindow
	}
	return 0
}

// RateLimitLogin 登录速率限制中间件
// 默认值：每 IP 每 5 分钟 10 次
// 可通过 application.conf 中的 app.max_login_per_window 配置
func RateLimitLogin() gin.HandlerFunc {
	cfg := config.Load()
	max := getMaxLogin()
	if max <= 0 {
		max = 10
	}
	windowSec := cfg.App.RateLimitWindowSec
	if windowSec <= 0 {
		windowSec = 300 // 5 minutes default
	}
	rlCfg := RateLimitConfig{MaxRequests: max, Window: time.Duration(windowSec) * time.Second}
	_ = cfg // ensure config is loaded

	return func(c *gin.Context) {
		if !rlLogin.checkAndRecord(rlCfg) {
			logger.Warn("rate limit exceeded: login", "ip", clientIP(c))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"msg": "登录尝试次数过多，请在 " + windowStr(windowSec) + " 后重试",
			})
			return
		}
		c.Next()
	}
}

// RateLimitRegister 注册速率限制中间件
// 默认值：每 IP 每 10 分钟 5 次
// 可通过 application.conf 中的 app.max_register_per_window 配置
func RateLimitRegister() gin.HandlerFunc {
	cfg := config.Load()
	max := getMaxRegister()
	if max <= 0 {
		max = 5
	}
	windowSec := cfg.App.RateLimitWindowSec
	if windowSec <= 0 {
		windowSec = 600 // 10 minutes default for register
	}
	rlCfg := RateLimitConfig{MaxRequests: max, Window: time.Duration(windowSec) * time.Second}
	_ = cfg

	return func(c *gin.Context) {
		if !rlRegister.checkAndRecord(rlCfg) {
			logger.Warn("rate limit exceeded: register", "ip", clientIP(c))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"msg": "注册操作过于频繁，请在 " + windowStr(windowSec) + " 后重试",
			})
			return
		}
		c.Next()
	}
}

func windowStr(sec int) string {
	if sec >= 60 {
		return fmt.Sprintf("%d 分钟", sec/60)
	}
	return fmt.Sprintf("%d 秒", sec)
}
