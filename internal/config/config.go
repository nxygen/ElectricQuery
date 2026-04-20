// Package config 负责加载和解析 JSON 格式的 application.conf
// 配置文件为标准 JSON 格式（application.conf.example 为模板）
package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// 全局单例
var (
	once sync.Once
	cfg  *AppConfig
)

// AppConfig 是整个应用的强类型配置结构
type AppConfig struct {
	App          AppSection
	Database     DatabaseSection
	SMTP         SMTPSection
	PowerChecker PowerCheckerSection
	Scheduler    SchedulerSection
}

type AppSection struct {
	Host           string
	Port           int
	JWTSecret      string
	JWTExpireHours int
	InternalToken  string
	Mode           string
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
	App    rawApp    `json:"app"`
	Db     rawDb     `json:"database"`
	SMTP   rawSMTP   `json:"smtp"`
	PC     rawPC     `json:"power_checker"`
	Sch    rawSch    `json:"scheduler"`
}

type rawApp struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	JWTSecret      string `json:"jwt_secret"`
	JWTExpireHours int    `json:"jwt_expire_hours"`
	InternalToken  string `json:"internal_token"`
	Mode           string `json:"mode"`
}
type rawDb struct {
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
type rawPC struct {
	LoginURL       string `json:"login_url"`
	UserAgent      string `json:"user_agent"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}
type rawSch struct {
	PollInterval        int     `json:"poll_interval"`
	AlertThreshold      float64 `json:"alert_threshold"`
	WeeklyReportWeekday int     `json:"weekly_report_weekday"`
	WeeklyReportHour    int     `json:"weekly_report_hour"`
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
		log.Printf("[config] 工作目录: %s", wd)
		log.Printf("[config] 正在加载配置文件: %s", absPath)

		raw, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("[config] 读取配置文件失败: %v\n请确认 application.conf 存在于项目根目录", err)
		}

		// 去除 UTF-8 BOM（某些编辑器写入的 BOM 会导致 JSON 解析失败）
		raw = stripBOM(raw)
		log.Printf("[config] 文件大小: %d bytes（已去除 BOM）", len(raw))

		var rawCfg rawJSON
		if err := json.Unmarshal(raw, &rawCfg); err != nil {
			log.Fatalf("[config] JSON 解析失败: %v\n请检查 application.conf 格式是否正确", err)
		}

		// 映射到强类型配置，零值字段使用默认值
		cfg = &AppConfig{
			App: AppSection{
				Host:           strDef(rawCfg.App.Host, "0.0.0.0"),
				Port:           intDef(rawCfg.App.Port, 8080),
				JWTSecret:      strDef(rawCfg.App.JWTSecret, "changeme"),
				JWTExpireHours: intDef(rawCfg.App.JWTExpireHours, 72),
				InternalToken:  rawCfg.App.InternalToken,
				Mode:           strDef(rawCfg.App.Mode, "debug"),
			},
			Database: DatabaseSection{
				Driver: strDef(rawCfg.Db.Driver, "sqlite"),
				SQLite: SQLiteSection{Path: strDef(rawCfg.Db.SQLite.Path, "data/electricquery.db")},
				MySQL: MySQLSection{
					Host:     strDef(rawCfg.Db.MySQL.Host, "127.0.0.1"),
					Port:     intDef(rawCfg.Db.MySQL.Port, 3306),
					User:     strDef(rawCfg.Db.MySQL.User, "root"),
					Password: rawCfg.Db.MySQL.Password,
					DBName:   strDef(rawCfg.Db.MySQL.DBName, "electricquery"),
					Charset:  strDef(rawCfg.Db.MySQL.Charset, "utf8mb4"),
					Loc:      strDef(rawCfg.Db.MySQL.Loc, "Asia%2FShanghai"),
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
				LoginURL:       rawCfg.PC.LoginURL,
				UserAgent:      strDef(rawCfg.PC.UserAgent, "Mozilla/5.0"),
				TimeoutSeconds: intDef(rawCfg.PC.TimeoutSeconds, 15),
			},
			Scheduler: SchedulerSection{
				PollInterval:        intDef(rawCfg.Sch.PollInterval, 600),
				AlertThreshold:      floatDef(rawCfg.Sch.AlertThreshold, 20.0),
				WeeklyReportWeekday: intDef(rawCfg.Sch.WeeklyReportWeekday, 1),
				WeeklyReportHour:    intDef(rawCfg.Sch.WeeklyReportHour, 8),
			},
		}

		log.Printf("[config] 配置加载成功，数据库驱动: %s，服务端口: %d", cfg.Database.Driver, cfg.App.Port)
		log.Printf("[config] 爬虫配置: login_url=[%s] timeout=%ds", cfg.PowerChecker.LoginURL, cfg.PowerChecker.TimeoutSeconds)
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
