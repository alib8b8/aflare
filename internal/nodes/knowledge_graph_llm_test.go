// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌​‌​​‌‌​​‌​‌‌​​‌‌​​‌​‌‌​​‌​​​‌‌​‌‌​‌​​‌​​‌‌​‌​​​​​​​​​​​​​​​​​​​​‌‌​‌‌‌‌‌​‌‌​⁠
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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alib8b8/aflare/internal/memory"
)

// --- C-2: knowledge_graph extract_llm tests ---

// TestKGExtractLLM_HappyPath verifies the full pipeline: LLM returns
// entities and relations as JSON, the node parses and builds the graph.
func TestKGExtractLLM_HappyPath(t *testing.T) {
	llmResp := `{
		"entities": [
			{"name": "Alice", "type": "Person", "properties": {"role": "engineer"}},
			{"name": "Acme Corp", "type": "Organization"}
		],
		"relations": [
			{"from": "Alice", "to": "Acme Corp", "relation": "works_for", "confidence": 0.9}
		]
	}`
	srv := mockStructuredServer(t, llmResp)
	defer srv.Close()

	n := &KnowledgeGraphNode{}
	out, err := n.Execute(context.Background(), "Alice works at Acme Corp as an engineer", map[string]string{
		"action":   "extract_llm",
		"endpoint": srv.URL,
		"api_key":  "sk-test",
		"model":    "test-model",
		"format":   "json",
	})
	if err != nil {
		t.Fatalf("Execute extract_llm: %v", err)
	}

	var kg KnowledgeGraph
	if err := json.Unmarshal([]byte(out), &kg); err != nil {
		t.Fatalf("output not valid KG JSON: %v (out=%s)", err, out)
	}
	if _, ok := kg.Entities["Alice"]; !ok {
		t.Errorf("expected entity Alice, got entities: %v", kg.Entities)
	}
	if _, ok := kg.Entities["Acme Corp"]; !ok {
		t.Errorf("expected entity Acme Corp, got entities: %v", kg.Entities)
	}
	if len(kg.Relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(kg.Relations))
	}
	rel := kg.Relations[0]
	if rel.From != "Alice" || rel.To != "Acme Corp" || rel.Relation != "works_for" {
		t.Errorf("unexpected relation: %+v", rel)
	}
	if rel.Confidence != 0.9 {
		t.Errorf("confidence = %v, want 0.9", rel.Confidence)
	}
}

