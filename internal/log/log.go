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
	"sync"
	"sync/atomic"
	"time"
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

// 使用 atomic 确保跨协程可见性（Start() 协程写入，HTTP 协程读取）
var broadcaster atomic.Pointer[BroadcastFunc]

// SetBroadcaster 注册日志广播器（由 webui 在启动时调用）
func SetBroadcaster(fn BroadcastFunc) {
	fnPtr := new(BroadcastFunc)
	*fnPtr = fn
	broadcaster.Store(fnPtr)
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
	formatted := formatBroadcast(msg, args)
	pushRing("debug", formatted)
	if fn := broadcaster.Load(); fn != nil {
		(*fn)("debug", formatted)
	}
}

func Info(msg string, args ...any) {
	slog.Info(msg, args...)
	formatted := formatBroadcast(msg, args)
	pushRing("info", formatted)
	if fn := broadcaster.Load(); fn != nil {
		(*fn)("info", formatted)
	}
}

func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
	formatted := formatBroadcast(msg, args)
	pushRing("warn", formatted)
	if fn := broadcaster.Load(); fn != nil {
		(*fn)("warn", formatted)
	}
}

func Error(msg string, args ...any) {
	slog.Error(msg, args...)
	formatted := formatBroadcast(msg, args)
	pushRing("error", formatted)
	if fn := broadcaster.Load(); fn != nil {
		(*fn)("error", formatted)
	}
}

func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	formatted := formatBroadcast(msg, args)
	pushRing("error", formatted)
	if fn := broadcaster.Load(); fn != nil {
		(*fn)("error", formatted)
	}
	os.Exit(1)
}

// ===== 日志环形缓冲区（用于 HTTP 轮询回退） =====

// Entry 是单条日志记录
type Entry struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
	Time  string `json:"time"`
}

const ringBufSize = 200

var (
	ringMu     sync.Mutex
	ringBuf    [ringBufSize]Entry
	ringNext   int
	ringWrapped bool
)

// pushRing 向环形缓冲区添加一条日志
func pushRing(level, msg string) {
	ringMu.Lock()
	defer ringMu.Unlock()
	ringBuf[ringNext] = Entry{
		Level: level,
		Msg:   msg,
		Time:  time.Now().Format("15:04:05.000"),
	}
	ringNext++
	if ringNext >= ringBufSize {
		ringNext = 0
		ringWrapped = true
	}
}

// RecentEntries 返回最近的 N 条日志（最多 ringBufSize 条）
func RecentEntries(n int) []Entry {
	ringMu.Lock()
	defer ringMu.Unlock()

	if n <= 0 || n > ringBufSize {
		n = ringBufSize
	}

	var result []Entry
	if ringWrapped {
		// 缓冲区已绕回，从 ringNext 到末尾 + 从开头到 ringNext-1
		result = make([]Entry, 0, n)
		// 从 ringNext 开始（最旧的已覆盖位置）
		for i := 0; i < ringBufSize && len(result) < n; i++ {
			idx := (ringNext + i) % ringBufSize
			if ringBuf[idx].Msg != "" {
				result = append(result, ringBuf[idx])
			}
		}
	} else {
		// 还没绕回，直接取前 ringNext 条
		count := ringNext
		if count > n {
			count = n
		}
		start := ringNext - count
		if start < 0 {
			start = 0
		}
		result = make([]Entry, count)
		copy(result, ringBuf[start:ringNext])
	}
	return result
}

// 在每条日志中推入环形缓冲区
// 注：各日志函数在各自函数体内调用 pushRing
