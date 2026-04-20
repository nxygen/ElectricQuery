// Package scheduler 实现后台定时任务
// 替代原 daemon/runner.py 的功能，以 Goroutine 形式内嵌在主进程中
package scheduler

import (
	"log"
	"sync"
	"time"

	"electricquery/internal/config"
	"electricquery/internal/service"
)

// Scheduler 持有定时任务的状态
type Scheduler struct {
	cfg         *config.AppConfig
	stopCh      chan struct{}
	once        sync.Once
	reportSent  map[string]bool // 记录周报是否已发送（key: "YYYY-MM-DD"，避免同一天重复发送）
	reportMu    sync.Mutex
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
		log.Printf("[scheduler] 定时任务已启动，轮询间隔=%ds，告警阈值=%.1f度",
			s.cfg.Scheduler.PollInterval, s.cfg.Scheduler.AlertThreshold)
	})
}

// Stop 优雅关闭调度器
func (s *Scheduler) Stop() {
	close(s.stopCh)
	log.Println("[scheduler] 定时任务已停止")
}

func (s *Scheduler) run() {
	ticker := time.NewTicker(time.Duration(s.cfg.Scheduler.PollInterval) * time.Second)
	defer ticker.Stop()

	// 启动后立即执行一次（可选，注释掉可取消）
	s.pollAll()

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
func (s *Scheduler) pollAll() {
	users, err := service.GetAllUsersWithDorms()
	if err != nil {
		log.Printf("[scheduler] 获取用户列表失败: %v", err)
		return
	}

	if len(users) == 0 {
		log.Println("[scheduler] 当前无绑定宿舍的用户")
		return
	}

	// 对同一宿舍去重，避免重复查询（多人住同一宿舍）
	dormSet := make(map[string]struct{})
	for _, u := range users {
		dormSet[u.DormRoom] = struct{}{}
	}

	log.Printf("[scheduler] 开始轮询 %d 个宿舍", len(dormSet))
	for dorm := range dormSet {
		result, err := service.QueryAndSavePower(dorm, s.cfg)
		if err != nil {
			log.Printf("[scheduler] 查询失败 dorm=%s err=%v", dorm, err)
		} else {
			var waterLog string
			if result.WaterAmount != "" {
				waterLog = " 水=" + result.WaterAmount + "吨"
			}
			log.Printf("[scheduler] 查询完成 dorm=%s 电=%s度%s", dorm, result.RemainingKwh, waterLog)
		}
		// 两次查询之间稍作间隔，避免对目标系统造成过大压力
		time.Sleep(2 * time.Second)
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

	log.Println("[scheduler] 触发每周用电报告发送")
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
