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

// Package memory provides session-aware memory management with per-session
// isolation, memory usage tracking, and persistent storage.
package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
)

// MemoryEntry represents a single memory entry.
type MemoryEntry struct {
	ID         string    `json:"id"`
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	Type       string    `json:"type"`
	Level      string    `json:"level"`
	Score      float64   `json:"score"`
	ExpiresAt  time.Time `json:"expires_at"`
	AccessedAt time.Time `json:"accessed_at"`
	CreatedAt  time.Time `json:"created_at"`
	Tags       []string  `json:"tags"`
	Source     string    `json:"source"`
	Confidence float64   `json:"confidence"`
}

// MemoryGraph represents semantic relationships between memory entries.
type MemoryGraph struct {
	Nodes []MemoryEntry `json:"nodes"`
	Edges []MemoryEdge  `json:"edges"`
}

// MemoryEdge represents a relationship between two memory entries.
type MemoryEdge struct {
	Source    string    `json:"source"`
	Target    string    `json:"target"`
	Relation  string    `json:"relation"`
	Weight    float64   `json:"weight"`
	CreatedAt time.Time `json:"created_at"`
}

// MemoryStats contains statistics for a memory session.
type MemoryStats struct {
	TotalEntries    int     `json:"total_entries"`
	ShortTermCount  int     `json:"short_term_count"`
	MediumTermCount int     `json:"medium_term_count"`
	LongTermCount   int     `json:"long_term_count"`
	AvgConfidence   float64 `json:"avg_confidence"`
	TotalAccesses   int     `json:"total_accesses"`
	RetentionRate   float64 `json:"retention_rate"`
	EstimatedBytes  int64   `json:"estimated_bytes"`
}

// SessionMemory holds the isolated memory state for a single session.
type SessionMemory struct {
	mu          sync.RWMutex
	SessionID   string
	entries     map[string]*MemoryEntry
	graph       MemoryGraph
	accessCount int
	maxEntries  int
	createdAt   time.Time
	updatedAt   time.Time
	lastUsedAt  time.Time

	// C-1: vector retrieval state. embedder is immutable after assignment
	// (set once by SetEmbedder); vectors is mutated under sm.mu AND its own
	// internal lock, so callers may search vectors without holding sm.mu.
	// We keep vectors in sync with entries: every Store adds an embedding,
	// every Delete/Forget/evict removes one. KGNodeRefs holds the C-3
	// memory↔graph linkage (memory key -> KG node names).
	embedder   Embedder
	vectors    *VectorIndex
	KGNodeRefs map[string][]string // key -> []KG entity name
}

// NewSessionMemory creates a new isolated memory session.
func NewSessionMemory(sessionID string, maxEntries int) *SessionMemory {
	if maxEntries <= 0 {
		maxEntries = 5000 // Default per-session limit
	}
	now := time.Now()
	return &SessionMemory{
		SessionID:  sessionID,
		entries:    make(map[string]*MemoryEntry),
		maxEntries: maxEntries,
		createdAt:  now,
		updatedAt:  now,
		lastUsedAt: now,
		vectors:    NewVectorIndex(),
		KGNodeRefs: make(map[string][]string),
	}
}

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

// Touch updates the last used timestamp.
func (sm *SessionMemory) Touch() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastUsedAt = time.Now()
}

// Store adds or updates a memory entry.
func (sm *SessionMemory) Store(key, value, level, memType string, tags []string, ttlHours int, confidence float64, source string) (string, time.Time, error) {
	return sm.StoreCtx(context.Background(), key, value, level, memType, tags, ttlHours, confidence, source)
}

