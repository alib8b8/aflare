package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func saveState() (slog.Level, string, *slog.Logger) {
	return currentLevel, currentFormat, defaultLogger.Load()
}

func restoreState(level slog.Level, format string, logger *slog.Logger) {
	Close()
	initLogger(level, format, "stderr")
	if logger != nil {
		defaultLogger.Store(logger)
	}
}

func TestGetLevelFromEnv(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	tests := []struct {
		name  string
		env   string
		level slog.Level
	}{
		{"debug", "debug", slog.LevelDebug},
		{"info", "info", slog.LevelInfo},
		{"warn", "warn", slog.LevelWarn},
		{"warning", "warning", slog.LevelWarn},
		{"error", "error", slog.LevelError},
		{"empty", "", slog.LevelInfo},
		{"unknown", "fatal", slog.LevelInfo},
		{"uppercase", "DEBUG", slog.LevelDebug},
		{"mixed", "DeBuG", slog.LevelDebug},
		{"trim", "  debug  ", slog.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LLM_BOX_LOG_LEVEL", tt.env)
			got := getLevelFromEnv()
			if got != tt.level {
				t.Errorf("getLevelFromEnv() = %v, want %v", got, tt.level)
			}
		})
	}
}

func TestGetFormatFromEnv(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	tests := []struct {
		name   string
		env    string
		format string
	}{
		{"json", "json", "json"},
		{"text", "text", "text"},
		{"empty", "", "text"},
		{"unknown", "yaml", "text"},
		{"uppercase", "JSON", "json"},
		{"trim", "  json  ", "json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LLM_BOX_LOG_FORMAT", tt.env)
			got := getFormatFromEnv()
			if got != tt.format {
				t.Errorf("getFormatFromEnv() = %v, want %v", got, tt.format)
			}
		})
	}
}

func TestGetOutputFromEnv(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	t.Setenv("LLM_BOX_LOG_FILE", "/tmp/test.log")
	got := getOutputFromEnv()
	if got != "/tmp/test.log" {
		t.Errorf("getOutputFromEnv() = %v, want %v", got, "/tmp/test.log")
	}

	t.Setenv("LLM_BOX_LOG_FILE", "")
	got = getOutputFromEnv()
	if got != "stderr" {
		t.Errorf("getOutputFromEnv() = %v, want %v", got, "stderr")
	}
}

func TestInitLogger(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "init.log")

	tests := []struct {
		level  slog.Level
		format string
		output string
	}{
		{slog.LevelDebug, "text", "stderr"},
		{slog.LevelInfo, "json", "stderr"},
		{slog.LevelWarn, "text", logPath},
		{slog.LevelError, "json", logPath},
	}

	for _, tt := range tests {
		initLogger(tt.level, tt.format, tt.output)
		if currentLevel != tt.level {
			t.Errorf("currentLevel = %v, want %v", currentLevel, tt.level)
		}
		if currentFormat != tt.format {
			t.Errorf("currentFormat = %v, want %v", currentFormat, tt.format)
		}
		if tt.output != "stderr" && logFile == nil {
			t.Error("expected logFile to be set for file output")
		}
	}
}

func TestInitLoggerInvalidFile(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	initLogger(slog.LevelInfo, "text", "/dev/null/invalid/file.log")
	if logFile != nil {
		t.Error("expected logFile to be nil after fallback to stderr")
	}
	Info("should work with stderr fallback")
}

func TestInitLoggerCreateDir(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "dir", "log.log")

	initLogger(slog.LevelInfo, "text", nestedPath)
	Info("nested dir test")

	_, err := os.Stat(nestedPath)
	if err != nil {
		t.Errorf("expected nested log file to be created: %v", err)
	}
}

func TestSetLevel(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "level.log")

	SetOutput(logPath)
	SetLevel(slog.LevelError)

	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	s := string(content)
	if strings.Contains(s, "debug msg") || strings.Contains(s, "info msg") || strings.Contains(s, "warn msg") {
		t.Error("expected no debug/info/warn messages at error level")
	}
	if !strings.Contains(s, "error msg") {
		t.Error("expected error message in log file")
	}

	if GetLevel() != slog.LevelError {
		t.Errorf("GetLevel() = %v, want %v", GetLevel(), slog.LevelError)
	}
}

func TestSetFormat(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "format.log")

	SetOutput(logPath)
	SetLevel(slog.LevelInfo)

	SetFormat("json")
	Info("json message")

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "{") || !strings.Contains(s, "}") {
		t.Error("expected JSON format to contain braces")
	}

	SetFormat("text")
	Info("text message")

	content, err = os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	s = string(content)
	if !strings.Contains(s, "level=INFO") {
		t.Error("expected text format to contain level=INFO")
	}

	currentFmt := currentFormat
	SetFormat("yaml")
	if currentFormat != currentFmt {
		t.Error("expected invalid format to be ignored")
	}
}

func TestSetOutput(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "output.log")

	SetLevel(slog.LevelInfo)
	SetOutput(logPath)

	Info("output test")

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "output test") {
		t.Error("expected log file to contain message")
	}

	stat, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("failed to stat log file: %v", err)
	}
	if stat.Mode().Perm() != 0600 {
		t.Errorf("expected file perm 0600, got %o", stat.Mode().Perm())
	}

	SetOutput("stderr")
	if logFile != nil {
		t.Error("expected logFile to be nil when output is stderr")
	}

	SetOutput(logPath)
	SetOutput("")
	if logFile != nil {
		t.Error("expected logFile to be nil when output is empty")
	}
}

func TestClose(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "close.log")

	SetOutput(logPath)
	Close()

	if logFile != nil {
		t.Error("expected logFile to be nil after Close")
	}

	Info("after close")
}

func TestLoggerFallback(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	defaultLogger.Store(nil)

	l := logger()
	if l == nil {
		t.Error("expected non-nil logger from logger() fallback")
	}

	Info("fallback test")
}

func TestWith(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "with.log")

	SetOutput(logPath)
	SetLevel(slog.LevelInfo)

	l := With("attr", "value")
	l.Info("with test")

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "attr") || !strings.Contains(s, "value") {
		t.Error("expected With attributes in log output")
	}
}

func TestDebugInfoWarnError(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "all_levels.log")

	SetOutput(logPath)
	SetLevel(slog.LevelDebug)

	Debug("debug line")
	Info("info line")
	Warn("warn line")
	Error("error line")

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	s := string(content)
	for _, msg := range []string{"debug line", "info line", "warn line", "error line"} {
		if !strings.Contains(s, msg) {
			t.Errorf("expected log to contain %q", msg)
		}
	}
}

func TestReplaceAttr(t *testing.T) {
	attr := slog.String("attr", "value")
	result := replaceAttr(nil, attr)
	if result.Key != "attr" || result.Value.String() != "value" {
		t.Error("expected replaceAttr to pass through non-time attrs")
	}

	timeAttr := slog.Time(slog.TimeKey, time.Now())
	result = replaceAttr(nil, timeAttr)
	if result.Key != slog.TimeKey {
		t.Error("expected replaceAttr to preserve time key")
	}
}
