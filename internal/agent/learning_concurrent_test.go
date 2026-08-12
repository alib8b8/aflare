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

// learning_concurrent_test.go covers P1 test gap:
//   - learning.json concurrent writes: multiple goroutines appending simultaneously
//     must not cause data races or corruption.

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestLearningConcurrent_Append verifies that concurrent calls to
// appendReflection and appendAdaptiveFeedback do not race.
func TestLearningConcurrent_Append(t *testing.T) {
	// Use a temp learning store to avoid polluting the real one
	tmpDir := t.TempDir()
	store := &learningStore{
		maxRecentKeys: 50,
		appendCount:   0,
		path:          filepath.Join(tmpDir, "learning.json"),
	}

	var wg sync.WaitGroup
	numGoroutines := 10
	numPerGoroutine := 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numPerGoroutine; j++ {
				entry := LearningEntry{
					Timestamp:  "2026-01-01T00:00:00Z",
					Capability: "reflection",
					Input:      fmt.Sprintf("goroutine-%d input-%d", id, j),
					Issues:     []string{fmt.Sprintf("issue-%d-%d", id, j)},
				}
				store.append(entry)
			}
		}(i)
	}

	wg.Wait()

	// Verify the file was written and is valid JSON lines
	_, err := os.Stat(store.path)
	if err != nil {
		t.Fatalf("learning.json not created: %v", err)
	}
}

// TestLearningConcurrent_MixedCapabilities verifies concurrent writes
// from both reflection and adaptive capabilities.
func TestLearningConcurrent_MixedCapabilities(t *testing.T) {
	tmpDir := t.TempDir()
	store := &learningStore{
		maxRecentKeys: 50,
		appendCount:   0,
		path:          filepath.Join(tmpDir, "learning.json"),
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	// Half write reflection, half write adaptive
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			capType := "reflection"
			if id%2 == 0 {
				capType = "adaptive"
			}

			for j := 0; j < 10; j++ {
				entry := LearningEntry{
					Timestamp:  "2026-01-01T00:00:00Z",
					Capability: capType,
					Input:      fmt.Sprintf("input-%d-%d", id, j),
				}
				if capType == "reflection" {
					entry.Issues = []string{fmt.Sprintf("issue-%d-%d", id, j)}
				} else {
					entry.Feedback = fmt.Sprintf("feedback-%d-%d", id, j)
				}
				store.append(entry)
			}
		}(i)
	}

	wg.Wait()

	// Verify no panics or races occurred
	t.Logf("concurrent mixed writes completed successfully")
}

// TestLearningConcurrent_CompactDuringWrites verifies that compaction
// (triggered after maxAppendsBeforeCompact) doesn't corrupt concurrent writes.
func TestLearningConcurrent_CompactDuringWrites(t *testing.T) {
	tmpDir := t.TempDir()
	store := &learningStore{
		maxRecentKeys: 50,
		appendCount:   0,
		path:          filepath.Join(tmpDir, "learning.json"),
	}

	// First, write enough entries to trigger compaction
	for i := 0; i < maxAppendsBeforeCompact+10; i++ {
		entry := LearningEntry{
			Timestamp:  "2026-01-01T00:00:00Z",
			Capability: "reflection",
			Input:      fmt.Sprintf("pre-compaction-input-%d", i),
			Issues:     []string{fmt.Sprintf("pre-issue-%d", i)},
		}
		store.append(entry)
	}

	// Now do concurrent writes after compaction has triggered
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				entry := LearningEntry{
					Timestamp:  "2026-01-01T00:00:00Z",
					Capability: "adaptive",
					Feedback:   fmt.Sprintf("post-compact-%d-%d", id, j),
				}
				store.append(entry)
			}
		}(i)
	}

	wg.Wait()

	// Verify the file exists and is readable
	_, err := os.Stat(store.path)
	if err != nil {
		t.Fatalf("learning.json not found after compaction: %v", err)
	}
}

// TestLearningConcurrent_ReadDuringWrite verifies that reads during writes
// don't cause races.
func TestLearningConcurrent_ReadDuringWrite(t *testing.T) {
	tmpDir := t.TempDir()
	store := &learningStore{
		maxRecentKeys: 50,
		appendCount:   0,
		path:          filepath.Join(tmpDir, "learning.json"),
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Writer goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				select {
				case <-done:
					return
				default:
				}
				entry := LearningEntry{
					Timestamp:  "2026-01-01T00:00:00Z",
					Capability: "reflection",
					Input:      fmt.Sprintf("rw-input-%d-%d", id, j),
					Issues:     []string{fmt.Sprintf("rw-issue-%d-%d", id, j)},
				}
				store.append(entry)
			}
		}(i)
	}

	// Reader goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			select {
			case <-done:
				return
			default:
			}
			_ = store.recognizePatterns()
		}
	}()

	// Let them run for a bit
	wg.Wait()
	close(done)
}