// TestKGExtractLLM_ParsesFencedJSON verifies the defensive parser
// handles ```json-fenced responses.
func TestKGExtractLLM_ParsesFencedJSON(t *testing.T) {
	fenced := "```json\n" + `{"entities":[{"name":"Bob","type":"Person"}],"relations":[]}` + "\n```"
	srv := mockStructuredServer(t, fenced)
	defer srv.Close()

	n := &KnowledgeGraphNode{}
	out, err := n.Execute(context.Background(), "Bob is a person", map[string]string{
		"action":   "extract_llm",
		"endpoint": srv.URL,
		"api_key":  "sk-test",
		"model":    "test-model",
		"format":   "json",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var kg KnowledgeGraph
	if err := json.Unmarshal([]byte(out), &kg); err != nil {
		t.Fatalf("output not valid KG JSON: %v (out=%s)", err, out)
	}
	if _, ok := kg.Entities["Bob"]; !ok {
		t.Errorf("expected entity Bob, got: %v", kg.Entities)
	}
}

// TestKGExtractLLM_ParsesJSONWithPreamble verifies the brace-matching
// fallback extracts JSON even when the model prepends prose.
func TestKGExtractLLM_ParsesJSONWithPreamble(t *testing.T) {
	withPreamble := `Here is the extracted graph:
{"entities":[{"name":"Carol","type":"Person"}],"relations":[]}
Hope this helps!`
	srv := mockStructuredServer(t, withPreamble)
	defer srv.Close()

	n := &KnowledgeGraphNode{}
	out, err := n.Execute(context.Background(), "Carol is a person", map[string]string{
		"action":   "extract_llm",
		"endpoint": srv.URL,
		"api_key":  "sk-test",
		"model":    "test-model",
		"format":   "json",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var kg KnowledgeGraph
	if err := json.Unmarshal([]byte(out), &kg); err != nil {
		t.Fatalf("output not valid KG JSON: %v (out=%s)", err, out)
	}
	if _, ok := kg.Entities["Carol"]; !ok {
		t.Errorf("expected entity Carol, got: %v", kg.Entities)
	}
}

// TestKGExtractLLM_RejectsEmptyInput ensures the node errors when no
// text is provided for extraction.
func TestKGExtractLLM_RejectsEmptyInput(t *testing.T) {
	n := &KnowledgeGraphNode{}
	_, err := n.Execute(context.Background(), "   ", map[string]string{
		"action":  "extract_llm",
		"api_key": "sk-test",
	})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !strings.Contains(err.Error(), "input text is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestKGExtractLLM_MergesIntoExistingGraph verifies that when graph_path
// points to an existing graph, extraction is additive.
func TestKGExtractLLM_MergesIntoExistingGraph(t *testing.T) {
	// Save/Load use validateWritePath/validateReadPath which require
	// relative paths within workDir. Set workDir to a temp dir.
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	graphPath := "kg.json"
	existing := NewKnowledgeGraph()
	existing.AddEntity("Dave", "Person", nil)
	if err := existing.Save(graphPath); err != nil {
		t.Fatalf("Save initial: %v", err)
	}

	llmResp := `{"entities":[{"name":"Eve","type":"Person"}],"relations":[{"from":"Dave","to":"Eve","relation":"knows","confidence":0.8}]}`
	srv := mockStructuredServer(t, llmResp)
	defer srv.Close()

	n := &KnowledgeGraphNode{}
	_, err := n.Execute(context.Background(), "Dave knows Eve", map[string]string{
		"action":     "extract_llm",
		"graph_path": graphPath,
		"endpoint":   srv.URL,
		"api_key":    "sk-test",
		"model":      "test-model",
		"format":     "json",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	loaded, err := LoadKnowledgeGraph(graphPath)
	if err != nil {
		t.Fatalf("LoadKnowledgeGraph: %v", err)
	}
	if _, ok := loaded.Entities["Dave"]; !ok {
		t.Error("expected pre-existing entity Dave to survive merge")
	}
	if _, ok := loaded.Entities["Eve"]; !ok {
		t.Error("expected new entity Eve to be added")
	}
	foundKnows := false
	for _, r := range loaded.Relations {
		if r.From == "Dave" && r.To == "Eve" && r.Relation == "knows" {
			foundKnows = true
		}
	}
	if !foundKnows {
		t.Error("expected Dave-knows-Eve relation to be present after merge")
	}
}

// TestKGExtractLLM_LinksToMemory verifies the C-3 integration: when
// memory_key and session_id are supplied, extracted entities are linked
// to that memory entry.
func TestKGExtractLLM_LinksToMemory(t *testing.T) {
	// First store a memory entry to link to.
	storeNode := &MemoryNode{}
	storeOut, err := storeNode.Execute(context.Background(), "Alice works at Acme Corp", map[string]string{
		"operation":  "store",
		"key":        "alice_fact",
		"session_id": "kg-link-test",
		"level":      "long",
	})
	if err != nil {
		t.Fatalf("store: %v (out=%s)", err, storeOut)
	}

	llmResp := `{"entities":[{"name":"Alice","type":"Person"},{"name":"Acme Corp","type":"Organization"}],"relations":[{"from":"Alice","to":"Acme Corp","relation":"works_for","confidence":0.9}]}`
	srv := mockStructuredServer(t, llmResp)
	defer srv.Close()

	n := &KnowledgeGraphNode{}
	_, err = n.Execute(context.Background(), "Alice works at Acme Corp", map[string]string{
		"action":     "extract_llm",
		"endpoint":   srv.URL,
		"api_key":    "sk-test",
		"model":      "test-model",
		"format":     "json",
		"session_id": "kg-link-test",
		"memory_key": "alice_fact",
	})
	if err != nil {
		t.Fatalf("Execute extract_llm with link: %v", err)
	}

	// Verify the link via expand_kg.
	expandNode := &MemoryNode{}
	expandOut, err := expandNode.Execute(context.Background(), "", map[string]string{
		"operation":  "expand_kg",
		"session_id": "kg-link-test",
		"query":      "Alice",
		"top_k":      "10",
		"threshold":  "0.0",
	})
	if err != nil {
		t.Fatalf("expand_kg: %v", err)
	}
	var expandResult map[string]interface{}
	if err := json.Unmarshal([]byte(expandOut), &expandResult); err != nil {
		t.Fatalf("expand_kg output not JSON: %v", err)
	}
	entities, _ := expandResult["kg_entities"].([]interface{})
	if len(entities) < 2 {
		t.Errorf("expected at least 2 linked KG entities, got %d (%v)", len(entities), entities)
	}
}

// TestParseKGExtraction_Unit directly exercises the parser with tricky
// inputs (this is a unit test for the parse function).
func TestParseKGExtraction_Unit(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
		wantEnt int
		wantRel int
	}{
		{"clean", `{"entities":[{"name":"A"}],"relations":[]}`, false, 1, 0},
		{"fenced", "```json\n" + `{"entities":[],"relations":[]}` + "\n```", false, 0, 0},
		{"preamble", `sure! {"entities":[{"name":"A"},{"name":"B"}],"relations":[{"from":"A","to":"B","relation":"r"}]} done`, false, 2, 1},
		{"no_json", `just plain text`, true, 0, 0},
		{"empty", ``, true, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ex, err := parseKGExtraction(c.input)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(ex.Entities) != c.wantEnt {
				t.Errorf("entities = %d, want %d", len(ex.Entities), c.wantEnt)
			}
			if len(ex.Relations) != c.wantRel {
				t.Errorf("relations = %d, want %d", len(ex.Relations), c.wantRel)
			}
		})
	}
}

// --- C-4: memory compression tests ---

// TestCompress_NoopWhenUnderBudget verifies that compression does
// nothing when total tokens are within budget.
func TestCompress_NoopWhenUnderBudget(t *testing.T) {
	// Use an isolated session via the manager so we don't collide with
	// other tests' global state.
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "compress-noop"
	s := mgr.GetSession(sessionID)

	// Store a small low-confidence entry (well under the budget).
	_, _, _ = s.Store("small", "tiny content", "long", "fact", nil, 24, 0.2, "test")

	n := &MemoryNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{
		"operation":      "compress",
		"session_id":     sessionID,
		"token_budget":   "10000",
		"min_confidence": "0.5",
		"level":          "long",
	})
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(out), &res)
	if res["status"] != "noop" {
		t.Errorf("expected status noop, got %v (full: %s)", res["status"], out)
	}
	if res["compressed"] != float64(0) {
		t.Errorf("expected 0 compressed, got %v", res["compressed"])
	}
}

// TestCompress_NoopWhenNoLowConfidenceEntries verifies that high-
// confidence entries are NOT compressed.
func TestCompress_NoopWhenNoLowConfidenceEntries(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "compress-highconf"
	s := mgr.GetSession(sessionID)
	_, _, _ = s.Store("hi", "high confidence content that is reasonably long to add tokens", "long", "fact", nil, 24, 0.95, "test")

	n := &MemoryNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{
		"operation":      "compress",
		"session_id":     sessionID,
		"token_budget":   "1",
		"min_confidence": "0.5",
		"level":          "long",
	})
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(out), &res)
	if res["status"] != "noop" {
		t.Errorf("expected noop (no low-conf entries), got %v (out=%s)", res["status"], out)
	}
}

// TestCompress_DeterministicFallback verifies the offline compression
// path: when no LLM is configured (no api_key, no server), the
// deterministic truncation fallback still compresses entries and the
// originals are deleted.
func TestCompress_DeterministicFallback(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "compress-det"
	s := mgr.GetSession(sessionID)

	// Store several low-confidence entries whose combined size exceeds
	// the small budget, forcing compression.
	for i := 0; i < 5; i++ {
		_, _, _ = s.Store(
			"low"+string(rune('a'+i)),
			strings.Repeat("memory content about topic number ", 10)+string(rune('a'+i)),
			"long", "fact", nil, 24, 0.2, "test",
		)
	}

	n := &MemoryNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{
		"operation":      "compress",
		"session_id":     sessionID,
		"token_budget":   "50", // small budget forces compression
		"min_confidence": "0.5",
		"level":          "long",
		// No api_key/endpoint -> LLM call fails -> deterministic fallback.
	})
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(out), &res)
	if res["status"] != "success" {
		t.Fatalf("expected status success, got %v (out=%s)", res["status"], out)
	}
	if res["llm_used"] != false {
		t.Errorf("expected llm_used=false (deterministic fallback), got %v", res["llm_used"])
	}
	deleted, _ := res["deleted_keys"].([]interface{})
	if len(deleted) == 0 {
		t.Error("expected at least one deleted key")
	}
	// The compressed entry should exist and be smaller.
	origTokens := int(res["original_tokens"].(float64))
	compTokens := int(res["compressed_tokens"].(float64))
	if compTokens >= origTokens {
		t.Errorf("compression did not reduce tokens: orig=%d comp=%d", origTokens, compTokens)
	}
	// Retention ratio should be < 1.
	retention := res["retention_ratio"].(float64)
	if retention >= 1.0 {
		t.Errorf("retention_ratio = %v, want < 1.0", retention)
	}
}

