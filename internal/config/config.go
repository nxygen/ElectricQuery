// Package config 负责加载和解析 JSON 格式的 application.conf
// 配置文件为标准 JSON 格式（application.conf.example 为模板）
package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"electricquery/internal/logger"
)

// initLog 提供统一的日志输出
// - logger 就绪前：使用标准 log（启动早期）
// - logger 就绪后：自动切换到结构化 slog（所有业务日志）
func initLog(format string, args ...any) {
	if logger.GetLogger() != nil {
		logger.Info(fmt.Sprintf(format, args...))
	} else {
		log.Printf(format, args...)
	}
}

func initFatal(args ...any) {
	if logger.GetLogger() != nil {
		logger.Fatal(fmt.Sprintf("%v", args...))
	} else {
		log.Fatal(args...)
	}
}

// 全局单例
var (
	once sync.Once
	cfg  *AppConfig
)

// AppConfig 是整个应用的强类型配置结构
type AppConfig struct {
	App          AppSection
	Log          LogSection
	Database     DatabaseSection
	SMTP         SMTPSection
	PowerChecker PowerCheckerSection
	Scheduler    SchedulerSection
}

// LogSection 日志配置
type LogSection struct {
	Level      string // 日志级别：debug, info, warn, error
	Path       string // 日志文件路径，如 "logs/app.log"（相对/绝对均可）
	MaxSizeMB  int    // 单文件最大大小（MB），超过则轮转
	MaxBackups int    // 保留历史文件份数
	MaxAgeDays int    // 保留历史文件天数
	Compress   bool   // 轮转后是否 gzip 压缩
	Console    bool   // 是否同时输出到 stdout
}

type AppSection struct {
	Host                string
	Port                int
	JWTSecret           string
	JWTExpireHours      int
	InternalToken       string
	AdminToken          string // 管理后台专用 Token，与 InternalToken 独立
	Mode                string
	AllowedOrigin       string // 生产环境前端域名，如 "https://eq.example.com"；debug 模式下允许 *
	MaxLoginPerWindow   int    // 登录速率限制：同一 IP N 次 / RateLimitWindowSec
	MaxRegisterPerWindow int   // 注册速率限制：同一 IP N 次 / RateLimitWindowSec
	RateLimitWindowSec  int    // 速率限制时间窗口秒数（默认 300，即 5 分钟）
}

type DatabaseSection struct {
	Driver string
	SQLite SQLiteSection
	MySQL  MySQLSection
}

type SQLiteSection struct {
	Path string
}

type MySQLSection struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	Charset  string
	Loc      string
}

type SMTPSection struct {
	Enabled     bool
	SenderEmail string
	SenderName  string
	Server      string
	Port        int
	UseSSL      bool
	Password    string
}

type PowerCheckerSection struct {
	LoginURL       string
	UserAgent      string
	TimeoutSeconds int
}

type SchedulerSection struct {
	PollInterval        int
	AlertThreshold      float64
	WeeklyReportWeekday int
	WeeklyReportHour    int
}

// ---- JSON 解码用的原始结构（与 JSON 键一一对应）----

type rawJSON struct {
	App          rawApp          `json:"app"`
	Log          rawLog          `json:"log"`
	Database     rawDatabase     `json:"database"`
	SMTP         rawSMTP         `json:"smtp"`
	PowerChecker rawPowerChecker `json:"power_checker"`
	Scheduler    rawScheduler    `json:"scheduler"`
}

