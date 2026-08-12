// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	defaultLogger atomic.Pointer[slog.Logger]
	once          sync.Once
	currentLevel  = slog.LevelInfo
	currentFormat = "text"
	// rotWriter is non-nil when logging to a file. It owns the underlying
	// *os.File and performs size-based rotation.
	rotWriter *rotatingWriter
)

const (
	defaultLogMaxMB      = 100
	defaultLogMaxBackups = 3
)

func init() {
	once.Do(func() {
		level := getLevelFromEnv()
		format := getFormatFromEnv()
		output := getOutputFromEnv()
		initLogger(level, format, output)
	})
}

func getLevelFromEnv() slog.Level {
	envLevel := strings.ToLower(strings.TrimSpace(os.Getenv("AFLARE_LOG_LEVEL")))
	switch envLevel {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func getFormatFromEnv() string {
	format := strings.ToLower(strings.TrimSpace(os.Getenv("AFLARE_LOG_FORMAT")))
	if format == "json" || format == "text" {
		return format
	}
	return "text"
}

func getOutputFromEnv() string {
	output := strings.TrimSpace(os.Getenv("AFLARE_LOG_FILE"))
	if output != "" {
		return output
	}
	return "stderr"
}

// getLogMaxMB reads AFLARE_LOG_MAX_MB and returns the rotation threshold in
// bytes. A value of 0 disables rotation. Invalid values fall back to the
// default.
func getLogMaxMB() int64 {
	raw := strings.TrimSpace(os.Getenv("AFLARE_LOG_MAX_MB"))
	if raw == "" {
		return int64(defaultLogMaxMB) * 1024 * 1024
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return int64(defaultLogMaxMB) * 1024 * 1024
	}
	return n * 1024 * 1024
}

// getLogMaxBackups reads AFLARE_LOG_MAX_BACKUPS and returns the number of
// rotated backup files to retain. Invalid values fall back to the default.
func getLogMaxBackups() int {
	raw := strings.TrimSpace(os.Getenv("AFLARE_LOG_MAX_BACKUPS"))
	if raw == "" {
		return defaultLogMaxBackups
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return defaultLogMaxBackups
	}
	return n
}

// rotatingWriter is an io.Writer that appends to a file and rotates it by
// size. When a Write would push the current file past maxSize, the writer
// closes the file, shifts existing backups (.1 -> .2, .2 -> .3, ...), deletes
// the oldest backup beyond maxBackups, and opens a fresh file. Rotation is
// guarded by a mutex so concurrent Write calls are safe.
type rotatingWriter struct {
	mu         sync.Mutex
	filename   string
	f          *os.File
	maxSize    int64
	maxBackups int
}

// newRotatingWriter opens filename in append mode (creating it if missing)
// and returns a writer that will rotate once the file exceeds maxSize.
func newRotatingWriter(filename string, maxSize int64, maxBackups int) (*rotatingWriter, error) {
	dir := filepath.Dir(filename)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0750) // best-effort: dir creation
	}
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &rotatingWriter{
		filename:   filename,
		f:          f,
		maxSize:    maxSize,
		maxBackups: maxBackups,
	}, nil
}

// Write writes p to the current file, rotating first if the write would
// exceed maxSize. The rotation threshold is best-effort: a single Write
// larger than maxSize still lands in a fresh file (no chunking).
func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.maxSize > 0 && w.f != nil {
		if fi, err := w.f.Stat(); err == nil {
			if fi.Size()+int64(len(p)) > w.maxSize {
				w.rotateLocked()
			}
		}
	}
	if w.f == nil {
		return 0, fmt.Errorf("logger: rotating writer closed")
	}
	return w.f.Write(p)
}

// rotateLocked performs the file rotation. Caller must hold w.mu.
func (w *rotatingWriter) rotateLocked() {
	if w.f != nil {
		if err := w.f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "logger: failed to close log file during rotation: %v\n", err)
		}
		w.f = nil
	}

	if w.maxBackups <= 0 {
		// No backups retained: just remove the current file.
		_ = os.Remove(w.filename) // best-effort: rotate cleanup
	} else {
		// Drop the oldest backup, then shift .(N-1) -> .N, ..., .1 -> .2.
		for i := w.maxBackups; i >= 1; i-- {
			src := fmt.Sprintf("%s.%d", w.filename, i)
			if i == w.maxBackups {
				_ = os.Remove(src) // best-effort: rotate cleanup
				continue
			}
			dst := fmt.Sprintf("%s.%d", w.filename, i+1)
			_ = os.Rename(src, dst) // best-effort: rotate shift
		}
		// Promote the current file to .1.
		_ = os.Rename(w.filename, fmt.Sprintf("%s.1", w.filename)) // best-effort: rotate promote
	}

	f, err := os.OpenFile(w.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: failed to reopen log file after rotation: %v\n", err)
		return
	}
	w.f = f
}