// StoreCtx is the context-aware variant of Store. It computes the
// embedding (if an embedder is configured) using ctx, allowing callers
// to time-bound or cancel the embedding HTTP call. The entries map is
// updated atomically: the embedding is computed BEFORE acquiring the
// write lock so a slow embedder doesn't block other readers.
func (sm *SessionMemory) StoreCtx(ctx context.Context, key, value, level, memType string, tags []string, ttlHours int, confidence float64, source string) (string, time.Time, error) {
	// Snapshot the embedder under read lock; Embed may do network I/O.
	sm.mu.RLock()
	e := sm.embedder
	sm.mu.RUnlock()

	var vec Vector
	if e != nil {
		v, err := e.Embed(ctx, value)
		if err == nil && len(v) > 0 {
			vec = v
		}
		// On embedding error we proceed anyway: the entry will be
		// retrievable by key and matched by the bag-of-words fallback.
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Evict expired and LRU if at capacity
	if len(sm.entries) >= sm.maxEntries {
		sm.cleanupExpiredLocked()
		if len(sm.entries) >= sm.maxEntries {
			sm.evictLRULocked()
		}
	}

	expiresAt := time.Now().Add(time.Duration(ttlHours) * time.Hour)
	if level == "long" {
		expiresAt = time.Now().Add(365 * 24 * time.Hour)
	}

	entry := &MemoryEntry{
		ID:         fmt.Sprintf("entry_%d", time.Now().UnixNano()),
		Key:        key,
		Value:      value,
		Type:       memType,
		Level:      level,
		Score:      confidence * 100,
		ExpiresAt:  expiresAt,
		AccessedAt: time.Now(),
		CreatedAt:  time.Now(),
		Tags:       tags,
		Source:     source,
		Confidence: confidence,
	}

	// If overwriting an existing key, keep its KG links intact.
	sm.entries[key] = entry
	sm.updatedAt = time.Now()

	// Update the vector index. We do this under sm.mu (which also
	// serialises against Delete/Forget) AND vectors' own internal lock
	// — two locks, but always acquired in the same order elsewhere
	// (vectors lock is only ever taken alone in its own methods), so
	// no deadlock risk.
	if len(vec) > 0 {
		sm.vectors.Add(key, vec, map[string]string{
			"level":      level,
			"type":       memType,
			"session_id": sm.SessionID,
		})
	}
	return entry.ID, expiresAt, nil
}

// Retrieve fetches a memory entry by key.
func (sm *SessionMemory) Retrieve(key string) (*MemoryEntry, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	entry, ok := sm.entries[key]
	if !ok {
		return nil, fmt.Errorf("memory not found: %s", key)
	}

	if time.Now().After(entry.ExpiresAt) {
		// Clean up all three indices (entries / vectors / KGNodeRefs)
		// so expired entries don't leak memory or leave dangling refs.
		// Matches the cleanup in Delete and cleanupExpiredLocked.
		delete(sm.entries, key)
		delete(sm.KGNodeRefs, key)
		sm.vectors.Remove(key)
		return nil, fmt.Errorf("memory expired: %s", key)
	}

	entry.AccessedAt = time.Now()
	sm.accessCount++
	sm.updatedAt = time.Now()

	// Return a copy to avoid data races
	entryCopy := *entry
	return &entryCopy, nil
}

// Delete removes a memory entry.
func (sm *SessionMemory) Delete(key string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.entries[key]; !ok {
		return fmt.Errorf("memory not found: %s", key)
	}

	delete(sm.entries, key)
	delete(sm.KGNodeRefs, key)
	sm.vectors.Remove(key)
	sm.updatedAt = time.Now()
	return nil
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

// cleanupExpiredLocked removes expired entries (must hold write lock).
func (sm *SessionMemory) cleanupExpiredLocked() {
	now := time.Now()
	for key, entry := range sm.entries {
		if now.After(entry.ExpiresAt) {
			delete(sm.entries, key)
			delete(sm.KGNodeRefs, key)
			sm.vectors.Remove(key)
		}
	}
}

// evictLRULocked removes the least recently used entry (must hold write lock).
func (sm *SessionMemory) evictLRULocked() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range sm.entries {
		if oldestKey == "" || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
		}
	}

	if oldestKey != "" {
		delete(sm.entries, oldestKey)
		delete(sm.KGNodeRefs, oldestKey)
		sm.vectors.Remove(oldestKey)
	}
}

// Forget removes all entries or entries of a specific level.
func (sm *SessionMemory) Forget(level string) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	deletedCount := 0
	for key, entry := range sm.entries {
		if level == "" || entry.Level == level {
			delete(sm.entries, key)
			delete(sm.KGNodeRefs, key)
			sm.vectors.Remove(key)
			deletedCount++
		}
	}

	if deletedCount > 0 {
		sm.updatedAt = time.Now()
	}

	return deletedCount
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

