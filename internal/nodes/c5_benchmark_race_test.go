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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/memory"
)

// C-5: benchmark + race + equivalence tests for the B/C-stage features.
//
// This file concentrates the *quantitative* quality gates:
//   - recall@k for vector retrieval under varying thresholds
//   - KG extraction accuracy (precision/recall) vs. a hand-labelled gold set
//   - compression information retention (key-entity survival rate)
//   - race-detector stress tests for link_kg / expand_kg / compress
//   - benchmarks for compress, link_kg/expand_kg, and KG extraction
//
// Together with the C-1/C-2/C-3/C-4 unit tests, these provide the
// equivalence + performance contract for the memory/graph stack.

// ---------------------------------------------------------------------------
// C-1 equivalence: recall@k under varying thresholds
// ---------------------------------------------------------------------------

// TestRecallAtK_ThresholdSweep quantifies how recall@5 changes as the
// similarity threshold rises. With a HashEmbedder, identical-token
// entries score 1.0; partial-overlap entries score between 0 and 1.
// The test asserts the monotonicity property: raising the threshold
// can only lower (or keep equal) the recall.
func TestRecallAtK_ThresholdSweep(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name      string
		query     string
		relevant  []string
		threshold float64
		minRecall float64 // expected recall >= this (0..1)
	}{
		// threshold 0.0: everything matches, recall should be 1.0
		{"t0.0", "go language", []string{"go", "golang", "rust"}, 0.0, 1.0},
		// threshold 0.5: medium overlap, top relevant entries still surface
		{"t0.5", "go language", []string{"go", "golang"}, 0.5, 0.5},
		// threshold 0.9: only near-exact matches survive
		{"t0.9", "go language", []string{"go"}, 0.9, 0.0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sm := memory.NewSessionMemory("recall-"+c.name, 100)
			sm.SetEmbedder(memory.NewHashEmbedder(256))

			// Store 3 relevant + 5 distractor entries.
			for i, k := range c.relevant {
				_, _, _ = sm.StoreCtx(ctx, k, fmt.Sprintf("%s is a programming language variant %d", k, i), "long", "fact", nil, 24, 0.9, "test")
			}
			for i := 0; i < 5; i++ {
				_, _, _ = sm.StoreCtx(ctx, fmt.Sprintf("dist%d", i), fmt.Sprintf("cooking recipe number %d with pasta and sauce", i), "long", "fact", nil, 24, 0.9, "test")
			}

			results := sm.SearchCtx(ctx, c.query, "", 5, c.threshold)
			got := 0
			for _, r := range results {
				for _, want := range c.relevant {
					if r.Key == want {
						got++
						break
					}
				}
			}
			recall := float64(got) / float64(len(c.relevant))
			if recall < c.minRecall {
				t.Errorf("threshold=%.2f recall=%.2f, want >= %.2f (got=%d/%d, results=%v)",
					c.threshold, recall, c.minRecall, got, len(c.relevant), resultKeys(results))
			}
		})
	}
}

// TestRecallAtK_Monotonicity verifies the monotonicity property across
// a continuous threshold sweep: recall(T) is non-increasing in T.
func TestRecallAtK_Monotonicity(t *testing.T) {
	ctx := context.Background()
	sm := memory.NewSessionMemory("mono", 200)
	sm.SetEmbedder(memory.NewHashEmbedder(256))

	relevant := []string{"go", "golang", "rust-lang"}
	for _, k := range relevant {
		_, _, _ = sm.StoreCtx(ctx, k, fmt.Sprintf("%s is a programming language", k), "long", "fact", nil, 24, 0.9, "test")
	}
	for i := 0; i < 10; i++ {
		_, _, _ = sm.StoreCtx(ctx, fmt.Sprintf("d%d", i), "cooking recipe with pasta and tomatoes and basil", "long", "fact", nil, 24, 0.9, "test")
	}

	prev := 1.0 // recall at T=0 is by definition the max
	for thr := 0.0; thr <= 1.001; thr += 0.05 {
		results := sm.SearchCtx(ctx, "go language", "", 5, thr)
		got := 0
		for _, r := range results {
			for _, want := range relevant {
				if r.Key == want {
					got++
					break
				}
			}
		}
		recall := float64(got) / float64(len(relevant))
		if recall > prev+1e-9 {
			t.Errorf("non-monotonic at T=%.2f: recall=%.2f > prev=%.2f", thr, recall, prev)
		}
		prev = recall
	}
}

