package sync

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"electricquery/internal/cache"
	"electricquery/internal/checker"
	"electricquery/internal/config"
	"electricquery/internal/logger"
	"electricquery/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SyncStatus struct {
	LastSyncAt  *time.Time `json:"last_sync_at"`
	TotalRooms  int64      `json:"total_rooms"`
	IsRunning   bool       `json:"is_running"`
	NextSyncAt  *time.Time `json:"next_sync_at"`
	Initialized bool       `json:"initialized"`
}

type Syncer struct {
	db         *gorm.DB
	checker    *checker.Checker
	mu         sync.Mutex
	running    bool
	lastSyncAt *time.Time
	stopCh     chan struct{}
}

func NewSyncer(db *gorm.DB, cfg *config.AppConfig) *Syncer {
	s := &Syncer{
		db:      db,
		checker: checker.NewChecker(cfg),
		stopCh:  make(chan struct{}),
	}
	s.loadLastSyncAt()
	return s
}

func (s *Syncer) loadLastSyncAt() {
	var meta model.SyncMeta
	if err := s.db.Where("id = ?", "sync-meta").First(&meta).Error; err == nil && meta.LastSyncAt != nil {
		s.lastSyncAt = meta.LastSyncAt
	}
}

func (s *Syncer) saveLastSyncAt(t *time.Time) {
	meta := model.SyncMeta{
		ID:         "sync-meta",
		LastSyncAt: t,
	}
	s.db.Save(&meta)
}

func (s *Syncer) EnsureInitialized() {
	var count int64
	s.db.Model(&model.DormOption{}).Where("level = ?", model.OptionLevelRoom).Count(&count)
	if count > 0 {
		logger.Info("下拉选项已存在，跳过启动同步", "room_count", count)
		return
	}
	logger.Info("下拉选项为空，执行首次同步（阻塞启动）...")
	if err := s.SyncAll(); err != nil {
		logger.Warn("首次同步失败，服务仍将启动", "err", err)
	}
}

func (s *Syncer) StartPeriodicSync() {
	const period = 30 * 24 * time.Hour
	go func() {
		ticker := time.NewTicker(period)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				logger.Info("定时触发下拉选项同步（30天周期）")
				if err := s.SyncAll(); err != nil {
					logger.Warn("定时同步失败", "err", err)
				}
			}
		}
	}()
	logger.Info("下拉选项定时同步已启动", "period", "30d")
}

func (s *Syncer) Stop() {
	close(s.stopCh)
}

func (s *Syncer) Status() SyncStatus {
	s.mu.Lock()
	running := s.running
	lastAt := s.lastSyncAt
	s.mu.Unlock()

	var totalRooms int64
	s.db.Model(&model.DormOption{}).Where("level = ?", model.OptionLevelRoom).Count(&totalRooms)

	st := SyncStatus{
		LastSyncAt:  lastAt,
		TotalRooms:  totalRooms,
		IsRunning:   running,
		Initialized: totalRooms > 0,
	}
	if lastAt != nil {
		next := lastAt.Add(30 * 24 * time.Hour)
		st.NextSyncAt = &next
	}
	return st
}

func (s *Syncer) SyncAll() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("同步任务正在进行中，请稍后再试")
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	logger.Info("开始全量同步下拉选项")
	startAt := time.Now()

	syncedKeys := make(map[string]struct{})

	buildings := []string{"01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12", "13", "14"}

	successCount := 0
	for _, building := range buildings {
		if err := s.syncBuilding(building, syncedKeys); err != nil {
			logger.Warn("楼栋抓取失败", "building", building, "err", err)
		} else {
			successCount++
		}
		time.Sleep(500 * time.Millisecond)
	}

	if len(syncedKeys) > 0 {
		var keys []string
		for k := range syncedKeys {
			keys = append(keys, k)
		}
		if err := s.db.Where("key NOT IN ?", keys).Delete(&model.DormOption{}).Error; err != nil {
			logger.Warn("清理旧选项失败", "err", err)
		}
	}

	now := time.Now()
	s.mu.Lock()
	s.lastSyncAt = &now
	s.mu.Unlock()
	s.saveLastSyncAt(&now)
	cache.DeletePrefix("dorm_lookup:")
	cache.DeletePrefix("water_lookup:")
	cache.DeletePrefix("dorm_options")

	logger.Info("全量同步完成",
		"duration", time.Since(startAt).Round(time.Second).String(),
		"buildings_success", successCount,
		"buildings_total", len(buildings),
		"synced_keys", len(syncedKeys),
	)
	return nil
}