// GetStats returns a snapshot of memory statistics.
func (sm *SessionMemory) GetStats() MemoryStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	short := 0
	medium := 0
	long := 0
	totalConfidence := 0.0
	count := 0
	var estBytes int64

	now := time.Now()
	for _, entry := range sm.entries {
		if now.After(entry.ExpiresAt) {
			continue
		}
		switch entry.Level {
		case "short":
			short++
		case "medium":
			medium++
		case "long":
			long++
		}
		totalConfidence += entry.Confidence
		count++
		estBytes += int64(len(entry.Key) + len(entry.Value) + len(entry.ID) + len(entry.Type) + len(entry.Level) + len(entry.Source) + 256)
		for _, t := range entry.Tags {
			estBytes += int64(len(t))
		}
	}

	avgConfidence := 0.0
	if count > 0 {
		avgConfidence = totalConfidence / float64(count)
	}

	return MemoryStats{
		TotalEntries:    count,
		ShortTermCount:  short,
		MediumTermCount: medium,
		LongTermCount:   long,
		AvgConfidence:   avgConfidence,
		TotalAccesses:   sm.accessCount,
		RetentionRate:   float64(count) / float64(len(sm.entries)+1),
		EstimatedBytes:  estBytes,
	}
}

// SessionMemoryManager manages multiple isolated memory sessions.
type SessionMemoryManager struct {
	mu            sync.RWMutex
	sessions      map[string]*SessionMemory
	maxSessions   int
	maxPerSession int
	storageDir    string
	globalAccess  int64
}

// GlobalSessionManager is the default session memory manager.
var GlobalSessionManager = NewSessionMemoryManager("", 100, 5000)

// NewSessionMemoryManager creates a new session memory manager.
func NewSessionMemoryManager(storageDir string, maxSessions, maxPerSession int) *SessionMemoryManager {
	if maxSessions <= 0 {
		maxSessions = 100
	}
	if maxPerSession <= 0 {
		maxPerSession = 5000
	}

	mgr := &SessionMemoryManager{
		sessions:      make(map[string]*SessionMemory),
		maxSessions:   maxSessions,
		maxPerSession: maxPerSession,
		storageDir:    storageDir,
	}

	if storageDir != "" {
		if err := os.MkdirAll(storageDir, 0755); err != nil {
			logger.Warn("failed to create session storage dir", "dir", storageDir, "err", err)
		}
	}

	return mgr
}

var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

// sanitizeSessionID maps an arbitrary session identifier onto a
// filesystem-safe one. IDs that already match the strict pattern pass
// through unchanged; anything else (path separators, ".." traversal,
// over-long or exotic identifiers) is replaced by a stable short hash.
// Every code path that derives a storage path from a session ID MUST go
// through this, so a hostile ID such as "x/../../victim" can never steer
// file operations outside the session storage directory.
func sanitizeSessionID(sessionID string) string {
	if sessionIDPattern.MatchString(sessionID) {
		return sessionID
	}
	h := sha256.Sum256([]byte(sessionID))
	return "s_" + hex.EncodeToString(h[:16])
}

// GetSession retrieves or creates a memory session.
func (mgr *SessionMemoryManager) GetSession(sessionID string) *SessionMemory {
	if sessionID == "" {
		sessionID = "default"
	}
	sessionID = sanitizeSessionID(sessionID)

	mgr.mu.RLock()
	session, exists := mgr.sessions[sessionID]
	mgr.mu.RUnlock()

	if exists {
		session.Touch()
		return session
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// Double-check after lock
	session, exists = mgr.sessions[sessionID]
	if exists {
		session.Touch()
		return session
	}

	// Evict oldest if at capacity
	if len(mgr.sessions) >= mgr.maxSessions {
		mgr.evictOldestLocked()
	}

	session = NewSessionMemory(sessionID, mgr.maxPerSession)
	mgr.sessions[sessionID] = session

	// Try to load from persistent storage
	if mgr.storageDir != "" {
		mgr.loadSessionLocked(session)
	}

	return session
}

// evictOldestLocked evicts the least recently used session (must hold write lock).
func (mgr *SessionMemoryManager) evictOldestLocked() {
	var oldestID string
	var oldestTime time.Time

	for id, s := range mgr.sessions {
		s.mu.RLock()
		lu := s.lastUsedAt
		s.mu.RUnlock()
		if oldestID == "" || lu.Before(oldestTime) {
			oldestID = id
			oldestTime = lu
		}
	}

	if oldestID != "" {
		// Save before evicting
		if mgr.storageDir != "" {
			if s, ok := mgr.sessions[oldestID]; ok {
				mgr.saveSessionLocked(s)
			}
		}
		delete(mgr.sessions, oldestID)
	}
}

// DeleteSession removes a memory session.
func (mgr *SessionMemoryManager) DeleteSession(sessionID string) {
	if sessionID == "" {
		sessionID = "default"
	}
	// Map the caller-supplied ID onto the same filesystem-safe ID that
	// GetSession derives, so the delete hits the same map entry and the
	// same storage file that reads/writes use — and so a hostile ID
	// (e.g. "x/../../victim") can never steer os.Remove outside the
	// session storage directory (path traversal via delete).
	safeID := sanitizeSessionID(sessionID)

	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	delete(mgr.sessions, safeID)

	if mgr.storageDir != "" {
		path := mgr.sessionFilePath(safeID)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.Warn("failed to remove session file", "path", path, "err", err)
		}
	}
}

