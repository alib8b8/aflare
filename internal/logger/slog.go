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
	"strings"
)

// SlogAdapter wraps a *slog.Logger and provides an object-oriented logging
// interface with pre-configured structured fields. It is designed to be
// passed around to components that need contextual logging (e.g. workflow
// name, step index, node type).
type SlogAdapter struct {
	logger *slog.Logger
}

// NewSlogAdapter creates a new SlogAdapter that wraps the current default
// logger. The adapter inherits all existing handler configuration (format,
// level, output, rotation, redaction).
func NewSlogAdapter() *SlogAdapter {
	return &SlogAdapter{logger: logger()}
}

// With returns a new SlogAdapter with the given key-value pairs added as
// structured fields. The original adapter is not modified.
func (a *SlogAdapter) With(args ...any) *SlogAdapter {
	return &SlogAdapter{logger: a.logger.With(args...)}
}

// WithWorkflow is a convenience method that adds workflow_name, step_index,
// and node_type as structured fields. It returns a new SlogAdapter.
func (a *SlogAdapter) WithWorkflow(name string, stepIndex int, nodeType string) *SlogAdapter {
	return a.With("workflow_name", name, "step_index", stepIndex, "node_type", nodeType)
}

// Debug logs at debug level.
func (a *SlogAdapter) Debug(msg string, args ...any) {
	a.logger.Debug(msg, args...)
}

// Info logs at info level.
func (a *SlogAdapter) Info(msg string, args ...any) {
	a.logger.Info(msg, args...)
}

// Warn logs at warn level.
func (a *SlogAdapter) Warn(msg string, args ...any) {
	a.logger.Warn(msg, args...)
}

// Error logs at error level.
func (a *SlogAdapter) Error(msg string, args ...any) {
	a.logger.Error(msg, args...)
}

// InitSlog initializes the slog logger from environment variables. It reads
// LOG_FORMAT (json/text) and LOG_LEVEL (debug/info/warn/error). If those are
// not set, it falls back to AFLARE_LOG_FORMAT and AFLARE_LOG_LEVEL. This is
// safe to call multiple times to reconfigure the logger at runtime.
func InitSlog() {
	level := getLevelFromInit()
	format := getFormatFromInit()
	output := getOutputFromEnv()
	initLogger(level, format, output)
}

// getLevelFromInit reads LOG_LEVEL first, falling back to AFLARE_LOG_LEVEL.
func getLevelFromInit() slog.Level {
	envLevel := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL")))
	if envLevel == "" {
		envLevel = strings.ToLower(strings.TrimSpace(os.Getenv("AFLARE_LOG_LEVEL")))
	}
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

// getFormatFromInit reads LOG_FORMAT first, falling back to AFLARE_LOG_FORMAT.
func getFormatFromInit() string {
	format := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))
	if format == "" {
		format = strings.ToLower(strings.TrimSpace(os.Getenv("AFLARE_LOG_FORMAT")))
	}
	if format == "json" || format == "text" {
		return format
	}
	return "text"
}