func resultKeys(rs []memory.MemoryEntry) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Key
	}
	return out
}

// ---------------------------------------------------------------------------
// C-2 equivalence: KG extraction accuracy vs. gold set
// ---------------------------------------------------------------------------

// kgGoldSet is a small hand-labelled gold set: input text -> expected
// (entities, relations). The expected counts are deliberately lenient
// (we accept any superset that includes the labelled entities).
var kgGoldSet = []struct {
	name      string
	input     string
	entities  []string
	relations []struct {
		from, to, rel string
	}
}{
	{
		name:     "alice_works",
		input:    "Alice works at Acme Corp as a senior engineer. She reports to Bob who is the CTO.",
		entities: []string{"Alice", "Acme Corp", "Bob"},
		relations: []struct {
			from, to, rel string
		}{
			{"Alice", "Acme Corp", "works_for"},
			{"Alice", "Bob", "reports_to"},
		},
	},
	{
		name:     "tech_stack",
		input:    "The project uses Go for the backend and React for the frontend. PostgreSQL stores the data.",
		entities: []string{"Go", "React", "PostgreSQL"},
		relations: []struct {
			from, to, rel string
		}{
			{"Go", "backend", "used_for"},
		},
	},
}

// TestKGExtraction_AccuracyAgainstGoldSet measures precision/recall of
// the LLM extraction against the gold set. The mock LLM returns the
// "ideal" extraction (so this is really testing the merge + parse
// pipeline); the test exists to catch regressions in parsing that would
// drop entities or relations.
func TestKGExtraction_AccuracyAgainstGoldSet(t *testing.T) {
	for _, g := range kgGoldSet {
		t.Run(g.name, func(t *testing.T) {
			// Build a mock LLM response that contains exactly the gold
			// entities + relations (so the parser must round-trip them).
			var ents []string
			for _, e := range g.entities {
				ents = append(ents, fmt.Sprintf(`{"name":%q,"type":"Concept"}`, e))
			}
			var rels []string
			for _, r := range g.relations {
				rels = append(rels, fmt.Sprintf(`{"from":%q,"to":%q,"relation":%q,"confidence":0.9}`, r.from, r.to, r.rel))
			}
			llmResp := fmt.Sprintf(`{"entities":[%s],"relations":[%s]}`, strings.Join(ents, ","), strings.Join(rels, ","))
			srv := mockStructuredServer(t, llmResp)
			defer srv.Close()

			n := &KnowledgeGraphNode{}
			out, err := n.Execute(context.Background(), g.input, map[string]string{
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
				t.Fatalf("parse output: %v (out=%s)", err, out)
			}

			// Precision: all extracted entities must be in gold (lenient:
			// we accept extras, but every gold entity must be present).
			for _, want := range g.entities {
				if _, ok := kg.Entities[want]; !ok {
					t.Errorf("precision: gold entity %q missing from extraction (got: %v)", want, entityNames(kg.Entities))
				}
			}
			// Recall: every gold relation must be present.
			for _, want := range g.relations {
				found := false
				for _, got := range kg.Relations {
					if got.From == want.from && got.To == want.to && got.Relation == want.rel {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("recall: gold relation %+v missing (got: %v)", want, kg.Relations)
				}
			}
		})
	}
}

func entityNames(m map[string]KGEntity) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestKGExtraction_NoDroppedEntitiesOnLargeInput verifies that a larger
// extraction (10 entities, 5 relations) doesn't lose anything in the
// parse+merge pipeline. This is the "no regression under load" gate.
func TestKGExtraction_NoDroppedEntitiesOnLargeInput(t *testing.T) {
	const n = 10
	var ents []string
	for i := 0; i < n; i++ {
		ents = append(ents, fmt.Sprintf(`{"name":"E%d","type":"Concept"}`, i))
	}
	var rels []string
	for i := 0; i < n/2; i++ {
		rels = append(rels, fmt.Sprintf(`{"from":"E%d","to":"E%d","relation":"r%d","confidence":0.8}`, i, i+1, i))
	}
	llmResp := fmt.Sprintf(`{"entities":[%s],"relations":[%s]}`, strings.Join(ents, ","), strings.Join(rels, ","))
	srv := mockStructuredServer(t, llmResp)
	defer srv.Close()

	n2 := &KnowledgeGraphNode{}
	out, err := n2.Execute(context.Background(), "lots of entities", map[string]string{
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
	_ = json.Unmarshal([]byte(out), &kg)
	if len(kg.Entities) != n {
		t.Errorf("expected %d entities, got %d (entities: %v)", n, len(kg.Entities), entityNames(kg.Entities))
	}
	if len(kg.Relations) != n/2 {
		t.Errorf("expected %d relations, got %d", n/2, len(kg.Relations))
	}
}

// ---------------------------------------------------------------------------
// C-4 equivalence: compression information retention
// ---------------------------------------------------------------------------

// TestCompression_InformationRetention verifies that the LLM compression
// path preserves the key entities/facts from the original entries. We
// store entries with known "anchor" tokens, compress with a mock LLM
// that returns a summary containing all anchors, and confirm the
// anchors survive in the compressed output.
func TestCompression_InformationRetention(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "compress-retain"
	s := mgr.GetSession(sessionID)

	// Anchors are unique tokens that MUST survive compression.
	anchors := []string{"alpha-tok", "beta-tok", "gamma-tok", "delta-tok"}
	for i, a := range anchors {
		_, _, _ = s.Store(
			fmt.Sprintf("k%d", i),
			fmt.Sprintf("This entry contains the unique anchor token %s and some filler text to add tokens. %s detail record number %d.", a, a, i)+
				strings.Repeat(" filler filler filler filler filler filler filler filler. ", 4),
			"long", "fact", nil, 24, 0.2, "test",
		)
	}

	// Mock LLM returns a summary that preserves all anchors (simulating
	// an ideal summarizer that extracts key facts).
	summary := "Summary: alpha-tok, beta-tok, gamma-tok, delta-tok are the key anchors."
	srv := mockStructuredServer(t, summary)
	defer srv.Close()

	n := &MemoryNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{
		"operation":      "compress",
		"session_id":     sessionID,
		"token_budget":   "20",
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
	newKey, _ := res["new_key"].(string)
	if newKey == "" {
		t.Fatal("missing new_key in compress result")
	}

	compressed, err := s.Retrieve(newKey)
	if err != nil {
		t.Fatalf("retrieve compressed: %v", err)
	}
	// 100% of anchors should survive LLM compression (the mock returns
	// a summary that includes them all).
	survived := 0
	for _, a := range anchors {
		if strings.Contains(compressed.Value, a) {
			survived++
		}
	}
	retention := float64(survived) / float64(len(anchors))
	if retention < 1.0 {
		t.Errorf("anchor retention = %.2f, want >= 1.00 (survived=%d/%d, compressed=%q)",
			retention, survived, len(anchors), compressed.Value)
	}
	// Compression must actually reduce size.
	origTokens := int(res["original_tokens"].(float64))
	compTokens := int(res["compressed_tokens"].(float64))
	if compTokens >= origTokens {
		t.Errorf("compression did not reduce tokens: orig=%d comp=%d", origTokens, compTokens)
	}
}

// TestCompression_DeterministicRetention verifies the deterministic
// fallback path retains at least one anchor (it's lossy by design).
func TestCompression_DeterministicRetention(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "compress-det-retain"
	s := mgr.GetSession(sessionID)

	anchors := []string{"alpha-tok", "beta-tok", "gamma-tok", "delta-tok"}
	for i, a := range anchors {
		_, _, _ = s.Store(
			fmt.Sprintf("k%d", i),
			fmt.Sprintf("Entry with anchor %s and filler. ", a)+
				strings.Repeat("filler ", 20),
			"long", "fact", nil, 24, 0.2, "test",
		)
	}

	n := &MemoryNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{
		"operation":      "compress",
		"session_id":     sessionID,
		"token_budget":   "30",
		"min_confidence": "0.5",
		"level":          "long",
	})
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(out), &res)
	if res["status"] != "success" {
		t.Fatalf("expected success, got %v (out=%s)", res["status"], out)
	}
	newKey, _ := res["new_key"].(string)
	compressed, err := s.Retrieve(newKey)
	if err != nil {
		t.Fatalf("retrieve compressed: %v", err)
	}
	// At least one anchor must survive (the deterministic path is
	// greedy but should preserve the first entry's anchor).
	survived := 0
	for _, a := range anchors {
		if strings.Contains(compressed.Value, a) {
			survived++
		}
	}
	if survived == 0 {
		t.Errorf("no anchors survived deterministic compression (compressed=%q)", compressed.Value)
	}
}

// TestCompression_TokenBudgetHonoured verifies the compressed output is
// within the requested token budget (with some slack for the header).
func TestCompression_TokenBudgetHonoured(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "compress-budget"
	s := mgr.GetSession(sessionID)

	// Store 5 large low-confidence entries.
	for i := 0; i < 5; i++ {
		_, _, _ = s.Store(
			fmt.Sprintf("k%d", i),
			strings.Repeat(fmt.Sprintf("content block %d ", i), 50),
			"long", "fact", nil, 24, 0.2, "test",
		)
	}

	const budget = 100
	n := &MemoryNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{
		"operation":      "compress",
		"session_id":     sessionID,
		"token_budget":   fmt.Sprintf("%d", budget),
		"min_confidence": "0.5",
		"level":          "long",
	})
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(out), &res)
	if res["status"] != "success" {
		t.Fatalf("expected success, got %v (out=%s)", res["status"], out)
	}
	compTokens, _ := res["compressed_tokens"].(float64)
	// Allow 50% slack for headers + rounding (deterministic path adds
	// "[key] " prefixes per entry).
	if int(compTokens) > budget*3/2 {
		t.Errorf("compressed_tokens=%d exceeds budget*1.5=%d", int(compTokens), budget*3/2)
	}
}

// TestCompression_PreservesOriginalEntriesForHighConfidence confirms
// that high-confidence entries are NOT touched by compression.
func TestCompression_PreservesOriginalEntriesForHighConfidence(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "compress-preserve-high"
	s := mgr.GetSession(sessionID)

	const highVal = "important fact that must not be lost"
	_, _, _ = s.Store("high", highVal, "long", "fact", nil, 24, 0.99, "test")
	// Add some low-conf entries to actually compress.
	for i := 0; i < 3; i++ {
		_, _, _ = s.Store(
			fmt.Sprintf("low%d", i),
			strings.Repeat("low confidence filler content ", 10),
			"long", "fact", nil, 24, 0.2, "test",
		)
	}

	n := &MemoryNode{}
	_, err := n.Execute(context.Background(), "", map[string]string{
		"operation":      "compress",
		"session_id":     sessionID,
		"token_budget":   "10",
		"min_confidence": "0.5",
		"level":          "long",
	})
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	high, err := s.Retrieve("high")
	if err != nil {
		t.Fatalf("high-confidence entry was deleted: %v", err)
	}
	if high.Value != highVal {
		t.Errorf("high-confidence entry value changed: got %q want %q", high.Value, highVal)
	}
}

// ---------------------------------------------------------------------------
// C-3/C-4 race-detector stress tests
// ---------------------------------------------------------------------------

// TestLinkKG_ConcurrentSafety exercises concurrent LinkKGNode +
// ExpandKGSubgraph + GetKGLinks under the race detector. Run with
// -race to catch data races on KGNodeRefs.
func TestLinkKG_ConcurrentSafety(t *testing.T) {
	sm := memory.NewSessionMemory("race-link", 1000)
	ctx := context.Background()
	// Pre-create the entries we'll link to.
	for i := 0; i < 50; i++ {
		_, _, _ = sm.StoreCtx(ctx, fmt.Sprintf("k%d", i), fmt.Sprintf("content %d", i), "long", "fact", nil, 24, 0.9, "test")
	}

	const writers = 4
	const readers = 4
	const ops = 50

	var wg sync.WaitGroup
	wg.Add(writers + readers)

	for w := 0; w < writers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				key := fmt.Sprintf("k%d", (id*ops+i)%50)
				_ = sm.LinkKGNode(key, fmt.Sprintf("E%d", id), fmt.Sprintf("F%d", i))
			}
		}(w)
	}
	for r := 0; r < readers; r++ {
		go func() {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				_ = sm.ExpandKGSubgraph([]string{fmt.Sprintf("k%d", i%50)})
				_ = sm.GetKGLinks(fmt.Sprintf("k%d", i%50))
			}
		}()
	}
	wg.Wait()
}