// ListSessions returns a list of active session IDs.
func (mgr *SessionMemoryManager) ListSessions() []string {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	ids := make([]string, 0, len(mgr.sessions))
	for id := range mgr.sessions {
		ids = append(ids, id)
	}
	return ids
}

// GetGlobalStats returns aggregate statistics across all sessions.
type GlobalMemoryStats struct {
	ActiveSessions   int                    `json:"active_sessions"`
	TotalEntries     int                    `json:"total_entries"`
	TotalEstimatedMB float64                `json:"total_estimated_mb"`
	TotalAccesses    int64                  `json:"total_accesses"`
	PerSession       map[string]MemoryStats `json:"per_session"`
}

// GetGlobalStats returns aggregate memory statistics.
func (mgr *SessionMemoryManager) GetGlobalStats() GlobalMemoryStats {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	stats := GlobalMemoryStats{
		ActiveSessions: len(mgr.sessions),
		PerSession:     make(map[string]MemoryStats, len(mgr.sessions)),
	}

	var totalBytes int64
	for id, s := range mgr.sessions {
		ss := s.GetStats()
		stats.PerSession[id] = ss
		stats.TotalEntries += ss.TotalEntries
		totalBytes += ss.EstimatedBytes
	}

	stats.TotalEstimatedMB = float64(totalBytes) / (1024 * 1024)
	stats.TotalAccesses = mgr.globalAccess

	return stats
}

