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

// DB 是全局数据库连接，整个应用共用同一个连接池
var DB *gorm.DB

// InitDB 根据配置初始化数据库连接并自动迁移表结构
// SQLite 默认启用 WAL 模式 + 外键约束，适合高读低写场景
// MySQL 生产部署时配置连接池，支持高并发写
func InitDB(cfg *config.AppConfig) {
	// InternalToken 为空时强制拒绝启动（安全硬性要求）
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
			if err := os.MkdirAll(dir, 0o750); err != nil {
				logger.Fatal("创建数据库目录失败", "err", err, "path", dbPath)
			}
		}
		// _pragma=journal_mode(WAL)   ── 写不阻塞读，适合高并发读
		// _pragma=foreign_keys(ON)    ── 强制外键约束
		// _pragma=busy_timeout(5000)  ── 写锁等待 5s，避免 SQLITE_BUSY
		// _pragma=synchronous(NORMAL) ── WAL 下 NORMAL 已足够安全且更快
		dsn := dbPath +
			"?_pragma=journal_mode(WAL)" +
			"&_pragma=foreign_keys(ON)" +
			"&_pragma=busy_timeout(5000)" +
			"&_pragma=synchronous(NORMAL)"
		dialector = sqlite.Open(dsn)
	}

	gormCfg := &gorm.Config{
		// 禁用默认事务（单条语句不自动开事务），提升写入吞吐
		SkipDefaultTransaction: true,
		// 预编译 SQL 缓存，减少重复 parse 开销
		PrepareStmt: true,
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

	// 配置连接池
	sqlDB, err := DB.DB()
	if err != nil {
		logger.Fatal("获取底层 sql.DB 失败", "err", err)
	}
	if cfg.Database.Driver == "mysql" {
		// MySQL 生产环境：高并发配置
		sqlDB.SetMaxOpenConns(50)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(30 * time.Minute)
		sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	} else {
		// SQLite：WAL 模式下并发读 OK，写仍串行，限制连接数
		sqlDB.SetMaxOpenConns(1)    // 写操作串行化，避免 SQLITE_BUSY
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetConnMaxLifetime(0) // 不超时，保持 WAL 一直开着
	}

	// 启动连通性检查
	if err := sqlDB.Ping(); err != nil {
		logger.Fatal("数据库 Ping 失败", "err", err)
	}

	// 自动迁移所有表（只增不删，安全）
	if err := DB.AutoMigrate(&User{}, &UserChannel{}, &PowerLog{}, &DormOption{}, &SyncMeta{}); err != nil {
		logger.Fatal("数据库迁移失败", "err", err)
	}

	logger.Info("数据库初始化完成", "driver", cfg.Database.Driver)
}
