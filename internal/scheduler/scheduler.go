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
	maxConcurrency = 5
	notifyBufSize  = 64
)

type notifyKind int

const (
	kindWeeklyReport notifyKind = iota
)

type notifyJob struct {
	kind notifyKind
}

type Scheduler struct {
	cfg         *config.AppConfig
	stopCh      chan struct{}
	once        sync.Once
	notifyCh    chan notifyJob
	pollResetCh chan struct{}
}

func New(cfg *config.AppConfig) *Scheduler {
	return &Scheduler{
		cfg:         cfg,
		stopCh:      make(chan struct{}),
		notifyCh:    make(chan notifyJob, notifyBufSize),
		pollResetCh: make(chan struct{}, 1),
	}
}

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

func (s *Scheduler) Stop() {
	close(s.stopCh)
	logger.Info("定时任务已停止")
}

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
			resetTimer(timer, interval)
			logger.Debug("轮询计时器已重置（通知触发）")

		case <-timer.C:
			go s.pollAll(false)
			timer.Reset(interval)
		}
	}
}

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
			go func() {
				s.pollAll(true)
				select {
				case s.pollResetCh <- struct{}{}:
				default:
				}
			}()
		}
	}
}

func (s *Scheduler) nextNotifyTime() time.Time {
	cfg := s.cfg.Scheduler
	now := time.Now()

	daysUntil := (cfg.WeeklyReportWeekday - int(now.Weekday()) + 7) % 7
	target := time.Date(now.Year(), now.Month(), now.Day(),
		cfg.WeeklyReportHour, 0, 0, 0, now.Location()).
		AddDate(0, 0, daysUntil)

	if !target.After(now) || time.Until(target) < 30*time.Second {
		target = target.AddDate(0, 0, 7)
	}
	return target
}

func (s *Scheduler) runNotifyWorker() {
	defer safeRecover("runNotifyWorker")

	for {
		select {
		case <-s.stopCh:
			s.drainNotifyQueue()
			return
		case job := <-s.notifyCh:
			s.handleNotify(job)
		}
	}
}

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

	type dormPair struct {
		dormRoom       string
		waterDormRoom  string
	}
	dormMap := make(map[string]dormPair)
	for _, u := range users {
		if u.DormRoom == "" {
			continue
		}
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

	if notify {
		select {
		case s.notifyCh <- notifyJob{kind: kindWeeklyReport}:
		default:
			logger.Warn("通知队列已满，本次通知已丢弃")
		}
	}
}

func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

func safeRecover(name string) {
	if r := recover(); r != nil {
		logger.Error("goroutine panic recovered", "goroutine", name, "panic", r)
	}
}
