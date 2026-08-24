// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌​​‌​‌​​‌​​​​‌‌​​‌​​‌​​​‌​‌​​​‌​​‌‌​‌‌‌​​​‌‌​​​​​​​​​​​​​​​​​​​​​‌‌‌‌‌‌​‌‌​​​​⁠
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
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewSessionMemory(t *testing.T) {
	sm := NewSessionMemory("test-session", 100)
	if sm.SessionID != "test-session" {
		t.Errorf("expected session ID test-session, got %s", sm.SessionID)
	}
	if sm.maxEntries != 100 {
		t.Errorf("expected maxEntries 100, got %d", sm.maxEntries)
	}
}

func TestStoreAndRetrieve(t *testing.T) {
	sm := NewSessionMemory("test", 100)
	_, _, err := sm.Store("key1", "value1", "short", "text", []string{"tag1"}, 24, 0.9, "test")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	entry, err := sm.Retrieve("key1")
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	if entry.Value != "value1" {
		t.Errorf("expected value1, got %s", entry.Value)
	}
	if entry.Key != "key1" {
		t.Errorf("expected key1, got %s", entry.Key)
	}
}

func TestRetrieveNotFound(t *testing.T) {
	sm := NewSessionMemory("test", 100)
	_, err := sm.Retrieve("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
}

func TestDelete(t *testing.T) {
	sm := NewSessionMemory("test", 100)
	sm.Store("key1", "value1", "short", "text", nil, 24, 0.9, "test")

	err := sm.Delete("key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = sm.Retrieve("key1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestSearch(t *testing.T) {
	sm := NewSessionMemory("test", 100)
	sm.Store("k1", "hello world from go", "short", "text", nil, 24, 0.9, "test")
	sm.Store("k2", "goodbye world from python", "short", "text", nil, 24, 0.9, "test")
	sm.Store("k3", "completely unrelated content", "short", "text", nil, 24, 0.9, "test")

	results := sm.Search("hello world", "", 10, 0.1)
	if len(results) == 0 {
		t.Error("expected at least one search result")
	}
}

func TestSearchByLevel(t *testing.T) {
	sm := NewSessionMemory("test", 100)
	sm.Store("k1", "short term memory", "short", "text", nil, 24, 0.9, "test")
	sm.Store("k2", "medium term memory", "medium", "text", nil, 24, 0.9, "test")
	sm.Store("k3", "long term memory", "long", "text", nil, 24, 0.9, "test")

	results := sm.Search("memory", "short", 10, 0.0)
	if len(results) != 1 {
		t.Errorf("expected 1 short-term result, got %d", len(results))
	}

	results = sm.Search("memory", "long", 10, 0.0)
	if len(results) != 1 {
		t.Errorf("expected 1 long-term result, got %d", len(results))
	}
}

func TestForgetAll(t *testing.T) {
	sm := NewSessionMemory("test", 100)
	sm.Store("k1", "value1", "short", "text", nil, 24, 0.9, "test")
	sm.Store("k2", "value2", "medium", "text", nil, 24, 0.9, "test")
	sm.Store("k3", "value3", "long", "text", nil, 24, 0.9, "test")

	deleted := sm.Forget("")
	if deleted != 3 {
		t.Errorf("expected 3 deleted, got %d", deleted)
	}

	stats := sm.GetStats()
	if stats.TotalEntries != 0 {
		t.Errorf("expected 0 entries after forget all, got %d", stats.TotalEntries)
	}
}

func TestForgetByLevel(t *testing.T) {
	sm := NewSessionMemory("test", 100)
	sm.Store("k1", "value1", "short", "text", nil, 24, 0.9, "test")
	sm.Store("k2", "value2", "medium", "text", nil, 24, 0.9, "test")
	sm.Store("k3", "value3", "long", "text", nil, 24, 0.9, "test")

	deleted := sm.Forget("short")
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	stats := sm.GetStats()
	if stats.ShortTermCount != 0 {
		t.Errorf("expected 0 short-term entries, got %d", stats.ShortTermCount)
	}
	if stats.MediumTermCount != 1 {
		t.Errorf("expected 1 medium-term entry, got %d", stats.MediumTermCount)
	}
	if stats.LongTermCount != 1 {
		t.Errorf("expected 1 long-term entry, got %d", stats.LongTermCount)
	}
}

func TestTouch(t *testing.T) {
	sm := NewSessionMemory("test", 100)
	before := sm.lastUsedAt
	time.Sleep(1 * time.Millisecond)
	sm.Touch()
	if !sm.lastUsedAt.After(before) {
		t.Error("expected lastUsedAt to be updated after Touch")
	}
}

func TestGetStats(t *testing.T) {
	sm := NewSessionMemory("test", 100)
	sm.Store("k1", "value1", "short", "text", nil, 24, 0.9, "test")
	sm.Store("k2", "value2", "medium", "text", nil, 24, 0.8, "test")
	sm.Store("k3", "value3", "long", "text", nil, 24, 0.7, "test")

	stats := sm.GetStats()
	if stats.TotalEntries != 3 {
		t.Errorf("expected 3 total entries, got %d", stats.TotalEntries)
	}
	if stats.ShortTermCount != 1 {
		t.Errorf("expected 1 short-term, got %d", stats.ShortTermCount)
	}
	if stats.MediumTermCount != 1 {
		t.Errorf("expected 1 medium-term, got %d", stats.MediumTermCount)
	}
	if stats.LongTermCount != 1 {
		t.Errorf("expected 1 long-term, got %d", stats.LongTermCount)
	}
}

func TestCalculateSimilarity(t *testing.T) {
	sim := calculateSimilarity("hello world", "hello world from go")
	if sim <= 0 {
		t.Error("expected positive similarity for matching words")
	}

	sim = calculateSimilarity("hello world", "completely different text")
	if sim != 0 {
		t.Errorf("expected 0 similarity for no matching words, got %f", sim)
	}

	sim = calculateSimilarity("", "some text")
	if sim != 0 {
		t.Errorf("expected 0 similarity for empty query, got %f", sim)
	}
}

func TestSessionManagerGetSession(t *testing.T) {
	mgr := NewSessionMemoryManager(t.TempDir(), 10, 100)

	s1 := mgr.GetSession("session1")
	if s1.SessionID != "session1" {
		t.Errorf("expected session1, got %s", s1.SessionID)
	}

	s2 := mgr.GetSession("session1")
	if s1 != s2 {
		t.Error("expected same session instance on second call")
	}
}

func TestSessionManagerInvalidSessionID(t *testing.T) {
	mgr := NewSessionMemoryManager(t.TempDir(), 10, 100)

	s := mgr.GetSession("invalid id with spaces!")
	if s.SessionID == "invalid id with spaces!" {
		t.Error("expected session ID to be sanitized")
	}
}

func TestSessionManagerListSessions(t *testing.T) {
	mgr := NewSessionMemoryManager(t.TempDir(), 10, 100)

	mgr.GetSession("s1")
	mgr.GetSession("s2")
	mgr.GetSession("s3")

	sessions := mgr.ListSessions()
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestSessionManagerDeleteSession(t *testing.T) {
	mgr := NewSessionMemoryManager(t.TempDir(), 10, 100)

	mgr.GetSession("s1")
	mgr.DeleteSession("s1")

	sessions := mgr.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after delete, got %d", len(sessions))
	}
}

// TestSessionManagerDeleteSessionTraversal verifies that a hostile session
// ID containing path-traversal sequences cannot steer DeleteSession's
// os.Remove outside the session storage directory, and that a session
// created under a hostile ID is still deletable via the same hostile ID
// (the sanitized storage key is used consistently on both sides).
func TestSessionManagerDeleteSessionTraversal(t *testing.T) {
	base := t.TempDir()
	storageDir := filepath.Join(base, "sessions")
	mgr := NewSessionMemoryManager(storageDir, 10, 100)

	// A victim file OUTSIDE the storage directory that a traversal ID
	// would try to reach: storageDir/session-x/../../victim.json
	// resolves to base/victim.json.
	victim := filepath.Join(base, "victim.json")
	if err := os.WriteFile(victim, []byte(`{"keep": true}`), 0600); err != nil {
		t.Fatalf("write victim: %v", err)
	}

	// Create a session under the hostile ID — GetSession sanitizes it to
	// a hashed ID, so both the map entry and (after SaveAll) the storage
	// file live under the sanitized name.
	hostile := "x/../../victim"
	s := mgr.GetSession(hostile)
	if s.SessionID == hostile {
		t.Fatal("GetSession should sanitize hostile session IDs")
	}

	// Deleting via the hostile ID must not touch the victim file.
	mgr.DeleteSession(hostile)
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("victim file outside storageDir was removed: %v", err)
	}

	// The in-memory session created under the sanitized key must be gone.
	for _, id := range mgr.ListSessions() {
		if id == s.SessionID {
			t.Fatalf("session %q still listed after DeleteSession(hostile)", id)
		}
	}

	// A well-formed ID round-trips: create → persist → delete removes the
	// storage file inside storageDir.
	ok := mgr.GetSession("normal")
	ok.Store("k", "v", "short", "text", nil, 24, 0.9, "test")
	mgr.SaveAll()
	wantPath := filepath.Join(storageDir, "session-normal.json")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected persisted session file: %v", err)
	}
	mgr.DeleteSession("normal")
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Fatalf("expected session file removed, stat err = %v", err)
	}
}

