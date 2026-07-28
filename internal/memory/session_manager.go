// Copyright (c) 2026 llm-box Contributors
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

package memory

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// SessionManager extends SessionMemoryManager with cross-session
// capabilities inspired by jcode: a shared memory namespace that all
// sessions can read/write, session fork (inherit memory from a parent),
// and session merge (combine memory from one session into another).
//
// The shared namespace is backed by a synthetic session named
// "__shared__". Reads from a session fall back to the shared namespace
// when the key is not found locally, so shared facts (e.g. "user prefers
// Go") are visible to every agent session without duplicating them.
type SessionManager struct {
	mu          sync.RWMutex
	memMgr      *SessionMemoryManager
	sharedID    string
	active      string // most recently touched session id (advisory)
	parentLinks map[string]string
}

// GlobalCrossSessionManager is the default cross-session manager,
// layered on top of the package-global SessionMemoryManager.
var GlobalCrossSessionManager = NewSessionManager(GlobalSessionManager)

// NewSessionManager wraps an existing SessionMemoryManager with
// cross-session features. Passing nil falls back to the package global.
func NewSessionManager(memMgr *SessionMemoryManager) *SessionManager {
	if memMgr == nil {
		memMgr = GlobalSessionManager
	}
	return &SessionManager{
		memMgr:      memMgr,
		sharedID:    "__shared__",
		parentLinks: make(map[string]string, 32),
	}
}

// SharedSessionID returns the id of the synthetic shared session.
func (m *SessionManager) SharedSessionID() string { return m.sharedID }

// Create makes a new session (or returns the existing one). If parent
// is non-empty, the new session inherits the parent's memory entries
// (a shallow copy: each entry is re-stored under the child so later
// divergence does not mutate the parent).
func (m *SessionManager) Create(sessionID, parent string) (*SessionMemory, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID is required")
	}
	if sessionID == m.sharedID {
		return nil, fmt.Errorf("sessionID %q is reserved", sessionID)
	}
	if parent != "" && parent != sessionID {
		if err := m.fork(parent, sessionID); err != nil {
			return nil, err
		}
	}
	m.mu.Lock()
	m.active = sessionID
	m.mu.Unlock()
	return m.memMgr.GetSession(sessionID), nil
}

// fork copies all non-expired entries from parent into child. The
// parent must exist; the child is created if necessary.
func (m *SessionManager) fork(parent, child string) error {
	pSession := m.memMgr.GetSession(parent)
	if pSession == nil {
		return fmt.Errorf("parent session %q not found", parent)
	}
	cSession := m.memMgr.GetSession(child)
	if cSession == nil {
		return fmt.Errorf("failed to create child session %q", child)
	}
	pSession.mu.RLock()
	src := make([]*MemoryEntry, 0, len(pSession.entries))
	for _, e := range pSession.entries {
		if e.ExpiresAt.IsZero() || e.ExpiresAt.After(time.Now()) {
			src = append(src, e)
		}
	}
	pSession.mu.RUnlock()

	for _, e := range src {
		// Preserve remaining lifetime; default to 24h when the source
		// entry has no expiry (long-term memories re-stored as long).
		ttl := 24
		if !e.ExpiresAt.IsZero() {
			remaining := time.Until(e.ExpiresAt).Hours()
			if remaining > 1 {
				ttl = int(remaining)
			}
		}
		// Ignore per-entry store errors during fork: we want to copy as
		// many entries as possible rather than fail the whole fork.
		_, _, _ = cSession.Store(e.Key, e.Value, e.Level, e.Type, e.Tags, ttl, e.Confidence, e.Source)
	}
	m.mu.Lock()
	m.parentLinks[child] = parent
	m.mu.Unlock()
	return nil
}

// Switch records sessionID as the most recently active session and
// returns its memory. Returns an error if the session does not exist.
func (m *SessionManager) Switch(sessionID string) (*SessionMemory, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID is required")
	}
	m.memMgr.mu.RLock()
	_, exists := m.memMgr.sessions[sessionID]
	m.memMgr.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}
	m.mu.Lock()
	m.active = sessionID
	m.mu.Unlock()
	return m.memMgr.GetSession(sessionID), nil
}

// Active returns the most recently active session id. May be empty.
func (m *SessionManager) Active() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// List returns the ids of all sessions (excluding the shared namespace).
func (m *SessionManager) List() []string {
	ids := m.memMgr.ListSessions()
	out := ids[:0]
	for _, id := range ids {
		if id != m.sharedID {
			out = append(out, id)
		}
	}
	return out
}

// Delete removes a session. The shared namespace cannot be deleted.
func (m *SessionManager) Delete(sessionID string) error {
	if sessionID == m.sharedID {
		return fmt.Errorf("cannot delete the shared namespace")
	}
	m.memMgr.DeleteSession(sessionID)
	m.mu.Lock()
	delete(m.parentLinks, sessionID)
	if m.active == sessionID {
		m.active = ""
	}
	m.mu.Unlock()
	return nil
}

