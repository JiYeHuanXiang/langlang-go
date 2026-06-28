// Package log 提供统一的日志接口。
// 底层使用 slog（Go 1.21+ 标准库封装）。
package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

// ===== 广播器（用于 WebSocket 日志推送） =====

// BroadcastFunc 是日志广播回调
type BroadcastFunc func(level, msg string)

var broadcaster BroadcastFunc

// SetBroadcaster 注册日志广播器（由 webui 在启动时调用）
func SetBroadcaster(fn BroadcastFunc) {
	broadcaster = fn
}

// formatBroadcast 将结构化参数格式化为可读字符串
func formatBroadcast(msg string, args []any) string {
	if len(args) == 0 {
		return msg
	}
	var sb strings.Builder
	sb.WriteString(msg)
	sb.WriteString("  ")
	for i := 0; i < len(args); i += 2 {
		if i > 0 {
			sb.WriteString("  ")
		}
		key := fmt.Sprintf("%v", args[i])
		var val string
		if i+1 < len(args) {
			val = fmt.Sprintf("%v", args[i+1])
		} else {
			val = "<missing>"
		}
		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(val)
	}
	return sb.String()
}

func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
	if broadcaster != nil {
		broadcaster("debug", formatBroadcast(msg, args))
	}
}

func Info(msg string, args ...any) {
	slog.Info(msg, args...)
	if broadcaster != nil {
		broadcaster("info", formatBroadcast(msg, args))
	}
}

func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
	if broadcaster != nil {
		broadcaster("warn", formatBroadcast(msg, args))
	}
}

func Error(msg string, args ...any) {
	slog.Error(msg, args...)
	if broadcaster != nil {
		broadcaster("error", formatBroadcast(msg, args))
	}
}

func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	if broadcaster != nil {
		broadcaster("error", formatBroadcast(msg, args))
	}
	os.Exit(1)
}
