// Package sync 实现官网下拉选项同步功能
// 遍历所有楼栋/楼层，抓取完整的 drlouming/ablou/drceng 下拉选项并存储到数据库
package sync

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"electricquery/internal/checker"
	"electricquery/internal/config"
	"electricquery/internal/logger"
	"electricquery/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SyncStatus 同步状态快照（用于管理后台展示）
type SyncStatus struct {
	LastSyncAt  *time.Time `json:"last_sync_at"`  // 上次成功同步时间，nil 表示从未同步
	TotalRooms  int64      `json:"total_rooms"`   // 当前有效房间数
	IsRunning   bool       `json:"is_running"`    // 是否正在同步
	NextSyncAt  *time.Time `json:"next_sync_at"`  // 下次定时同步时间（30天周期）
	Initialized bool       `json:"initialized"`  // 是否已完成首次初始化（有数据）
}

// Syncer 负责同步官网下拉选项
type Syncer struct {
	db         *gorm.DB
	checker    *checker.Checker
	mu         sync.Mutex
	running    bool
	lastSyncAt *time.Time
	stopCh     chan struct{}
}

// NewSyncer 创建同步器
func NewSyncer(db *gorm.DB, cfg *config.AppConfig) *Syncer {
	s := &Syncer{
		db:      db,
		checker: checker.NewChecker(cfg),
		stopCh:  make(chan struct{}),
	}
	// 启动时从数据库恢复上次同步时间（服务重启后仍能正确显示）
	s.loadLastSyncAt()
	return s
}

// loadLastSyncAt 从 SyncMeta 表加载最近一次同步时间
func (s *Syncer) loadLastSyncAt() {
	var meta model.SyncMeta
	// ID 固定为 "sync-meta"，直接用 First 而非 Find
	if err := s.db.Where("id = ?", "sync-meta").First(&meta).Error; err == nil && meta.LastSyncAt != nil {
		s.lastSyncAt = meta.LastSyncAt
	}
}

// saveLastSyncAt 将最近一次同步时间写入 SyncMeta 表
func (s *Syncer) saveLastSyncAt(t *time.Time) {
	meta := model.SyncMeta{
		ID:         "sync-meta",
		LastSyncAt: t,
	}
	// UPSERT：不存在则创建，存在则更新 LastSyncAt
	s.db.Save(&meta)
}

// EnsureInitialized 启动检查：若数据库中无房间数据则同步一次（阻塞直到完成）
// 用于保证服务启动时下拉选项可用。热更新场景下调用者可跳过。
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

// StartPeriodicSync 启动 30 天定时后台同步任务（不阻塞）
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

// Stop 停止后台定时同步任务
func (s *Syncer) Stop() {
	close(s.stopCh)
}

// Status 返回当前同步状态（供管理后台展示）
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

// SyncAll 遍历所有已知楼栋，抓取完整的下拉选项并存储（UPSERT）
func (s *Syncer) SyncAll() error {
	// 防止并发同步
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

	// 本次同步涉及的 key 集合，用于最后清理旧数据
	syncedKeys := make(map[string]struct{})

	// 已知楼栋列表（2位数字）
	buildings := []string{"01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12", "13", "14"}

	successCount := 0
	for _, building := range buildings {
		if err := s.syncBuilding(building, syncedKeys); err != nil {
			logger.Warn("楼栋抓取失败", "building", building, "err", err)
			// 单楼失败不影响其他楼，继续
		} else {
			successCount++
		}
		time.Sleep(500 * time.Millisecond) // 避免请求过快
	}

	// 软删除本次同步未涉及的旧记录
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
	// 持久化到数据库，使服务重启后仍能正确显示
	s.saveLastSyncAt(&now)

	logger.Info("全量同步完成",
		"duration", time.Since(startAt).Round(time.Second).String(),
		"buildings_success", successCount,
		"buildings_total", len(buildings),
		"synced_keys", len(syncedKeys),
	)
	return nil
}

