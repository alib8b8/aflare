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
// The Reflection capability writes its logs to
// ~/.config/aflare/learning.json, accumulating across sessions.
//
// Key features:
//   - Deduplication: skips entries too similar to recent ones (Jaccard-based)
//   - Auto-compaction: periodically rewrites the log to remove stale entries

package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LearningEntry is a single learning record persisted to learning.json.
type LearningEntry struct {
	Timestamp  string   `json:"timestamp"`
	Capability string   `json:"capability"` // "reflection"
	Input      string   `json:"input,omitempty"`
	Issues     []string `json:"issues,omitempty"`
}

// learningStore persists learning entries to ~/.config/aflare/learning.json
// as JSON Lines (one JSON object per line). This is append-only and thread-safe.
type learningStore struct {
	mu            sync.Mutex
	path          string
	appendCount   int      // number of appends since last compaction
	recentKeys    []string // recent keys for dedup window
	maxRecentKeys int      // max size of dedup window
}

const (
	maxAppendsBeforeCompact  = 100 // compact after this many appends
	maxEntriesKept           = 500 // keep at most this many entries
	dedupWindowSize          = 50  // check last N entries for duplicates
	dedupSimilarityThreshold = 0.7 // Jaccard similarity threshold for dedup
)

var sharedLearning = &learningStore{
	maxRecentKeys: dedupWindowSize,
	appendCount:   0,
}

// ensurePathLocked ensures the config directory exists and sets s.path.
// The caller MUST hold s.mu: the path field is mutated here and read under
// the same lock by append/compact/loadEntries, so all path access is
// serialized (fixing the race where loadEntries read sharedLearning.path
// without the lock while append wrote it under the lock).
func (s *learningStore) ensurePathLocked() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".config", "aflare")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("[learning] failed to create config dir: dir=%s err=%v", dir, err)
	}
	s.path = filepath.Join(dir, "learning.json")
}

// appendLearning appends a learning entry to the learning.json file.
// It performs deduplication: if a very similar entry was recently appended,
// the new entry is skipped to avoid log bloat.
func (s *learningStore) append(entry LearningEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		s.ensurePathLocked()
	}

	// Build a dedup key for this entry.
	entryKey := s.buildEntryKey(entry)

	// Check if this entry is too similar to recent entries.
	if s.isDuplicate(entryKey) {
		return // skip duplicate
	}

	// Track this key for future dedup.
	s.recentKeys = append(s.recentKeys, entryKey)
	if len(s.recentKeys) > s.maxRecentKeys {
		s.recentKeys = s.recentKeys[len(s.recentKeys)-s.maxRecentKeys:]
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

	n, err := f.Write(data)
	if err != nil || n == 0 {
		if closeErr := f.Close(); closeErr != nil {
			log.Printf("[learning] close after write failure: %v", closeErr)
		}
		return
	}
	if closeErr := f.Close(); closeErr != nil {
		log.Printf("[learning] close failed: %v", closeErr)
		return
	}

	s.appendCount++

	// Periodically compact the learning log.
	if s.appendCount >= maxAppendsBeforeCompact {
		s.compact()
		s.appendCount = 0
	}
}

// buildEntryKey creates a normalized key for dedup comparison.
func (s *learningStore) buildEntryKey(entry LearningEntry) string {
	var parts []string
	if entry.Input != "" {
		parts = append(parts, entry.Input)
	}
	for _, issue := range entry.Issues {
		parts = append(parts, issue)
	}
	return strings.ToLower(strings.Join(parts, " | "))
}

// isDuplicate checks if the given key is too similar to any recent key.
// Uses Jaccard similarity on word sets for fuzzy matching.
func (s *learningStore) isDuplicate(key string) bool {
	keyWords := toWordSet(key)
	if len(keyWords) == 0 {
		return false
	}

	for _, recentKey := range s.recentKeys {
		recentWords := toWordSet(recentKey)
		if len(recentWords) == 0 {
			continue
		}
		similarity := jaccardSimilarity(keyWords, recentWords)
		if similarity >= dedupSimilarityThreshold {
			return true
		}
	}
	return false
}

// compact rewrites the learning log to keep only the most recent entries.
// Old entries beyond maxEntriesKept are removed. Duplicates are also pruned.
func (s *learningStore) compact() {
	entries := s.readAllEntries()
	if len(entries) <= maxEntriesKept {
		return
	}

	// Keep only the most recent entries.
	kept := entries
	if len(entries) > maxEntriesKept {
		kept = entries[len(entries)-maxEntriesKept:]
	}

	// Rewrite the file.
	f, err := os.Create(s.path)
	if err != nil {
		return
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, e := range kept {
		if err := enc.Encode(e); err != nil {
			return
		}
	}
	if err := f.Sync(); err != nil {
		log.Printf("[learning] failed to sync learning store file: path=%s err=%v", s.path, err)
	}
}

// readAllEntries reads all entries from the learning log (for compaction).
func (s *learningStore) readAllEntries() []LearningEntry {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil
	}

	var entries []LearningEntry
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
		entries = append(entries, entry)
	}
	return entries
}

// ── Helper functions ──────────────────────────────────────────────────────

// toWordSet converts a string to a set of words for fuzzy matching.
func toWordSet(s string) map[string]struct{} {
	words := tokenize(s)
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		set[w] = struct{}{}
	}
	return set
}

// jaccardSimilarity computes the Jaccard similarity between two word sets.
func jaccardSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	intersection := 0
	for w := range a {
		if _, ok := b[w]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// ── Public API (used by capabilities) ─────────────────────────────────────

// appendReflection writes a reflection learning entry with dedup.
func appendReflection(input string, issues []string) {
	sharedLearning.append(LearningEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Capability: "reflection",
		Input:      truncateStr(input, 200),
		Issues:     issues,
	})
}

// loadEntries reads all learning entries from learning.json. All path and
// file access is performed under sharedLearning.mu so it cannot race with
// concurrent append/compact (which mutate the same file and path field
// under the same lock).
func loadEntries() (reflection []LearningEntry) {
	sharedLearning.mu.Lock()
	defer sharedLearning.mu.Unlock()

	sharedLearning.ensurePathLocked()

	data, err := os.ReadFile(sharedLearning.path)
	if err != nil {
		return nil
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
		if entry.Capability == "reflection" {
			reflection = append(reflection, entry)
		}
	}
	return reflection
}

// loadRecentReflectionIssues loads the most recent reflection issues from
// the learning journal, up to maxEntries. Returns a list of issue descriptions.
func loadRecentReflectionIssues(maxEntries int) []string {
	entries := loadEntries()
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