func TestSessionManagerGetGlobalStats(t *testing.T) {
	mgr := NewSessionMemoryManager(t.TempDir(), 10, 100)

	s1 := mgr.GetSession("s1")
	s1.Store("k1", "v1", "short", "text", nil, 24, 0.9, "test")

	s2 := mgr.GetSession("s2")
	s2.Store("k2", "v2", "long", "text", nil, 24, 0.9, "test")
	s2.Store("k3", "v3", "medium", "text", nil, 24, 0.9, "test")

	stats := mgr.GetGlobalStats()
	if stats.ActiveSessions != 2 {
		t.Errorf("expected 2 active sessions, got %d", stats.ActiveSessions)
	}
	if stats.TotalEntries != 3 {
		t.Errorf("expected 3 total entries, got %d", stats.TotalEntries)
	}
}

func TestSessionManagerPersistence(t *testing.T) {
	storageDir := t.TempDir()
	mgr := NewSessionMemoryManager(storageDir, 10, 100)

	s := mgr.GetSession("persist-test")
	s.Store("key1", "persisted value", "long", "text", nil, 24, 0.9, "test")

	mgr.SaveAll()

	mgr2 := NewSessionMemoryManager(storageDir, 10, 100)
	s2 := mgr2.GetSession("persist-test")

	entry, err := s2.Retrieve("key1")
	if err != nil {
		t.Fatalf("failed to retrieve persisted entry: %v", err)
	}
	if entry.Value != "persisted value" {
		t.Errorf("expected 'persisted value', got %s", entry.Value)
	}
}