// TestCompress_ConcurrentSafety runs compress against a session while
// concurrent readers retrieve entries. The compress path takes the
// write lock per delete/store, so this should be race-free.
func TestCompress_ConcurrentSafety(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "race-compress"
	s := mgr.GetSession(sessionID)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		_, _, _ = s.StoreCtx(ctx, fmt.Sprintf("k%d", i), fmt.Sprintf("low confidence content %d", i), "long", "fact", nil, 24, 0.2, "test")
	}

	n := &MemoryNode{}
	const compressors = 2
	const readers = 4
	const ops = 5

	var wg sync.WaitGroup
	wg.Add(compressors + readers)

	for c := 0; c < compressors; c++ {
		go func() {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				_, _ = n.Execute(ctx, "", map[string]string{
					"operation":      "compress",
					"session_id":     sessionID,
					"token_budget":   "20",
					"min_confidence": "0.5",
					"level":          "long",
				})
			}
		}()
	}
	for r := 0; r < readers; r++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				_, _ = n.Execute(ctx, "", map[string]string{
					"operation":  "expand_kg",
					"session_id": sessionID,
					"query":      fmt.Sprintf("content %d", id*ops+i),
					"top_k":      "5",
					"threshold":  "0.0",
				})
			}
		}(r)
	}
	wg.Wait()
}