// TestCompress_LLMCompression verifies the LLM path: a mock server
// returns a summary, and the originals are replaced.
func TestCompress_LLMCompression(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "compress-llm"
	s := mgr.GetSession(sessionID)

	_, _, _ = s.Store("a", strings.Repeat("Alice works at Acme Corp on the Go team. ", 5), "long", "fact", nil, 24, 0.3, "test")
	_, _, _ = s.Store("b", strings.Repeat("Bob works at Beta Inc on the Rust team. ", 5), "long", "fact", nil, 24, 0.3, "test")

	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		_, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "Alice and Bob work on Go and Rust teams respectively."}},
			},
		})
	}))
	defer srv.Close()

	n := &MemoryNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{
		"operation":      "compress",
		"session_id":     sessionID,
		"token_budget":   "10",
		"min_confidence": "0.5",
		"level":          "long",
		"endpoint":       srv.URL,
		"api_key":        "sk-test",
		"model":          "test-model",
	})
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(out), &res)
	if res["status"] != "success" {
		t.Fatalf("expected success, got %v (out=%s)", res["status"], out)
	}
	if res["llm_used"] != true {
		t.Errorf("expected llm_used=true, got %v", res["llm_used"])
	}
	if atomic.LoadInt32(&callCount) == 0 {
		t.Error("expected the LLM mock to be called at least once")
	}
}

