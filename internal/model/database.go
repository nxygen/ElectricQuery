package model

import (
	"log"
	"os"
	"path/filepath"

	"electricquery/internal/config"

	"github.com/glebarez/sqlite" // 纯 Go SQLite（无需 CGO）
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 根据配置初始化数据库连接并自动迁移表结构
// 支持 SQLite（默认，无需 CGO）和 MySQL/MariaDB（生产推荐）
// 切换方式：修改 application.conf 中的 database.driver 为 "mysql" 即可
func InitDB(cfg *config.AppConfig) {
	var dialector gorm.Dialector

	switch cfg.Database.Driver {
	case "mysql":
		dialector = mysql.Open(cfg.DSN())
	default: // sqlite
		dbPath := cfg.Database.SQLite.Path
		// 确保目录存在
		if dir := filepath.Dir(dbPath); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Fatalf("[db] 创建数据库目录失败: %v", err)
			}
		}
		dialector = sqlite.Open(dbPath)
	}

	gormCfg := &gorm.Config{}
	if cfg.App.Mode == "debug" {
		gormCfg.Logger = logger.Default.LogMode(logger.Info)
	} else {
		gormCfg.Logger = logger.Default.LogMode(logger.Warn)
	}

	var err error
	DB, err = gorm.Open(dialector, gormCfg)
	if err != nil {
		log.Fatalf("[db] 数据库连接失败: %v", err)
	}

	// 自动迁移所有表（只增不删，安全）
	if err := DB.AutoMigrate(&User{}, &UserChannel{}, &PowerLog{}); err != nil {
		log.Fatalf("[db] 数据库迁移失败: %v", err)
	}

	log.Printf("[db] 数据库初始化完成（driver: %s）", cfg.Database.Driver)
}