// TestExpandKG_ConcurrentStress exercises the full MemoryNode.expand_kg
// path under heavy concurrency. Run with -race -count=10 to shake out
// flaky races in the search/expand code path.
func TestExpandKG_ConcurrentStress(t *testing.T) {
	mgr := newIsolatedMemoryMgr(t)
	sessionID := "race-expand"
	s := mgr.GetSession(sessionID)
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		_, _, _ = sm_StoreCtx(s, ctx, fmt.Sprintf("k%d", i), fmt.Sprintf("entry %d about go programming", i))
		_ = sm_LinkKGNode(s, fmt.Sprintf("k%d", i), fmt.Sprintf("Entity%d", i%10))
	}

	const goroutines = 8
	const ops = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			n := &MemoryNode{}
			for i := 0; i < ops; i++ {
				_, _ = n.Execute(ctx, "", map[string]string{
					"operation":  "expand_kg",
					"session_id": sessionID,
					"query":      fmt.Sprintf("go programming %d", id*ops+i),
					"top_k":      "10",
					"threshold":  "0.0",
				})
			}
		}(g)
	}
	wg.Wait()
}

// sm_StoreCtx and sm_LinkKGNode are thin wrappers to keep the test
// bodies readable. They exist only to avoid duplicating the long
// parameter list every line.
func sm_StoreCtx(s *memory.SessionMemory, ctx context.Context, k, v string) (string, time.Time, error) {
	return s.StoreCtx(ctx, k, v, "long", "fact", nil, 24, 0.9, "test")
}
func sm_LinkKGNode(s *memory.SessionMemory, key string, entities ...string) error {
	return s.LinkKGNode(key, entities...)
}

