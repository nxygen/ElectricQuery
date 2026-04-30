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

// isExpired 判断条目是否已过期（超过窗口时间未使用）
func (e *windowEntry) isExpired(window time.Duration) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return time.Since(e.tsStart) >= window
}

// ipLimiter per-IP 速率限制器
type ipLimiter struct {
	mu      sync.RWMutex
	entries map[string]*windowEntry
	window  time.Duration
}

func newIPLimiter(window time.Duration) *ipLimiter {
	l := &ipLimiter{
		entries: make(map[string]*windowEntry),
		window:  window,
	}
	go l.cleanupLoop()
	return l
}

// get 获取或创建指定 IP 的 entry
func (l *ipLimiter) get(ip string) *windowEntry {
	l.mu.RLock()
	e, ok := l.entries[ip]
	l.mu.RUnlock()
	if ok {
		return e
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	// double-check
	if e, ok = l.entries[ip]; ok {
		return e
	}
	e = &windowEntry{count: 0, tsStart: time.Now()}
	l.entries[ip] = e
	return e
}

// cleanupLoop 定期清理过期条目，防止内存泄漏
func (l *ipLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		for ip, e := range l.entries {
			if e.isExpired(l.window * 2) {
				delete(l.entries, ip)
			}
		}
		l.mu.Unlock()
	}
}

var (
	rlLogin    *ipLimiter
	rlRegister *ipLimiter
)

// InitRateLimiter 初始化所有速率限制器
func InitRateLimiter() {
	cfg := config.Load()
	loginWindow := time.Duration(cfg.App.RateLimitWindowSec) * time.Second
	if loginWindow <= 0 {
		loginWindow = 300 * time.Second
	}
	registerWindow := loginWindow
	rlLogin = newIPLimiter(loginWindow)
	rlRegister = newIPLimiter(registerWindow)
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

	return func(c *gin.Context) {
		ip := c.ClientIP()
		entry := rlLogin.get(ip)
		if !entry.checkAndRecord(rlCfg) {
			logger.Warn("rate limit exceeded: login", "ip", ip)
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

	return func(c *gin.Context) {
		ip := c.ClientIP()
		entry := rlRegister.get(ip)
		if !entry.checkAndRecord(rlCfg) {
			logger.Warn("rate limit exceeded: register", "ip", ip)
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
