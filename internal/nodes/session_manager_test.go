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

package nodes

import (
	"encoding/json"
	"testing"

	"github.com/alib8b8/aflare/internal/memory"
)

func TestSessionManagerNode_Metadata(t *testing.T) {
	n := &SessionManagerNode{}
	if n.Name() != "session_manager" {
		t.Errorf("Name = %q", n.Name())
	}
	s := n.Schema()
	if s.Name != "session_manager" {
		t.Errorf("Schema Name = %q", s.Name)
	}
}

func TestSessionManagerNode_Registered(t *testing.T) {
	if _, ok := Get("session_manager"); !ok {
		t.Fatal("session_manager node not registered")
	}
}

// withTestSessionManager swaps the package-global cross-session manager
// for an isolated test instance and restores it on cleanup.
func withTestSessionManager(t *testing.T) *memory.SessionManager {
	t.Helper()
	orig := memory.GlobalCrossSessionManager
	memMgr := memory.NewSessionMemoryManager("", 50, 100)
	mgr := memory.NewSessionManager(memMgr)
	memory.GlobalCrossSessionManager = mgr
	t.Cleanup(func() { memory.GlobalCrossSessionManager = orig })
	return mgr
}

func TestSessionManagerNode_CreateAndList(t *testing.T) {
	withTestSessionManager(t)
	n := &SessionManagerNode{}

	out, err := n.Execute(t.Context(), "", map[string]string{
		"action":     "create",
		"session_id": "agent-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(out), &res)
	if res["session"] != "agent-1" {
		t.Errorf("create result = %+v", res)
	}

	out, err = n.Execute(t.Context(), "", map[string]string{"action": "list"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	_ = json.Unmarshal([]byte(out), &res)
	count, _ := res["count"].(float64)
	if count < 1 {
		t.Errorf("list count = %v, want >=1", count)
	}
}

func TestSessionManagerNode_SharedPutAndGet(t *testing.T) {
	withTestSessionManager(t)
	n := &SessionManagerNode{}

	_, err := n.Execute(t.Context(), "Go is the language", map[string]string{
		"action": "shared_put",
		"key":    "lang",
	})
	if err != nil {
		t.Fatalf("shared_put: %v", err)
	}

	out, err := n.Execute(t.Context(), "", map[string]string{
		"action": "shared_get",
		"key":    "lang",
	})
	if err != nil {
		t.Fatalf("shared_get: %v", err)
	}
	var entry map[string]interface{}
	_ = json.Unmarshal([]byte(out), &entry)
	if entry["value"] != "Go is the language" {
		t.Errorf("shared_get value = %v", entry["value"])
	}
}

func TestSessionManagerNode_ForkAndRecall(t *testing.T) {
	withTestSessionManager(t)
	n := &SessionManagerNode{}

	// Put a shared fact.
	_, _ = n.Execute(t.Context(), "shared-fact", map[string]string{
		"action": "shared_put",
		"key":    "global",
	})
	// Create parent with a local fact.
	_, _ = n.Execute(t.Context(), "", map[string]string{
		"action":     "create",
		"session_id": "parent",
	})
	// Use recall on parent to store locally via shared_put won't work;
	// instead use the memory manager directly through a second action.
	// Here we just verify fork inherits shared facts via fallback.
	_, err := n.Execute(t.Context(), "", map[string]string{
		"action":     "create",
		"session_id": "child",
		"parent":     "parent",
	})
	if err != nil {
		t.Fatalf("fork: %v", err)
	}

	out, err := n.Execute(t.Context(), "", map[string]string{
		"action":     "recall",
		"session_id": "child",
		"key":        "global",
	})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	var entry map[string]interface{}
	_ = json.Unmarshal([]byte(out), &entry)
	if entry["value"] != "shared-fact" {
		t.Errorf("recall value = %v, want shared-fact", entry["value"])
	}
}

func TestSessionManagerNode_Merge(t *testing.T) {
	mgr := withTestSessionManager(t)
	n := &SessionManagerNode{}

	// Seed src session with two facts.
	src, _ := mgr.Create("src", "")
	_, _, _ = src.Store("a", "1", "short", "fact", nil, 24, 1, "test")
	_, _, _ = src.Store("b", "2", "short", "fact", nil, 24, 1, "test")
	_, _ = mgr.Create("dst", "")

	out, err := n.Execute(t.Context(), "", map[string]string{
		"action": "merge",
		"src":    "src",
		"dst":    "dst",
	})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(out), &res)
	count, _ := res["merged_count"].(float64)
	if count != 2 {
		t.Errorf("merged_count = %v, want 2", count)
	}
}

func TestSessionManagerNode_UnknownAction(t *testing.T) {
	withTestSessionManager(t)
	n := &SessionManagerNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{"action": "bogus"})
	if err == nil {
		t.Error("expected error for unknown action")
	}
}

func TestSessionManagerNode_CreateMissingID(t *testing.T) {
	withTestSessionManager(t)
	n := &SessionManagerNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{"action": "create"})
	if err == nil {
		t.Error("expected error when session_id missing")
	}
}

func TestSessionManagerNode_SwitchNonExistent(t *testing.T) {
	withTestSessionManager(t)
	n := &SessionManagerNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{
		"action":     "switch",
		"session_id": "nope",
	})
	if err == nil {
		t.Error("expected error switching to non-existent session")
	}
}

func TestSessionManagerNode_Delete(t *testing.T) {
	mgr := withTestSessionManager(t)
	n := &SessionManagerNode{}
	_, _ = mgr.Create("temp", "")

	_, err := n.Execute(t.Context(), "", map[string]string{
		"action":     "delete",
		"session_id": "temp",
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	for _, id := range mgr.List() {
		if id == "temp" {
			t.Error("session still present after delete")
		}
	}
}

func TestSessionManagerNode_RecallMissingKey(t *testing.T) {
	withTestSessionManager(t)
	n := &SessionManagerNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{
		"action":     "recall",
		"session_id": "any",
		"key":        "missing",
	})
	if err == nil {
		t.Error("expected error for missing key")
	}
}
