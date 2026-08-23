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
	"testing"
	"time"
)

// fakeClock makes LRU ordering and TTL expiry deterministic: tests advance
// it explicitly instead of sleeping (issue #85 — real sleeps let LRU order
// flip under parallel load).
type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

// newSessionManagerWithClock wires a fake clock into a SessionManager.
func newSessionManagerWithClock(maxSessions int, ttl time.Duration) (*SessionManager, *fakeClock) {
	sm := NewSessionManager(maxSessions, ttl)
	clock := &fakeClock{t: time.Now()}
	sm.now = clock.Now
	return sm, clock
}

// ── SessionManager: Basic Operations ────────────────────────────────────────

func TestSessionManager_GetOrCreate(t *testing.T) {
	sm := NewSessionManager(5, 10*time.Minute)
	s1 := sm.GetOrCreate("session-1")
	if s1 == nil {
		t.Fatal("GetOrCreate returned nil")
	}
	if sm.Count() != 1 {
		t.Errorf("expected count 1, got %d", sm.Count())
	}

	// Same ID should return the same session
	s2 := sm.GetOrCreate("session-1")
	if s1 != s2 {
		t.Error("GetOrCreate should return same session for same ID")
	}
	if sm.Count() != 1 {
		t.Errorf("expected count still 1, got %d", sm.Count())
	}
}

func TestSessionManager_Remove(t *testing.T) {
	sm := NewSessionManager(5, 10*time.Minute)
	sm.GetOrCreate("session-1")
	sm.Remove("session-1")
	if sm.Count() != 0 {
		t.Errorf("expected count 0 after remove, got %d", sm.Count())
	}
}

func TestSessionManager_Reset(t *testing.T) {
	sm := NewSessionManager(5, 10*time.Minute)
	s := sm.GetOrCreate("session-1")

	// Reset should not panic on existing session
	sm.Reset("session-1")

	// After reset, session should still exist
	s2 := sm.GetOrCreate("session-1")
	if s != s2 {
		t.Error("same session should be returned after reset")
	}
	if sm.Count() != 1 {
		t.Errorf("expected 1 session after reset, got %d", sm.Count())
	}

	// Reset non-existent session should not panic
	sm.Reset("nonexistent")
}

// ── SessionManager: LRU Eviction ───────────────────────────────────────────

func TestSessionManager_LRUEviction(t *testing.T) {
	sm, clock := newSessionManagerWithClock(3, 1*time.Hour) // max 3 sessions

	// Create 3 sessions with distinct timestamps
	sm.GetOrCreate("session-1")
	clock.Advance(time.Millisecond)
	sm.GetOrCreate("session-2")
	clock.Advance(time.Millisecond)
	sm.GetOrCreate("session-3")

	if sm.Count() != 3 {
		t.Fatalf("expected 3 sessions, got %d", sm.Count())
	}

	// Access session-1 to make it recently used
	clock.Advance(time.Millisecond)
	sm.GetOrCreate("session-1")

	// Create session-4 — should evict session-2 (the LRU, never re-accessed)
	clock.Advance(time.Millisecond)
	sm.GetOrCreate("session-4")

	if sm.Count() != 3 {
		t.Errorf("expected 3 sessions after eviction, got %d", sm.Count())
	}

	// Verify all 3 remaining sessions are removable (no double-free)
	sm.Remove("session-1")
	sm.Remove("session-3")
	sm.Remove("session-4")
	if sm.Count() != 0 {
		t.Errorf("expected 0 sessions after cleanup, got %d", sm.Count())
	}
}

func TestSessionManager_LRUEviction_AccessUpdatesLRU(t *testing.T) {
	sm, clock := newSessionManagerWithClock(2, 1*time.Hour)

	sm.GetOrCreate("session-1")
	clock.Advance(time.Millisecond)
	sm.GetOrCreate("session-2")

	// Access session-1 (now it's most recent)
	clock.Advance(time.Millisecond)
	sm.GetOrCreate("session-1")

	// Create session-3 — should evict session-2 (LRU since session-1 was accessed)
	clock.Advance(time.Millisecond)
	sm.GetOrCreate("session-3")

	if sm.Count() != 2 {
		t.Errorf("expected 2 sessions, got %d", sm.Count())
	}
}

