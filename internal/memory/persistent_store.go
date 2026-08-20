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
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
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

	// keyTokens/valueTokens cache the lowercased word frequencies of
	// Key/Value for Search. Unexported, so never serialized; rebuilt
	// whenever the entry is created or its text updated (under the store
	// write lock, read under the read lock).
	keyTokens   map[string]int
	valueTokens map[string]int
}

// tokenize returns the lowercased word-frequency map of s.
func tokenize(s string) map[string]int {
	fields := strings.Fields(strings.ToLower(s))
	if len(fields) == 0 {
		return nil
	}
	counts := make(map[string]int, len(fields))
	for _, f := range fields {
		counts[f]++
	}
	return counts
}

// rebuildTokens recomputes the search token caches. Caller must hold the
// store write lock (or be constructing the entry).
func (e *PersistentMemoryEntry) rebuildTokens() {
	e.keyTokens = tokenize(e.Key)
	e.valueTokens = tokenize(e.Value)
}

// Hybrid retrieval tuning constants (P1-5).
const (
	// hybridKeywordScale is the keyword-score saturation point: a keyword
	// score of 6 maps to 0.5 on the [0,1] hybrid scale. Keyword scores are
	// unbounded ints (exact key match=3, value match=2, substring=1, plus
	// AccessCount/2) while vector cosine similarity is already [0,1], so
	// keyword scores are squashed through kw/(kw+scale) before merging.
	// The transform is strictly monotonic, so keyword-only ranking order
	// is preserved when no embedder is configured.
	hybridKeywordScale = 6.0

	// hybridAgreementBonus rewards entries confirmed by both retrieval
	// signals: combined = max(kw, vec) + bonus*min(kw, vec).
	hybridAgreementBonus = 0.2

	// hybridMinVectorScore is the noise floor for vector contributions;
	// cosine similarities below it are treated as no signal. With the
	// default HashEmbedder a single shared token scores ~0.4, so the
	// floor suppresses hash-collision noise while keeping lexical
	// near-matches. Real embedding models typically score ≥0.5 for
	// related text, so the floor costs little semantic recall.
	hybridMinVectorScore = 0.35
)

// PersistentMemoryStore provides file-backed persistent memory storage.
// Both MemoryNode and MemoryCapability share a single store instance.
//
// P1-5: Search is hybrid — entries are matched by keyword overlap AND by
// cosine similarity over embeddings when an embedder is configured. The
// store defaults to a HashEmbedder (offline, deterministic) so the hybrid
// path is always active; install an HTTPEmbedder via SetEmbedder for
// semantic retrieval, then call ReindexVectors to re-embed existing
// entries with the new embedder. The vector index is kept in sync with
// entries under the store write lock and is never serialized to disk:
// vectors are recomputed at load time (ReindexVectors) and on every Store.
type PersistentMemoryStore struct {
	mu       sync.RWMutex
	entries  map[string]*PersistentMemoryEntry
	path     string
	embedder Embedder
	vectors  *VectorIndex
}

var sharedPersistentStore *PersistentMemoryStore

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
	sharedPersistentStore = newPersistentMemoryStore(filepath.Join(dir, "memory.json"))
}

// newPersistentMemoryStore returns a store backed by path. It installs
// the default HashEmbedder, loads persisted entries, and back-fills
// their embeddings so hybrid search covers them immediately.
func newPersistentMemoryStore(path string) *PersistentMemoryStore {
	s := &PersistentMemoryStore{
		entries:  make(map[string]*PersistentMemoryEntry),
		path:     path,
		embedder: NewHashEmbedder(128),
		vectors:  NewVectorIndex(),
	}
	s.load()
	_ = s.ReindexVectors(context.Background())
	return s
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
		e.rebuildTokens()
		entries[e.Key] = &e
	}
	s.entries = entries
}

// SetEmbedder installs the embedder used for the vector half of hybrid
// search. If nil is passed the store falls back to keyword-only ranking.
// Existing vectors are NOT recomputed; call ReindexVectors afterwards so
// all entries are embedded with the new embedder (dimensions from a
// previous embedder yield zero cosine similarity, i.e. the vector path
// degrades to no signal until reindexed).
func (s *PersistentMemoryStore) SetEmbedder(e Embedder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.embedder = e
}

// GetEmbedder returns the currently configured embedder (may be nil).
func (s *PersistentMemoryStore) GetEmbedder() Embedder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.embedder
}

// ReindexVectors (re)embeds every existing entry using the current
// embedder. If no embedder is set this is a no-op. This is the
// recommended path after SetEmbedder, and is used at load time to
// back-fill embeddings for entries restored from disk.
func (s *PersistentMemoryStore) ReindexVectors(ctx context.Context) error {
	s.mu.RLock()
	e := s.embedder
	entriesCopy := make([]*PersistentMemoryEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		entriesCopy = append(entriesCopy, entry)
	}
	s.mu.RUnlock()

	if e == nil {
		return nil
	}
	for _, entry := range entriesCopy {
		v, err := e.Embed(ctx, entry.Key+" "+entry.Value)
		if err != nil || len(v) == 0 {
			continue
		}
		s.vectors.Add(entry.Key, v, map[string]string{
			"category": entry.Category,
		})
	}
	return nil
}

// Store persists a memory entry to disk.
func (s *PersistentMemoryStore) Store(key, value, category string) error {
	return s.StoreCtx(context.Background(), key, value, category)
}