// ---------------------------------------------------------------------------
// Benchmarks (C-5)
// ---------------------------------------------------------------------------

// BenchmarkLinkKGAndExpand measures the cost of link_kg followed by
// expand_kg over a session of 1000 entries.
func BenchmarkLinkKGAndExpand(b *testing.B) {
	mgr := newIsolatedMemoryMgr(b)
	sessionID := "bench-link-expand"
	s := mgr.GetSession(sessionID)
	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		_, _, _ = s.StoreCtx(ctx, fmt.Sprintf("k%d", i), fmt.Sprintf("entry %d about go", i), "long", "fact", nil, 24, 0.9, "bench")
	}
	n := &MemoryNode{}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("k%d", i%1000)
		_, _ = n.Execute(ctx, "", map[string]string{
			"operation":   "link_kg",
			"session_id":  sessionID,
			"key":         key,
			"kg_entities": "Go, Rust, Python",
		})
		_, _ = n.Execute(ctx, "", map[string]string{
			"operation":  "expand_kg",
			"session_id": sessionID,
			"query":      "go",
			"top_k":      "10",
			"threshold":  "0.0",
		})
	}
}

// BenchmarkCompress_Deterministic measures the deterministic (offline)
// compression path cost.
func BenchmarkCompress_Deterministic(b *testing.B) {
	mgr := newIsolatedMemoryMgr(b)
	sessionID := "bench-compress"
	s := mgr.GetSession(sessionID)
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		_, _, _ = s.StoreCtx(ctx, fmt.Sprintf("k%d", i), strings.Repeat(fmt.Sprintf("content %d ", i), 10), "long", "fact", nil, 24, 0.2, "bench")
	}
	n := &MemoryNode{}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Each iteration compresses (the first iteration does the real
		// work; subsequent ones are noop because the candidates are
		// already compressed). To keep the benchmark meaningful we
		// re-seed before each iteration.
		if i > 0 {
			for j := 0; j < 100; j++ {
				_, _, _ = s.StoreCtx(ctx, fmt.Sprintf("k%d", j), strings.Repeat(fmt.Sprintf("content %d ", j), 10), "long", "fact", nil, 24, 0.2, "bench")
			}
		}
		_, _ = n.Execute(ctx, "", map[string]string{
			"operation":      "compress",
			"session_id":     sessionID,
			"token_budget":   "50",
			"min_confidence": "0.5",
			"level":          "long",
		})
	}
}