type rawApp struct {
	Host                 string `json:"host"`
	Port                 int    `json:"port"`
	JWTSecret            string `json:"jwt_secret"`
	JWTExpireHours       int    `json:"jwt_expire_hours"`
	InternalToken        string `json:"internal_token"`
	AdminToken           string `json:"admin_token"` // 管理后台专用 Token
	Mode                 string `json:"mode"`
	AllowedOrigin        string `json:"allowed_origin"`
	MaxLoginPerWindow    int    `json:"max_login_per_window"`
	MaxRegisterPerWindow int    `json:"max_register_per_window"`
	RateLimitWindowSec   int    `json:"rate_limit_window_sec"`
}
type rawDatabase struct {
	Driver string    `json:"driver"`
	SQLite rawSQLite `json:"sqlite"`
	MySQL  rawMySQL  `json:"mysql"`
}
type rawSQLite struct {
	Path string `json:"path"`
}
type rawMySQL struct {
	Host, User, Password, DBName, Charset, Loc string
	Port                                       int
}
type rawSMTP struct {
	Enabled     bool   `json:"enabled"`
	SenderEmail string `json:"sender_email"`
	SenderName  string `json:"sender_name"`
	Server      string `json:"server"`
	Port        int    `json:"port"`
	UseSSL      bool   `json:"use_ssl"`
	Password    string `json:"password"`
}
type rawPowerChecker struct {
	LoginURL       string `json:"login_url"`
	UserAgent      string `json:"user_agent"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}
type rawScheduler struct {
	PollInterval        int     `json:"poll_interval"`
	AlertThreshold      float64 `json:"alert_threshold"`
	WeeklyReportWeekday int     `json:"weekly_report_weekday"`
	WeeklyReportHour    int     `json:"weekly_report_hour"`
}
type rawLog struct {
	Level      string `json:"level"`
	Path       string `json:"path"`
	MaxSizeMB  int    `json:"max_size_mb"`
	MaxBackups int    `json:"max_backups"`
	MaxAgeDays int    `json:"max_age_days"`
	Compress   bool   `json:"compress"`
	Console    bool   `json:"console"`
}

// Load 读取并解析 application.conf（JSON 格式），返回强类型配置，线程安全单例
func Load() *AppConfig {
	once.Do(func() {
		path := os.Getenv("CONFIG_PATH")
		if path == "" {
			path = "application.conf"
		}

		absPath, _ := filepath.Abs(path)
		wd, _ := os.Getwd()
		initLog("工作目录: %s", wd)
		initLog("正在加载配置文件: %s", absPath)

		raw, err := os.ReadFile(path)
		if err != nil {
			initFatal("读取配置文件失败: ", err, "\n请确认 application.conf 存在于项目根目录")
		}

		// 去除 UTF-8 BOM（某些编辑器写入的 BOM 会导致 JSON 解析失败）
		raw = stripBOM(raw)
		initLog("配置文件大小: %d bytes（已去除 BOM）", len(raw))

		var rawCfg rawJSON
		if err := json.Unmarshal(raw, &rawCfg); err != nil {
			initFatal("JSON 解析失败: ", err, "\n请检查 application.conf 格式是否正确")
		}

		// 映射到强类型配置，零值字段使用默认值
			cfg = &AppConfig{
				App: AppSection{
					Host:                 strDef(rawCfg.App.Host, "0.0.0.0"),
					Port:                 intDef(rawCfg.App.Port, 8080),
					JWTSecret:            strDef(rawCfg.App.JWTSecret, "changeme"),
					JWTExpireHours:       intDef(rawCfg.App.JWTExpireHours, 72),
					InternalToken:        rawCfg.App.InternalToken,
					AdminToken:           rawCfg.App.AdminToken, // 独立配置，无默认值兜底
					Mode:                 strDef(rawCfg.App.Mode, "debug"),
					AllowedOrigin:        rawCfg.App.AllowedOrigin,
					MaxLoginPerWindow:    intDef(rawCfg.App.MaxLoginPerWindow, 0),
					MaxRegisterPerWindow: intDef(rawCfg.App.MaxRegisterPerWindow, 0),
					RateLimitWindowSec:   intDef(rawCfg.App.RateLimitWindowSec, 300),
				},
			Log: LogSection{
				Level:      strDef(rawCfg.Log.Level, "info"),
				Path:       strDef(rawCfg.Log.Path, "logs/app.log"),
				MaxSizeMB:  intDef(rawCfg.Log.MaxSizeMB, 10),
				MaxBackups: intDef(rawCfg.Log.MaxBackups, 7),
				MaxAgeDays: intDef(rawCfg.Log.MaxAgeDays, 30),
				Compress:   rawCfg.Log.Compress,
				Console:    rawCfg.Log.Console,
			},
			Database: DatabaseSection{
				Driver: strDef(rawCfg.Database.Driver, "sqlite"),
				SQLite: SQLiteSection{Path: strDef(rawCfg.Database.SQLite.Path, "data/electricquery.db")},
				MySQL: MySQLSection{
					Host:     strDef(rawCfg.Database.MySQL.Host, "127.0.0.1"),
					Port:     intDef(rawCfg.Database.MySQL.Port, 3306),
					User:     strDef(rawCfg.Database.MySQL.User, "root"),
					Password: rawCfg.Database.MySQL.Password,
					DBName:   strDef(rawCfg.Database.MySQL.DBName, "electricquery"),
					Charset:  strDef(rawCfg.Database.MySQL.Charset, "utf8mb4"),
					Loc:      strDef(rawCfg.Database.MySQL.Loc, "Asia%2FShanghai"),
				},
			},
			SMTP: SMTPSection{
				Enabled:     rawCfg.SMTP.Enabled,
				SenderEmail: rawCfg.SMTP.SenderEmail,
				SenderName:  strDef(rawCfg.SMTP.SenderName, "ElectricQuery"),
				Server:      rawCfg.SMTP.Server,
				Port:        intDef(rawCfg.SMTP.Port, 465),
				UseSSL:      rawCfg.SMTP.UseSSL,
				Password:    rawCfg.SMTP.Password,
			},
			PowerChecker: PowerCheckerSection{
				LoginURL:       rawCfg.PowerChecker.LoginURL,
				UserAgent:      strDef(rawCfg.PowerChecker.UserAgent, "Mozilla/5.0"),
				TimeoutSeconds: intDef(rawCfg.PowerChecker.TimeoutSeconds, 15),
			},
			Scheduler: SchedulerSection{
				PollInterval:        intDef(rawCfg.Scheduler.PollInterval, 600),
				AlertThreshold:      floatDef(rawCfg.Scheduler.AlertThreshold, 20.0),
				WeeklyReportWeekday: intDef(rawCfg.Scheduler.WeeklyReportWeekday, 1),
				WeeklyReportHour:    intDef(rawCfg.Scheduler.WeeklyReportHour, 8),
			},
		}

		// 环境变量覆盖（优先级高于配置文件）
		applyEnvOverrides(cfg)

		// ---- 安全校验 ----
		// JWT 密钥长度必须 >= 32 字节，防止使用弱密钥
		if len(cfg.App.JWTSecret) < 32 {
			initFatal("安全配置错误：jwt_secret 长度必须 >= 32 字节，当前长度=", len(cfg.App.JWTSecret),
				"\n请使用随机字符串，例如：openssl rand -base64 32")
		}
		// 生产模式必须配置 allowed_origin，防止通配 * 被滥用
		if cfg.App.Mode != "debug" && cfg.App.AllowedOrigin == "" {
			initFatal("安全配置错误：生产模式必须设置 allowed_origin（前端域名），当前为空")
		}

		initLog("配置加载成功，数据库驱动: %s，服务端口: %d", cfg.Database.Driver, cfg.App.Port)
		initLog("爬虫配置: login_url=%s timeout=%ds", cfg.PowerChecker.LoginURL, cfg.PowerChecker.TimeoutSeconds)
	})
	return cfg
}

// DSN 根据配置中的 driver 返回对应的数据库连接字符串
func (c *AppConfig) DSN() string {
	switch c.Database.Driver {
	case "mysql":
		m := c.Database.MySQL
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=%s",
			m.User, m.Password, m.Host, m.Port, m.DBName, m.Charset, m.Loc)
	default:
		return c.Database.SQLite.Path
	}
}

// stripBOM 去除 UTF-8 BOM 头（0xEF 0xBB 0xBF）
func stripBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

// 默认值辅助函数
func strDef(v, def string) string   { if v == "" { return def }; return v }
func intDef(v, def int) int         { if v == 0 { return def }; return v }
func floatDef(v, def float64) float64 { if v == 0 { return def }; return v }

// envInt 读取环境变量为 int（空/解析失败时返回零值）
func envInt(key string) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return 0
}

// envBool 读取环境变量为 bool（"true"/"1" 不区分大小写）
func envBool(key string) bool {
	return strings.EqualFold(os.Getenv(key), "true") || os.Getenv(key) == "1"
}

// applyEnvOverrides 用环境变量覆盖配置文件值
// 前缀统一为 EQ_，环境变量存在时优先于配置文件
// 示例：
//   EQ_LOG_LEVEL=debug
//   EQ_APP_PORT=9000
//   EQ_JWT_SECRET=your-secret-key
//   EQ_INTERNAL_TOKEN=your-token
//   EQ_SMTP_PASSWORD=your-password
//   EQ_DB_DRIVER=mysql
//   EQ_SMTP_ENABLED=true
func applyEnvOverrides(cfg *AppConfig) {
	// ---- App ----
	if v := os.Getenv("EQ_HOST"); v != "" {
		cfg.App.Host = v
	}
	if v := envInt("EQ_PORT"); v > 0 {
		cfg.App.Port = v
	}
	if v := os.Getenv("EQ_JWT_SECRET"); v != "" {
		cfg.App.JWTSecret = v
	}
	if v := os.Getenv("EQ_INTERNAL_TOKEN"); v != "" {
		cfg.App.InternalToken = v
	}
	if v := os.Getenv("EQ_ADMIN_TOKEN"); v != "" {
		cfg.App.AdminToken = v
	}
	if v := os.Getenv("EQ_ALLOWED_ORIGIN"); v != "" {
		cfg.App.AllowedOrigin = v
	}

	// ---- Log ----
	if v := os.Getenv("EQ_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("EQ_LOG_PATH"); v != "" {
		cfg.Log.Path = v
	}
	if v := envInt("EQ_LOG_MAX_SIZE_MB"); v > 0 {
		cfg.Log.MaxSizeMB = v
	}
	if v := envInt("EQ_LOG_MAX_BACKUPS"); v > 0 {
		cfg.Log.MaxBackups = v
	}
	if envBool("EQ_LOG_CONSOLE") {
		cfg.Log.Console = true
	}

	// ---- Database ----
	if v := os.Getenv("EQ_DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("EQ_SQLITE_PATH"); v != "" {
		cfg.Database.SQLite.Path = v
	}
	if v := os.Getenv("EQ_MYSQL_HOST"); v != "" {
		cfg.Database.MySQL.Host = v
	}
	if v := envInt("EQ_MYSQL_PORT"); v > 0 {
		cfg.Database.MySQL.Port = v
	}
	if v := os.Getenv("EQ_MYSQL_USER"); v != "" {
		cfg.Database.MySQL.User = v
	}
	if v := os.Getenv("EQ_MYSQL_PASSWORD"); v != "" {
		cfg.Database.MySQL.Password = v
	}
	if v := os.Getenv("EQ_MYSQL_DBNAME"); v != "" {
		cfg.Database.MySQL.DBName = v
	}

	// ---- SMTP ----
	if envBool("EQ_SMTP_ENABLED") {
		cfg.SMTP.Enabled = true
	}
	if v := os.Getenv("EQ_SMTP_SERVER"); v != "" {
		cfg.SMTP.Server = v
	}
	if v := envInt("EQ_SMTP_PORT"); v > 0 {
		cfg.SMTP.Port = v
	}
	if v := os.Getenv("EQ_SMTP_USER"); v != "" {
		cfg.SMTP.SenderEmail = v
	}
	if v := os.Getenv("EQ_SMTP_PASSWORD"); v != "" {
		cfg.SMTP.Password = v
	}

	// ---- PowerChecker ----
	if v := os.Getenv("EQ_LOGIN_URL"); v != "" {
		cfg.PowerChecker.LoginURL = v
	}
	if v := envInt("EQ_TIMEOUT_SECONDS"); v > 0 {
		cfg.PowerChecker.TimeoutSeconds = v
	}

	// ---- Scheduler ----
	if v := envInt("EQ_POLL_INTERVAL"); v > 0 {
		cfg.Scheduler.PollInterval = v
	}
}
