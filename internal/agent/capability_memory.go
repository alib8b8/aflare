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

// capability_memory.go implements MemoryCapability — cross-session persistent
// memory with file-backed key-value storage, preference extraction, and
// semantic recall.
//
// This implements the "有状态 Agent" type from the taxonomy:
//   Maintains long-term memory across sessions, remembering user preferences,
//   past decisions, and contextual facts. Persisted to ~/.config/aflare/memory.json
//   as JSON Lines for durability.

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryEntry is a single memory record persisted to disk.
type MemoryEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Category  string `json:"category"`  // "preference", "fact", "decision", "context"
	Timestamp string `json:"timestamp"`
	AccessCount int  `json:"access_count"`
}

// MemoryCapability provides cross-session long-term memory with persistence.
type MemoryCapability struct {
	mu      sync.RWMutex
	entries map[string]*MemoryEntry // key → entry
	store   *memoryStore
}

// memoryStore handles file I/O for memory persistence.
type memoryStore struct {
	mu   sync.Mutex
	path string
}

var sharedMemoryStore = &memoryStore{}

func initMemoryStore() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".config", "aflare")
	_ = os.MkdirAll(dir, 0o755)
	sharedMemoryStore.path = filepath.Join(dir, "memory.json")
}

func (s *memoryStore) save(entries map[string]*MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		initMemoryStore()
	}

	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return f.Sync()
}

func (s *memoryStore) load() (map[string]*MemoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		initMemoryStore()
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*MemoryEntry), nil
		}
		return nil, err
	}

	entries := make(map[string]*MemoryEntry)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e MemoryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		entries[e.Key] = &e
	}
	return entries, nil
}

func NewMemoryCapability() *MemoryCapability {
	return &MemoryCapability{
		entries: make(map[string]*MemoryEntry),
		store:   sharedMemoryStore,
	}
}

func (m *MemoryCapability) Name() string       { return "memory" }
func (m *MemoryCapability) Description() string { return "Cross-session memory: remembers preferences and history across sessions (有状态 Agent)" }

func (m *MemoryCapability) Init(loop *AgentLoop) error {
	entries, err := m.store.load()
	if err != nil {
		return fmt.Errorf("memory load failed: %w", err)
	}
	m.entries = entries
	return nil
}

func (m *MemoryCapability) PreProcess(ctx context.Context, input string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.entries) == 0 {
		return "", nil
	}

	// Find relevant memories for this input.
	relevant := m.searchRelevant(input, 5)
	if len(relevant) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("\n[Long-Term Memory — relevant past context]\n")
	for _, e := range relevant {
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", e.Category, e.Key, e.Value))
		e.AccessCount++
	}
	sb.WriteString("Use this context when responding.\n")
	return input + sb.String(), nil
}

func (m *MemoryCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Extract user preferences: "I prefer X", "always do Y", "never do Z"
	m.extractPreference(input, output)

	// Extract factual statements: "my name is X", "I work at Y"
	m.extractFact(input)

	// Extract decisions: "let's use X", "we'll go with Y"
	m.extractDecision(input, output)

	// Persist if we have new entries.
	if len(m.entries) > 0 {
		_ = m.store.save(m.entries)
	}

	return "", nil
}

func (m *MemoryCapability) Shutdown() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.entries) > 0 {
		return m.store.save(m.entries)
	}
	return nil
}

