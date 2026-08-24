// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌‌‌​​‌​​‌​‌​‌​​‌‌‌​‌​​​‌​​​‌​​‌​‌​‌‌​​​​​​​​​​​‌‌​‌‌‌​‌‌‌‌​‌‌​​​​​​​​​​​​​​​​​​‌​‌‌‌​​‌‌​‌​​‌‌⁠
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

package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SetEmbedder installs the embedder used for semantic search. If nil is
// passed the session falls back to bag-of-words similarity. Once an
// embedder is set, all subsequent Store calls will compute embeddings;
// existing entries are NOT retroactively embedded. Call ReindexVectors
// to embed existing entries.
//
// This method is safe to call before any Store; calling it after entries
// exist without reindexing leaves those entries without vectors (they
// will still be matched by the bag-of-words fallback in Search).
func (sm *SessionMemory) SetEmbedder(e Embedder) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.embedder = e
}

// GetEmbedder returns the currently configured embedder (may be nil).
func (sm *SessionMemory) GetEmbedder() Embedder {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.embedder
}

// ReindexVectors (re)embeds every existing entry using the current
// embedder. If no embedder is set this is a no-op. This is the
// recommended path after SetEmbedder when migrating a live session.
func (sm *SessionMemory) ReindexVectors(ctx context.Context) error {
	sm.mu.RLock()
	e := sm.embedder
	entriesCopy := make([]*MemoryEntry, 0, len(sm.entries))
	for _, entry := range sm.entries {
		ec := *entry
		entriesCopy = append(entriesCopy, &ec)
	}
	sm.mu.RUnlock()

	if e == nil {
		return nil
	}
	for _, entry := range entriesCopy {
		v, err := e.Embed(ctx, entry.Value)
		if err != nil || len(v) == 0 {
			continue
		}
		sm.vectors.Add(entry.Key, v, map[string]string{
			"level":      entry.Level,
			"type":       entry.Type,
			"session_id": sm.SessionID,
		})
	}
	return nil
}

// LinkKGNode associates a memory entry with one or more knowledge-graph
// entity names (C-3). Later, ExpandKGSubgraph can be used at retrieval
// time to fetch related entities. Returns an error if the key doesn't
// exist so callers don't silently create dangling links.
func (sm *SessionMemory) LinkKGNode(key string, entityNames ...string) error {
	if len(entityNames) == 0 {
		return nil
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.entries[key]; !ok {
		return fmt.Errorf("cannot link KG node: memory not found: %s", key)
	}
	existing := sm.KGNodeRefs[key]
	seen := make(map[string]bool, len(existing))
	for _, n := range existing {
		seen[n] = true
	}
	for _, n := range entityNames {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		sm.KGNodeRefs[key] = append(sm.KGNodeRefs[key], n)
	}
	return nil
}

// GetKGLinks returns the KG entity names linked to a memory key.
func (sm *SessionMemory) GetKGLinks(key string) []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	refs := sm.KGNodeRefs[key]
	out := make([]string, len(refs))
	copy(out, refs)
	return out
}

