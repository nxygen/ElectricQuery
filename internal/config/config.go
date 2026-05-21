package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"electricquery/internal/logger"

	"github.com/gurkankaymak/hocon"
)

func initLog(format string, args ...any) {
	if logger.GetLogger() != nil {
		logger.Info(fmt.Sprintf(format, args...))
	} else if strings.EqualFold(os.Getenv("EQ_LOG_LEVEL"), "debug") {
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

var (
	once sync.Once
	cfg  *AppConfig
)

type AppConfig struct {
	App          AppSection
	Log          LogSection
	Database     DatabaseSection
	SMTP         SMTPSection
	PowerChecker PowerCheckerSection
	Scheduler    SchedulerSection
}

type LogSection struct {
	Level      string
	Path       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
	Console    bool
}

type AppSection struct {
	Host                 string
	Port                 int
	JWTSecret            string
	JWTExpireHours       int
	InternalToken        string
	AdminToken           string
	Mode                 string
	AllowedOrigin        string
	MaxLoginPerWindow    int
	MaxRegisterPerWindow int
	RateLimitWindowSec   int
}

type DatabaseSection struct {
	Driver       string
	MaxOpenConns int
	MaxIdleConns int
	SQLite       SQLiteSection
	MySQL        MySQLSection
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

type rawConfig struct {
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
	AdminToken           string `json:"admin_token"`
	Mode                 string `json:"mode"`
	AllowedOrigin        string `json:"allowed_origin"`
	MaxLoginPerWindow    int    `json:"max_login_per_window"`
	MaxRegisterPerWindow int    `json:"max_register_per_window"`
	RateLimitWindowSec   int    `json:"rate_limit_window_sec"`
}
type rawDatabase struct {
	Driver       string    `json:"driver"`
	MaxOpenConns int       `json:"max_open_conns"`
	MaxIdleConns int       `json:"max_idle_conns"`
	SQLite       rawSQLite `json:"sqlite"`
	MySQL        rawMySQL  `json:"mysql"`
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

		raw = stripBOM(raw)
		initLog("配置文件大小: %d bytes（已去除 BOM）", len(raw))

		rawCfg, err := parseRawConfig(raw)
		if err != nil {
			initFatal("HOCON 解析失败: ", err, "\n请检查 application.conf 格式是否正确")
		}

		cfg = &AppConfig{
			App: AppSection{
				Host:                 strDef(rawCfg.App.Host, "0.0.0.0"),
				Port:                 intDef(rawCfg.App.Port, 8080),
				JWTSecret:            strDef(rawCfg.App.JWTSecret, "changeme"),
				JWTExpireHours:       intDef(rawCfg.App.JWTExpireHours, 72),
				InternalToken:        rawCfg.App.InternalToken,
				AdminToken:           rawCfg.App.AdminToken,
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
				Driver:       strDef(rawCfg.Database.Driver, "sqlite"),
				MaxOpenConns: intDef(rawCfg.Database.MaxOpenConns, 0),
				MaxIdleConns: intDef(rawCfg.Database.MaxIdleConns, 0),
				SQLite:       SQLiteSection{Path: strDef(rawCfg.Database.SQLite.Path, "data/electricquery.db")},
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

		applyEnvOverrides(cfg)

		if len(cfg.App.JWTSecret) < 32 {
			initFatal("安全配置错误：jwt_secret 长度必须 >= 32 字节，当前长度=", len(cfg.App.JWTSecret),
				"\n请使用随机字符串，例如：openssl rand -base64 32")
		}
		if cfg.App.Mode != "debug" && cfg.App.AllowedOrigin == "" {
			initFatal("安全配置错误：生产模式必须设置 allowed_origin（前端域名），当前为空")
		}

		initLog("配置加载成功，数据库驱动: %s，服务端口: %d", cfg.Database.Driver, cfg.App.Port)
		initLog("爬虫配置: login_url=%s timeout=%ds", cfg.PowerChecker.LoginURL, cfg.PowerChecker.TimeoutSeconds)
	})
	return cfg
}

func ParseLogConfigFile(path string) (LogSection, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return LogSection{}, err
	}

	rawCfg, err := parseRawConfig(raw)
	if err != nil {
		return LogSection{}, err
	}

	return LogSection{
		Level:      strDef(rawCfg.Log.Level, "info"),
		Path:       strDef(rawCfg.Log.Path, "logs/app.log"),
		MaxSizeMB:  intDef(rawCfg.Log.MaxSizeMB, 10),
		MaxBackups: intDef(rawCfg.Log.MaxBackups, 7),
		MaxAgeDays: intDef(rawCfg.Log.MaxAgeDays, 30),
		Compress:   rawCfg.Log.Compress,
		Console:    rawCfg.Log.Console,
	}, nil
}

func parseRawConfig(raw []byte) (rawConfig, error) {
	hoconCfg, err := hocon.ParseString(string(stripHOCONComments(stripBOM(raw))))
	if err != nil {
		return rawConfig{}, err
	}

	return rawConfig{
		App: rawApp{
			Host:                 hoconString(hoconCfg, "app.host"),
			Port:                 hoconInt(hoconCfg, "app.port"),
			JWTSecret:            hoconString(hoconCfg, "app.jwt_secret"),
			JWTExpireHours:       hoconInt(hoconCfg, "app.jwt_expire_hours"),
			InternalToken:        hoconString(hoconCfg, "app.internal_token"),
			AdminToken:           hoconString(hoconCfg, "app.admin_token"),
			Mode:                 hoconString(hoconCfg, "app.mode"),
			AllowedOrigin:        hoconString(hoconCfg, "app.allowed_origin"),
			MaxLoginPerWindow:    hoconInt(hoconCfg, "app.max_login_per_window"),
			MaxRegisterPerWindow: hoconInt(hoconCfg, "app.max_register_per_window"),
			RateLimitWindowSec:   hoconInt(hoconCfg, "app.rate_limit_window_sec"),
		},
		Log: rawLog{
			Level:      hoconString(hoconCfg, "log.level"),
			Path:       hoconString(hoconCfg, "log.path"),
			MaxSizeMB:  hoconInt(hoconCfg, "log.max_size_mb"),
			MaxBackups: hoconInt(hoconCfg, "log.max_backups"),
			MaxAgeDays: hoconInt(hoconCfg, "log.max_age_days"),
			Compress:   hoconBool(hoconCfg, "log.compress"),
			Console:    hoconBool(hoconCfg, "log.console"),
		},
		Database: rawDatabase{
			Driver:       hoconString(hoconCfg, "database.driver"),
			MaxOpenConns: hoconInt(hoconCfg, "database.max_open_conns"),
			MaxIdleConns: hoconInt(hoconCfg, "database.max_idle_conns"),
			SQLite: rawSQLite{
				Path: hoconString(hoconCfg, "database.sqlite.path"),
			},
			MySQL: rawMySQL{
				Host:     hoconString(hoconCfg, "database.mysql.host"),
				Port:     hoconInt(hoconCfg, "database.mysql.port"),
				User:     hoconString(hoconCfg, "database.mysql.user"),
				Password: hoconString(hoconCfg, "database.mysql.password"),
				DBName:   hoconString(hoconCfg, "database.mysql.dbname"),
				Charset:  hoconString(hoconCfg, "database.mysql.charset"),
				Loc:      hoconString(hoconCfg, "database.mysql.loc"),
			},
		},
		SMTP: rawSMTP{
			Enabled:     hoconBool(hoconCfg, "smtp.enabled"),
			SenderEmail: hoconString(hoconCfg, "smtp.sender_email"),
			SenderName:  hoconString(hoconCfg, "smtp.sender_name"),
			Server:      hoconString(hoconCfg, "smtp.server"),
			Port:        hoconInt(hoconCfg, "smtp.port"),
			UseSSL:      hoconBool(hoconCfg, "smtp.use_ssl"),
			Password:    hoconString(hoconCfg, "smtp.password"),
		},
		PowerChecker: rawPowerChecker{
			LoginURL:       hoconString(hoconCfg, "power_checker.login_url"),
			UserAgent:      hoconString(hoconCfg, "power_checker.user_agent"),
			TimeoutSeconds: hoconInt(hoconCfg, "power_checker.timeout_seconds"),
		},
		Scheduler: rawScheduler{
			PollInterval:        hoconInt(hoconCfg, "scheduler.poll_interval"),
			AlertThreshold:      hoconFloat64(hoconCfg, "scheduler.alert_threshold"),
			WeeklyReportWeekday: hoconInt(hoconCfg, "scheduler.weekly_report_weekday"),
			WeeklyReportHour:    hoconInt(hoconCfg, "scheduler.weekly_report_hour"),
		},
	}, nil
}

func hoconString(cfg *hocon.Config, path string) string {
	value := cfg.Get(path)
	if value == nil {
		return ""
	}
	if s, ok := value.(hocon.String); ok {
		return string(s)
	}
	return value.String()
}

func hoconInt(cfg *hocon.Config, path string) int {
	if cfg.Get(path) == nil {
		return 0
	}
	return cfg.GetInt(path)
}

func hoconFloat64(cfg *hocon.Config, path string) float64 {
	if cfg.Get(path) == nil {
		return 0
	}
	return cfg.GetFloat64(path)
}

func hoconBool(cfg *hocon.Config, path string) bool {
	if cfg.Get(path) == nil {
		return false
	}
	return cfg.GetBoolean(path)
}

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

func stripBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

func stripHOCONComments(raw []byte) []byte {
	out := make([]byte, 0, len(raw))
	inString := false
	escaped := false

	for i := 0; i < len(raw); i++ {
		ch := raw[i]

		if inString {
			out = append(out, ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			out = append(out, ch)
			continue
		}

		if ch == '#' || (ch == '/' && i+1 < len(raw) && raw[i+1] == '/') {
			for i < len(raw) && raw[i] != '\n' {
				i++
			}
			if i < len(raw) {
				out = append(out, raw[i])
			}
			continue
		}

		out = append(out, ch)
	}

	return out
}

func strDef(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
func intDef(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
func floatDef(v, def float64) float64 {
	if v == 0 {
		return def
	}
	return v
}

func envInt(key string) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return 0
}

func envBoolValue(key string) (bool, bool) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return false, false
	}
	return strings.EqualFold(v, "true") || v == "1", true
}

func applyEnvOverrides(cfg *AppConfig) {
	if v := os.Getenv("EQ_HOST"); v != "" {
		cfg.App.Host = v
	}
	if v := envInt("EQ_PORT"); v > 0 {
		cfg.App.Port = v
	}
	if v := os.Getenv("EQ_MODE"); v != "" {
		cfg.App.Mode = v
	}
	if v := envInt("EQ_JWT_EXPIRE_HOURS"); v > 0 {
		cfg.App.JWTExpireHours = v
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
	if v := envInt("EQ_LOG_MAX_AGE_DAYS"); v > 0 {
		cfg.Log.MaxAgeDays = v
	}
	if v, ok := envBoolValue("EQ_LOG_COMPRESS"); ok {
		cfg.Log.Compress = v
	}
	if v, ok := envBoolValue("EQ_LOG_CONSOLE"); ok {
		cfg.Log.Console = v
	}

	if v := os.Getenv("EQ_DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := envInt("EQ_DB_MAX_OPEN"); v > 0 {
		cfg.Database.MaxOpenConns = v
	}
	if v := envInt("EQ_DB_MAX_IDLE"); v > 0 {
		cfg.Database.MaxIdleConns = v
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
	if v := os.Getenv("EQ_MYSQL_CHARSET"); v != "" {
		cfg.Database.MySQL.Charset = v
	}
	if v := os.Getenv("EQ_MYSQL_LOC"); v != "" {
		cfg.Database.MySQL.Loc = v
	}

	if v, ok := envBoolValue("EQ_SMTP_ENABLED"); ok {
		cfg.SMTP.Enabled = v
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
	if v, ok := envBoolValue("EQ_SMTP_USE_SSL"); ok {
		cfg.SMTP.UseSSL = v
	}

	if v := os.Getenv("EQ_LOGIN_URL"); v != "" {
		cfg.PowerChecker.LoginURL = v
	}
	if v := envInt("EQ_TIMEOUT_SECONDS"); v > 0 {
		cfg.PowerChecker.TimeoutSeconds = v
	}

	if v := envInt("EQ_POLL_INTERVAL"); v > 0 {
		cfg.Scheduler.PollInterval = v
	}
	if v := envInt("EQ_WEEKLY_REPORT_WEEKDAY"); v > 0 {
		cfg.Scheduler.WeeklyReportWeekday = v
	}
	if v := envInt("EQ_WEEKLY_REPORT_HOUR"); v > 0 {
		cfg.Scheduler.WeeklyReportHour = v
	}
}
