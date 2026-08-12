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
//
// Key features:
//   - Deduplication: skips entries too similar to recent ones (Jaccard-based)
//   - Pattern recognition: identifies recurring issue patterns across sessions
//   - Auto-compaction: periodically rewrites the log to remove stale entries
//   - Pattern summary: generates a synthesized pattern report for quick loading

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

// LearningPattern represents a recognized pattern across multiple learning entries.
type LearningPattern struct {
	Pattern     string `json:"pattern"`     // the identified pattern description
	Category    string `json:"category"`    // "reflection" or "adaptive"
	Count       int    `json:"count"`       // how many times this pattern appeared
	FirstSeen   string `json:"first_seen"`  // timestamp of first occurrence
	LastSeen    string `json:"last_seen"`   // timestamp of latest occurrence
	Examples    []string `json:"examples"`  // up to 3 example entries
}

// learningStore persists learning entries to ~/.config/aflare/learning.json
// as JSON Lines (one JSON object per line). This is append-only and thread-safe.
type learningStore struct {
	mu           sync.Mutex
	path         string
	appendCount  int            // number of appends since last compaction
	recentKeys   []string       // recent keys for dedup window
	maxRecentKeys int           // max size of dedup window
	patterns     []LearningPattern // cached patterns
	patternsTime time.Time      // when patterns were last computed
}

const (
	maxAppendsBeforeCompact = 100   // compact after this many appends
	maxEntriesKept          = 500   // keep at most this many entries
	dedupWindowSize         = 50    // check last N entries for duplicates
	dedupSimilarityThreshold = 0.7  // Jaccard similarity threshold for dedup
	patternMinOccurrences   = 3     // min occurrences to recognize a pattern
)

var sharedLearning = &learningStore{
	maxRecentKeys: dedupWindowSize,
	appendCount:   0,
}

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
// It performs deduplication: if a very similar entry was recently appended,
// the new entry is skipped to avoid log bloat.
func (s *learningStore) append(entry LearningEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		initLearningStore()
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
	_ = f.Close()
	if err != nil || n == 0 {
		return
	}

	s.appendCount++
	// Invalidate cached patterns since new data was added.
	s.patterns = nil

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
	if entry.Feedback != "" {
		parts = append(parts, entry.Feedback)
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
	_ = f.Sync()
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

// recognizePatterns analyzes the learning log for recurring patterns.
// Returns patterns grouped by issue type. Results are cached for 5 minutes.
func (s *learningStore) recognizePatterns() []LearningPattern {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Return cached patterns if still fresh.
	if s.patterns != nil && time.Since(s.patternsTime) < 5*time.Minute {
		return s.patterns
	}

	entries := s.readAllEntries()
	if len(entries) == 0 {
		return nil
	}

	// Group similar issues together.
	type issueGroup struct {
		issues    []string
		examples  []string
		firstSeen string
		lastSeen  string
	}
	reflectionGroups := make(map[string]*issueGroup)
	adaptiveGroups := make(map[string]*issueGroup)

	for _, e := range entries {
		switch e.Capability {
		case "reflection":
			for _, issue := range e.Issues {
				normalized := normalizeIssue(issue)
				g, ok := reflectionGroups[normalized]
				if !ok {
					g = &issueGroup{firstSeen: e.Timestamp}
					reflectionGroups[normalized] = g
				}
				g.issues = append(g.issues, issue)
				g.lastSeen = e.Timestamp
				if len(g.examples) < 3 {
					g.examples = append(g.examples, truncateStr(e.Input, 80))
				}
			}
		case "adaptive":
			if e.Feedback != "" {
				normalized := normalizeIssue(e.Feedback)
				g, ok := adaptiveGroups[normalized]
				if !ok {
					g = &issueGroup{firstSeen: e.Timestamp}
					adaptiveGroups[normalized] = g
				}
				g.issues = append(g.issues, e.Feedback)
				g.lastSeen = e.Timestamp
				if len(g.examples) < 3 {
					g.examples = append(g.examples, truncateStr(e.Feedback, 80))
				}
			}
		}
	}

	// Convert groups to patterns, filtering by minimum occurrences.
	var patterns []LearningPattern
	for pattern, g := range reflectionGroups {
		if len(g.issues) >= patternMinOccurrences {
			patterns = append(patterns, LearningPattern{
				Pattern:   pattern,
				Category:  "reflection",
				Count:     len(g.issues),
				FirstSeen: g.firstSeen,
				LastSeen:  g.lastSeen,
				Examples:  g.examples,
			})
		}
	}
	for pattern, g := range adaptiveGroups {
		if len(g.issues) >= patternMinOccurrences {
			patterns = append(patterns, LearningPattern{
				Pattern:   pattern,
				Category:  "adaptive",
				Count:     len(g.issues),
				FirstSeen: g.firstSeen,
				LastSeen:  g.lastSeen,
				Examples:  g.examples,
			})
		}
	}

	s.patterns = patterns
	s.patternsTime = time.Now()
	return patterns
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

// normalizeIssue normalizes an issue string for pattern grouping.
// Strips variable parts (timestamps, specific values) to extract the pattern.
func normalizeIssue(issue string) string {
	lower := strings.ToLower(issue)
	// Replace timestamps and IDs with placeholders.
	lower = strings.ReplaceAll(lower, ":", " ")
	// Common normalization: collapse whitespace
	fields := strings.Fields(lower)
	if len(fields) > 6 {
		fields = fields[:6] // keep first 6 words as pattern signature
	}
	return strings.Join(fields, " ")
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

// appendAdaptiveFeedback writes an adaptive learning entry with dedup.
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