func (s *Syncer) syncBuilding(building string, syncedKeys map[string]struct{}) error {
	opt := model.DormOption{
		Level:       model.OptionLevelBuilding,
		Building:    building,
		Floor:       "",
		DrcengValue: building,
		FormValue:   building,
		Label:       "C" + building,
	}
	opt.Key = opt.Building + "_" + opt.Floor + "_" + opt.DrcengValue
	syncedKeys[opt.Key] = struct{}{}
	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"level", "building", "floor", "room_suffix", "drceng_value", "form_value", "label", "updated_at", "deleted_at"}),
	}).Create(&opt).Error; err != nil {
		return fmt.Errorf("存 building 选项失败: %w", err)
	}

	ablouValues, err := s.fetchAblouOptions(building)
	if err != nil {
		return fmt.Errorf("抓取 ablou 失败: %w", err)
	}

	for _, ablou := range ablouValues {
		floorOpt := model.DormOption{
			Level:       model.OptionLevelFloor,
			Building:    building,
			Floor:       ablou,
			DrcengValue: ablou,
			FormValue:   ablou,
			Label:       ablou,
		}
		floorOpt.Key = floorOpt.Building + "_" + floorOpt.Floor + "_" + floorOpt.DrcengValue
		syncedKeys[floorOpt.Key] = struct{}{}
		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"level", "building", "floor", "room_suffix", "drceng_value", "form_value", "label", "updated_at", "deleted_at"}),
		}).Create(&floorOpt).Error; err != nil {
			logger.Warn("存 ablou 选项失败", "building", building, "floor", ablou, "err", err)
			continue
		}

		drcengMap, err := s.fetchDrcengOptions(building, ablou)
		if err != nil {
			logger.Warn("抓取 drceng 失败", "building", building, "floor", ablou, "err", err)
			continue
		}

		for drceng, displayText := range drcengMap {
			label := displayText
			if label == "" {
				label = drceng
			}

			key := building + "_" + ablou + "_" + drceng
			syncedKeys[key] = struct{}{}

			var existing model.DormOption
			found := s.db.Unscoped().Where("key = ?", key).First(&existing).Error == nil

			if found {
				updates := map[string]interface{}{
					"label":        label,
					"drceng_value": drceng,
					"building":     building,
					"floor":        ablou,
					"room_suffix":  s.extractRoomSuffix(drceng),
					"level":        model.OptionLevelRoom,
					"form_value":   drceng,
					"updated_at":   time.Now(),
					"deleted_at":   nil,
				}
				if err := s.db.Unscoped().Model(&model.DormOption{}).Where("key = ?", key).Updates(updates).Error; err != nil {
					logger.Warn("更新 drceng 选项失败", "key", key, "err", err)
				}
			} else {
				roomOpt := model.DormOption{
					Level:       model.OptionLevelRoom,
					Building:    building,
					Floor:       ablou,
					RoomSuffix:  s.extractRoomSuffix(drceng),
					DrcengValue: drceng,
					FormValue:   drceng,
					Label:       label,
					Key:         key,
				}
				if err := s.db.Create(&roomOpt).Error; err != nil {
					logger.Warn("存 drceng 选项失败", "key", key, "err", err)
				}
			}
		}

		logger.Debug("楼层抓取完成", "building", building, "floor", ablou, "room_count", len(drcengMap))
		time.Sleep(300 * time.Millisecond)
	}

	logger.Debug("楼栋抓取完成", "building", building, "floor_count", len(ablouValues))
	return nil
}

func (s *Syncer) fetchAblouOptions(building string) ([]string, error) {
	_, html2, _, err := s.checker.StepByStep(context.Background(), building, "", "")
	if err != nil {
		return nil, err
	}
	return checker.ExtractDropOptions(html2, "ablou"), nil
}

func (s *Syncer) fetchDrcengOptions(building, floor string) (map[string]string, error) {
	_, html3, _, err := s.checker.StepByStep(context.Background(), building, floor, "")
	if err != nil {
		return nil, err
	}
	opts := checker.ExtractDropOptionsWithText(html3, "drceng")

	result := make(map[string]string, len(opts))
	for _, o := range opts {
		text := o.Text
		if text == "" {
			text = o.Value
		}
		result[o.Value] = text
	}
	return result, nil
}

func (s *Syncer) extractRoomSuffix(drcengValue string) string {
	var b strings.Builder
	for _, r := range drcengValue {
		if r < 0x4e00 || r > 0x9fff {
			b.WriteRune(r)
		}
	}
	v := b.String()
	if v == "" {
		return drcengValue
	}
	if strings.Contains(drcengValue, "电") || strings.Contains(drcengValue, "水") {
		return v
	}
	if len(v) >= 2 {
		return v[len(v)-2:]
	}
	return v
}
