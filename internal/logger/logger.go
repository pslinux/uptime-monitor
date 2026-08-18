// Package logger 提供轻量级日志能力, 支持输出到文件(含简单大小轮转)或 stdout。
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level 日志级别。
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "INFO"
	}
}

// parseLevel 将字符串转为级别。
func parseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return DEBUG
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

// Logger 线程安全日志器。
type Logger struct {
	mu      sync.Mutex
	level   Level
	file    *os.File
	maxSize int64 // 字节
}

// New 创建日志器。filePath 为空则仅输出到 stdout。
func New(level, filePath string, maxSizeMB int) (*Logger, error) {
	l := &Logger{level: parseLevel(level), maxSize: int64(maxSizeMB) * 1024 * 1024}
	if filePath != "" {
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件失败: %w", err)
		}
		l.file = f
	}
	return l, nil
}

// Close 关闭日志文件。
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}

// log 写入一条日志。
func (l *Logger) log(lv Level, format string, args ...interface{}) {
	if lv < l.level {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] [%s] %s", ts, lv.String(), fmt.Sprintf(format, args...))

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		l.rotateIfNeeded()
		fmt.Fprintln(l.file, line)
	}
	fmt.Fprintln(os.Stdout, line)
}

// rotateIfNeeded 简单大小轮转: 超限则重命名当前文件并新建。
func (l *Logger) rotateIfNeeded() {
	if l.file == nil {
		return
	}
	fi, err := l.file.Stat()
	if err == nil && fi.Size() >= l.maxSize {
		name := l.file.Name()
		l.file.Close()
		os.Rename(name, name+"."+time.Now().Format("20060102-150405"))
		f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			l.file = f
		}
	}
}

// Debug / Info / Warn / Error 便捷方法。
func (l *Logger) Debug(format string, args ...interface{}) { l.log(DEBUG, format, args...) }
func (l *Logger) Info(format string, args ...interface{})  { l.log(INFO, format, args...) }
func (l *Logger) Warn(format string, args ...interface{})  { l.log(WARN, format, args...) }
func (l *Logger) Error(format string, args ...interface{}) { l.log(ERROR, format, args...) }
