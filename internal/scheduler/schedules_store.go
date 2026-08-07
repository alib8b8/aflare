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

package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alib8b8/aflare/internal/meta"
)

// SchedulesFileName is the name of the file that stores persisted schedules.
const SchedulesFileName = "schedules.json"

// ScheduleEntry is the persisted representation of a scheduled task.
// It does not carry the runtime TaskFunc; that is rebuilt from WorkflowPath
// when the scheduler starts.
type ScheduleEntry struct {
	ID           string `json:"id"`
	Cron         string `json:"cron"`
	WorkflowPath string `json:"workflow_path"`
}

// DefaultSchedulesPath returns the default path to the schedules store file
// (DataDir/schedules.json, i.e. ~/.aflare/schedules.json).
func DefaultSchedulesPath() string {
	return filepath.Join(meta.DataDir(), SchedulesFileName)
}

// SaveSchedules writes the given schedule entries to path as JSON with 0600
// permissions. The parent directory is created with 0755 if missing.
func SaveSchedules(path string, entries []ScheduleEntry) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create schedules directory: %w", err)
		}
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schedules: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write schedules file: %w", err)
	}
	return nil
}

// LoadSchedules reads the schedule entries from path. A missing file is not
// an error and yields an empty (non-nil) slice.
func LoadSchedules(path string) ([]ScheduleEntry, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is internally derived
	if err != nil {
		if os.IsNotExist(err) {
			return []ScheduleEntry{}, nil
		}
		return nil, fmt.Errorf("failed to read schedules file: %w", err)
	}

	if len(data) == 0 {
		return []ScheduleEntry{}, nil
	}

	var entries []ScheduleEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse schedules file: %w", err)
	}
	if entries == nil {
		entries = []ScheduleEntry{}
	}
	return entries, nil
}
