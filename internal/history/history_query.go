// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌‌‌​​‌​​‌​‌​‌​‌​​​​‌​​​​​​‌‌‌‌​‌​​‌‌‌​​‌‌​‌​​‌​​​‌‌‌‌​‌​‌‌​‌‌​​​​​​​​​​​​​​​​​​​​‌‌​​‌​​​‌​​‌‌⁠
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
	"time"
)

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
