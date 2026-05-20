// Package scheduler 实现后台定时任务
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

const (
	maxConcurrency = 5  // 宿舍并发查询上限
	notifyBufSize  = 64 // 通知队列缓冲大小
)

// notifyKind 通知任务类型，未来可扩展 kindLowPower / kindWaterLow 等
type notifyKind int

const (
	kindWeeklyReport notifyKind = iota
)

// notifyJob 通知队列任务
type notifyJob struct {
	kind notifyKind
}

// Scheduler 持有定时任务的状态
type Scheduler struct {
	cfg         *config.AppConfig
	stopCh      chan struct{}
	once        sync.Once
	notifyCh    chan notifyJob // 通知队列（buffered）
	pollResetCh chan struct{}  // 通知 pollLoop 重置计时器（buffered 1）
}

// New 创建调度器实例
func New(cfg *config.AppConfig) *Scheduler {
	return &Scheduler{
		cfg:         cfg,
		stopCh:      make(chan struct{}),
		notifyCh:    make(chan notifyJob, notifyBufSize),
		pollResetCh: make(chan struct{}, 1),
	}
}

// Start 启动所有后台 Goroutine
func (s *Scheduler) Start() {
	s.once.Do(func() {
		go s.runPollLoop()
		go s.runNotifyLoop()
		go s.runNotifyWorker()
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

// ─── 轮询循环 ─────────────────────────────────────────────────────────────────
//
// 使用 time.Timer（而非 Ticker），支持被 pollResetCh 重置。
// 当通知任务完成后，pollResetCh 收到信号，计时器从当前时间重新开始。

func (s *Scheduler) runPollLoop() {
	defer safeRecover("runPollLoop")

	interval := time.Duration(s.cfg.Scheduler.PollInterval) * time.Second
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-s.stopCh:
			return

		case <-s.pollResetCh:
			// 通知查询已完成，重置轮询计时器，避免短时间内重复查询
			resetTimer(timer, interval)
			logger.Debug("轮询计时器已重置（通知触发）")

		case <-timer.C:
			// timer.C 已被当前 case 读出，channel 已空，直接 Reset 是安全的
			// 不会与 drain resetTimer 冲突，因为 drain 只会读空的 C
			go s.pollAll(false)
			timer.Reset(interval)
		}
	}
}

// ─── 通知定时器循环 ────────────────────────────────────────────────────────────
//
// 精确计算下次触发时间（按配置的星期几 + 小时），
// 与轮询周期完全解耦，不再依赖 ticker 间接触发。

func (s *Scheduler) runNotifyLoop() {
	defer safeRecover("runNotifyLoop")

	for {
		next := s.nextNotifyTime()
		logger.Info("下次通知定时", "at", next.Format("2006-01-02 15:04:05"))
		timer := time.NewTimer(time.Until(next))

		select {
		case <-s.stopCh:
			timer.Stop()
			return

		case <-timer.C:
			// 触发通知查询：异步执行，完成后 reset 轮询计时器
			go func() {
				s.pollAll(true)
				// 非阻塞投递，避免 notifyLoop 卡住
				select {
				case s.pollResetCh <- struct{}{}:
				default:
				}
			}()
		}
	}
}

// nextNotifyTime 计算下一次通知触发时间
// 按配置的 WeeklyReportWeekday（0=周日..6=周六）+ WeeklyReportHour（小时）
//
// 注意：若计算结果距离当前时间不足 30 秒（服务刚启动时可能发生），
// 视为"已过"直接推到下一周期，避免刚启动就立即触发一次。
func (s *Scheduler) nextNotifyTime() time.Time {
	cfg := s.cfg.Scheduler
	now := time.Now()

	daysUntil := (cfg.WeeklyReportWeekday - int(now.Weekday()) + 7) % 7
	target := time.Date(now.Year(), now.Month(), now.Day(),
		cfg.WeeklyReportHour, 0, 0, 0, now.Location()).
		AddDate(0, 0, daysUntil)

	// 目标时间已过（含今天同一时刻），或距离过近（<30s），推到下一周期
	if !target.After(now) || time.Until(target) < 30*time.Second {
		target = target.AddDate(0, 0, 7)
	}
	return target
}

// ─── 通知 Worker ──────────────────────────────────────────────────────────────
//
// 单 goroutine 顺序消费通知队列，避免并发发送导致重复通知。
// 扩展新通知类型时，在 handleNotify 的 switch 中追加即可。

func (s *Scheduler) runNotifyWorker() {
	defer safeRecover("runNotifyWorker")

	for {
		select {
		case <-s.stopCh:
			// 优雅退出：消费完队列中剩余的通知任务
			s.drainNotifyQueue()
			return
		case job := <-s.notifyCh:
			s.handleNotify(job)
		}
	}
}

// drainNotifyQueue 关闭前清空通知队列，逐条记录丢弃日志
func (s *Scheduler) drainNotifyQueue() {
	drained := 0
	for {
		select {
		case job := <-s.notifyCh:
			drained++
			logger.Warn("通知丢弃（服务关闭）", "kind", job.kind)
		default:
			if drained > 0 {
				logger.Info("通知队列已清空", "drained", drained)
			}
			return
		}
	}
}

func (s *Scheduler) handleNotify(job notifyJob) {
	switch job.kind {
	case kindWeeklyReport:
		logger.Info("触发每周用电报告发送")
		service.SendWeeklyReport()
	}
}

// ─── 查询核心 ─────────────────────────────────────────────────────────────────
//
// notify=false：正常轮询，查询 + 写DB
// notify=true ：通知模式，查询 + 写DB，全部完成后投递 notifyCh

func (s *Scheduler) pollAll(notify bool) {
	defer safeRecover("pollAll")

	users, err := service.GetAllUsersWithDorms()
	if err != nil {
		logger.Error("获取用户列表失败", "err", err)
		return
	}
	if len(users) == 0 {
		logger.Info("当前无绑定宿舍的用户")
		return
	}

	// 宿舍去重：按 (dormRoom, waterDormRoom) 去重，每个宿舍对只查一次
	type dormPair struct {
		dormRoom       string
		waterDormRoom  string
	}
	dormMap := make(map[string]dormPair)
	for _, u := range users {
		if u.DormRoom == "" {
			continue
		}
		// waterDormRoom 从电表 drceng_value 反查（C13/C14 水电合一返回同值，C11/C12 分房返回独立水表）
		if _, exists := dormMap[u.DormRoom]; !exists {
			dormMap[u.DormRoom] = dormPair{dormRoom: u.DormRoom, waterDormRoom: service.ResolveWaterDormRoom(u.DormRoom)}
		}
	}
	dorms := make([]dormPair, 0, len(dormMap))
	for _, dp := range dormMap {
		dorms = append(dorms, dp)
	}
	sort.Slice(dorms, func(i, j int) bool {
		return dorms[i].dormRoom < dorms[j].dormRoom
	})

	logger.Info("开始轮询宿舍",
		"total", len(dorms),
		"max_concurrency", maxConcurrency,
		"notify_mode", notify)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	for _, dp := range dorms {
		dp := dp
		g.Go(func() error {
			result, err := service.QueryAndSavePower(ctx, dp.dormRoom, dp.waterDormRoom, s.cfg)
			if err != nil {
				logger.Warn("查询失败", "dorm", dp.dormRoom, "err", err)
				return nil
			}
			logger.Debug("查询完成", "dorm", dp.dormRoom, "kwh", result.RemainingKwh)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		logger.Error("轮询异常退出", "err", err)
	}

	// 注：水电在同一调用中处理（QueryAndSavePower 内部自动查水电并写 electricity_logs 和 water_logs）

	// 全部宿舍查询完成后，投递通知任务（非阻塞，队列满则丢弃并记录）
	if notify {
		select {
		case s.notifyCh <- notifyJob{kind: kindWeeklyReport}:
		default:
			logger.Warn("通知队列已满，本次通知已丢弃")
		}
	}
}

// ─── 工具函数 ─────────────────────────────────────────────────────────────────

// resetTimer 安全重置计时器
// time.Timer.Reset 前需确保 C 已被消费，否则会泄漏
func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

// safeRecover 捕获 goroutine panic，记录日志后不崩进程
func safeRecover(name string) {
	if r := recover(); r != nil {
		logger.Error("goroutine panic recovered", "goroutine", name, "panic", r)
	}
}
