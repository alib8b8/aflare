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

package agent

import (
	"log"
	"sync"
	"time"
)

const (
	// DefaultMaxSessions is the default maximum number of concurrent sessions.
	DefaultMaxSessions = 10

	// DefaultSessionTTL is the default time-to-live for idle sessions.
	DefaultSessionTTL = 30 * time.Minute
)

// sessionEntry wraps a ChatSession with its last-used timestamp for LRU eviction.
type sessionEntry struct {
	session  *ChatSession
	lastUsed time.Time
}

// SessionManager manages multiple concurrent ChatSession instances with
// LRU eviction and TTL-based cleanup. Safe for concurrent use.
type SessionManager struct {
	mu          sync.RWMutex
	sessions    map[string]*sessionEntry
	maxSessions int
	ttl         time.Duration
	capabilities []string
}

// NewSessionManager creates a new SessionManager with the given limits.
// If maxSessions <= 0, DefaultMaxSessions is used.
// If ttl <= 0, DefaultSessionTTL is used.
func NewSessionManager(maxSessions int, ttl time.Duration) *SessionManager {
	if maxSessions <= 0 {
		maxSessions = DefaultMaxSessions
	}
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return &SessionManager{
		sessions:    make(map[string]*sessionEntry),
		maxSessions: maxSessions,
		ttl:         ttl,
	}
}

// SetCapabilities sets the capability names to enable for new sessions.
func (sm *SessionManager) SetCapabilities(caps []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.capabilities = caps
}

// Capabilities returns the current capability names.
func (sm *SessionManager) Capabilities() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.capabilities
}

// GetOrCreate returns the session for the given ID, creating a new one if
// it doesn't exist. If the session count exceeds maxSessions, the least
// recently used session is evicted first.
func (sm *SessionManager) GetOrCreate(sessionID string) *ChatSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Return existing session if found
	if entry, ok := sm.sessions[sessionID]; ok {
		entry.lastUsed = time.Now()
		return entry.session
	}

	// Evict expired sessions
	sm.evictExpiredLocked()

	// Evict LRU if at capacity
	if len(sm.sessions) >= sm.maxSessions {
		sm.evictLRULocked()
	}

	// Create new session
	cfg := DefaultConfig()
	cfg.Capabilities = sm.capabilities
	session := NewChatSession(cfg)

	sm.sessions[sessionID] = &sessionEntry{
		session:  session,
		lastUsed: time.Now(),
	}

	log.Printf("[session] created session %s (total: %d/%d)", sessionID, len(sm.sessions), sm.maxSessions)
	return session
}

// Reset clears the conversation history for the given session.
func (sm *SessionManager) Reset(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if entry, ok := sm.sessions[sessionID]; ok {
		entry.session.ResetSession()
		entry.lastUsed = time.Now()
	}
}

// Remove deletes a session by ID.
func (sm *SessionManager) Remove(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.sessions, sessionID)
	log.Printf("[session] removed session %s (total: %d)", sessionID, len(sm.sessions))
}

// SetMaxSessions updates the maximum session limit. If the current count
// exceeds the new limit, excess sessions are evicted using LRU.
func (sm *SessionManager) SetMaxSessions(n int) {
	if n <= 0 {
		n = DefaultMaxSessions
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.maxSessions = n
	for len(sm.sessions) > sm.maxSessions {
		sm.evictLRULocked()
	}
}

// Count returns the number of active sessions.
func (sm *SessionManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// evictExpiredLocked removes sessions that have exceeded the TTL.
// Caller must hold sm.mu.
func (sm *SessionManager) evictExpiredLocked() {
	now := time.Now()
	for id, entry := range sm.sessions {
		if now.Sub(entry.lastUsed) > sm.ttl {
			log.Printf("[session] evicting expired session %s (idle: %v)", id, now.Sub(entry.lastUsed))
			delete(sm.sessions, id)
		}
	}
}

// evictLRULocked removes the least recently used session.
// Caller must hold sm.mu.
func (sm *SessionManager) evictLRULocked() {
	var oldestID string
	var oldestTime time.Time

	for id, entry := range sm.sessions {
		if oldestID == "" || entry.lastUsed.Before(oldestTime) {
			oldestID = id
			oldestTime = entry.lastUsed
		}
	}

	if oldestID != "" {
		log.Printf("[session] evicting LRU session %s (last used: %v)", oldestID, time.Since(oldestTime))
		delete(sm.sessions, oldestID)
	}
}