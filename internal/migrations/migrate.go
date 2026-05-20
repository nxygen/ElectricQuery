// Package migrations 数据库迁移模块
// 所有迁移均为幂等设计，可安全重复执行
package migrations

import (
	"electricquery/internal/model"
	"electricquery/internal/service"
	"fmt"
	"log"
	"time"
)

// RunAll 执行所有迁移
func RunAll() error {
	if err := createElectricityLogTable(); err != nil {
		return fmt.Errorf("创建电表记录表失败: %w", err)
	}
	if err := createWaterLogTable(); err != nil {
		return fmt.Errorf("创建水表记录表失败: %w", err)
	}
	if err := migrateElectricityData(); err != nil {
		return fmt.Errorf("迁移电表数据失败: %w", err)
	}
	if err := migrateWaterData(); err != nil {
		return fmt.Errorf("迁移水表数据失败: %w", err)
	}
	log.Println("[迁移] 所有迁移完成")
	return nil
}

// createElectricityLogTable 创建电表记录表（幂等）
func createElectricityLogTable() error {
	if model.DB.Migrator().HasTable(&model.ElectricityLog{}) {
		log.Println("[迁移] electricity_logs 表已存在，跳过")
		return nil
	}
	if err := model.DB.Migrator().CreateTable(&model.ElectricityLog{}); err != nil {
		return fmt.Errorf("创建 electricity_logs 表失败: %w", err)
	}
	log.Println("[迁移] 创建 electricity_logs 表成功")
	return nil
}

// createWaterLogTable 创建水表记录表（幂等）
func createWaterLogTable() error {
	if model.DB.Migrator().HasTable(&model.WaterLog{}) {
		log.Println("[迁移] water_logs 表已存在，跳过")
		return nil
	}
	if err := model.DB.Migrator().CreateTable(&model.WaterLog{}); err != nil {
		return fmt.Errorf("创建 water_logs 表失败: %w", err)
	}
	log.Println("[迁移] 创建 water_logs 表成功")
	return nil
}

// migrateElectricityData 从 power_logs 迁移电表数据到 electricity_logs（幂等）
func migrateElectricityData() error {
	var oldLogs []model.PowerLog
	if err := model.DB.Where("remaining_kwh IS NOT NULL AND remaining_kwh != ''").Find(&oldLogs).Error; err != nil {
		return fmt.Errorf("查询旧电表数据失败: %w", err)
	}
	if len(oldLogs) == 0 {
		log.Println("[迁移] 无旧电表数据需迁移")
		return nil
	}

	migrated := 0
	for _, old := range oldLogs {
		// 转换 dorm_room 为 FormValue
		formValue := normalizeToFormValue(old.DormRoom)
		if formValue == "" {
			log.Printf("[迁移] 电表记录 %s 无法转换为 FormValue，跳过\n", old.DormRoom)
			continue
		}

		// 检查是否已迁移（根据 dorm_room + record_date）
		var existing model.ElectricityLog
		if err := model.DB.Where("dorm_room = ? AND record_date = ?", formValue, old.RecordDate).First(&existing).Error; err == nil {
			// 已存在，跳过
			continue
		}

		// 写入新表
		elecLog := &model.ElectricityLog{
			DormRoom:     formValue,
			RecordDate:   old.RecordDate,
			RemainingKwh: old.RemainingKwh,
			QueriedAt:    old.QueriedAt.Format(time.RFC3339),
		}
		if err := model.DB.Create(elecLog).Error; err != nil {
			log.Printf("[迁移] 写入电表记录失败: %s\n", err)
			continue
		}
		migrated++
	}
	log.Printf("[迁移] 电表数据迁移完成，共迁移 %d 条记录\n", migrated)
	return nil
}

// migrateWaterData 从 power_logs 迁移水表数据到 water_logs（幂等）
func migrateWaterData() error {
	var oldLogs []model.PowerLog
	if err := model.DB.Where("remaining_water IS NOT NULL AND remaining_water != ''").Find(&oldLogs).Error; err != nil {
		return fmt.Errorf("查询旧水表数据失败: %w", err)
	}
	if len(oldLogs) == 0 {
		log.Println("[迁移] 无旧水表数据需迁移")
		return nil
	}

	migrated := 0
	for _, old := range oldLogs {
		// 转换 dorm_room 为 FormValue
		formValue := normalizeToFormValue(old.DormRoom)
		if formValue == "" {
			log.Printf("[迁移] 水表记录 %s 无法转换为 FormValue，跳过\n", old.DormRoom)
			continue
		}

		// 检查是否已迁移
		var existing model.WaterLog
		if err := model.DB.Where("dorm_room = ? AND record_date = ?", formValue, old.RecordDate).First(&existing).Error; err == nil {
			continue
		}

		// 写入新表
		waterLog := &model.WaterLog{
			DormRoom:       formValue,
			RecordDate:     old.RecordDate,
			RemainingWater: old.RemainingWater,
			QueriedAt:      old.QueriedAt.Format(time.RFC3339),
		}
		if err := model.DB.Create(waterLog).Error; err != nil {
			log.Printf("[迁移] 写入水表记录失败: %s\n", err)
			continue
		}
		migrated++
	}
	log.Printf("[迁移] 水表数据迁移完成，共迁移 %d 条记录\n", migrated)
	return nil
}

// normalizeToFormValue 将 dorm_room 转换为 FormValue 格式
// 兼容物理 ID（如 "110132"）和 FormValue（如 "11|1101|110132"）
func normalizeToFormValue(dormRoom string) string {
	if dormRoom == "" {
		return ""
	}
	// 如果已经是 FormValue 格式，直接返回
	if len(dormRoom) > 10 && contains(dormRoom, '|') {
		return dormRoom
	}
	// 尝试从 DormOption 反查 FormValue
	lk := service.LookupByFormValue(dormRoom)
	if lk != nil && lk.Opt.FormValue != "" {
		return lk.Opt.FormValue
	}
	// 尝试用 ToWebValue 转换
	webValue := service.ToWebValue(dormRoom)
	if webValue != "" && webValue != dormRoom {
		lk2 := service.LookupByFormValue(webValue)
		if lk2 != nil && lk2.Opt.FormValue != "" {
			return lk2.Opt.FormValue
		}
	}
	return ""
}

// contains 检查字符串是否包含指定字符
func contains(s string, c rune) bool {
	for _, ch := range s {
		if ch == c {
			return true
		}
	}
	return false
}
