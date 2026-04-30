// Package logger 提供结构化日志能力，支持文件轮转（lumberjack）和美化格式
// 所有业务代码统一使用本包，禁止直接使用标准库 log
package logger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/natefinch/lumberjack"
)

// 全局 slog 实例
var defaultLogger *slog.Logger

// Init 使用配置文件中的日志配置初始化全局日志器
// - level: debug/info/warn/error
// - path: 日志文件路径（相对路径相对于工作目录，绝对路径直接使用）
//        轮转历史文件格式：app.log.YYYY-MM-DD.gz（Minecraft 风格）
// - maxSizeMB: 单文件最大大小（MB）
// - maxBackups: 保留历史文件份数
// - maxAgeDays: 保留历史文件天数（0=不按时间删除）
// - compress: 是否压缩历史文件
// - console: 是否同时输出到 stdout
func Init(level, path string, maxSizeMB, maxBackups, maxAgeDays int, compress, console bool) {
	// 解析日志级别
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	var writers []io.Writer

	// 文件 Writer（lumberjack 轮转）
	if path != "" {
		// 相对路径相对于工作目录，绝对路径直接使用
		if !filepath.IsAbs(path) {
			// 优先用工作目录，fallback 到可执行文件目录
			if workDir, err := os.Getwd(); err == nil {
				path = filepath.Join(workDir, path)
			} else {
				exePath, _ := os.Executable()
				path = filepath.Join(filepath.Dir(exePath), path)
			}
		}
		// 使用原始路径，lumberjack 会自动按日期生成轮转文件
		// 格式：app.log → app.log.2026-04-27.gz
		if dir := filepath.Dir(path); dir != "" {
			os.MkdirAll(dir, 0750)
		}
		writers = append(writers, &lumberjack.Logger{
			Filename:   path,
			MaxSize:    maxSizeMB,
			MaxBackups: maxBackups,
			MaxAge:     maxAgeDays,
			Compress:   compress,
			LocalTime:  true,
		})
	}

	// 控制台 Writer
	if console {
		writers = append(writers, os.Stdout)
	}

	// 默认：无文件也无控制台时，静默丢弃
	if len(writers) == 0 {
		writers = append(writers, io.Discard)
	}

	defaultLogger = slog.New(newPrettyHandler(lvl, &multiWriter{writers}))
	slog.SetDefault(defaultLogger)
}

// ---- prettyHandler：自定义格式化，输出 "时间 [级别] 消息 字段..." ----

type prettyHandler struct {
	lvl    slog.Level
	w      io.Writer
	prefix string
}

func newPrettyHandler(minLevel slog.Level, w io.Writer) *prettyHandler {
	return &prettyHandler{lvl: minLevel, w: w}
}

func (h *prettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.lvl
}

func (h *prettyHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer

	// 时间：YYYY-MM-DD HH:MM:SS.mmm
	buf.WriteString(r.Time.Format("2006-01-02 15:04:05.000"))

	// 级别
	buf.WriteString(" [")
	switch r.Level {
	case slog.LevelDebug:
		buf.WriteString("DEBUG")
	case slog.LevelInfo:
		buf.WriteString("INFO")
	case slog.LevelWarn:
		buf.WriteString("WARN")
	case slog.LevelError:
		buf.WriteString("ERROR")
	default:
		buf.WriteString(r.Level.String())
	}
	buf.WriteString("] ")

	// 消息（带空格避免黏连）
	buf.WriteString(r.Message)
	buf.WriteString(" ")

	// 其余 Attr（跳过 time 和 level）
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case slog.TimeKey, slog.LevelKey:
			return true
		}
		buf.WriteString(a.Key)
		buf.WriteString("=")
		buf.WriteString(fmt.Sprintf("%v", a.Value.Any()))
		buf.WriteString(" ")
		return true
	})

	// 去掉末尾多余空格，换行结束
	if b := buf.Bytes(); len(b) > 0 && b[len(b)-1] == ' ' {
		buf.Truncate(buf.Len() - 1)
	}
	buf.WriteByte('\n')

	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *prettyHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *prettyHandler) WithGroup(_ string) slog.Handler     { return h }

// ---- 短路径别名 ----

func Debug(msg string, args ...any) { defaultLogger.Debug(msg, args...) }
func Info(msg string, args ...any)  { defaultLogger.Info(msg, args...) }
func Warn(msg string, args ...any)  { defaultLogger.Warn(msg, args...) }
func Error(msg string, args ...any) { defaultLogger.Error(msg, args...) }
func Fatal(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
	os.Exit(1)
}

// GetLogger 返回当前全局 slog 实例，供需要 *slog.Logger 的场景使用（如 gin.Logger()）
func GetLogger() *slog.Logger { return defaultLogger }

// ---- multiWriter：同时写入多个 io.Writer ----

type multiWriter struct {
	writers []io.Writer
}

func (mw *multiWriter) Write(p []byte) (n int, err error) {
	for _, w := range mw.writers {
		if _, err := w.Write(p); err != nil {
			return n, err
		}
		n += len(p)
	}
	return n, nil
}