// FormatGlobalStats returns a human-readable global stats report.
func (mgr *SessionMemoryManager) FormatGlobalStats() string {
	stats := mgr.GetGlobalStats()

	var sb strings.Builder
	sb.WriteString("🧠 Memory Manager Status\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	sb.WriteString(fmt.Sprintf("Active sessions: %d\n", stats.ActiveSessions))
	sb.WriteString(fmt.Sprintf("Total entries:   %d\n", stats.TotalEntries))
	sb.WriteString(fmt.Sprintf("Estimated size:  %.2f MB\n", stats.TotalEstimatedMB))
	sb.WriteString(fmt.Sprintf("Total accesses:  %d\n", stats.TotalAccesses))

	if stats.ActiveSessions > 0 {
		sb.WriteString(fmt.Sprintf("\nAvg per session: %.0f entries\n", float64(stats.TotalEntries)/float64(stats.ActiveSessions)))

		// jcode comparison: 10 sessions = 117MB
		if stats.ActiveSessions >= 10 {
			projectedFor10 := (stats.TotalEstimatedMB / float64(stats.ActiveSessions)) * 10
			sb.WriteString(fmt.Sprintf("10 sessions est: %.0f MB (jcode: 117 MB)\n", projectedFor10))
		}
	}

	if len(stats.PerSession) > 0 && len(stats.PerSession) <= 10 {
		sb.WriteString("\n📋 Per-session breakdown:\n")
		for id, ss := range stats.PerSession {
			sb.WriteString(fmt.Sprintf("  %s: %d entries (%.2f KB)\n",
				id, ss.TotalEntries, float64(ss.EstimatedBytes)/1024))
		}
	}

	return sb.String()
}

// sessionFilePath returns the persistent storage path for a session.
func (mgr *SessionMemoryManager) sessionFilePath(sessionID string) string {
	return filepath.Join(mgr.storageDir, fmt.Sprintf("session-%s.json", sessionID))
}

// sessionData is the serializable format for a session.
type sessionData struct {
	SessionID   string                  `json:"session_id"`
	Entries     map[string]*MemoryEntry `json:"entries"`
	Graph       MemoryGraph             `json:"graph"`
	AccessCount int                     `json:"access_count"`
	MaxEntries  int                     `json:"max_entries"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
	LastUsedAt  time.Time               `json:"last_used_at"`
	// C-3: persisted memory↔KG linkage. Keyed by memory key, values
	// are KG entity names. The vector index is NOT persisted: vectors
	// are recomputed on load via ReindexVectors (embeddings are cheap
	// and model versions may change).
	KGNodeRefs map[string][]string `json:"kg_node_refs,omitempty"`
}

// SaveAll persists all active sessions to storage.
func (mgr *SessionMemoryManager) SaveAll() {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	for _, s := range mgr.sessions {
		mgr.saveSessionLocked(s)
	}
}

// saveSessionLocked persists a session (must hold read or write lock on mgr).
func (mgr *SessionMemoryManager) saveSessionLocked(session *SessionMemory) {
	if mgr.storageDir == "" {
		return
	}

	session.mu.RLock()
	// Deep-copy KGNodeRefs AND Entries so the marshalled JSON is stable
	// even if a concurrent writer mutates the maps while json.Marshal
	// runs. Entries stores *MemoryEntry pointers; without copying the
	// pointed-to struct, a concurrent Retrieve (which mutates
	// AccessedAt under the write lock) would race with marshalling.
	kgRefs := make(map[string][]string, len(session.KGNodeRefs))
	for k, v := range session.KGNodeRefs {
		cp := make([]string, len(v))
		copy(cp, v)
		kgRefs[k] = cp
	}
	data := sessionData{
		SessionID:   session.SessionID,
		Entries:     make(map[string]*MemoryEntry, len(session.entries)),
		Graph:       session.graph,
		AccessCount: session.accessCount,
		MaxEntries:  session.maxEntries,
		CreatedAt:   session.createdAt,
		UpdatedAt:   session.updatedAt,
		LastUsedAt:  session.lastUsedAt,
		KGNodeRefs:  kgRefs,
	}
	for k, v := range session.entries {
		ec := *v // value copy so concurrent writers can't race the marshal
		data.Entries[k] = &ec
	}
	session.mu.RUnlock()

	path := mgr.sessionFilePath(session.SessionID)

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	// Atomic write: write to tmp then rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, jsonData, 0600); err != nil {
		return
	}
	if err := os.Rename(tmpPath, path); err != nil {
		logger.Warn("failed to atomically persist session file", "path", path, "err", err)
	}
}

// loadSessionLocked loads a session from persistent storage (must hold write lock).
func (mgr *SessionMemoryManager) loadSessionLocked(session *SessionMemory) {
	path := mgr.sessionFilePath(session.SessionID)

	data, err := os.ReadFile(path) // #nosec G304 -- internally generated session path
	if err != nil {
		return // File doesn't exist, start fresh
	}

	var sd sessionData
	if err := json.Unmarshal(data, &sd); err != nil {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if sd.Entries == nil {
		sd.Entries = make(map[string]*MemoryEntry)
	}
	if sd.KGNodeRefs == nil {
		sd.KGNodeRefs = make(map[string][]string)
	}
	session.entries = sd.Entries
	session.graph = sd.Graph
	session.accessCount = sd.AccessCount
	session.maxEntries = sd.MaxEntries
	session.createdAt = sd.CreatedAt
	session.updatedAt = sd.UpdatedAt
	session.lastUsedAt = sd.LastUsedAt
	session.KGNodeRefs = sd.KGNodeRefs
	// Vectors are intentionally left empty: callers must invoke
	// ReindexVectors after load if they want semantic search.
}

// StartAutoSave starts periodic auto-saving of sessions.
func (mgr *SessionMemoryManager) StartAutoSave(ctx context.Context, interval time.Duration) {
	if mgr.storageDir == "" {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// saveAll wraps SaveAll with panic recovery so the auto-save
		// loop keeps running even if a single save panics.
		saveAll := func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("session auto-save panicked",
						"panic", r,
						"stack", string(debug.Stack()),
					)
				}
			}()
			mgr.SaveAll()
		}

		for {
			select {
			case <-ctx.Done():
				saveAll()
				return
			case <-ticker.C:
				saveAll()
			}
		}
	}()
}

// Convenience functions using the global manager

// GetSession returns a memory session from the global manager.
func GetSession(sessionID string) *SessionMemory {
	return GlobalSessionManager.GetSession(sessionID)
}

// DeleteSession removes a memory session from the global manager.
func DeleteSession(sessionID string) {
	GlobalSessionManager.DeleteSession(sessionID)
}

// ListSessions returns active session IDs from the global manager.
func ListSessions() []string {
	return GlobalSessionManager.ListSessions()
}

// GetGlobalStats returns aggregate statistics from the global manager.
func GetGlobalStats() GlobalMemoryStats {
	return GlobalSessionManager.GetGlobalStats()
}

// FormatGlobalStats returns a human-readable status report from the global manager.
func FormatGlobalStats() string {
	return GlobalSessionManager.FormatGlobalStats()
}