// StoreShared writes a key to the shared namespace, visible to all
// sessions via RetrieveShared / RetrieveWithFallback. When ttlHours is
// 0 or negative, a sensible default is derived from level so callers
// don't accidentally write entries that expire immediately.
func (m *SessionManager) StoreShared(key, value, level, memType string, tags []string, ttlHours int, confidence float64, source string) (string, time.Time, error) {
	if ttlHours <= 0 {
		switch level {
		case "long":
			ttlHours = 365 * 24
		case "medium":
			ttlHours = 24 * 7
		default: // short
			ttlHours = 24
		}
	}
	return m.memMgr.GetSession(m.sharedID).Store(key, value, level, memType, tags, ttlHours, confidence, source)
}

// RetrieveShared reads a key from the shared namespace.
func (m *SessionManager) RetrieveShared(key string) (*MemoryEntry, error) {
	return m.memMgr.GetSession(m.sharedID).Retrieve(key)
}

// RetrieveWithFallback reads a key from the given session, falling back
// to the shared namespace when not found locally.
func (m *SessionManager) RetrieveWithFallback(sessionID, key string) (*MemoryEntry, error) {
	if sessionID != "" {
		if entry, err := m.memMgr.GetSession(sessionID).Retrieve(key); err == nil {
			return entry, nil
		}
	}
	return m.memMgr.GetSession(m.sharedID).Retrieve(key)
}

// Merge copies all non-expired entries from src into dst. Both
// sessions must exist. The shared namespace is a valid source or
// destination. Existing keys in dst are overwritten.
func (m *SessionManager) Merge(src, dst string) (int, error) {
	if src == "" || dst == "" {
		return 0, fmt.Errorf("src and dst are required")
	}
	if src == dst {
		return 0, fmt.Errorf("src and dst must differ")
	}
	sSession := m.memMgr.GetSession(src)
	dSession := m.memMgr.GetSession(dst)
	if sSession == nil || dSession == nil {
		return 0, fmt.Errorf("session not found")
	}

	sSession.mu.RLock()
	entries := make([]*MemoryEntry, 0, len(sSession.entries))
	for _, e := range sSession.entries {
		if e.ExpiresAt.IsZero() || e.ExpiresAt.After(time.Now()) {
			entries = append(entries, e)
		}
	}
	sSession.mu.RUnlock()

	count := 0
	for _, e := range entries {
		ttl := 24
		if !e.ExpiresAt.IsZero() {
			remaining := time.Until(e.ExpiresAt).Hours()
			if remaining > 1 {
				ttl = int(remaining)
			}
		}
		if _, _, err := dSession.Store(e.Key, e.Value, e.Level, e.Type, e.Tags, ttl, e.Confidence, e.Source); err == nil {
			count++
		}
	}
	return count, nil
}

// Parent returns the parent session id recorded for sessionID, or ""
// if it was not created via Create(..., parent) or has no parent.
func (m *SessionManager) Parent(sessionID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.parentLinks[sessionID]
}

// SearchAcross searches both the given session and the shared
// namespace, returning merged results sorted by score. topK caps the
// total number of results.
func (m *SessionManager) SearchAcross(sessionID, query, level string, topK int, threshold float64) []MemoryEntry {
	var results []MemoryEntry
	if sessionID != "" {
		results = append(results, m.memMgr.GetSession(sessionID).Search(query, level, topK, threshold)...)
	}
	results = append(results, m.memMgr.GetSession(m.sharedID).Search(query, level, topK, threshold)...)

	// Deduplicate by key (session entry wins over shared).
	seen := make(map[string]bool, len(results))
	deduped := results[:0]
	for _, e := range results {
		if seen[e.Key] {
			continue
		}
		seen[e.Key] = true
		deduped = append(deduped, e)
	}
	if len(deduped) > topK {
		deduped = deduped[:topK]
	}
	return deduped
}

// FormatSessionList returns a human-readable summary of active sessions.
func (m *SessionManager) FormatSessionList() string {
	ids := m.List()
	if len(ids) == 0 {
		return "no active sessions"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("sessions (%d):\n", len(ids)))
	m.mu.RLock()
	active := m.active
	m.mu.RUnlock()
	for _, id := range ids {
		marker := "  "
		if id == active {
			marker = "* "
		}
		parent := m.Parent(id)
		if parent != "" {
			b.WriteString(fmt.Sprintf("%s%s (forked from %s)\n", marker, id, parent))
		} else {
			b.WriteString(fmt.Sprintf("%s%s\n", marker, id))
		}
	}
	return b.String()
}