// ExpandKGSubgraph returns the union of KG entity names linked to any
// of the given memory keys. It is the retrieval-time helper for C-3:
// after a vector search returns hits, the caller passes the hit keys
// here to find related KG nodes that should be surfaced alongside.
func (sm *SessionMemory) ExpandKGSubgraph(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	seen := make(map[string]bool)
	var out []string
	for _, k := range keys {
		for _, n := range sm.KGNodeRefs[k] {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
}

// Search finds memory entries matching a query.
//
// C-1: if an embedder is configured, the query is embedded and compared
// against stored entry vectors using cosine similarity. Entries without
// a stored vector fall back to bag-of-words similarity. If no embedder
// is configured, the entire search uses bag-of-words (legacy behaviour).
//
// The threshold is interpreted in [0,1] for both paths so existing
// callers don't need to know which backend is active.
func (sm *SessionMemory) Search(query, level string, topK int, threshold float64) []MemoryEntry {
	return sm.SearchCtx(context.Background(), query, level, topK, threshold)
}

// SearchCtx is the context-aware variant of Search. The context is
// honoured when computing the query embedding (e.g. an HTTPEmbedder).
func (sm *SessionMemory) SearchCtx(ctx context.Context, query, level string, topK int, threshold float64) []MemoryEntry {
	sm.mu.RLock()
	e := sm.embedder
	sm.mu.RUnlock()

	// Try the vector path first. If we get any hits, augment them with
	// bag-of-words fallback for entries that lack a vector, then return.
	if e != nil {
		qv, err := e.Embed(ctx, query)
		if err == nil && len(qv) > 0 {
			return sm.searchVector(qv, query, level, topK, threshold)
		}
		// On embed error, fall through to bag-of-words.
	}
	return sm.searchBagOfWords(query, level, topK, threshold)
}

// searchVector merges vector hits with bag-of-words hits for entries
// that don't have a stored vector. This keeps recall high when some
// entries were stored before the embedder was configured.
func (sm *SessionMemory) searchVector(qv Vector, query, level string, topK int, threshold float64) []MemoryEntry {
	// Collect vector hits (does NOT need sm.mu — VectorIndex has its
	// own lock), then attach full entry data under sm.mu.
	hits := sm.vectors.Search(qv, topK*2, threshold)

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	now := time.Now()
	type scored struct {
		entry MemoryEntry
		score float64
	}
	var merged []scored
	seen := make(map[string]bool, len(hits))

	for _, h := range hits {
		entry, ok := sm.entries[h.Key]
		if !ok {
			continue
		}
		if level != "" && entry.Level != level {
			continue
		}
		if now.After(entry.ExpiresAt) {
			continue
		}
		ec := *entry
		ec.Score = h.Score * 100
		merged = append(merged, scored{ec, h.Score})
		seen[h.Key] = true
	}

	// Bag-of-words fallback for entries not in the vector index.
	for key, entry := range sm.entries {
		if seen[key] {
			continue
		}
		if level != "" && entry.Level != level {
			continue
		}
		if now.After(entry.ExpiresAt) {
			continue
		}
		sim := calculateSimilarity(query, entry.Value)
		if sim >= threshold {
			ec := *entry
			ec.Score = sim * 100
			merged = append(merged, scored{ec, sim})
		}
	}

	// Sort by score descending (stable on key for determinism).
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].score != merged[j].score {
			return merged[i].score > merged[j].score
		}
		return merged[i].entry.Key < merged[j].entry.Key
	})
	if len(merged) > topK {
		merged = merged[:topK]
	}
	out := make([]MemoryEntry, len(merged))
	for i, m := range merged {
		out[i] = m.entry
	}
	return out
}

// searchBagOfWords is the legacy word-overlap search path, used when
// no embedder is configured or embedding the query fails.
func (sm *SessionMemory) searchBagOfWords(query, level string, topK int, threshold float64) []MemoryEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var results []MemoryEntry
	now := time.Now()

	for _, entry := range sm.entries {
		if level != "" && entry.Level != level {
			continue
		}
		if now.After(entry.ExpiresAt) {
			continue
		}

		similarity := calculateSimilarity(query, entry.Value)
		if similarity >= threshold {
			entryCopy := *entry
			entryCopy.Score = similarity * 100
			results = append(results, entryCopy)
		}
	}

	// Sort by score (descending)
	for i := range results {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if len(results) > topK {
		results = results[:topK]
	}

	return results
}

// calculateSimilarity computes word overlap similarity between query and text.
func calculateSimilarity(query, text string) float64 {
	queryWords := strings.Fields(strings.ToLower(query))
	textWords := strings.Fields(strings.ToLower(text))

	if len(queryWords) == 0 {
		return 0.0
	}

	matches := 0
	for _, qw := range queryWords {
		for _, tw := range textWords {
			if strings.Contains(tw, qw) || strings.Contains(qw, tw) {
				matches++
				break
			}
		}
	}

	return float64(matches) / float64(len(queryWords))
}
