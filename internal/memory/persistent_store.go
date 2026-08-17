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

// persistent_store.go provides a shared persistent key-value store for
// cross-session memory. Both MemoryNode and MemoryCapability use this
// store to read/write persisted memory entries, ensuring the two systems
// share the same data.

package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PersistentMemoryEntry is a lightweight memory record persisted to disk.
// This is the shared format used by both MemoryNode and MemoryCapability.
type PersistentMemoryEntry struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Category    string `json:"category"` // "preference", "fact", "decision", "context", "general"
	Timestamp   string `json:"timestamp"`
	AccessCount int    `json:"access_count"`
}

// PersistentMemoryStore provides file-backed persistent memory storage.
// Both MemoryNode and MemoryCapability share a single store instance.
type PersistentMemoryStore struct {
	mu      sync.RWMutex
	entries map[string]*PersistentMemoryEntry
	path    string
}

var sharedPersistentStore = &PersistentMemoryStore{}

func init() {
	initPersistentStore()
}

func initPersistentStore() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".config", "aflare")
	_ = os.MkdirAll(dir, 0o755) // best-effort init
	sharedPersistentStore.path = filepath.Join(dir, "memory.json")
	sharedPersistentStore.entries = make(map[string]*PersistentMemoryEntry)
	sharedPersistentStore.load()
}

// GetPersistentStore returns the shared persistent memory store.
func GetPersistentStore() *PersistentMemoryStore {
	return sharedPersistentStore
}

// load reads persisted entries from disk.
func (s *PersistentMemoryStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		return
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.entries = make(map[string]*PersistentMemoryEntry)
			return
		}
		return
	}

	entries := make(map[string]*PersistentMemoryEntry)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e PersistentMemoryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		entries[e.Key] = &e
	}
	s.entries = entries
}

// Store persists a memory entry to disk.
func (s *PersistentMemoryStore) Store(key, value, category string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.entries[key]
	if ok {
		existing.Value = value
		existing.Category = category
		existing.Timestamp = time.Now().UTC().Format(time.RFC3339)
	} else {
		s.entries[key] = &PersistentMemoryEntry{
			Key:       key,
			Value:     value,
			Category:  category,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
	}

	// Save immediately to keep in sync.
	return s.saveUnlocked()
}

// Retrieve gets a persisted memory entry by key.
func (s *PersistentMemoryStore) Retrieve(key string) (*PersistentMemoryEntry, error) {
	// Write lock: Retrieve bumps AccessCount, and a read lock would let
	// concurrent Retrieves race on the shared entry pointer.
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok {
		return nil, fmt.Errorf("persistent memory not found: %s", key)
	}
	entry.AccessCount++
	return entry, nil
}

// FenceValue renders a recalled memory value for prompt injection so stored
// content cannot forge the surrounding prompt structure: whitespace that
// could break line boundaries collapses to spaces, backticks are neutralized,
// and the value is wrapped in a backtick fence. Memories derive from user
// input persisted across sessions — treat them as untrusted at re-injection
// time.
func FenceValue(v string) string {
	v = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, v)
	v = strings.ReplaceAll(v, "`", "'")
	return "`" + v + "`"
}

// Delete removes a persisted memory entry.
func (s *PersistentMemoryStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.entries[key]; !ok {
		return fmt.Errorf("persistent memory not found: %s", key)
	}
	delete(s.entries, key)
	return s.saveUnlocked()
}

// Search finds persisted entries matching a query by keyword overlap.
func (s *PersistentMemoryStore) Search(query string, maxResults int) []*PersistentMemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type scored struct {
		entry *PersistentMemoryEntry
		score int
	}
	var candidates []scored

	queryWords := strings.Fields(strings.ToLower(query))
	for _, e := range s.entries {
		score := 0
		keyWords := strings.Fields(strings.ToLower(e.Key))
		valueWords := strings.Fields(strings.ToLower(e.Value))

		for _, qw := range queryWords {
			for _, kw := range keyWords {
				if qw == kw {
					score += 3
				} else if len(qw) > 3 && strings.Contains(kw, qw) {
					score += 1
				}
			}
			for _, vw := range valueWords {
				if qw == vw {
					score += 2
				} else if len(qw) > 3 && strings.Contains(vw, qw) {
					score += 1
				}
			}
		}
		score += e.AccessCount / 2

		if score > 0 {
			candidates = append(candidates, scored{e, score})
		}
	}

	// Sort by score descending
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	if len(candidates) > maxResults {
		candidates = candidates[:maxResults]
	}

	result := make([]*PersistentMemoryEntry, len(candidates))
	for i, c := range candidates {
		result[i] = c.entry
	}
	return result
}

// ListAll returns all persisted entries.
func (s *PersistentMemoryStore) ListAll() []*PersistentMemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*PersistentMemoryEntry, 0, len(s.entries))
	for _, e := range s.entries {
		result = append(result, e)
	}
	return result
}

// saveUnlocked writes entries to disk (caller must hold mu write lock).
func (s *PersistentMemoryStore) saveUnlocked() error {
	if s.path == "" {
		return nil
	}

	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, e := range s.entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return f.Sync()
}