// TestCompress_PreservesHighConfidenceEntries verifies that high-
// confidence entries are NOT deleted even when low-confidence ones are.
func TestCompress_PreservesHighConfidenceEntries(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "compress-preserve"
	s := mgr.GetSession(sessionID)

	_, _, _ = s.Store("low1", strings.Repeat("low confidence content one ", 8), "long", "fact", nil, 24, 0.2, "test")
	_, _, _ = s.Store("high1", "important high confidence fact that must be preserved", "long", "fact", nil, 24, 0.95, "test")
	_, _, _ = s.Store("low2", strings.Repeat("low confidence content two ", 8), "long", "fact", nil, 24, 0.2, "test")

	n := &MemoryNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{
		"operation":      "compress",
		"session_id":     sessionID,
		"token_budget":   "20",
		"min_confidence": "0.5",
		"level":          "long",
	})
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(out), &res)

	deleted, _ := res["deleted_keys"].([]interface{})
	deletedSet := make(map[string]bool, len(deleted))
	for _, d := range deleted {
		deletedSet[d.(string)] = true
	}
	if deletedSet["high1"] {
		t.Error("high-confidence entry 'high1' was deleted by compression")
	}
	// The high-confidence entry should still be retrievable.
	if _, err := s.Retrieve("high1"); err != nil {
		t.Errorf("expected high1 to survive compression, got: %v", err)
	}
}

