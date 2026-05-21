package model

import (
	"os"
	"path/filepath"
	"time"

	"electricquery/internal/config"
	"electricquery/internal/logger"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB(cfg *config.AppConfig) {
	if cfg.App.InternalToken == "" {
		logger.Fatal("安全配置缺失：app.internal_token 不能为空，请在 application.conf 中设置")
	}

	var dialector gorm.Dialector

	switch cfg.Database.Driver {
	case "mysql":
		dialector = mysql.Open(cfg.DSN())
	default: // sqlite
		dbPath := cfg.Database.SQLite.Path
		if dir := filepath.Dir(dbPath); dir != "" {
			if err := os.MkdirAll(dir, 0750); err != nil {
				logger.Fatal("创建数据库目录失败", "err", err, "path", dbPath)
			}
		}
		dsn := dbPath +
			"?_pragma=journal_mode(WAL)" +
			"&_pragma=foreign_keys(ON)" +
			"&_pragma=busy_timeout(5000)" +
			"&_pragma=synchronous(NORMAL)"
		dialector = sqlite.Open(dsn)
	}

	gormCfg := &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	}
	if cfg.App.Mode == "debug" {
		gormCfg.Logger = gormLogger.Default.LogMode(gormLogger.Info)
	} else {
		gormCfg.Logger = gormLogger.Default.LogMode(gormLogger.Warn)
	}

	var err error
	DB, err = gorm.Open(dialector, gormCfg)
	if err != nil {
		logger.Fatal("数据库连接失败", "err", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		logger.Fatal("获取底层 sql.DB 失败", "err", err)
	}
	if cfg.Database.Driver == "mysql" {
		maxOpen := cfg.Database.MaxOpenConns
		if maxOpen <= 0 {
			maxOpen = 50
		}
		maxIdle := cfg.Database.MaxIdleConns
		if maxIdle <= 0 {
			maxIdle = 10
		}
		sqlDB.SetMaxOpenConns(maxOpen)
		sqlDB.SetMaxIdleConns(maxIdle)
		sqlDB.SetConnMaxLifetime(30 * time.Minute)
		sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	} else {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetConnMaxLifetime(0)
	}

	if err := sqlDB.Ping(); err != nil {
		logger.Fatal("数据库 Ping 失败", "err", err)
	}

	if err := DB.AutoMigrate(&User{}, &UserChannel{}, &PowerLog{}, &DormOption{}, &SyncMeta{}, &ElectricityLog{}, &WaterLog{}); err != nil {
		logger.Fatal("数据库迁移失败", "err", err)
	}

	logger.Info("数据库初始化完成", "driver", cfg.Database.Driver)
}
