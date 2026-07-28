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

package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		historyDir = filepath.Join(home, ".config", "llm-box", "history")
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

// SaveRecord saves a workflow execution record to the history directory
func SaveRecord(record Record) error {
	dir := getHistoryDir()
	if dir == "" {
		return fmt.Errorf("history directory not available")
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	if record.ID == "" {
		record.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	filename := filepath.Join(dir, record.ID+".json")
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return nil
}

// ListRecords returns all history records, sorted by time (newest first)
func ListRecords() ([]Record, error) {
	dir := getHistoryDir()
	if dir == "" {
		return nil, fmt.Errorf("history directory not available")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil
		}
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	var records []Record
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path) // #nosec G304 -- internally generated history path
		if err != nil {
			continue
		}

		var record Record
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}
		records = append(records, record)
	}

	// Sort by started_at descending (newest first)
	for i := 0; i < len(records)-1; i++ {
		for j := i + 1; j < len(records); j++ {
			if records[j].StartedAt.After(records[i].StartedAt) {
				records[i], records[j] = records[j], records[i]
			}
		}
	}

	return records, nil
}

// GetRecord retrieves a single history record by ID
func GetRecord(id string) (*Record, error) {
	dir := getHistoryDir()
	if dir == "" {
		return nil, fmt.Errorf("history directory not available")
	}

	// Validate ID to prevent path traversal (e.g. id="../config")
	if !isValidRecordID(id) {
		return nil, fmt.Errorf("invalid record ID: %q", id)
	}

	path := filepath.Join(dir, id+".json")
	data, err := os.ReadFile(path) // #nosec G304 -- internally generated history path
	if err != nil {
		return nil, fmt.Errorf("failed to read record: %w", err)
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("failed to parse record: %w", err)
	}

	return &record, nil
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

// ClearHistory removes all history records
func ClearHistory() error {
	dir := getHistoryDir()
	if dir == "" {
		return fmt.Errorf("history directory not available")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read history directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}

	return nil
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
}

const auditLogFileName = "audit.log.jsonl"

// AppendAuditLog appends an audit log entry to the audit log file
func AppendAuditLog(log AuditLog) error {
	dir := getHistoryDir()
	if dir == "" {
		return fmt.Errorf("history directory not available")
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	if log.ID == "" {
		log.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal audit log: %w", err)
	}

	auditPath := filepath.Join(dir, auditLogFileName)
	f, err := os.OpenFile(auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // #nosec G304 -- internally generated history path
	if err != nil {
		return fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit log: %w", err)
	}

	return nil
}

// RecordFilter defines filters for listing records
type RecordFilter struct {
	StartTime *time.Time
	EndTime   *time.Time
	Success   *bool
	Workflow  string
}

// ListRecordsWithFilter returns history records filtered by the given criteria
func ListRecordsWithFilter(filter RecordFilter) ([]Record, error) {
	records, err := ListRecords()
	if err != nil {
		return nil, err
	}

	var filtered []Record
	for _, r := range records {
		if filter.StartTime != nil && r.StartedAt.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && r.StartedAt.After(*filter.EndTime) {
			continue
		}
		if filter.Success != nil && r.Success != *filter.Success {
			continue
		}
		if filter.Workflow != "" && !strings.Contains(strings.ToLower(r.Name), strings.ToLower(filter.Workflow)) {
			continue
		}
		filtered = append(filtered, r)
	}

	return filtered, nil
}

// Stats contains execution statistics
type Stats struct {
	TotalCount      int           `json:"total_count"`
	SuccessCount    int           `json:"success_count"`
	FailureCount    int           `json:"failure_count"`
	SuccessRate     float64       `json:"success_rate"`
	AverageDuration time.Duration `json:"average_duration"`
	Last24hCount    int           `json:"last_24h_count"`
}

// GetStats returns execution statistics based on all records
func GetStats() (Stats, error) {
	records, err := ListRecords()
	if err != nil {
		return Stats{}, err
	}

	stats := Stats{
		TotalCount: len(records),
	}

	if len(records) == 0 {
		return stats, nil
	}

	var totalDuration time.Duration
	now := time.Now()
	cutoff24h := now.Add(-24 * time.Hour)

	for _, r := range records {
		if r.Success {
			stats.SuccessCount++
		} else {
			stats.FailureCount++
		}
		if r.Duration > 0 {
			totalDuration += r.Duration
		} else if !r.EndedAt.IsZero() && !r.StartedAt.IsZero() {
			totalDuration += r.EndedAt.Sub(r.StartedAt)
		}
		if r.StartedAt.After(cutoff24h) {
			stats.Last24hCount++
		}
	}

	stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalCount)

	var countWithDuration int
	for _, r := range records {
		if r.Duration > 0 || (!r.EndedAt.IsZero() && !r.StartedAt.IsZero()) {
			countWithDuration++
		}
	}
	if countWithDuration > 0 {
		stats.AverageDuration = totalDuration / time.Duration(countWithDuration)
	}

	return stats, nil
}

// ReadAuditLogs reads all audit log entries
func ReadAuditLogs() ([]AuditLog, error) {
	dir := getHistoryDir()
	if dir == "" {
		return nil, fmt.Errorf("history directory not available")
	}

	auditPath := filepath.Join(dir, auditLogFileName)
	data, err := os.ReadFile(auditPath) // #nosec G304 -- internally generated history path
	if err != nil {
		if os.IsNotExist(err) {
			return []AuditLog{}, nil
		}
		return nil, fmt.Errorf("failed to read audit log: %w", err)
	}

	var logs []AuditLog
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var log AuditLog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			continue
		}
		logs = append(logs, log)
	}

	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Timestamp.After(logs[j].Timestamp)
	})

	return logs, nil
}
