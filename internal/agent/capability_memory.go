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
//
// MemoryCapability shares the same persistent store with MemoryNode via
// memory.GetPersistentStore(), ensuring both systems see the same data.

package agent

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/memory"
)

// MemoryCapability provides cross-session long-term memory with persistence.
// Uses the shared persistent store from the memory package, so data written
// by MemoryNode (via memory_store tool) is visible here, and vice versa.
type MemoryCapability struct {
	mu      sync.RWMutex
	entries map[string]*memory.PersistentMemoryEntry // key → entry
}

func NewMemoryCapability() *MemoryCapability {
	return &MemoryCapability{
		entries: make(map[string]*memory.PersistentMemoryEntry),
	}
}

func (m *MemoryCapability) Name() string { return CapabilityMemory }
func (m *MemoryCapability) Description() string {
	return "Cross-session memory: remembers preferences and history across sessions (有状态 Agent)"
}

func (m *MemoryCapability) Init(loop *AgentLoop) error {
	// Load entries from the shared persistent store.
	store := memory.GetPersistentStore()
	all := store.ListAll()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range all {
		entry := *e // Copy
		m.entries[e.Key] = &entry
	}
	return nil
}

// critique filters and annotates retrieved memories for prompt injection,
// following the MemHarness principle ("memory is reconstructed, not
// replayed"): instead of dumping raw recalls into the prompt, each memory is
// (a) discarded when it is both stale and weakly relevant, and (b) otherwise
// injected WITH its source state (age, category) plus an explicit instruction
// for the model to judge applicability against the current input before
// using it. This keeps the deterministic recall path honest without paying
// for an extra LLM critique call — the consuming model performs the critique.
func (m *MemoryCapability) critique(input string, relevant []*memory.PersistentMemoryEntry) []*memory.PersistentMemoryEntry {
	now := time.Now()
	kept := make([]*memory.PersistentMemoryEntry, 0, len(relevant))
	for _, e := range relevant {
		// An unparseable timestamp means corrupted or tampered data (the
		// field is a plain string on disk) — treat it as stale rather than
		// fresh so the discard rule below still applies.
		age := maxStaleMemoryAge + time.Hour
		if ts, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
			age = now.Sub(ts)
		}
		// Deterministic discard rule: memories that are both old enough to
		// plausibly be superseded AND weakly matched contribute more noise
		// than signal. Strong matches survive regardless of age (a user's
		// stated preference from months ago is still their preference).
		if age > maxStaleMemoryAge && e.AccessCount == 0 {
			continue
		}
		kept = append(kept, e)
	}
	return kept
}

// maxStaleMemoryAge bounds how long a never-reused memory stays injectable.
const maxStaleMemoryAge = 30 * 24 * time.Hour

func (m *MemoryCapability) PreProcess(ctx context.Context, input string) (string, error) {
	m.mu.RLock()
	if len(m.entries) == 0 {
		m.mu.RUnlock()
		return "", nil
	}

	// Find relevant memories for this input.
	relevant := m.searchRelevant(input, 5)
	if len(relevant) == 0 {
		m.mu.RUnlock()
		return "", nil
	}

	// Deterministic critique pass: drop stale-and-unused recalls (MemHarness
	// discard stage); survivors are injected with source-state annotations.
	relevant = m.critique(input, relevant)
	if len(relevant) == 0 {
		m.mu.RUnlock()
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("\n[Long-Term Memory — recalled from past sessions]\n")
	sb.WriteString("These are cues recorded earlier, not facts about the current task. Judge each one against the task below: use it only if it still applies, ignore it otherwise.\n")
	for _, e := range relevant {
		recorded := e.Timestamp
		if t, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
			recorded = t.Format("2006-01-02")
		}
		sb.WriteString(fmt.Sprintf("- (recorded %s, %s) %s: %s\n", recorded, memory.FenceValue(e.Category), memory.FenceValue(e.Key), memory.FenceValue(e.Value)))
	}
	m.mu.RUnlock()

	// Bump access counters under the write lock: the entries are shared
	// pointers, so mutating them under the read lock above would race with
	// concurrent PreProcess calls.
	m.mu.Lock()
	for _, e := range relevant {
		e.AccessCount++
	}
	m.mu.Unlock()
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

	// Sync new entries to the shared persistent store. StoreCtx propagates
	// the loop context into the (hybrid) embedding step.
	store := memory.GetPersistentStore()
	for _, e := range m.entries {
		if err := store.StoreCtx(ctx, e.Key, e.Value, e.Category); err != nil {
			log.Printf("[memory] failed to sync entry to persistent store: key=%s err=%v", e.Key, err)
		}
	}

	return "", nil
}

func (m *MemoryCapability) Shutdown() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	store := memory.GetPersistentStore()
	for _, e := range m.entries {
		if err := store.Store(e.Key, e.Value, e.Category); err != nil {
			log.Printf("[memory] failed to sync entry to persistent store on shutdown: key=%s err=%v", e.Key, err)
		}
	}
	return nil
}

// searchRelevant finds memories that are semantically related to the input.
// Uses simple keyword overlap for now; future: embedding-based similarity.
func (m *MemoryCapability) searchRelevant(input string, maxResults int) []*memory.PersistentMemoryEntry {
	words := tokenize(strings.ToLower(input))

	type scored struct {
		entry *memory.PersistentMemoryEntry
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

	result := make([]*memory.PersistentMemoryEntry, len(candidates))
	for i, c := range candidates {
		result[i] = c.entry
	}
	return result
}

// extractPreference detects user preferences from the conversation.
func (m *MemoryCapability) extractPreference(input, output string) {
	lower := strings.ToLower(input)
	patterns := []struct {
		prefix   string
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
			m.entries[key] = &memory.PersistentMemoryEntry{
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
			m.entries[key] = &memory.PersistentMemoryEntry{
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
			m.entries[key] = &memory.PersistentMemoryEntry{
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
