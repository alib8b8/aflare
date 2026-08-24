// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​​​‌‌‌​​​‌‌​​‌‌​‌‌‌​‌​‌‌‌‌‌‌‌‌​​​​‌‌​‌​​​‌‌‌‌​​​​​​​​​​​​​​​​​​​​​‌​‌‌‌‌‌‌​‌‌‌⁠
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
	"fmt"
	"os"
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