// syncBuilding 抓取单个楼栋的 ablou 和 drceng 选项，upsert 到数据库
//
// 数据构造规则：
//   - DrcengValue = drceng 下拉框原始值（直接 POST 给网站），如 "140328"
//   - FormValue   = 用于前端存储：
//       普通房间：FormValue = DrcengValue（"140328"）
//       水电分房（displayText 含"水表"）：FormValue = building+"|"+floor+"|"+drceng
//   - Label = 从 drceng 后两位提取真实房间号，格式 "C{楼} {房间号}" + "水"后缀
//     注意：网站的 displayText（如 "C10-207"）完全错误，Label 不能直接用
func (s *Syncer) syncBuilding(building string, syncedKeys map[string]struct{}) error {
	// 先存 drlouming 选项（楼栋本身）
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

	// 抓取 ablou（楼层）选项：模拟 step1→step2
	ablouValues, err := s.fetchAblouOptions(building)
	if err != nil {
		return fmt.Errorf("抓取 ablou 失败: %w", err)
	}

	for _, ablou := range ablouValues {
		// 存 ablou 选项（upsert）
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

		// 抓取该楼层的 drceng（房间）选项，value->text 映射
		drcengMap, err := s.fetchDrcengOptions(building, ablou)
		if err != nil {
			logger.Warn("抓取 drceng 失败", "building", building, "floor", ablou, "err", err)
			continue
		}

		for drceng, displayText := range drcengMap {
			// Label：直接用网站的 displayText（已通过 GB2312 解码）
			// 网站给的显示名是绝对真理，不做任何计算
			// 例如：C10-207（普通）、C10-201水表（水表）
			// displayText 为空时（如公区），fallback 到 drceng 值
			label := displayText
			if label == "" {
				label = drceng
			}

			// Key = building_floor_drceng（统一，不区分水电，统一匹配旧记录）
			// 这样旧的水表记录也能被更新，不会产生重复
			key := building + "_" + ablou + "_" + drceng
			syncedKeys[key] = struct{}{}

			// 查找已存在的记录（忽略 level，因为旧数据可能用不同 level）
			var existing model.DormOption
			found := s.db.Unscoped().Where("key = ?", key).First(&existing).Error == nil

			// 强制更新 Label：直接用网站的 displayText 作为正确值
			if found {
				updates := map[string]interface{}{
					"label":        label, // 强制更新为正确值
					"drceng_value": drceng,
					"building":     building,
					"floor":        ablou,
					"room_suffix":  s.extractRoomSuffix(drceng),
					"level":        model.OptionLevelRoom, // 强制统一 level
					"updated_at":   time.Now(),
					"deleted_at":   nil, // 恢复软删除的记录
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
					FormValue:   drceng, // 直接用 drceng 值作为存储标识
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

// fetchAblouOptions 获取某楼栋的 ablou 下拉选项
func (s *Syncer) fetchAblouOptions(building string) ([]string, error) {
	_, html2, _, err := s.checker.StepByStep(context.Background(), building, "", "")
	if err != nil {
		return nil, err
	}
	return checker.ExtractDropOptions(html2, "ablou"), nil
}

// fetchDrcengOptions 获取某楼栋+楼层的 drceng 下拉选项列表
// 返回 map[drceng原始值]displayText，displayText 用于判断是否水电，不用于 Label 生成
func (s *Syncer) fetchDrcengOptions(building, floor string) (map[string]string, error) {
	_, html3, _, err := s.checker.StepByStep(context.Background(), building, floor, "")
	if err != nil {
		return nil, err
	}
	values := checker.ExtractDropOptions(html3, "drceng")
	texts := checker.ExtractDropOptionTexts(html3, "drceng")

	result := make(map[string]string)
	for i, val := range values {
		var text string
		if i < len(texts) && texts[i] != "" {
			text = texts[i]
		} else {
			text = val // fallback
		}
		result[val] = text
	}
	return result, nil
}

// extractRoomSuffix 从 drceng 原始值中提取房间后缀（用于 RoomSuffix 字段）
// 对于普通6位数字 "140328" → 后2位 "28"（房间号）
// 对于水电分房 "132水表" → 整个值（已是后缀）
func (s *Syncer) extractRoomSuffix(drcengValue string) string {
	// 含水/电字符：直接返回原始值（已是房间后缀）
	if strings.Contains(drcengValue, "电") || strings.Contains(drcengValue, "水") {
		return drcengValue
	}
	// 纯数字6位：取后2位
	if len(drcengValue) >= 2 {
		return drcengValue[len(drcengValue)-2:]
	}
	return drcengValue
}