func TestUserProfileSetAndGet(t *testing.T) {
	mgr := &UserProfileManager{
		profiles:   make(map[string]*UserProfile),
		storageDir: "",
		mu:         sync.RWMutex{},
		maxPerUser: defaultMaxPrefsPerUser,
	}

	p := mgr.GetProfile("user1")
	p.SetPreference(PrefCodingStyle, "language", "go", "test", 0.9)

	val, conf, ok := p.GetPreference(PrefCodingStyle, "language")
	if !ok {
		t.Fatal("expected preference to be found")
	}
	if val != "go" {
		t.Errorf("expected go, got %s", val)
	}
	if conf != 0.9 {
		t.Errorf("expected confidence 0.9, got %f", conf)
	}
}

func TestUserProfileGetAllByCategory(t *testing.T) {
	mgr := &UserProfileManager{
		profiles:   make(map[string]*UserProfile),
		storageDir: "",
		mu:         sync.RWMutex{},
		maxPerUser: defaultMaxPrefsPerUser,
	}

	p := mgr.GetProfile("user1")
	p.SetPreference(PrefCodingStyle, "language", "go", "test", 0.9)
	p.SetPreference(PrefCodingStyle, "indent", "4", "test", 0.8)
	p.SetPreference(PrefOutputFormat, "format", "markdown", "test", 0.7)

	coding := p.GetAllByCategory(PrefCodingStyle)
	if len(coding) != 2 {
		t.Errorf("expected 2 coding style prefs, got %d", len(coding))
	}
	if coding["language"] != "go" {
		t.Errorf("expected language=go, got %s", coding["language"])
	}

	output := p.GetAllByCategory(PrefOutputFormat)
	if len(output) != 1 {
		t.Errorf("expected 1 output format pref, got %d", len(output))
	}
}

func TestUserProfileReinforce(t *testing.T) {
	mgr := &UserProfileManager{
		profiles:   make(map[string]*UserProfile),
		storageDir: "",
		mu:         sync.RWMutex{},
		maxPerUser: defaultMaxPrefsPerUser,
	}

	p := mgr.GetProfile("user1")
	p.SetPreference(PrefCodingStyle, "language", "go", "test", 0.9)
	p.SetPreference(PrefCodingStyle, "language", "go", "test", 0.95)

	val, conf, _ := p.GetPreference(PrefCodingStyle, "language")
	if val != "go" {
		t.Errorf("expected go, got %s", val)
	}
	if conf != 0.95 {
		t.Errorf("expected confidence upgraded to 0.95, got %f", conf)
	}
}

