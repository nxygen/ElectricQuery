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

type RateLimitConfig struct {
	MaxRequests int
	Window      time.Duration
}

type windowEntry struct {
	count   int
	tsStart time.Time
	mu      sync.Mutex
}

func (e *windowEntry) checkAndRecord(cfg RateLimitConfig) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	if now.Sub(e.tsStart) >= cfg.Window {
		e.count = 0
		e.tsStart = now
	}

	if e.count >= cfg.MaxRequests {
		return false
	}
	e.count++
	return true
}

func (e *windowEntry) isExpired(window time.Duration) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return time.Since(e.tsStart) >= window
}

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

func (l *ipLimiter) get(ip string) *windowEntry {
	l.mu.RLock()
	e, ok := l.entries[ip]
	l.mu.RUnlock()
	if ok {
		return e
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok = l.entries[ip]; ok {
		return e
	}
	e = &windowEntry{count: 0, tsStart: time.Now()}
	l.entries[ip] = e
	return e
}

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

func getMaxLogin() int {
	cfg := config.Load()
	if cfg.App.MaxLoginPerWindow > 0 {
		return cfg.App.MaxLoginPerWindow
	}
	return 0
}

func getMaxRegister() int {
	cfg := config.Load()
	if cfg.App.MaxRegisterPerWindow > 0 {
		return cfg.App.MaxRegisterPerWindow
	}
	return 0
}

func RateLimitLogin() gin.HandlerFunc {
	cfg := config.Load()
	max := getMaxLogin()
	if max <= 0 {
		max = 10
	}
	windowSec := cfg.App.RateLimitWindowSec
	if windowSec <= 0 {
		windowSec = 300
	}
	rlCfg := RateLimitConfig{MaxRequests: max, Window: time.Duration(windowSec) * time.Second}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		entry := rlLogin.get(ip)
		if !entry.checkAndRecord(rlCfg) {
			logger.Warn("rate limit exceeded: login", "ip", ip)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": 429, "msg": "登录尝试次数过多，请在 " + windowStr(windowSec) + " 后重试"})
			return
		}
		c.Next()
	}
}

func RateLimitRegister() gin.HandlerFunc {
	cfg := config.Load()
	max := getMaxRegister()
	if max <= 0 {
		max = 5
	}
	windowSec := cfg.App.RateLimitWindowSec
	if windowSec <= 0 {
		windowSec = 600
	}
	rlCfg := RateLimitConfig{MaxRequests: max, Window: time.Duration(windowSec) * time.Second}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		entry := rlRegister.get(ip)
		if !entry.checkAndRecord(rlCfg) {
			logger.Warn("rate limit exceeded: register", "ip", ip)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": 429, "msg": "注册操作过于频繁，请在 " + windowStr(windowSec) + " 后重试"})
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
