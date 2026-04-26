// Package scheduler 实现后台定时任务
// 替代原 daemon/runner.py 的功能，以 Goroutine 形式内嵌在主进程中
package scheduler

import (
	"context"
	"sort"
	"sync"
	"time"

	"electricquery/internal/config"
	"electricquery/internal/logger"
	"electricquery/internal/service"

	"golang.org/x/sync/errgroup"
)

const maxConcurrency = 10 // 最多并发查询宿舍数

// Scheduler 持有定时任务的状态
type Scheduler struct {
	cfg        *config.AppConfig
	stopCh     chan struct{}
	once       sync.Once
	reportSent map[string]bool // 记录周报是否已发送（key: "YYYY-MM-DD"，避免同一天重复发送）
	reportMu   sync.Mutex
}

// New 创建调度器实例
func New(cfg *config.AppConfig) *Scheduler {
	return &Scheduler{
		cfg:        cfg,
		stopCh:     make(chan struct{}),
		reportSent: make(map[string]bool),
	}
}

// Start 在后台 Goroutine 中启动定时任务
func (s *Scheduler) Start() {
	s.once.Do(func() {
		go s.run()
		logger.Info("定时任务已启动",
			"poll_interval_sec", s.cfg.Scheduler.PollInterval,
			"alert_threshold", s.cfg.Scheduler.AlertThreshold)
	})
}

// Stop 优雅关闭调度器
func (s *Scheduler) Stop() {
	close(s.stopCh)
	logger.Info("定时任务已停止")
}

func (s *Scheduler) run() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("scheduler run goroutine panic recovered", "panic", r)
		}
	}()

	ticker := time.NewTicker(time.Duration(s.cfg.Scheduler.PollInterval) * time.Second)
	defer ticker.Stop()

	// 启动后不立即轮询，等 ticker 触发

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.pollAll()
			s.checkWeeklyReport()
		}
	}
}

// pollAll 轮询所有绑定宿舍的用户，查询电量并触发告警
// - 对宿舍去重（多人住同宿舍只查一次）
// - errgroup 并发（最大 5 个），单宿舍 15s 超时熔断
// - 按宿舍号排序遍历，日志顺序一致便于对账
func (s *Scheduler) pollAll() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("scheduler pollAll panic recovered", "panic", r)
		}
	}()

	users, err := service.GetAllUsersWithDorms()
	if err != nil {
		logger.Error("获取用户列表失败", "err", err)
		return
	}

	if len(users) == 0 {
		logger.Info("当前无绑定宿舍的用户")
		return
	}

	// 对同一宿舍去重，避免重复查询（多人住同一宿舍）
	dormSet := make(map[string]struct{})
	for _, u := range users {
		dormSet[u.DormRoom] = struct{}{}
	}

	// 排序，保证日志顺序一致
	dorms := make([]string, 0, len(dormSet))
	for d := range dormSet {
		dorms = append(dorms, d)
	}
	sort.Strings(dorms)

	logger.Info("开始轮询宿舍",
		"total", len(dorms),
		"max_concurrency", maxConcurrency)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency) // 信号量：最多 5 并发

	for _, dorm := range dorms {
		dorm := dorm // 闭包捕获循环变量
		g.Go(func() error {
			result, err := service.QueryAndSavePower(dorm, s.cfg)
			if err != nil {
				logger.Warn("查询失败", "dorm", dorm, "err", err)
				return nil // errgroup 需要返回 nil 才不会 cancel 其他 goroutine
			}
			var waterLog string
			if result.WaterAmount != "" {
				waterLog = " 水=" + result.WaterAmount + "吨"
			}
			logger.Debug("查询完成", "dorm", dorm, "电", result.RemainingKwh+"度", "water", waterLog)
			return nil
		})
	}

	// 等待所有 goroutine 完成（超时由 ctx 控制）
	if err := g.Wait(); err != nil {
		logger.Error("轮询异常退出", "err", err)
	}
}

// checkWeeklyReport 检查是否需要发送周报
func (s *Scheduler) checkWeeklyReport() {
	now := time.Now()
	cfg := s.cfg.Scheduler

	// 检查星期几和小时是否匹配
	if int(now.Weekday()) != cfg.WeeklyReportWeekday {
		return
	}
	if now.Hour() != cfg.WeeklyReportHour {
		return
	}

	// 同一天只发一次
	today := now.Format("2006-01-02")
	s.reportMu.Lock()
	defer s.reportMu.Unlock()
	if s.reportSent[today] {
		return
	}

	logger.Info("触发每周用电报告发送")
	service.SendWeeklyReport()
	s.reportSent[today] = true

	// 清理 7 天前的记录，防止 map 无限增长
	cutoff := now.AddDate(0, 0, -7).Format("2006-01-02")
	for k := range s.reportSent {
		if k < cutoff {
			delete(s.reportSent, k)
		}
	}
}
