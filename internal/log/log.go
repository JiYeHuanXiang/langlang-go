// Package log 提供统一的日志接口。
// 底层使用 slog（Go 1.21+ 标准库封装）。
package log

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

var levelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// Init 初始化日志系统
func Init(level string, filePath string) error {
	lvl, ok := levelMap[level]
	if !ok {
		lvl = slog.LevelInfo
	}

	var writer io.Writer = os.Stdout
	if filePath != "" {
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return err
		}
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		writer = io.MultiWriter(os.Stdout, f)
	}

	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: lvl,
	})

	slog.SetDefault(slog.New(handler))
	return nil
}

func Debug(msg string, args ...any) { slog.Debug(msg, args...) }
func Info(msg string, args ...any)  { slog.Info(msg, args...) }
func Warn(msg string, args ...any)  { slog.Warn(msg, args...) }
func Error(msg string, args ...any) { slog.Error(msg, args...) }

func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
