package logger

import (
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
)

var (
	defaultLogger atomic.Pointer[slog.Logger]
	once          sync.Once
)

func init() {
	once.Do(func() {
		opts := &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}
		handler := slog.NewTextHandler(os.Stderr, opts)
		defaultLogger.Store(slog.New(handler))
	})
}

// SetLevel sets the minimum log level
func SetLevel(level slog.Level) {
	opts := &slog.HandlerOptions{Level: level}
	handler := slog.NewTextHandler(os.Stderr, opts)
	defaultLogger.Store(slog.New(handler))
}

func logger() *slog.Logger {
	l := defaultLogger.Load()
	if l == nil {
		// Defensive fallback if SetLevel/once haven't run yet
		opts := &slog.HandlerOptions{Level: slog.LevelInfo}
		handler := slog.NewTextHandler(os.Stderr, opts)
		l = slog.New(handler)
		defaultLogger.Store(l)
	}
	return l
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	logger().Debug(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	logger().Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	logger().Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	logger().Error(msg, args...)
}

// With returns a logger with the given attributes
func With(args ...any) *slog.Logger {
	return logger().With(args...)
}
