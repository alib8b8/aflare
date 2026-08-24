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

package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TriggerType represents the type of trigger that started the workflow
type TriggerType string

const (
	TriggerManual   TriggerType = "manual"
	TriggerCLI      TriggerType = "cli"
	TriggerAPI      TriggerType = "api"
	TriggerSchedule TriggerType = "schedule"
)

// Record represents a single workflow execution record
type Record struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Path        string        `json:"path,omitempty"`
	Trigger     TriggerType   `json:"trigger,omitempty"`
	User        string        `json:"user,omitempty"`
	Version     string        `json:"version,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	StartedAt   time.Time     `json:"started_at"`
	EndedAt     time.Time     `json:"ended_at,omitempty"`
	Success     bool          `json:"success"`
	Steps       []StepRecord  `json:"steps,omitempty"`
	FinalOutput string        `json:"final_output,omitempty"`
	Error       string        `json:"error,omitempty"`
}

// StepRecord represents a single step execution record
type StepRecord struct {
	Index      int           `json:"index"`
	Node       string        `json:"node"`
	Params     string        `json:"params,omitempty"`
	RetryCount int           `json:"retry_count,omitempty"`
	InputSize  int           `json:"input_size,omitempty"`
	OutputSize int           `json:"output_size,omitempty"`
	Duration   time.Duration `json:"duration"`
	Success    bool          `json:"success"`
	Error      string        `json:"error,omitempty"`
}

var (
	historyDir   string
	historyDirMu sync.RWMutex
)

func init() {
	home, err := os.UserHomeDir()
	if err == nil {
		historyDir = filepath.Join(home, ".config", "aflare", "history")
	}
}

// SetHistoryDir sets a custom history directory (useful for tests)
func SetHistoryDir(dir string) {
	historyDirMu.Lock()
	defer historyDirMu.Unlock()
	historyDir = dir
}

// getHistoryDir returns the current history directory under a read lock.
func getHistoryDir() string {
	historyDirMu.RLock()
	defer historyDirMu.RUnlock()
	return historyDir
}

// AuditLogPath returns the path of the append-only audit hash-chain log in
// the history directory, or "" when no history directory is configured.
// Read-only consumers (e.g. doctor) use this plus ReadAuditLogFile.
func AuditLogPath() string {
	dir := getHistoryDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, auditLogFileName)
}

// GetAuditLogPath returns the path to the audit log file in the current history
// directory. The directory may be empty if history is unavailable.
func GetAuditLogPath() string {
	dir := getHistoryDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, auditLogFileName)
}

// isValidRecordID ensures the ID is safe to use as a filename component.
// It rejects empty IDs, path separators, and dot-segments like "..".
func isValidRecordID(id string) bool {
	if id == "" || len(id) > 100 {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

var sensitiveKeywords = []string{"key", "token", "secret", "password"}

// SanitizeParams sanitizes parameter values to remove sensitive information.
// It replaces values of fields containing key/token/secret/password with ***.
func SanitizeParams(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}
	result := make(map[string]interface{}, len(params))
	for k, v := range params {
		if isSensitiveKey(k) {
			result[k] = "***"
		} else {
			switch val := v.(type) {
			case map[string]interface{}:
				result[k] = SanitizeParams(val)
			default:
				result[k] = v
			}
		}
	}
	return result
}

func isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, kw := range sensitiveKeywords {
		if strings.Contains(lowerKey, kw) {
			return true
		}
	}
	return false
}

// SummarizeParams creates a sanitized, truncated string summary of parameters.
// The summary is truncated to maxLen characters (200 by default if maxLen <= 0).
func SummarizeParams(params map[string]interface{}, maxLen int) string {
	if params == nil {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 200
	}
	sanitized := SanitizeParams(params)
	data, err := json.Marshal(sanitized)
	if err != nil {
		return ""
	}
	summary := string(data)
	if len(summary) > maxLen {
		summary = summary[:maxLen] + "..."
	}
	return summary
}

// AuditAction represents the type of action being audited
type AuditAction string

const (
	AuditActionLogin         AuditAction = "login"
	AuditActionLogout        AuditAction = "logout"
	AuditActionConfigChange  AuditAction = "config_change"
	AuditActionSensitiveOp   AuditAction = "sensitive_operation"
	AuditActionWorkflowStart AuditAction = "workflow_start"
	AuditActionWorkflowEnd   AuditAction = "workflow_end"
)

// AuditLog represents a single audit log entry
type AuditLog struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Action    AuditAction `json:"action"`
	User      string      `json:"user,omitempty"`
	Resource  string      `json:"resource,omitempty"`
	Detail    string      `json:"detail,omitempty"`
	Success   bool        `json:"success"`
	IP        string      `json:"ip,omitempty"`
	UserAgent string      `json:"user_agent,omitempty"`
	// PrevHash is the curr_hash of the previous record in the hash chain.
	// It is the 64-char hex zero hash for the first record.
	PrevHash string `json:"prev_hash,omitempty"`
	// CurrHash is the HMAC of (prev_hash || record_content) for this record.
	CurrHash string `json:"curr_hash,omitempty"`
	// MACAlgo names the HMAC algorithm used to compute CurrHash:
	// "sha256" (default; legacy records omit the field) or "sm3" for the
	// Chinese national cryptography suite. Verification recomputes each
	// record with its own algorithm, so mixed-algorithm chains stay valid.
	MACAlgo string `json:"mac_algo,omitempty"`
}

const auditLogFileName = "audit.log.jsonl"

// auditZeroHash is the 32-byte zero value encoded as 64 hex chars, used as the
// prev_hash of the first record in the chain.
const auditZeroHash = "0000000000000000000000000000000000000000000000000000000000000000"
