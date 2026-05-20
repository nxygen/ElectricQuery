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

func RunAll() error {
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
		formValue := normalizeToFormValue(old.DormRoom)
		if formValue == "" {
			log.Printf("[迁移] 电表记录 %s 无法转换为 FormValue，跳过\n", old.DormRoom)
			continue
		}

		var existing model.ElectricityLog
		if err := model.DB.Where("dorm_room = ? AND record_date = ?", formValue, old.RecordDate).First(&existing).Error; err == nil {
			continue
		}

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
		formValue := normalizeToFormValue(old.DormRoom)
		if formValue == "" {
			log.Printf("[迁移] 水表记录 %s 无法转换为 FormValue，跳过\n", old.DormRoom)
			continue
		}

		var existing model.WaterLog
		if err := model.DB.Where("dorm_room = ? AND record_date = ?", formValue, old.RecordDate).First(&existing).Error; err == nil {
			continue
		}

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

func normalizeToFormValue(dormRoom string) string {
	if dormRoom == "" {
		return ""
	}
	if len(dormRoom) > 10 && contains(dormRoom, '|') {
		return dormRoom
	}
	lk := service.LookupByFormValue(dormRoom)
	if lk != nil && lk.Opt.FormValue != "" {
		return lk.Opt.FormValue
	}
	webValue := service.ToWebValue(dormRoom)
	if webValue != "" && webValue != dormRoom {
		lk2 := service.LookupByFormValue(webValue)
		if lk2 != nil && lk2.Opt.FormValue != "" {
			return lk2.Opt.FormValue
		}
	}
	return ""
}

func contains(s string, c rune) bool {
	for _, ch := range s {
		if ch == c {
			return true
		}
	}
	return false
}

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

		if u.DormRoom != "" && (u.DormFloor == "" || u.DormFloor == "0") {
			var opt model.DormOption
			if err := model.DB.Where("level = ? AND drceng_value = ?", model.OptionLevelRoom, u.DormRoom).First(&opt).Error; err == nil {
				updates["dorm_floor"] = opt.Floor
				updates["building"] = opt.Building
			}
		}

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

			parts := strings.Split(val, "|")
			if len(parts) != 3 {
				continue
			}
			drceng := strings.TrimSpace(parts[2])
			if drceng == "" {
				continue
			}

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

func dropPowerLogsTable() error {
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

func backupDatabase() error {
	cfg := config.Load()
	if cfg.Database.Driver != "sqlite" {
		log.Println("[迁移] 非 SQLite 数据库，跳过文件备份（请自行 mysqldump）")
		return nil
	}

	dbPath := cfg.Database.SQLite.Path
	if dbPath == "" {
		dbPath = "data/electricquery.db"
	}

	if !filepath.IsAbs(dbPath) {
		wd, err := os.Getwd()
		if err == nil {
			dbPath = filepath.Join(wd, dbPath)
		}
	}

	bakPath := dbPath + ".bak"

	if _, err := os.Stat(bakPath); err == nil {
		log.Printf("[迁移] 数据库备份已存在，跳过: %s\n", bakPath)
		return nil
	}

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
		os.Remove(bakPath)
		return fmt.Errorf("复制数据库文件失败: %w", err)
	}

	if err := dst.Sync(); err != nil {
		return fmt.Errorf("备份文件刷盘失败: %w", err)
	}

	srcInfo, _ := src.Stat()
	log.Printf("[迁移] 数据库已备份: %s → %s（%d bytes）\n", dbPath, bakPath, srcInfo.Size())
	return nil
}