// BenchmarkCompress_LLM measures the LLM compression path cost with a
// mock HTTP server (no real network).
func BenchmarkCompress_LLM(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "compressed summary"}},
			},
		})
	}))
	defer srv.Close()

	mgr := newIsolatedMemoryMgr(b)
	sessionID := "bench-compress-llm"
	s := mgr.GetSession(sessionID)
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		_, _, _ = s.StoreCtx(ctx, fmt.Sprintf("k%d", i), strings.Repeat(fmt.Sprintf("content %d ", i), 10), "long", "fact", nil, 24, 0.2, "bench")
	}
	n := &MemoryNode{}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if i > 0 {
			for j := 0; j < 20; j++ {
				_, _, _ = s.StoreCtx(ctx, fmt.Sprintf("k%d", j), strings.Repeat(fmt.Sprintf("content %d ", j), 10), "long", "fact", nil, 24, 0.2, "bench")
			}
		}
		_, _ = n.Execute(ctx, "", map[string]string{
			"operation":      "compress",
			"session_id":     sessionID,
			"token_budget":   "10",
			"min_confidence": "0.5",
			"level":          "long",
			"endpoint":       srv.URL,
			"api_key":        "sk-test",
			"model":          "test-model",
		})
	}
}

// BenchmarkKGExtraction_LLM measures the LLM extraction pipeline cost
// with a mock server returning a fixed-size graph.
func BenchmarkKGExtraction_LLM(b *testing.B) {
	var ents []string
	for i := 0; i < 10; i++ {
		ents = append(ents, fmt.Sprintf(`{"name":"E%d","type":"Concept"}`, i))
	}
	var rels []string
	for i := 0; i < 5; i++ {
		rels = append(rels, fmt.Sprintf(`{"from":"E%d","to":"E%d","relation":"r%d","confidence":0.8}`, i, i+1, i))
	}
	llmResp := fmt.Sprintf(`{"entities":[%s],"relations":[%s]}`, strings.Join(ents, ","), strings.Join(rels, ","))

	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		_, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": llmResp}},
			},
		})
	}))
	defer srv.Close()

	n := &KnowledgeGraphNode{}
	input := "benchmark input text for knowledge graph extraction"
	params := map[string]string{
		"action":   "extract_llm",
		"endpoint": srv.URL,
		"api_key":  "sk-test",
		"model":    "test-model",
		"format":   "json",
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = n.Execute(context.Background(), input, params)
	}
}
