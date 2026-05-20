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

var defaultLogger *slog.Logger

func Init(level, path string, maxSizeMB, maxBackups, maxAgeDays int, compress, console bool) {
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

	if path != "" {
		if !filepath.IsAbs(path) {
			if workDir, err := os.Getwd(); err == nil {
				path = filepath.Join(workDir, path)
			} else {
				exePath, _ := os.Executable()
				path = filepath.Join(filepath.Dir(exePath), path)
			}
		}
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

	if console {
		writers = append(writers, os.Stdout)
	}

	if len(writers) == 0 {
		writers = append(writers, io.Discard)
	}

	defaultLogger = slog.New(newPrettyHandler(lvl, &multiWriter{writers}))
	slog.SetDefault(defaultLogger)
}

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

	buf.WriteString(r.Time.Format("2006-01-02 15:04:05.000"))

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

	buf.WriteString(r.Message)
	buf.WriteString(" ")

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

	if b := buf.Bytes(); len(b) > 0 && b[len(b)-1] == ' ' {
		buf.Truncate(buf.Len() - 1)
	}
	buf.WriteByte('\n')

	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *prettyHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *prettyHandler) WithGroup(_ string) slog.Handler     { return h }

func Debug(msg string, args ...any) { defaultLogger.Debug(msg, args...) }
func Info(msg string, args ...any)  { defaultLogger.Info(msg, args...) }
func Warn(msg string, args ...any)  { defaultLogger.Warn(msg, args...) }
func Error(msg string, args ...any) { defaultLogger.Error(msg, args...) }
func Fatal(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
	os.Exit(1)
}

func GetLogger() *slog.Logger { return defaultLogger }

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
