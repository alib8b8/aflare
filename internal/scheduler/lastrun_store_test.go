// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌​​​​​​​​‌‌​​‌​​​‌‌​​‌‌‌‌​​​‌‌​‌​‌​‌​‌​‌‌​‌​‌‌‌​‌​​‌​‌​‌‌‌‌​‌‌‌​​​​​​​​​​​​​​​​​​‌​‌​‌‌​‌​​​‌‌‌‌‌⁠
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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLastRunStore_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), LastRunFileName)

	t1 := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 2, 9, 30, 0, 0, time.UTC)
	runs := map[string]time.Time{
		"nightly-report": t1,
		"morning-digest": t2,
	}
	if err := SaveLastRuns(path, runs); err != nil {
		t.Fatalf("SaveLastRuns: %v", err)
	}

	loaded, err := LoadLastRuns(path)
	if err != nil {
		t.Fatalf("LoadLastRuns: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded))
	}
	for id, want := range runs {
		got, ok := loaded[id]
		if !ok {
			t.Errorf("task %q missing after round-trip", id)
			continue
		}
		if !got.Equal(want) {
			t.Errorf("task %q: expected %v, got %v", id, want, got)
		}
	}
}

func TestLastRunStore_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), LastRunFileName)
	loaded, err := LoadLastRuns(path)
	if err != nil {
		t.Fatalf("missing file must not be an error: %v", err)
	}
	if loaded == nil || len(loaded) != 0 {
		t.Fatalf("expected empty non-nil map, got %v", loaded)
	}
}

// TestLastRunStore_CorruptFilePreserved verifies that a truncated/corrupt
// last-run file is moved aside instead of being overwritten or silently
// discarded — it is the misfire history of every scheduled task.
func TestLastRunStore_CorruptFilePreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), LastRunFileName)
	if err := os.WriteFile(path, []byte(`[{"task_id":"x","last_run":"2026-09-0`), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadLastRuns(path)
	if err == nil {
		t.Fatal("expected parse error for corrupt file")
	}
	if !strings.Contains(err.Error(), "corrupt copy preserved") {
		t.Errorf("error should mention preserved copy: %v", err)
	}

	matches, globErr := filepath.Glob(path + ".corrupt-*")
	if globErr != nil || len(matches) != 1 {
		t.Fatalf("expected one preserved .corrupt-* file, got %v (err=%v)", matches, globErr)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("corrupt original should have been moved aside, stat err=%v", err)
	}
}

// TestLastRunStore_JunkEntriesSkipped verifies that empty task IDs and zero
// timestamps are dropped on load — a zero last-run would read as "never
// fired" and fabricate a misfire count from 1970.
func TestLastRunStore_JunkEntriesSkipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), LastRunFileName)
	valid := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	if err := SaveLastRuns(path, map[string]time.Time{
		"good":  valid,
		"":      valid,
		"never": time.Time{},
	}); err != nil {
		t.Fatalf("SaveLastRuns: %v", err)
	}

	loaded, err := LoadLastRuns(path)
	if err != nil {
		t.Fatalf("LoadLastRuns: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected only the good entry to survive, got %d: %v", len(loaded), loaded)
	}
	if got := loaded["good"]; !got.Equal(valid) {
		t.Errorf("good entry mangled: %v", got)
	}
}

// TestLastRunStore_EmptyMapWritesCleanFile covers the save-side of the
// filtering: an empty map must round-trip to an empty map, not an error.
func TestLastRunStore_EmptyMapWritesCleanFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", LastRunFileName)
	if err := SaveLastRuns(path, map[string]time.Time{}); err != nil {
		t.Fatalf("SaveLastRuns(empty): %v", err)
	}
	loaded, err := LoadLastRuns(path)
	if err != nil {
		t.Fatalf("LoadLastRuns: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected empty map, got %v", loaded)
	}
}