func TestSessionManager_LRUEvictionMultiple(t *testing.T) {
	sm := NewSessionManager(2, 1*time.Hour)

	sm.GetOrCreate("session-1")
	sm.GetOrCreate("session-2")
	sm.GetOrCreate("session-3")
	sm.GetOrCreate("session-4")
	sm.GetOrCreate("session-5")

	if sm.Count() != 2 {
		t.Errorf("expected 2 sessions after multiple evictions, got %d", sm.Count())
	}
}

// ── SessionManager: TTL Eviction ───────────────────────────────────────────

func TestSessionManager_TTLEviction(t *testing.T) {
	sm, clock := newSessionManagerWithClock(10, 50*time.Millisecond)

	sm.GetOrCreate("session-1")
	if sm.Count() != 1 {
		t.Fatalf("expected 1 session, got %d", sm.Count())
	}

	// Advance past the TTL
	clock.Advance(100 * time.Millisecond)

	// Creating a new session triggers evictExpiredLocked
	sm.GetOrCreate("session-2")

	// session-1 should be evicted by TTL
	if sm.Count() != 1 {
		t.Errorf("expected 1 session after TTL eviction, got %d", sm.Count())
	}
}

func TestSessionManager_TTLRefreshOnAccess(t *testing.T) {
	sm, clock := newSessionManagerWithClock(10, 100*time.Millisecond)

	sm.GetOrCreate("session-1")
	clock.Advance(60 * time.Millisecond)

	// Access session-1 to refresh its TTL
	sm.GetOrCreate("session-1")
	clock.Advance(60 * time.Millisecond)

	// Create a new session — session-1 should still be alive (TTL refreshed)
	sm.GetOrCreate("session-2")

	if sm.Count() != 2 {
		t.Errorf("expected 2 sessions when TTL refreshed, got %d", sm.Count())
	}
}

// ── SessionManager: SetMaxSessions ─────────────────────────────────────────

func TestSessionManager_SetMaxSessions(t *testing.T) {
	sm := NewSessionManager(5, 1*time.Hour)

	sm.GetOrCreate("session-1")
	sm.GetOrCreate("session-2")
	sm.GetOrCreate("session-3")

	if sm.Count() != 3 {
		t.Fatalf("expected 3 sessions, got %d", sm.Count())
	}

	// Reduce max to 2 — should evict one
	sm.SetMaxSessions(2)

	if sm.Count() != 2 {
		t.Errorf("expected 2 sessions after SetMaxSessions(2), got %d", sm.Count())
	}
}

func TestSessionManager_SetMaxSessions_Zero(t *testing.T) {
	sm := NewSessionManager(5, 1*time.Hour)

	sm.GetOrCreate("session-1")
	sm.GetOrCreate("session-2")

	sm.SetMaxSessions(0) // should default to DefaultMaxSessions

	if sm.Count() != 2 {
		t.Errorf("expected 2 sessions when setting max to 0, got %d", sm.Count())
	}
}

// ── SessionManager: Capabilities ───────────────────────────────────────────

func TestSessionManager_Capabilities(t *testing.T) {
	sm := NewSessionManager(5, 10*time.Minute)
	caps := sm.Capabilities()
	if len(caps) != 0 {
		t.Errorf("expected 0 capabilities initially, got %d", len(caps))
	}

	sm.SetCapabilities([]string{"reflection", "utility"})
	caps = sm.Capabilities()
	if len(caps) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(caps))
	}
}

// ── SessionManager: Defaults ────────────────────────────────────────────────

func TestSessionManager_Defaults(t *testing.T) {
	sm := NewSessionManager(0, 0) // zero values should use defaults
	// Should not panic, and should use defaults
	s := sm.GetOrCreate("test")
	if s == nil {
		t.Fatal("GetOrCreate returned nil")
	}
	if sm.Count() != 1 {
		t.Errorf("expected 1 session, got %d", sm.Count())
	}
}