// searchRelevant finds memories that are semantically related to the input.
// Uses simple keyword overlap for now; future: embedding-based similarity.
func (m *MemoryCapability) searchRelevant(input string, maxResults int) []*MemoryEntry {
	words := tokenize(strings.ToLower(input))

	type scored struct {
		entry *MemoryEntry
		score int
	}
	var candidates []scored

	for _, e := range m.entries {
		score := 0
		keyWords := tokenize(strings.ToLower(e.Key))
		valueWords := tokenize(strings.ToLower(e.Value))

		for _, w := range words {
			for _, kw := range keyWords {
				if w == kw {
					score += 3
				} else if len(w) > 3 && strings.Contains(kw, w) {
					score += 1
				}
			}
			for _, vw := range valueWords {
				if w == vw {
					score += 2
				} else if len(w) > 3 && strings.Contains(vw, w) {
					score += 1
				}
			}
		}
		// Boost frequently accessed memories.
		score += e.AccessCount / 2

		if score > 0 {
			candidates = append(candidates, scored{e, score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > maxResults {
		candidates = candidates[:maxResults]
	}

	result := make([]*MemoryEntry, len(candidates))
	for i, c := range candidates {
		result[i] = c.entry
	}
	return result
}

// extractPreference detects user preferences from the conversation.
func (m *MemoryCapability) extractPreference(input, output string) {
	lower := strings.ToLower(input)
	patterns := []struct {
		prefix string
		category string
	}{
		{"i prefer", "preference"},
		{"i like", "preference"},
		{"i don't like", "preference"},
		{"always", "preference"},
		{"never", "preference"},
		{"my name is", "fact"},
		{"i work at", "fact"},
		{"i work for", "fact"},
		{"i am a", "fact"},
		{"i'm a", "fact"},
		{"use", "decision"},
		{"go with", "decision"},
		{"choose", "decision"},
	}

	for _, p := range patterns {
		if strings.Contains(lower, p.prefix) {
			key := fmt.Sprintf("user_%s_%d", p.category, len(m.entries))
			value := truncateStr(input, 120)
			// Avoid duplicates: check if a similar entry exists.
			if m.hasSimilarEntry(p.category, value) {
				continue
			}
			m.entries[key] = &MemoryEntry{
				Key:       key,
				Value:     value,
				Category:  p.category,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			return // Only store one per turn.
		}
	}
}

// extractFact detects factual statements.
func (m *MemoryCapability) extractFact(input string) {
	lower := strings.ToLower(input)
	factPatterns := []string{
		"my name is", "i am", "i'm", "i work", "i live in",
		"my email", "my phone", "i use", "i have",
	}

	for _, p := range factPatterns {
		if strings.Contains(lower, p) {
			key := fmt.Sprintf("fact_%s_%d", sanitizeKey(p), len(m.entries))
			value := truncateStr(input, 120)
			if m.hasSimilarEntry("fact", value) {
				continue
			}
			m.entries[key] = &MemoryEntry{
				Key:       key,
				Value:     value,
				Category:  "fact",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			return
		}
	}
}

// extractDecision detects decision statements.
func (m *MemoryCapability) extractDecision(input, output string) {
	lower := strings.ToLower(output)
	decisionPatterns := []string{
		"let's use", "we'll go with", "i'll use", "i choose",
		"best option is", "recommend", "go with",
	}

	for _, p := range decisionPatterns {
		if strings.Contains(lower, p) {
			key := fmt.Sprintf("decision_%s_%d", sanitizeKey(p), len(m.entries))
			value := truncateStr(output, 120)
			if m.hasSimilarEntry("decision", value) {
				continue
			}
			m.entries[key] = &MemoryEntry{
				Key:       key,
				Value:     value,
				Category:  "decision",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			return
		}
	}
}

// hasSimilarEntry checks if a similar entry already exists to avoid duplicates.
func (m *MemoryCapability) hasSimilarEntry(category, value string) bool {
	for _, e := range m.entries {
		if e.Category == category && strings.Contains(e.Value, value[:min(len(value), 30)]) {
			return true
		}
	}
	return false
}

// sanitizeKey converts a string to a safe key fragment.
func sanitizeKey(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "'", "")
	return s
}

// tokenize splits text into lowercase words.
func tokenize(text string) []string {
	var words []string
	for _, w := range strings.Fields(text) {
		w = strings.Trim(w, ".,!?;:()[]{}'\"")
		if len(w) > 1 {
			words = append(words, w)
		}
	}
	return words
}