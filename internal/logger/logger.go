package logger

import (
	"log/slog"
	"os"
	"sync"
)

var (
	defaultLogger *slog.Logger
	once          sync.Once
)

func init() {
	once.Do(func() {
		opts := &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}
		handler := slog.NewTextHandler(os.Stderr, opts)
		defaultLogger = slog.New(handler)
	})
}

// SetLevel sets the minimum log level
func SetLevel(level slog.Level) {
	opts := &slog.HandlerOptions{Level: level}
	handler := slog.NewTextHandler(os.Stderr, opts)
	defaultLogger = slog.New(handler)
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}

// With returns a logger with the given attributes
func With(args ...any) *slog.Logger {
	return defaultLogger.With(args...)
}
