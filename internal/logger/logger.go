// Copyright (c) 2026 llm-box Contributors
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
	"strings"
	"sync"
	"sync/atomic"
)

var (
	defaultLogger atomic.Pointer[slog.Logger]
	once          sync.Once
	currentLevel  = slog.LevelInfo
	currentFormat = "text"
	logFile       *os.File
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
	envLevel := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_BOX_LOG_LEVEL")))
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
	format := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_BOX_LOG_FORMAT")))
	if format == "json" || format == "text" {
		return format
	}
	return "text"
}

func getOutputFromEnv() string {
	output := strings.TrimSpace(os.Getenv("LLM_BOX_LOG_FILE"))
	if output != "" {
		return output
	}
	return "stderr"
}

func initLogger(level slog.Level, format string, output string) {
	currentLevel = level
	currentFormat = format

	var writer io.Writer
	if output == "stderr" || output == "" {
		writer = os.Stderr
	} else {
		dir := filepath.Dir(output)
		if dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0750)
		}
		f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open log file %s: %v, falling back to stderr\n", output, err)
			writer = os.Stderr
		} else {
			logFile = f
			writer = io.MultiWriter(os.Stderr, f)
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

func SetLevel(level slog.Level) {
	currentLevel = level
	reinitLogger()
}

func SetFormat(format string) {
	if format == "json" || format == "text" {
		currentFormat = format
		reinitLogger()
	}
}

func SetOutput(output string) {
	if logFile != nil {
		if err := logFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "logger: failed to close log file: %v\n", err)
		}
		logFile = nil
	}
	initLogger(currentLevel, currentFormat, output)
}

func reinitLogger() {
	output := "stderr"
	if logFile != nil {
		output = logFile.Name()
	}
	if logFile != nil {
		if err := logFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "logger: failed to close log file: %v\n", err)
		}
		logFile = nil
	}
	initLogger(currentLevel, currentFormat, output)
}

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

func Debug(msg string, args ...any) {
	logger().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	logger().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	logger().Warn(msg, args...)
}

func Error(msg string, args ...any) {
	logger().Error(msg, args...)
}

func With(args ...any) *slog.Logger {
	return logger().With(args...)
}

func Close() {
	if logFile != nil {
		if err := logFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "logger: failed to close log file: %v\n", err)
		}
		logFile = nil
	}
}
