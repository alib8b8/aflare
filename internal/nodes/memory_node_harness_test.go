// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌‌​‌‌‌‌​‌‌​‌​‌‌​​‌‌‌‌‌‌​‌​‌‌‌​​‌‌‌‌​​‌​‌​​​‌‌​​​​​​​​​​​​​​​​​​​‌​‌‌‌‌‌​​‌‌​​⁠
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
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMemoryNode_HarnessSearch(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "harness-search-test"
	s := mgr.GetSession(sessionID)
	_, _, _ = s.Store("deploy_style", "team prefers blue-green deploys", "long", "preference", nil, 24, 0.9, "retro-2025-01")
	_, _, _ = s.Store("old_tool", "we used Jenkins back then", "long", "fact", nil, 24, 0.6, "retro-2025-01")

	n := &MemoryNode{}
	out, err := n.Execute(context.Background(), "deploy the payment service", map[string]string{
		"operation":  "harness_search",
		"session_id": sessionID,
		"query":      "blue-green deploys Jenkins",
		"top_k":      "5",
		"threshold":  "0.0",
	})
	if err != nil {
		t.Fatalf("harness_search: %v (out=%s)", err, out)
	}

	var res map[string]interface{}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if res["operation"] != "harness_search" {
		t.Fatalf("unexpected operation %v", res["operation"])
	}

	cands, _ := res["candidates"].([]interface{})
	if len(cands) == 0 {
		t.Fatal("expected candidates from harness_search")
	}
	for _, c := range cands {
		m, ok := c.(map[string]interface{})
		if !ok {
			t.Fatalf("candidate is not an object: %v", c)
		}
		if m["key"] == "" || m["value"] == "" {
			t.Fatalf("candidate missing key/value: %v", m)
		}
		ss, ok := m["source_state"].(map[string]interface{})
		if !ok {
			t.Fatalf("candidate missing source_state: %v", m)
		}
		// MemHarness: the source state (not just the value) must travel
		// with every candidate so the critique stage can judge staleness.
		for _, field := range []string{"type", "level", "source", "confidence", "created_at", "score"} {
			if _, present := ss[field]; !present {
				t.Fatalf("source_state missing %q: %v", field, ss)
			}
		}
	}

	// The critique prompt must be self-contained: candidates + current task
	// + explicit keep/rewrite/discard instructions + <EMPTY> contract.
	cp, _ := res["critique_prompt"].(string)
	if cp == "" {
		t.Fatal("critique_prompt missing")
	}
	for _, want := range []string{"blue-green deploys Jenkins", "keep|rewrite|discard", "<EMPTY>", "source_state"} {
		if !strings.Contains(cp, want) {
			t.Fatalf("critique_prompt missing %q:\n%.400s", want, cp)
		}
	}
	if !strings.Contains(cp, "deploy_style") {
		t.Fatal("critique_prompt must include the candidate keys")
	}
}

func TestMemoryNode_HarnessSearch_EmptyMemory(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "harness-empty-test"
	_ = mgr.GetSession(sessionID) // ensure the session exists, no entries

	n := &MemoryNode{}
	out, err := n.Execute(context.Background(), "fresh task", map[string]string{
		"operation":  "harness_search",
		"session_id": sessionID,
		"query":      "anything",
		"threshold":  "0.0",
	})
	if err != nil {
		t.Fatalf("harness_search on empty memory must succeed: %v", err)
	}
	var res map[string]interface{}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	if res["count"].(float64) != 0 {
		t.Fatalf("expected 0 candidates, got %v", res["count"])
	}
	cands, _ := res["candidates"].([]interface{})
	if len(cands) != 0 {
		t.Fatalf("expected empty candidates, got %v", cands)
	}
}

func TestMemoryNode_HarnessSearch_CritiquePromptIsRunnable(t *testing.T) {
	// The emitted critique_prompt must be usable verbatim as a downstream
	// LLM step's prompt: system instructions, then candidates, then the
	// task, in that order.
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "harness-runnable-test"
	s := mgr.GetSession(sessionID)
	_, _, _ = s.Store("k1", "value one", "long", "fact", nil, 24, 0.8, "test")

	n := &MemoryNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{
		"operation":  "harness_search",
		"session_id": sessionID,
		"query":      "value",
		"threshold":  "0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(out), &res)
	cp := res["critique_prompt"].(string)

	sysIdx := strings.Index(cp, "critique stage of a memory harness")
	candIdx := strings.Index(cp, "Retrieved memory candidates")
	taskIdx := strings.Index(cp, "# Current task")
	if sysIdx < 0 || candIdx < 0 || taskIdx < 0 {
		t.Fatalf("critique_prompt missing sections:\n%.400s", cp)
	}
	if !(sysIdx < candIdx && candIdx < taskIdx) {
		t.Fatalf("critique_prompt sections out of order (sys=%d cands=%d task=%d)", sysIdx, candIdx, taskIdx)
	}
}
