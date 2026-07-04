package logger

import (
	"log/slog"
	"testing"
)

func TestInit(t *testing.T) {
	// Logger is initialized via init(), just verify it doesn't panic
	Debug("test debug message")
	Info("test info message")
	Warn("test warn message")
	Error("test error message")
}

func TestSetLevel(t *testing.T) {
	SetLevel(slog.LevelDebug)
	Debug("should be visible at debug level")
	SetLevel(slog.LevelInfo)
}

func TestWith(t *testing.T) {
	l := With("key", "value")
	if l == nil {
		t.Error("expected non-nil logger from With()")
	}
}

func TestLogMessages(t *testing.T) {
	// Verify all log functions work without panicking
	Info("info message", "module", "test")
	Warn("warn message", "module", "test")
	Error("error message", "module", "test")
	Debug("debug message", "module", "test")
}