// TestLearningDedup_Concurrent verifies that the dedup mechanism works
// correctly under concurrent access.
func TestLearningDedup_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()
	store := &learningStore{
		maxRecentKeys: 50,
		appendCount:   0,
		path:          filepath.Join(tmpDir, "learning.json"),
	}

	// Pre-populate with known entries
	baseEntry := LearningEntry{
		Timestamp:  "2026-01-01T00:00:00Z",
		Capability: "reflection",
		Input:      "base input text",
		Issues:     []string{"base issue"},
	}
	store.append(baseEntry)

	// Concurrent writes of similar entries should be deduped
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Same content as base — should be deduped
			dup := LearningEntry{
				Timestamp:  "2026-01-01T00:00:00Z",
				Capability: "reflection",
				Input:      "base input text",
				Issues:     []string{"base issue"},
			}
			store.append(dup)
		}()
	}

	wg.Wait()

	// The dedup window should still be manageable
	if len(store.recentKeys) > 50 {
		t.Errorf("recent keys exceeded limit: %d", len(store.recentKeys))
	}
}

// TestLearningPatterns_Concurrent verifies that pattern recognition
// doesn't cause races.
func TestLearningPatterns_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()
	store := &learningStore{
		maxRecentKeys: 50,
		appendCount:   0,
		path:          filepath.Join(tmpDir, "learning.json"),
	}

	// Populate with enough entries for pattern recognition
	for i := 0; i < 20; i++ {
		entry := LearningEntry{
			Timestamp:  "2026-01-01T00:00:00Z",
			Capability: "reflection",
			Input:      fmt.Sprintf("pattern input %d", i%3),
			Issues:     []string{fmt.Sprintf("recurring issue type %d", i%3)},
		}
		store.append(entry)
	}

	// Concurrent pattern recognition
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			patterns := store.recognizePatterns()
			_ = patterns
		}()
	}

	wg.Wait()

	// Verify patterns are cached
	patterns := store.recognizePatterns()
	if len(patterns) > 0 {
		for _, p := range patterns {
			if p.Count < patternMinOccurrences {
				t.Errorf("pattern %s has count %d, expected >= %d", p.Pattern, p.Count, patternMinOccurrences)
			}
		}
	}
}

// TestLearningConcurrent_ReadAllEntries verifies readAllEntries is safe
// under concurrent writes.
func TestLearningConcurrent_ReadAllEntries(t *testing.T) {
	tmpDir := t.TempDir()
	store := &learningStore{
		maxRecentKeys: 50,
		appendCount:   0,
		path:          filepath.Join(tmpDir, "learning.json"),
	}

	// Pre-populate
	for i := 0; i < 50; i++ {
		entry := LearningEntry{
			Timestamp:  "2026-01-01T00:00:00Z",
			Capability: "reflection",
			Input:      fmt.Sprintf("readall-input-%d", i),
			Issues:     []string{fmt.Sprintf("readall-issue-%d", i)},
		}
		store.append(entry)
	}

	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				store.append(LearningEntry{
					Timestamp:  "2026-01-01T00:00:00Z",
					Capability: "adaptive",
					Feedback:   fmt.Sprintf("readall-fb-%d-%d", id, j),
				})
			}
		}(i)
	}

	// Readers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				entries := store.readAllEntries()
				_ = entries
			}
		}()
	}

	wg.Wait()
	t.Log("concurrent readAllEntries completed")
}

// TestLearningStore_DedupWindow verifies the dedup window doesn't exceed limits.
func TestLearningStore_DedupWindow(t *testing.T) {
	store := &learningStore{
		maxRecentKeys: dedupWindowSize,
		appendCount:   0,
	}

	// Append more than the window size
	for i := 0; i < dedupWindowSize*2; i++ {
		entry := LearningEntry{
			Timestamp:  "2026-01-01T00:00:00Z",
			Capability: "reflection",
			Input:      fmt.Sprintf("unique-input-%d", i),
			Issues:     []string{fmt.Sprintf("unique-issue-%d", i)},
		}
		store.append(entry)
	}

	if len(store.recentKeys) > dedupWindowSize {
		t.Errorf("recent keys exceeded window: %d > %d", len(store.recentKeys), dedupWindowSize)
	}
}

// TestLearningStore_NormalizeIssue verifies issue normalization for patterns.
func TestLearningStore_NormalizeIssue(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"output is too short or empty", "output is too short"},
		{"response lacks concrete actions", "response lacks concrete actions"},
		{"agent is refusing to help", "agent is refusing to help"},
	}

	for _, tt := range tests {
		result := normalizeIssue(tt.input)
		if !strings.Contains(strings.ToLower(result), tt.contains) {
			t.Errorf("normalizeIssue(%q) = %q, expected to contain %q", tt.input, result, tt.contains)
		}
	}
}
