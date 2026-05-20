// Package migrations 数据库迁移模块
// 所有迁移均为幂等设计，可安全重复执行
package migrations

import (
	"electricquery/internal/config"
	"electricquery/internal/model"
	"electricquery/internal/service"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunAll 执行所有迁移
func RunAll() error {
	// 迁移前先备份整个数据库文件，防止迁移失败导致数据丢失
	if err := backupDatabase(); err != nil {
		return fmt.Errorf("数据库备份失败，中止迁移: %w", err)
	}
	if err := createElectricityLogTable(); err != nil {
		return fmt.Errorf("创建电表记录表失败: %w", err)
	}
	if err := createWaterLogTable(); err != nil {
		return fmt.Errorf("创建水表记录表失败: %w", err)
	}
	if err := addMissingUserColumns(); err != nil {
		return fmt.Errorf("扩展 users 表字段失败: %w", err)
	}
	if err := migrateElectricityData(); err != nil {
		return fmt.Errorf("迁移电表数据失败: %w", err)
	}
	if err := migrateWaterData(); err != nil {
		return fmt.Errorf("迁移水表数据失败: %w", err)
	}
	if err := backfillUserDormFloors(); err != nil {
		return fmt.Errorf("回填用户宿舍楼层失败: %w", err)
	}
	if err := migrateUserDormRoomFormat(); err != nil {
		return fmt.Errorf("迁移用户宿舍号格式失败: %w", err)
	}
	if err := dropPowerLogsTable(); err != nil {
		return fmt.Errorf("删除旧表失败: %w", err)
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

// backfillUserDormFloors 为已有用户回填 dorm_floor 和 water_dorm_floor（幂等）
// 解决历史用户缺少楼层字段的问题。服务每次启动都会执行，重复执行安全。
func addMissingUserColumns() error {
	migrated := 0
	for _, col := range []struct {
		name    string
		sqlType string
	}{
		{"dorm_floor", "VARCHAR(16) DEFAULT ''"},
		{"water_dorm_floor", "VARCHAR(16) DEFAULT ''"},
	} {
		if !model.DB.Migrator().HasColumn(&model.User{}, col.name) {
			if err := model.DB.Exec(fmt.Sprintf("ALTER TABLE users ADD COLUMN %s %s", col.name, col.sqlType)).Error; err != nil {
				log.Printf("[迁移] 添加 users.%s 列失败: %v\n", col.name, err)
				continue
			}
			log.Printf("[迁移] 添加 users.%s 列成功\n", col.name)
		}
		migrated++
	}
	if migrated == 0 {
		log.Println("[迁移] users 表字段已完整，跳过")
	}
	return nil
}

func backfillUserDormFloors() error {
	var users []model.User
	if err := model.DB.Where("dorm_room != '' AND (dorm_floor = '' OR dorm_floor IS NULL)").Find(&users).Error; err != nil {
		return fmt.Errorf("查询用户失败: %w", err)
	}
	if len(users) == 0 {
		log.Println("[迁移] 无需回填用户宿舍楼层")
		return nil
	}

	migrated := 0
	for _, u := range users {
		updates := make(map[string]interface{})

		// 回填电宿舍的 floor + building
		if u.DormRoom != "" && (u.DormFloor == "" || u.DormFloor == "0") {
			var opt model.DormOption
			if err := model.DB.Where("level = ? AND drceng_value = ?", model.OptionLevelRoom, u.DormRoom).First(&opt).Error; err == nil {
				updates["dorm_floor"] = opt.Floor
				updates["building"] = opt.Building
			}
		}

		// 回填水宿舍的 floor
		if u.WaterDormRoom != "" && (u.WaterDormFloor == "" || u.WaterDormFloor == "0") {
			var opt model.DormOption
			if err := model.DB.Where("level = ? AND drceng_value = ?", model.OptionLevelRoom, u.WaterDormRoom).First(&opt).Error; err == nil {
				updates["water_dorm_floor"] = opt.Floor
			}
		}

		if len(updates) > 0 {
			if err := model.DB.Model(&model.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
				log.Printf("[迁移] 回填用户楼层失败 user_id=%s: %v\n", u.ID, err)
				continue
			}
			migrated++
		}
	}
	log.Printf("[迁移] 用户宿舍楼层回填完成，共更新 %d 个用户\n", migrated)
	return nil
}

// migrateUserDormRoomFormat 将 users.dorm_room / water_dorm_room 中的旧格式转为 drceng_value（幂等）
//
// 旧格式：三段式 "11|1101|110169"（building|floor|drceng）
// 新格式：纯 drceng_value "110169"
//
// QueryAndSavePower 已改为按 drceng_value 查 DormOption，旧格式会匹配失败导致调度查询中断。
func migrateUserDormRoomFormat() error {
	migrated := 0
	for _, col := range []string{"dorm_room", "water_dorm_room"} {
		var users []model.User
		if err := model.DB.Where(col+" != '' AND "+col+" LIKE ?", "%|%").Find(&users).Error; err != nil {
			log.Printf("[迁移] 查询 %s 旧格式用户失败: %v\n", col, err)
			continue
		}
		if len(users) == 0 {
			continue
		}

		for _, u := range users {
			var val string
			switch col {
			case "dorm_room":
				val = u.DormRoom
			case "water_dorm_room":
				val = u.WaterDormRoom
			}

			// 从三段式中提取第三段作为 drceng_value
			parts := strings.Split(val, "|")
			if len(parts) != 3 {
				continue
			}
			drceng := strings.TrimSpace(parts[2])
			if drceng == "" {
				continue
			}

			// 验证 drceng_value 存在于 dorm_options
			var opt model.DormOption
			if err := model.DB.Where("level = ? AND drceng_value = ?", model.OptionLevelRoom, drceng).First(&opt).Error; err != nil {
				log.Printf("[迁移] drceng_value %s 不存在于 dorm_options，跳过用户 %s\n", drceng, u.ID)
				continue
			}

			if err := model.DB.Model(&model.User{}).Where("id = ?", u.ID).Update(col, drceng).Error; err != nil {
				log.Printf("[迁移] 更新用户 %s 的 %s 失败: %v\n", u.ID, col, err)
				continue
			}
			migrated++
			log.Printf("[迁移] 用户 %s %s: %s → %s\n", u.ID, col, val, drceng)
		}
	}
	if migrated > 0 {
		log.Printf("[迁移] 用户宿舍号格式迁移完成，共更新 %d 条记录\n", migrated)
	} else {
		log.Println("[迁移] 无需迁移用户宿舍号格式")
	}
	return nil
}

// dropPowerLogsTable 删除旧 power_logs 表
// 数据库文件已由 backupDatabase() 备份为 .bak，此处直接 DROP TABLE，幂等安全
func dropPowerLogsTable() error {
	// 检查新表是否有数据（迁移完成的标志）
	var elecCount, waterCount int64
	model.DB.Model(&model.ElectricityLog{}).Count(&elecCount)
	model.DB.Model(&model.WaterLog{}).Count(&waterCount)

	if elecCount == 0 && waterCount == 0 {
		log.Println("[迁移] 新表无数据，跳过删除 power_logs")
		return nil
	}

	if !model.DB.Migrator().HasTable(&model.PowerLog{}) {
		log.Println("[迁移] power_logs 表不存在，跳过")
		return nil
	}

	if err := model.DB.Migrator().DropTable(&model.PowerLog{}); err != nil {
		return fmt.Errorf("删除 power_logs 表失败: %w", err)
	}
	log.Printf("[迁移] power_logs 表已删除（数据已迁移至 electricity_logs/water_logs）\n")
	return nil
}

// backupDatabase 迁移前备份整个数据库文件，防止迁移失败导致数据丢失
// 仅对 SQLite 生效；若 .bak 文件已存在则跳过（幂等）
func backupDatabase() error {
	cfg := config.Load()
	if cfg.Database.Driver != "sqlite" {
		log.Println("[迁移] 非 SQLite 数据库，跳过文件备份（请自行 mysqldump）")
		return nil
	}

	dbPath := cfg.Database.SQLite.Path
	if dbPath == "" {
		dbPath = "data/electricquery.db" // 默认值兜底
	}

	// 转成绝对路径，确保相对路径也能正确找到
	if !filepath.IsAbs(dbPath) {
		wd, err := os.Getwd()
		if err == nil {
			dbPath = filepath.Join(wd, dbPath)
		}
	}

	bakPath := dbPath + ".bak"

	// 幂等：.bak 已存在则跳过
	if _, err := os.Stat(bakPath); err == nil {
		log.Printf("[迁移] 数据库备份已存在，跳过: %s\n", bakPath)
		return nil
	}

	// 源文件必须存在
	src, err := os.Open(dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库文件失败: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(bakPath)
	if err != nil {
		return fmt.Errorf("创建备份文件失败: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(bakPath) // 清理不完整备份
		return fmt.Errorf("复制数据库文件失败: %w", err)
	}

	// 确保数据写入磁盘
	if err := dst.Sync(); err != nil {
		return fmt.Errorf("备份文件刷盘失败: %w", err)
	}

	srcInfo, _ := src.Stat()
	log.Printf("[迁移] 数据库已备份: %s → %s（%d bytes）\n", dbPath, bakPath, srcInfo.Size())
	return nil
}