// StoreCtx is the context-aware variant of Store. It computes the entry
// embedding (if an embedder is configured) using ctx, allowing callers
// to time-bound or cancel the embedding HTTP call. The embedding is
// computed BEFORE acquiring the write lock so a slow embedder doesn't
// block other readers. On embedding error the entry is still stored and
// remains retrievable by keyword search; any stale vector from a
// previous version of the entry is dropped so the vector index never
// serves an outdated embedding.
func (s *PersistentMemoryStore) StoreCtx(ctx context.Context, key, value, category string) error {
	s.mu.RLock()
	e := s.embedder
	s.mu.RUnlock()

	var vec Vector
	if e != nil {
		if v, err := e.Embed(ctx, key+" "+value); err == nil && len(v) > 0 {
			vec = v
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.entries[key]
	if ok {
		existing.Value = value
		existing.Category = category
		existing.Timestamp = time.Now().UTC().Format(time.RFC3339)
		existing.rebuildTokens()
	} else {
		e := &PersistentMemoryEntry{
			Key:       key,
			Value:     value,
			Category:  category,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		e.rebuildTokens()
		s.entries[key] = e
	}

	// Keep the vector index in sync with the entry (same lock ordering
	// as Delete/SearchCtx: s.mu → vectors.mu, never the reverse).
	if len(vec) > 0 {
		s.vectors.Add(key, vec, map[string]string{
			"category": category,
		})
	} else {
		s.vectors.Remove(key)
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
	s.vectors.Remove(key)
	return s.saveUnlocked()
}

// Search finds persisted entries matching a query using hybrid retrieval.
func (s *PersistentMemoryStore) Search(query string, maxResults int) []*PersistentMemoryEntry {
	return s.SearchCtx(context.Background(), query, maxResults)
}

// SearchCtx is the context-aware variant of Search; the context is
// honoured when computing the query embedding (e.g. an HTTPEmbedder).
//
// P1-5 hybrid retrieval: entries are matched by BOTH keyword overlap and
// vector cosine similarity, then ranked on a merged score. Keyword
// scoring is unchanged from the historical behaviour: per query word, an
// exact key-word match scores 3, an exact value-word match scores 2, and
// a substring match (query word longer than 3 chars contained in a longer
// token) scores 1, with repeated tokens counted per occurrence; access
// frequency adds AccessCount/2. The int keyword score is squashed to
// [0,1] via kw/(kw+hybridKeywordScale) and merged with the cosine
// similarity as max + agreementBonus*min, so an entry confirmed by both
// signals outranks one confirmed by either alone. Entries are included
// when either signal fires, so recall is never lower than keyword-only
// search. With no embedder configured (SetEmbedder(nil)) this degrades
// to the legacy keyword ranking — the saturation transform is monotonic,
// so ordering is identical.
//
// Determinism: ties are broken by key, not map iteration order.
func (s *PersistentMemoryStore) SearchCtx(ctx context.Context, query string, maxResults int) []*PersistentMemoryEntry {
	if maxResults <= 0 {
		return nil
	}
	searchStart := time.Now()
	mode := "keyword" // flipped to "hybrid" when the query embeds successfully

	// Phase 1: keyword scoring over an entry snapshot (read lock).
	s.mu.RLock()
	queryWords := strings.Fields(strings.ToLower(query))
	entries := make([]*PersistentMemoryEntry, 0, len(s.entries))
	kwScores := make(map[string]int, len(s.entries))
	for _, e := range s.entries {
		entries = append(entries, e)
		score := 0
		for _, qw := range queryWords {
			for tok, n := range e.keyTokens {
				if tok == qw {
					score += 3 * n
				} else if len(qw) > 3 && strings.Contains(tok, qw) {
					score += 1 * n
				}
			}
			for tok, n := range e.valueTokens {
				if tok == qw {
					score += 2 * n
				} else if len(qw) > 3 && strings.Contains(tok, qw) {
					score += 1 * n
				}
			}
		}
		score += e.AccessCount / 2
		if score > 0 {
			kwScores[e.Key] = score
		}
	}
	embedder := s.embedder
	s.mu.RUnlock()

	// Phase 2: embed the query outside the store lock (may do network
	// I/O), then score stored vectors via the index's own lock.
	var vecScores map[string]float64
	if embedder != nil {
		if qv, err := embedder.Embed(ctx, query); err == nil && len(qv) > 0 {
			mode = "hybrid"
			hits := s.vectors.Search(qv, len(entries), hybridMinVectorScore)
			if len(hits) > 0 {
				vecScores = make(map[string]float64, len(hits))
				for _, h := range hits {
					vecScores[h.Key] = h.Score
				}
			}
		}
		// On embed error the vector half is skipped and the search
		// degrades to keyword-only ranking.
	}

	// Phase 3: merge the two signals over the snapshot. Entries deleted
	// concurrently between phases simply drop out; entries added after
	// the snapshot are not returned by this call.
	type merged struct {
		entry *PersistentMemoryEntry
		score float64
	}
	var candidates []merged
	for _, e := range entries {
		kw := 0
		hasKw := false
		if sc, ok := kwScores[e.Key]; ok {
			kw, hasKw = sc, true
		}
		vec := 0.0
		hasVec := false
		if vecScores != nil {
			if v, ok := vecScores[e.Key]; ok {
				vec, hasVec = v, true
			}
		}
		if !hasKw && !hasVec {
			continue
		}
		kwNorm := float64(kw) / (float64(kw) + hybridKeywordScale)
		combined := math.Max(kwNorm, vec) + hybridAgreementBonus*math.Min(kwNorm, vec)
		candidates = append(candidates, merged{e, combined})
	}

	// Sort by combined score descending; ties broken by key so results
	// are deterministic regardless of map iteration order.
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].entry.Key < candidates[j].entry.Key
	})

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