func TestUserProfileGetSummary(t *testing.T) {
	mgr := &UserProfileManager{
		profiles:   make(map[string]*UserProfile),
		storageDir: "",
		mu:         sync.RWMutex{},
		maxPerUser: defaultMaxPrefsPerUser,
	}

	p := mgr.GetProfile("user1")
	p.SetPreference(PrefCodingStyle, "language", "go", "test", 0.9)
	p.SetPreference(PrefOutputFormat, "format", "markdown", "test", 0.8)

	summary := p.GetSummary()
	if summary["user_id"] != "user1" {
		t.Errorf("expected user_id=user1, got %v", summary["user_id"])
	}
	if summary["total_prefs"] != 2 {
		t.Errorf("expected total_prefs=2, got %v", summary["total_prefs"])
	}
}

func TestUserProfileBuildPrompt(t *testing.T) {
	mgr := &UserProfileManager{
		profiles:   make(map[string]*UserProfile),
		storageDir: "",
		mu:         sync.RWMutex{},
		maxPerUser: defaultMaxPrefsPerUser,
	}

	p := mgr.GetProfile("user1")
	result := p.BuildSystemPromptAddon()
	if result != "" {
		t.Errorf("expected empty prompt for no prefs, got %s", result)
	}

	p.SetPreference(PrefCodingStyle, "language", "go", "test", 0.9)
	result = p.BuildSystemPromptAddon()
	if result == "" {
		t.Error("expected non-empty prompt with prefs")
	}
}

func TestUserProfileLearnFromInteraction(t *testing.T) {
	mgr := &UserProfileManager{
		profiles:   make(map[string]*UserProfile),
		storageDir: "",
		mu:         sync.RWMutex{},
		maxPerUser: defaultMaxPrefsPerUser,
	}

	p := mgr.GetProfile("user1")
	p.LearnFromInteraction("user1", string(PrefCodingStyle), "language", "go", "interaction")

	val, conf, ok := p.GetPreference(PrefCodingStyle, "language")
	if !ok {
		t.Fatal("expected preference to be learned")
	}
	if val != "go" {
		t.Errorf("expected go, got %s", val)
	}
	if conf != 0.6 {
		t.Errorf("expected confidence 0.6 from LearnFromInteraction, got %f", conf)
	}
}

// TestUserProfileStartAutoSave_FlushOnCancel verifies that cancelling the
// context triggers a final flush, persisting in-memory profile state to disk
// before the goroutine exits. This is the key behavior that lets callers use
// StartAutoSave as a "save on shutdown" hook.
func TestUserProfileStartAutoSave_FlushOnCancel(t *testing.T) {
	dir := t.TempDir()
	mgr := &UserProfileManager{
		profiles:   make(map[string]*UserProfile),
		storageDir: dir,
		mu:         sync.RWMutex{},
		maxPerUser: defaultMaxPrefsPerUser,
	}

	path := filepath.Join(dir, sanitizeUserID("user1")+".json")

	// GetProfile persists the (empty) profile on creation. SetPreference
	// mutates in memory only — so on disk the preference is absent until a
	// save fires.
	p := mgr.GetProfile("user1")
	p.SetPreference(PrefCodingStyle, "language", "go", "test", 0.9)

	if preferenceOnDisk(t, path, "go") {
		t.Fatal("precondition: preference should NOT be on disk before flush")
	}

	// Long interval so no periodic tick fires — only the cancellation flush
	// should write the updated state.
	ctx, cancel := context.WithCancel(context.Background())
	mgr.StartAutoSave(ctx, time.Hour)
	cancel()

	// Poll for the flush to land; it happens synchronously inside the
	// goroutine's ctx.Done() branch right before return.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if preferenceOnDisk(t, path, "go") {
			return // flush confirmed
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Error("expected updated profile to be flushed to disk on ctx cancellation")
}

// preferenceOnDisk reports whether the persisted profile file at path contains
// the given preference value. Returns false on any read/parse error.
func preferenceOnDisk(t *testing.T, path, want string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var temp struct {
		Preferences map[string]*PreferenceEntry `json:"preferences"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return false
	}
	for _, e := range temp.Preferences {
		if e.Value == want {
			return true
		}
	}
	return false
}