// Name returns the base log file path (the one rotation keeps reopening).
func (w *rotatingWriter) Name() string {
	return w.filename
}

// Close closes the underlying file. Subsequent Write calls return an error.
func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

func initLogger(level slog.Level, format string, output string) {
	currentLevel = level
	currentFormat = format

	// Close any previously-opened rotating writer so re-init does not leak
	// file descriptors.
	if rotWriter != nil {
		if err := rotWriter.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "logger: failed to close previous log file: %v\n", err)
		}
		rotWriter = nil
	}

	var writer io.Writer
	if output == "stderr" || output == "" {
		writer = os.Stderr
	} else {
		rw, err := newRotatingWriter(output, getLogMaxMB(), getLogMaxBackups())
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open log file %s: %v, falling back to stderr\n", output, err)
			writer = os.Stderr
		} else {
			rotWriter = rw
			writer = io.MultiWriter(os.Stderr, rw)
		}
	}

	opts := &slog.HandlerOptions{
		Level:       level,
		AddSource:   false,
		ReplaceAttr: replaceAttr,
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	defaultLogger.Store(slog.New(handler))
}

// sensitiveKeys are attribute keys whose values should be redacted from logs.
var sensitiveKeys = map[string]bool{
	"token":         true,
	"auth_token":    true,
	"password":      true,
	"secret":        true,
	"api_key":       true,
	"apikey":        true,
	"key":           true,
	"credential":    true,
	"credentials":   true,
	"authorization": true,
}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		return a
	}
	// Redact sensitive attribute values
	lowerKey := strings.ToLower(a.Key)
	if sensitiveKeys[lowerKey] {
		a.Value = slog.StringValue("[REDACTED]")
	}
	return a
}

// SetLevel 设置日志级别并重新初始化 logger。
func SetLevel(level slog.Level) {
	currentLevel = level
	reinitLogger()
}

// SetFormat 设置日志输出格式（"json" 或 "text"），仅合法值会触发重新初始化。
func SetFormat(format string) {
	if format == "json" || format == "text" {
		currentFormat = format
		reinitLogger()
	}
}

// SetOutput 设置日志输出目标（文件路径或 "stderr"），并关闭旧文件。
func SetOutput(output string) {
	if rotWriter != nil {
		if err := rotWriter.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "logger: failed to close log file: %v\n", err)
		}
		rotWriter = nil
	}
	initLogger(currentLevel, currentFormat, output)
}

func reinitLogger() {
	output := "stderr"
	if rotWriter != nil {
		output = rotWriter.Name()
	}
	if rotWriter != nil {
		if err := rotWriter.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "logger: failed to close log file: %v\n", err)
		}
		rotWriter = nil
	}
	initLogger(currentLevel, currentFormat, output)
}

// GetLevel 返回当前日志级别。
func GetLevel() slog.Level {
	return currentLevel
}

func logger() *slog.Logger {
	l := defaultLogger.Load()
	if l == nil {
		opts := &slog.HandlerOptions{Level: slog.LevelInfo}
		handler := slog.NewTextHandler(os.Stderr, opts)
		l = slog.New(handler)
		defaultLogger.Store(l)
	}
	return l
}

// Debug 以 Debug 级别记录日志。
func Debug(msg string, args ...any) {
	logger().Debug(msg, args...)
}

// Info 以 Info 级别记录日志。
func Info(msg string, args ...any) {
	logger().Info(msg, args...)
}

// Warn 以 Warn 级别记录日志。
func Warn(msg string, args ...any) {
	logger().Warn(msg, args...)
}

// Error 以 Error 级别记录日志。
func Error(msg string, args ...any) {
	logger().Error(msg, args...)
}

// With 返回附带给定属性的 logger 实例。
func With(args ...any) *slog.Logger {
	return logger().With(args...)
}

// Close 关闭日志文件句柄，应在程序退出前调用。
func Close() {
	if rotWriter != nil {
		if err := rotWriter.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "logger: failed to close log file: %v\n", err)
		}
		rotWriter = nil
	}
}
