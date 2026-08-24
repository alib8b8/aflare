// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌‌​​‌‌‌‌​‌‌‌​‌​​‌‌‌‌​‌‌​‌​‌‌‌​​‌‌‌​​​​‌​​‌​​‌​​​​​​​​​​​​​​​​​‌‌‌‌​​​​‌​‌​‌‌‌‌⁠
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

package mcp

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// toolCallTimeout is the maximum duration allowed for a single tool call.
const toolCallTimeout = 30 * time.Second

// sanitizeError removes potentially sensitive information from error messages.
func sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// Remove file paths that may contain home directories
	if home, _ := os.UserHomeDir(); home != "" {
		msg = strings.ReplaceAll(msg, home, "~")
	}
	// Redact tokens/keys if they appear in the message
	sensitivePatterns := []string{"token", "key", "secret", "password", "credential"}
	lowerMsg := strings.ToLower(msg)
	for _, p := range sensitivePatterns {
		if strings.Contains(lowerMsg, p) {
			return fmt.Errorf("tool execution failed (sensitive details redacted)")
		}
	}
	return fmt.Errorf("%s", msg)
}

// requireString validates that args contains a non-empty string for the given key.
func requireString(args map[string]interface{}, key string) (string, error) {
	v, ok := args[key].(string)
	if !ok || strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("parameter %q is required and must be a non-empty string", key)
	}
	return v, nil
}

// optionalString returns the string value for key, or empty string if missing/invalid.
func optionalString(args map[string]interface{}, key string) string {
	v, ok := args[key].(string)
	if !ok {
		return ""
	}
	return v
}

// optionalBool returns the bool value for key, or the default if missing/invalid.
func optionalBool(args map[string]interface{}, key string, def bool) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		if strings.ToLower(v) == "true" {
			return true
		} else if strings.ToLower(v) == "false" {
			return false
		}
	}
	return def
}

// optionalInt returns the int value for key, or the default if missing/invalid.
func optionalInt(args map[string]interface{}, key string, def int) int {
	switch v := args[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return def
}
