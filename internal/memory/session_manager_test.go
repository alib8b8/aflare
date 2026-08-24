// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌‌​‌​‌​‌‌‌‌‌‌​​​​‌‌​​​​​‌‌‌‌‌​‌​​​​‌‌​​​​‌‌‌‌​​​​​​​​​​​​​​​​​‌​‌‌​‌‌​​‌‌​​​​​⁠
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
	"testing"
)

func newTestSessionManager(t *testing.T) *SessionManager {
	t.Helper()
	memMgr := NewSessionMemoryManager("", 50, 100)
	return NewSessionManager(memMgr)
}

func TestSessionManager_CreateAndSwitch(t *testing.T) {
	mgr := newTestSessionManager(t)

	s, err := mgr.Create("agent-1", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s == nil {
		t.Fatal("Create returned nil session")
	}
	if mgr.Active() != "agent-1" {
		t.Errorf("Active = %q, want agent-1", mgr.Active())
	}

	if _, err := mgr.Switch("nope"); err == nil {
		t.Error("Switch to non-existent session should error")
	}
	if _, err := mgr.Switch("agent-1"); err != nil {
		t.Errorf("Switch to existing session failed: %v", err)
	}
}

func TestSessionManager_CreateReservedID(t *testing.T) {
	mgr := newTestSessionManager(t)
	if _, err := mgr.Create("__shared__", ""); err == nil {
		t.Error("Create with reserved __shared__ id should error")
	}
	if _, err := mgr.Create("", ""); err == nil {
		t.Error("Create with empty id should error")
	}
}

func TestSessionManager_ListExcludesShared(t *testing.T) {
	mgr := newTestSessionManager(t)
	_, _ = mgr.Create("a", "")
	_, _ = mgr.Create("b", "")
	// Touch shared namespace so it exists internally.
	_, _, _ = mgr.StoreShared("k", "v", "short", "fact", nil, 0, 1, "test")

	ids := mgr.List()
	for _, id := range ids {
		if id == mgr.SharedSessionID() {
			t.Errorf("List should exclude shared namespace, got %q", id)
		}
	}
	if len(ids) != 2 {
		t.Errorf("List = %v, want 2 sessions", ids)
	}
}

func TestSessionManager_SharedNamespace(t *testing.T) {
	mgr := newTestSessionManager(t)
	_, _, err := mgr.StoreShared("lang", "Go", "long", "fact", nil, 0, 1.0, "test")
	if err != nil {
		t.Fatalf("StoreShared: %v", err)
	}
	entry, err := mgr.RetrieveShared("lang")
	if err != nil {
		t.Fatalf("RetrieveShared: %v", err)
	}
	if entry.Value != "Go" {
		t.Errorf("shared value = %q, want Go", entry.Value)
	}
}

func TestSessionManager_RetrieveWithFallback(t *testing.T) {
	mgr := newTestSessionManager(t)
	_, _, _ = mgr.StoreShared("global", "g-val", "short", "fact", nil, 0, 1, "test")
	s, _ := mgr.Create("s1", "")
	_, _, _ = s.Store("local", "l-val", "short", "fact", nil, 24, 1, "test")

	// Local key found in session.
	e, err := mgr.RetrieveWithFallback("s1", "local")
	if err != nil || e.Value != "l-val" {
		t.Errorf("local lookup: %+v %v", e, err)
	}
	// Missing local key falls back to shared.
	e, err = mgr.RetrieveWithFallback("s1", "global")
	if err != nil || e.Value != "g-val" {
		t.Errorf("fallback lookup: %+v %v", e, err)
	}
	// Missing everywhere.
	if _, err := mgr.RetrieveWithFallback("s1", "missing"); err == nil {
		t.Error("expected error for missing key")
	}
}

func TestSessionManager_Fork(t *testing.T) {
	mgr := newTestSessionManager(t)
	parent, _ := mgr.Create("parent", "")
	_, _, _ = parent.Store("skill", "Go", "long", "fact", nil, 24, 1, "test")
	_, _, _ = parent.Store("temp", "scratch", "short", "fact", nil, 24, 1, "test")

	child, err := mgr.Create("child", "parent")
	if err != nil {
		t.Fatalf("Create fork: %v", err)
	}
	// Child should have inherited entries.
	e, err := child.Retrieve("skill")
	if err != nil || e.Value != "Go" {
		t.Errorf("forked child missing inherited key: %+v %v", e, err)
	}
	if mgr.Parent("child") != "parent" {
		t.Errorf("Parent = %q, want parent", mgr.Parent("child"))
	}

	// Mutating child must not affect parent.
	_, _, _ = child.Store("skill", "Rust", "long", "fact", nil, 24, 1, "test")
	pe, _ := parent.Retrieve("skill")
	if pe.Value != "Go" {
		t.Errorf("parent mutated after child edit: got %q, want Go", pe.Value)
	}
}

func TestSessionManager_Merge(t *testing.T) {
	mgr := newTestSessionManager(t)
	src, _ := mgr.Create("src", "")
	_, _, _ = src.Store("a", "1", "short", "fact", nil, 24, 1, "test")
	_, _, _ = src.Store("b", "2", "short", "fact", nil, 24, 1, "test")
	dst, _ := mgr.Create("dst", "")
	_, _, _ = dst.Store("existing", "x", "short", "fact", nil, 24, 1, "test")

	count, err := mgr.Merge("src", "dst")
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if count != 2 {
		t.Errorf("merged count = %d, want 2", count)
	}
	e, _ := dst.Retrieve("a")
	if e == nil || e.Value != "1" {
		t.Errorf("dst missing merged key a: %+v", e)
	}
	// Existing key preserved.
	e, _ = dst.Retrieve("existing")
	if e == nil || e.Value != "x" {
		t.Errorf("dst lost pre-existing key: %+v", e)
	}
}

func TestSessionManager_MergeErrors(t *testing.T) {
	mgr := newTestSessionManager(t)
	if _, err := mgr.Merge("", "dst"); err == nil {
		t.Error("Merge with empty src should error")
	}
	if _, err := mgr.Merge("a", "a"); err == nil {
		t.Error("Merge with src==dst should error")
	}
}

func TestSessionManager_Delete(t *testing.T) {
	mgr := newTestSessionManager(t)
	_, _ = mgr.Create("doomed", "")
	if err := mgr.Delete("doomed"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	for _, id := range mgr.List() {
		if id == "doomed" {
			t.Error("session still listed after delete")
		}
	}
	if mgr.Active() == "doomed" {
		t.Error("Active should clear after delete")
	}
	// Cannot delete shared namespace.
	if err := mgr.Delete(mgr.SharedSessionID()); err == nil {
		t.Error("Delete shared namespace should error")
	}
}

func TestSessionManager_SearchAcross(t *testing.T) {
	mgr := newTestSessionManager(t)
	_, _, _ = mgr.StoreShared("note", "remember to deploy Go service", "short", "fact", nil, 24, 1, "test")
	s, _ := mgr.Create("s", "")
	_, _, _ = s.Store("local-note", "local Go tip", "short", "fact", nil, 24, 1, "test")

	results := mgr.SearchAcross("s", "Go", "", 10, 0)
	if len(results) < 1 {
		t.Errorf("SearchAcross returned %d results, want >=1", len(results))
	}
	// Dedup by key.
	keys := map[string]bool{}
	for _, r := range results {
		if keys[r.Key] {
			t.Errorf("duplicate key %q in search results", r.Key)
		}
		keys[r.Key] = true
	}
}

func TestSessionManager_FormatSessionList(t *testing.T) {
	mgr := newTestSessionManager(t)
	_, _ = mgr.Create("a", "")
	_, _ = mgr.Create("b", "a") // forked

	out := mgr.FormatSessionList()
	if out == "" {
		t.Error("FormatSessionList returned empty string")
	}
	// Should mention the parent link.
	if !contains(out, "forked from a") {
		t.Errorf("FormatSessionList missing fork info: %q", out)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
