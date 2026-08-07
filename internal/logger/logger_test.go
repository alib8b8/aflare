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
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
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
			t.Setenv("AFLARE_LOG_LEVEL", tt.env)
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
			t.Setenv("AFLARE_LOG_FORMAT", tt.env)
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

	t.Setenv("AFLARE_LOG_FILE", "/tmp/test.log")
	got := getOutputFromEnv()
	if got != "/tmp/test.log" {
		t.Errorf("getOutputFromEnv() = %v, want %v", got, "/tmp/test.log")
	}

	t.Setenv("AFLARE_LOG_FILE", "")
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
		if tt.output != "stderr" && rotWriter == nil {
			t.Error("expected rotWriter to be set for file output")
		}
	}
}

func TestInitLoggerInvalidFile(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	initLogger(slog.LevelInfo, "text", "/dev/null/invalid/file.log")
	if rotWriter != nil {
		t.Error("expected rotWriter to be nil after fallback to stderr")
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
	if rotWriter != nil {
		t.Error("expected rotWriter to be nil when output is stderr")
	}

	SetOutput(logPath)
	SetOutput("")
	if rotWriter != nil {
		t.Error("expected rotWriter to be nil when output is empty")
	}
}

func TestClose(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "close.log")

	SetOutput(logPath)
	Close()

	if rotWriter != nil {
		t.Error("expected rotWriter to be nil after Close")
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

func TestGetLogMaxMB(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	t.Setenv("AFLARE_LOG_MAX_MB", "")
	if got := getLogMaxMB(); got != int64(defaultLogMaxMB)*1024*1024 {
		t.Errorf("default maxSize = %d, want %d", got, int64(defaultLogMaxMB)*1024*1024)
	}

	t.Setenv("AFLARE_LOG_MAX_MB", "0")
	if got := getLogMaxMB(); got != 0 {
		t.Errorf("maxSize with 0 = %d, want 0 (rotation disabled)", got)
	}

	t.Setenv("AFLARE_LOG_MAX_MB", "2")
	if got := getLogMaxMB(); got != 2*1024*1024 {
		t.Errorf("maxSize with 2 = %d, want %d", got, 2*1024*1024)
	}

	t.Setenv("AFLARE_LOG_MAX_MB", "not-a-number")
	if got := getLogMaxMB(); got != int64(defaultLogMaxMB)*1024*1024 {
		t.Errorf("invalid maxSize = %d, want default %d", got, int64(defaultLogMaxMB)*1024*1024)
	}

	t.Setenv("AFLARE_LOG_MAX_MB", "-5")
	if got := getLogMaxMB(); got != int64(defaultLogMaxMB)*1024*1024 {
		t.Errorf("negative maxSize = %d, want default %d", got, int64(defaultLogMaxMB)*1024*1024)
	}
}

func TestGetLogMaxBackups(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	t.Setenv("AFLARE_LOG_MAX_BACKUPS", "")
	if got := getLogMaxBackups(); got != defaultLogMaxBackups {
		t.Errorf("default maxBackups = %d, want %d", got, defaultLogMaxBackups)
	}

	t.Setenv("AFLARE_LOG_MAX_BACKUPS", "5")
	if got := getLogMaxBackups(); got != 5 {
		t.Errorf("maxBackups with 5 = %d, want 5", got)
	}

	t.Setenv("AFLARE_LOG_MAX_BACKUPS", "0")
	if got := getLogMaxBackups(); got != 0 {
		t.Errorf("maxBackups with 0 = %d, want 0", got)
	}

	t.Setenv("AFLARE_LOG_MAX_BACKUPS", "garbage")
	if got := getLogMaxBackups(); got != defaultLogMaxBackups {
		t.Errorf("invalid maxBackups = %d, want default %d", got, defaultLogMaxBackups)
	}
}

// TestRotatingWriterRotation verifies that exceeding maxSize triggers a
// rotation, that the old file is moved to .1, and that repeated rotations
// shift backups up to maxBackups and drop the oldest.
func TestRotatingWriterRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "app.log")

	rw, err := newRotatingWriter(logPath, 100, 3)
	if err != nil {
		t.Fatalf("newRotatingWriter failed: %v", err)
	}
	defer rw.Close()

	// First write: fits within maxSize, no rotation yet.
	if _, err := rw.Write([]byte(strings.Repeat("a", 50))); err != nil {
		t.Fatalf("write 1 failed: %v", err)
	}
	if _, err := os.Stat(logPath + ".1"); !os.IsNotExist(err) {
		t.Errorf("expected no backup yet, got err=%v", err)
	}

	// Second write: pushes past 100 bytes -> rotation.
	if _, err := rw.Write([]byte(strings.Repeat("b", 60))); err != nil {
		t.Fatalf("write 2 failed: %v", err)
	}
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Errorf("expected .1 backup to exist after rotation: %v", err)
	}
	content, err := os.ReadFile(logPath + ".1")
	if err != nil {
		t.Fatalf("read .1 failed: %v", err)
	}
	if string(content) != strings.Repeat("a", 50) {
		t.Errorf("backup .1 content = %q, want 50 'a's", string(content))
	}

	// Current file should hold only the second write.
	content, err = os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read current failed: %v", err)
	}
	if string(content) != strings.Repeat("b", 60) {
		t.Errorf("current content = %q, want 60 'b's", string(content))
	}

	// Two more rotations to fill up the 3 backups.
	for k := 0; k < 2; k++ {
		// Force a rotation by writing more than maxSize.
		if _, err := rw.Write([]byte(strings.Repeat("c", 110))); err != nil {
			t.Fatalf("write loop %d failed: %v", k, err)
		}
	}

	for i := 1; i <= 3; i++ {
		if _, err := os.Stat(logPath + "." + strconv.Itoa(i)); err != nil {
			t.Errorf("expected %s.%d to exist: %v", logPath, i, err)
		}
	}
	// .4 must not exist (maxBackups=3).
	if _, err := os.Stat(logPath + ".4"); !os.IsNotExist(err) {
		t.Errorf("expected .4 to be pruned, got err=%v", err)
	}
}