// --- C-3: memory node link_kg / expand_kg operation tests ---

func TestMemoryNode_LinkKGAndExpand(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "link-expand-test"
	s := mgr.GetSession(sessionID)
	_, _, _ = s.Store("m1", "content about Go and Docker", "long", "fact", nil, 24, 0.9, "test")
	_, _, _ = s.Store("m2", "content about Rust", "long", "fact", nil, 24, 0.9, "test")

	n := &MemoryNode{}

	// Link m1 to Go and Docker.
	out, err := n.Execute(context.Background(), "", map[string]string{
		"operation":   "link_kg",
		"session_id":  sessionID,
		"key":         "m1",
		"kg_entities": "Go, Docker, Kubernetes",
	})
	if err != nil {
		t.Fatalf("link_kg: %v (out=%s)", err, out)
	}
	var linkRes map[string]interface{}
	_ = json.Unmarshal([]byte(out), &linkRes)
	if linkRes["total_links"].(float64) != 3 {
		t.Errorf("expected 3 links, got %v", linkRes["total_links"])
	}

	// Expand: search for "content" (matches both), expand KG.
	expandOut, err := n.Execute(context.Background(), "", map[string]string{
		"operation":  "expand_kg",
		"session_id": sessionID,
		"query":      "content",
		"top_k":      "10",
		"threshold":  "0.0",
	})
	if err != nil {
		t.Fatalf("expand_kg: %v", err)
	}
	var expRes map[string]interface{}
	_ = json.Unmarshal([]byte(expandOut), &expRes)
	entities, _ := expRes["kg_entities"].([]interface{})
	if len(entities) != 3 {
		t.Errorf("expected 3 expanded entities, got %d (%v)", len(entities), entities)
	}
}

func TestMemoryNode_LinkKG_RequiresKey(t *testing.T) {
	n := &MemoryNode{}
	_, err := n.Execute(context.Background(), "", map[string]string{
		"operation":   "link_kg",
		"kg_entities": "Entity1",
	})
	if err == nil {
		t.Fatal("expected error when key missing for link_kg")
	}
	if !strings.Contains(err.Error(), "key is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMemoryNode_LinkKG_RequiresEntities(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "link-req-ent"
	s := mgr.GetSession(sessionID)
	_, _, _ = s.Store("k", "v", "long", "fact", nil, 24, 0.9, "test")

	n := &MemoryNode{}
	_, err := n.Execute(context.Background(), "", map[string]string{
		"operation":  "link_kg",
		"session_id": sessionID,
		"key":        "k",
	})
	if err == nil {
		t.Fatal("expected error when kg_entities missing")
	}
	if !strings.Contains(err.Error(), "kg_entities") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- helpers ---

// newIsolatedMemoryMgr returns the global memory manager. The global
// manager is process-wide, so tests use unique session IDs (derived
// from the test name) to avoid colliding with each other. We return
// the manager so tests can call GetSession(sessionID) to seed entries
// before invoking the MemoryNode (which uses the same global).
func newIsolatedMemoryMgr(t testing.TB) *memory.SessionMemoryManager {
	t.Helper()
	return memory.GlobalSessionManager
}
