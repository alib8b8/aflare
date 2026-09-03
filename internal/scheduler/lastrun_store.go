// Copyright (c) 2026 aflare Contributors
//
// aflare​‌​​​​​​‌​​‌‌​​‌‌‌​​​​​‌​​​​‌‌​​‌​‌​​‌​​‌​​​​​‌​‌​​​​‌‌​​​​​‌‌​‌‌‌‌‌‌‌​‌​​​‌‌​‌‌​​​​‌​​‌​‌​​​‌​​‌‌​​​‌​​‌​‌​​‌‌​​​​​‌​​‌‌​‌​​​‌‌​‌​​‌‌‌‌‌‌​​‌​​​​‌‌​​‌‌​‌​‌‌​‌‌​​‌​​​​​‌‌​​‌‌‌​​‌​‌​‌​‌​‌‌‌‌‌​
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
	"time"

	"github.com/alib8b8/aflare/internal/fsutil"
	"github.com/alib8b8/aflare/internal/meta"
)

// LastRunFileName is the name of the file that stores the last-run
// timestamp of every persisted schedule.
const LastRunFileName = "lastrun.json"

// LastRunEntry is the persisted record of when a scheduled task last fired.
type LastRunEntry struct {
	TaskID  string    `json:"task_id"`
	LastRun time.Time `json:"last_run"`
}

// DefaultLastRunPath returns the default path to the last-run store file
// (DataDir/lastrun.json, i.e. ~/.aflare/lastrun.json).
func DefaultLastRunPath() string {
	return filepath.Join(meta.DataDir(), LastRunFileName)
}

// LoadLastRuns reads the last-run timestamps keyed by task ID. A missing
// file is not an error and yields an empty map. A corrupt file is moved
// aside (see fsutil.PreserveCorrupt) and reported as an error: silently
// discarding it would reset every task's misfire history.
func LoadLastRuns(path string) (map[string]time.Time, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is internally derived
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]time.Time{}, nil
		}
		return nil, fmt.Errorf("failed to read last-run file: %w", err)
	}

	if len(data) == 0 {
		return map[string]time.Time{}, nil
	}

	var entries []LastRunEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		if preserved := fsutil.PreserveCorrupt(path); preserved != "" {
			return nil, fmt.Errorf("failed to parse last-run file (corrupt copy preserved at %s): %w", preserved, err)
		}
		return nil, fmt.Errorf("failed to parse last-run file: %w", err)
	}

	runs := make(map[string]time.Time, len(entries))
	for _, e := range entries {
		// Empty IDs and zero timestamps carry no information and would
		// poison misfire counting (zero reads as "never fired, everything
		// since 1970 was missed").
		if e.TaskID == "" || e.LastRun.IsZero() {
			continue
		}
		runs[e.TaskID] = e.LastRun
	}
	return runs, nil
}

// SaveLastRuns atomically writes the last-run timestamps to path as JSON
// with 0600 permissions (temp file + rename: a crash mid-write must not
// destroy the misfire history of every task).
func SaveLastRuns(path string, runs map[string]time.Time) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create last-run directory: %w", err)
		}
	}

	entries := make([]LastRunEntry, 0, len(runs))
	for id, t := range runs {
		entries = append(entries, LastRunEntry{TaskID: id, LastRun: t})
	}
	sortLastRunEntries(entries)

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal last-run entries: %w", err)
	}
	if err := fsutil.WriteFileAtomic(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write last-run file: %w", err)
	}
	return nil
}

// sortLastRunEntries sorts entries by task ID (insertion sort — the map is
// small, and this matches the style of sortTasksByID in scheduler.go).
func sortLastRunEntries(entries []LastRunEntry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j-1].TaskID > entries[j].TaskID; j-- {
			entries[j-1], entries[j] = entries[j], entries[j-1]
		}
	}
}
