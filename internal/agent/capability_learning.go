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

// capability_learning.go provides cross-session learning persistence.
// Reflection and Adaptive capabilities write their logs to
// ~/.config/aflare/learning.json, accumulating across sessions.

package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LearningEntry is a single learning record persisted to learning.json.
type LearningEntry struct {
	Timestamp  string   `json:"timestamp"`
	Capability string   `json:"capability"` // "reflection" or "adaptive"
	Input      string   `json:"input,omitempty"`
	Issues     []string `json:"issues,omitempty"`
	Feedback   string   `json:"feedback,omitempty"`
	Output     string   `json:"output,omitempty"`
}

// learningStore persists learning entries to ~/.config/aflare/learning.json
// as JSON Lines (one JSON object per line). This is append-only and thread-safe.
type learningStore struct {
	mu   sync.Mutex
	path string
}

var sharedLearning = &learningStore{}

// initLearningStore ensures the config directory exists and sets the path.
func initLearningStore() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".config", "aflare")
	_ = os.MkdirAll(dir, 0o755)
	sharedLearning.path = filepath.Join(dir, "learning.json")
}

// appendLearning appends a learning entry to the learning.json file.
func (s *learningStore) append(entry LearningEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		initLearningStore()
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()

	_ = f.Sync() // ensure write hits disk
}

// appendReflection writes a reflection learning entry.
func appendReflection(input string, issues []string) {
	sharedLearning.append(LearningEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Capability: "reflection",
		Input:      truncateStr(input, 200),
		Issues:     issues,
	})
}

// appendAdaptiveFeedback writes an adaptive learning entry.
func appendAdaptiveFeedback(feedback string) {
	sharedLearning.append(LearningEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Capability: "adaptive",
		Feedback:   feedback,
	})
}

// loadEntries reads all learning entries from learning.json and returns
// them grouped by capability type.
func loadEntries() (reflection []LearningEntry, adaptive []LearningEntry) {
	initLearningStore()

	data, err := os.ReadFile(sharedLearning.path)
	if err != nil {
		return nil, nil
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry LearningEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		switch entry.Capability {
		case "reflection":
			reflection = append(reflection, entry)
		case "adaptive":
			adaptive = append(adaptive, entry)
		}
	}
	return reflection, adaptive
}

// loadRecentReflectionIssues loads the most recent reflection issues from
// the learning journal, up to maxEntries. Returns a list of issue descriptions.
func loadRecentReflectionIssues(maxEntries int) []string {
	entries, _ := loadEntries()
	if len(entries) == 0 {
		return nil
	}

	// Take the most recent entries.
	start := 0
	if len(entries) > maxEntries {
		start = len(entries) - maxEntries
	}

	var issues []string
	for _, e := range entries[start:] {
		for _, issue := range e.Issues {
			issues = append(issues, fmt.Sprintf("[%s] %s: %s", e.Timestamp, truncateStr(e.Input, 50), issue))
		}
	}
	return issues
}

// loadRecentAdaptiveFeedback loads the most recent adaptive feedback entries,
// up to maxEntries.
func loadRecentAdaptiveFeedback(maxEntries int) []string {
	_, entries := loadEntries()
	if len(entries) == 0 {
		return nil
	}

	start := 0
	if len(entries) > maxEntries {
		start = len(entries) - maxEntries
	}

	var feedback []string
	for _, e := range entries[start:] {
		if e.Feedback != "" {
			feedback = append(feedback, e.Feedback)
		}
	}
	return feedback
}