// TestRotatingWriterMaxBackupsZero verifies that maxBackups=0 prunes all
// backups and only ever keeps the current file.
func TestRotatingWriterMaxBackupsZero(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "rotate0.log")

	rw, err := newRotatingWriter(logPath, 20, 0)
	if err != nil {
		t.Fatalf("newRotatingWriter failed: %v", err)
	}
	defer rw.Close()

	if _, err := rw.Write([]byte(strings.Repeat("x", 10))); err != nil {
		t.Fatalf("write 1 failed: %v", err)
	}
	if _, err := rw.Write([]byte(strings.Repeat("y", 15))); err != nil {
		t.Fatalf("write 2 failed: %v", err)
	}
	if _, err := os.Stat(logPath + ".1"); !os.IsNotExist(err) {
		t.Errorf("expected no .1 backup with maxBackups=0, got err=%v", err)
	}
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read current failed: %v", err)
	}
	if string(content) != strings.Repeat("y", 15) {
		t.Errorf("current content = %q, want 15 'y's", string(content))
	}
}

// TestRotatingWriterThroughInitLogger exercises the full initLogger path with
// a small AFLARE_LOG_MAX_MB so rotation happens end-to-end.
func TestRotatingWriterThroughInitLogger(t *testing.T) {
	origLevel, origFormat, origLogger := saveState()
	defer restoreState(origLevel, origFormat, origLogger)

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "init_rotate.log")

	t.Setenv("AFLARE_LOG_MAX_MB", "1")
	t.Setenv("AFLARE_LOG_MAX_BACKUPS", "2")

	initLogger(slog.LevelInfo, "text", logPath)
	if rotWriter == nil {
		t.Fatal("expected rotWriter to be set")
	}

	// Write ~1.2 MB in chunks. Rotation should trigger once we cross 1 MB.
	big := strings.Repeat("z", 64*1024)
	for i := 0; i < 19; i++ {
		Info("chunk", "n", i, "payload", big)
	}

	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Errorf("expected .1 backup to exist after exceeding 1 MB: %v", err)
	}
}
