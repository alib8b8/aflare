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
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
)

// --- C-1: vector retrieval equivalence & recall tests ---

// TestVectorSearch_RetrievesSemanticallyRelevantEntry verifies the
// happy path: storing with an embedder and searching with the same
// embedder returns the relevant entry. The HashEmbedder is token-based,
// so an entry that shares query tokens will score highest.
func TestVectorSearch_RetrievesSemanticallyRelevantEntry(t *testing.T) {
	sm := NewSessionMemory("t", 100)
	sm.SetEmbedder(NewHashEmbedder(128))

	ctx := context.Background()
	if _, _, err := sm.StoreCtx(ctx, "go", "The Go programming language is statically typed and compiled", "long", "fact", nil, 24, 0.9, "test"); err != nil {
		t.Fatalf("Store go: %v", err)
	}
	if _, _, err := sm.StoreCtx(ctx, "rust", "Rust is a systems language focused on memory safety without garbage collection", "long", "fact", nil, 24, 0.9, "test"); err != nil {
		t.Fatalf("Store rust: %v", err)
	}
	if _, _, err := sm.StoreCtx(ctx, "recipe", "To make pasta, boil water and add salt then cook for ten minutes", "long", "fact", nil, 24, 0.9, "test"); err != nil {
		t.Fatalf("Store recipe: %v", err)
	}

	results := sm.SearchCtx(ctx, "go programming language", "", 5, 0.0)
	if len(results) == 0 {
		t.Fatal("expected at least one search result")
	}
	if results[0].Key != "go" {
		t.Errorf("expected top hit to be 'go', got %q (score=%.3f)", results[0].Key, results[0].Score)
	}
}

// TestVectorSearch_RecallAtK measures recall@k: with k=2, the two
// semantically relevant entries ("go" and "golang") should both be in
// the top-k. The distractor ("recipe") should not displace either.
func TestVectorSearch_RecallAtK(t *testing.T) {
	sm := NewSessionMemory("t", 100)
	sm.SetEmbedder(NewHashEmbedder(256))

	ctx := context.Background()
	// Two relevant entries (share the "go" token) and one distractor.
	_, _, _ = sm.StoreCtx(ctx, "go", "go is a programming language", "long", "fact", nil, 24, 0.9, "test")
	_, _, _ = sm.StoreCtx(ctx, "golang", "golang is another name for go language", "long", "fact", nil, 24, 0.9, "test")
	_, _, _ = sm.StoreCtx(ctx, "recipe", "boil water add salt cook pasta ten minutes", "long", "fact", nil, 24, 0.9, "test")

	results := sm.SearchCtx(ctx, "go language", "", 2, 0.0)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	relevant := map[string]bool{"go": true, "golang": true}
	hits := 0
	for _, r := range results {
		if relevant[r.Key] {
			hits++
		}
	}
	if hits < 2 {
		t.Errorf("recall@2 = %d/2, expected 2/2 (results: %v)", hits, keysOf(results))
	}
}

// TestVectorSearch_FallbackToBagOfWordsWhenNoEmbedder confirms the
// legacy path still works when no embedder is set. This is the
// equivalence guarantee: callers that don't opt into vectors get
// identical behaviour to before.
func TestVectorSearch_FallbackToBagOfWordsWhenNoEmbedder(t *testing.T) {
	sm := NewSessionMemory("t", 100)
	// Deliberately NOT calling SetEmbedder.

	_, _, _ = sm.Store("a", "hello world from go", "long", "fact", nil, 24, 0.9, "test")
	_, _, _ = sm.Store("b", "completely unrelated content about cooking", "long", "fact", nil, 24, 0.9, "test")

	results := sm.Search("hello world", "", 5, 0.1)
	if len(results) == 0 {
		t.Fatal("expected bag-of-words to still return results")
	}
	if results[0].Key != "a" {
		t.Errorf("expected top hit 'a', got %q", results[0].Key)
	}
}

// TestVectorSearch_FallbackWhenQueryEmbedFails verifies that if the
// query embedding fails (simulated via a failing embedder), the search
// falls back to bag-of-words rather than returning empty.
func TestVectorSearch_FallbackWhenQueryEmbedFails(t *testing.T) {
	sm := NewSessionMemory("t", 100)
	sm.SetEmbedder(&failingEmbedder{})

	_, _, _ = sm.Store("a", "hello world from go", "long", "fact", nil, 24, 0.9, "test")
	results := sm.Search("hello world", "", 5, 0.1)
	if len(results) == 0 {
		t.Fatal("expected bag-of-words fallback when query embed fails")
	}
	if results[0].Key != "a" {
		t.Errorf("expected 'a', got %q", results[0].Key)
	}
}

// TestVectorSearch_MixedIndexedAndUnindexedEntries ensures entries
// stored before the embedder was set (no vector) still get matched via
// the bag-of-words fallback, merged with the vector hits.
func TestVectorSearch_MixedIndexedAndUnindexedEntries(t *testing.T) {
	sm := NewSessionMemory("t", 100)

	// Store one entry WITHOUT an embedder (no vector).
	_, _, _ = sm.Store("legacy", "go programming language tutorial", "long", "fact", nil, 24, 0.9, "test")

	// Now set the embedder and store a second entry (has vector).
	sm.SetEmbedder(NewHashEmbedder(128))
	ctx := context.Background()
	_, _, _ = sm.StoreCtx(ctx, "modern", "go programming language guide", "long", "fact", nil, 24, 0.9, "test")

	results := sm.SearchCtx(ctx, "go programming language", "", 5, 0.0)
	// Both entries should be retrievable, even though "legacy" has no vector.
	found := map[string]bool{}
	for _, r := range results {
		found[r.Key] = true
	}
	if !found["legacy"] {
		t.Error("expected 'legacy' (unindexed) to be found via bag-of-words fallback")
	}
	if !found["modern"] {
		t.Error("expected 'modern' (indexed) to be found via vector search")
	}
}

// TestReindexVectors_BackfillsExistingEntries confirms that after
// setting an embedder on a session that already has entries, calling
// ReindexVectors makes those entries searchable via vectors.
func TestReindexVectors_BackfillsExistingEntries(t *testing.T) {
	sm := NewSessionMemory("t", 100)
	_, _, _ = sm.Store("a", "go programming language", "long", "fact", nil, 24, 0.9, "test")
	_, _, _ = sm.Store("b", "rust systems language", "long", "fact", nil, 24, 0.9, "test")

	sm.SetEmbedder(NewHashEmbedder(128))
	if err := sm.ReindexVectors(context.Background()); err != nil {
		t.Fatalf("ReindexVectors: %v", err)
	}

	if got := sm.vectors.Len(); got != 2 {
		t.Errorf("expected 2 vectors after reindex, got %d", got)
	}
}

// --- C-3: memory↔graph linkage tests ---

// TestLinkKGNode_AndExpandSubgraph verifies the core C-3 flow: link a
// memory entry to KG entities, then expand the subgraph for retrieved keys.
func TestLinkKGNode_AndExpandSubgraph(t *testing.T) {
	sm := NewSessionMemory("t", 100)
	_, _, _ = sm.Store("mem1", "content about Go and Docker", "long", "fact", nil, 24, 0.9, "test")
	_, _, _ = sm.Store("mem2", "content about Rust", "long", "fact", nil, 24, 0.9, "test")

	if err := sm.LinkKGNode("mem1", "Go", "Docker", "Kubernetes"); err != nil {
		t.Fatalf("LinkKGNode mem1: %v", err)
	}
	if err := sm.LinkKGNode("mem2", "Rust"); err != nil {
		t.Fatalf("LinkKGNode mem2: %v", err)
	}

	// Dedup: linking Go again should not duplicate.
	if err := sm.LinkKGNode("mem1", "Go"); err != nil {
		t.Fatalf("LinkKGNode mem1 Go again: %v", err)
	}
	links := sm.GetKGLinks("mem1")
	if len(links) != 3 {
		t.Errorf("expected 3 unique links for mem1, got %d (%v)", len(links), links)
	}

	// Expand for mem1 only.
	expanded := sm.ExpandKGSubgraph([]string{"mem1"})
	if len(expanded) != 3 {
		t.Errorf("expected 3 expanded entities for mem1, got %d", len(expanded))
	}

	// Expand for both mem1 and mem2 — should union (4 unique: Go, Docker, Kubernetes, Rust).
	expanded = sm.ExpandKGSubgraph([]string{"mem1", "mem2"})
	if len(expanded) != 4 {
		t.Errorf("expected 4 expanded entities, got %d (%v)", len(expanded), expanded)
	}
}

// TestLinkKGNode_RejectsMissingKey ensures we don't silently create
// dangling links to non-existent memory entries.
func TestLinkKGNode_RejectsMissingKey(t *testing.T) {
	sm := NewSessionMemory("t", 100)
	err := sm.LinkKGNode("nonexistent", "Entity")
	if err == nil {
		t.Error("expected error when linking to nonexistent key")
	}
}

// TestLinkKGNode_DeleteCleansUpLinks verifies that deleting a memory
// entry also removes its KG links (no dangling references).
func TestLinkKGNode_DeleteCleansUpLinks(t *testing.T) {
	sm := NewSessionMemory("t", 100)
	_, _, _ = sm.Store("mem1", "content", "long", "fact", nil, 24, 0.9, "test")
	_ = sm.LinkKGNode("mem1", "Go", "Docker")

	if err := sm.Delete("mem1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if links := sm.GetKGLinks("mem1"); len(links) != 0 {
		t.Errorf("expected 0 links after delete, got %d", len(links))
	}
}

// TestForget_CleansUpVectorsAndLinks verifies Forget removes vectors
// and KG links, not just entries.
func TestForget_CleansUpVectorsAndLinks(t *testing.T) {
	sm := NewSessionMemory("t", 100)
	sm.SetEmbedder(NewHashEmbedder(64))
	ctx := context.Background()
	_, _, _ = sm.StoreCtx(ctx, "mem1", "content one", "short", "fact", nil, 24, 0.9, "test")
	_, _, _ = sm.StoreCtx(ctx, "mem2", "content two", "long", "fact", nil, 24, 0.9, "test")
	_ = sm.LinkKGNode("mem1", "Go")
	_ = sm.LinkKGNode("mem2", "Rust")

	deleted := sm.Forget("short")
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}
	// mem1's vector and links should be gone.
	if sm.vectors.Len() != 1 {
		t.Errorf("expected 1 vector after forget, got %d", sm.vectors.Len())
	}
	if links := sm.GetKGLinks("mem1"); len(links) != 0 {
		t.Errorf("expected 0 links for forgotten mem1, got %d", len(links))
	}
}

// --- Concurrency / race tests ---

// TestVectorSearch_ConcurrentStoreAndSearch exercises the vector index
// under concurrent readers and writers. Run with -race to catch data
// races. This is the C-5 race detector entry point for C-1.
func TestVectorSearch_ConcurrentStoreAndSearch(t *testing.T) {
	sm := NewSessionMemory("t", 1000)
	sm.SetEmbedder(NewHashEmbedder(64))
	ctx := context.Background()

	const writers = 4
	const readers = 4
	const ops = 100

	var wg sync.WaitGroup
	wg.Add(writers + readers)

	for w := 0; w < writers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				key := fmt.Sprintf("w%d-%d", id, i)
				_, _, _ = sm.StoreCtx(ctx, key, "go programming language content", "long", "fact", nil, 24, 0.8, "test")
			}
		}(w)
	}
	for r := 0; r < readers; r++ {
		go func() {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				_ = sm.Search("go programming", "", 10, 0.0)
			}
		}()
	}
	wg.Wait()
}

// --- CosineSimilarity unit tests ---

func TestCosineSimilarity(t *testing.T) {
	cases := []struct {
		name string
		a, b Vector
		want float64
	}{
		{"identical", Vector{1, 0, 0}, Vector{1, 0, 0}, 1.0},
		{"orthogonal", Vector{1, 0}, Vector{0, 1}, 0.0},
		{"opposite", Vector{1, 0}, Vector{-1, 0}, -1.0},
		{"empty_a", Vector{}, Vector{1, 0}, 0.0},
		{"dim_mismatch", Vector{1, 0}, Vector{1, 0, 0}, 0.0},
		{"zero_vector", Vector{0, 0}, Vector{1, 1}, 0.0},
		{"45deg", Vector{1, 1}, Vector{1, 0}, 1.0 / math.Sqrt(2)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CosineSimilarity(c.a, c.b)
			if math.Abs(got-c.want) > 1e-9 {
				t.Errorf("CosineSimilarity = %.6f, want %.6f", got, c.want)
			}
		})
	}
}

// TestHashEmbedder_Deterministic verifies the same text always produces
// the same vector (essential for test reproducibility and for the
// vector index to be consistent across store/search).
func TestHashEmbedder_Deterministic(t *testing.T) {
	e := NewHashEmbedder(128)
	v1, _ := e.Embed(context.Background(), "hello world")
	v2, _ := e.Embed(context.Background(), "hello world")
	if len(v1) != len(v2) {
		t.Fatalf("dim mismatch: %d vs %d", len(v1), len(v2))
	}
	for i := range v1 {
		if v1[i] != v2[i] {
			t.Fatalf("vectors differ at index %d: %v vs %v", i, v1[i], v2[i])
		}
	}
}

// TestHashEmbedder_EmptyReturnsZeroVector ensures empty input doesn't
// panic and returns a zero vector (which the index rejects).
func TestHashEmbedder_EmptyReturnsZeroVector(t *testing.T) {
	e := NewHashEmbedder(64)
	v, _ := e.Embed(context.Background(), "")
	if len(v) != 64 {
		t.Fatalf("expected dim 64, got %d", len(v))
	}
	for _, x := range v {
		if x != 0 {
			t.Errorf("expected zero vector, got non-zero element %v", x)
		}
	}
}

// --- VectorIndex unit tests ---

func TestVectorIndex_AddRemoveSearch(t *testing.T) {
	idx := NewVectorIndex()
	e := NewHashEmbedder(32)

	v1, _ := e.Embed(context.Background(), "alpha")
	v2, _ := e.Embed(context.Background(), "beta")
	v3, _ := e.Embed(context.Background(), "alpha alpha")

	idx.Add("k1", v1, nil)
	idx.Add("k2", v2, nil)
	idx.Add("k3", v3, nil)

	if idx.Len() != 3 {
		t.Errorf("Len = %d, want 3", idx.Len())
	}

	q, _ := e.Embed(context.Background(), "alpha")
	hits := idx.Search(q, 2, 0.0)
	if len(hits) != 2 {
		t.Errorf("expected 2 hits, got %d", len(hits))
	}
	// "k3" has "alpha" twice so should score >= "k1".
	if hits[0].Key != "k3" && hits[0].Key != "k1" {
		t.Errorf("expected top hit k3 or k1, got %q", hits[0].Key)
	}

	// Remove k1 and confirm it's gone.
	if !idx.Remove("k1") {
		t.Error("Remove returned false for existing key")
	}
	if idx.Len() != 2 {
		t.Errorf("Len after remove = %d, want 2", idx.Len())
	}
	hits = idx.Search(q, 5, 0.0)
	for _, h := range hits {
		if h.Key == "k1" {
			t.Error("k1 should have been removed from results")
		}
	}

	// Remove nonexistent returns false.
	if idx.Remove("nope") {
		t.Error("Remove should return false for missing key")
	}
}

func TestVectorIndex_RejectsZeroVector(t *testing.T) {
	idx := NewVectorIndex()
	if idx.Add("k", Vector{}, nil) {
		t.Error("Add should reject zero-length vector")
	}
	if idx.Len() != 0 {
		t.Errorf("Len = %d, want 0", idx.Len())
	}
}

func TestVectorIndex_ReplaceExistingKey(t *testing.T) {
	idx := NewVectorIndex()
	idx.Add("k", Vector{1, 0}, nil)
	idx.Add("k", Vector{0, 1}, nil) // replace
	if idx.Len() != 1 {
		t.Errorf("Len = %d, want 1 (replace should not add)", idx.Len())
	}
}

// --- helpers ---

type failingEmbedder struct{}

func (failingEmbedder) Embed(context.Context, string) (Vector, error) {
	return nil, fmt.Errorf("embedding disabled")
}
func (failingEmbedder) Dim() int { return 128 }

func keysOf(results []MemoryEntry) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Key
	}
	return out
}

// --- Benchmarks (C-5) ---

// BenchmarkVectorSearch_Search measures search latency as the index
// grows. With brute-force O(n*dim) this should scale linearly; the
// benchmark exists to catch regressions (e.g. accidental O(n^2)).
func BenchmarkVectorSearch_Search(b *testing.B) {
	sm := NewSessionMemory("bench", 10000)
	sm.SetEmbedder(NewHashEmbedder(128))
	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		_, _, _ = sm.StoreCtx(ctx, fmt.Sprintf("k%d", i), fmt.Sprintf("entry number %d about go programming language", i), "long", "fact", nil, 24, 0.8, "bench")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = sm.SearchCtx(ctx, "go programming language", "", 10, 0.0)
	}
}

// BenchmarkVectorIndex_Search isolates the VectorIndex search cost from
// the SessionMemory overhead.
func BenchmarkVectorIndex_Search(b *testing.B) {
	idx := NewVectorIndex()
	e := NewHashEmbedder(128)
	for i := 0; i < 1000; i++ {
		v, _ := e.Embed(context.Background(), fmt.Sprintf("doc %d go rust python programming", i))
		idx.Add(fmt.Sprintf("k%d", i), v, nil)
	}
	q, _ := e.Embed(context.Background(), "go programming")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = idx.Search(q, 10, 0.0)
	}
}

// BenchmarkCosineSimilarity measures the inner loop of vector search.
func BenchmarkCosineSimilarity(b *testing.B) {
	a := make(Vector, 512)
	c := make(Vector, 512)
	for i := range a {
		a[i] = float32(i) * 0.001
		c[i] = float32(i) * 0.002
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CosineSimilarity(a, c)
	}
}

// BenchmarkBagOfWords_Search is the baseline: the legacy search path
// without an embedder. Comparing this against BenchmarkVectorSearch_Search
// quantifies the cost of the vector upgrade.
func BenchmarkBagOfWords_Search(b *testing.B) {
	sm := NewSessionMemory("bench", 10000)
	// No embedder -> bag-of-words path.
	for i := 0; i < 1000; i++ {
		_, _, _ = sm.Store(fmt.Sprintf("k%d", i), fmt.Sprintf("entry number %d about go programming language", i), "long", "fact", nil, 24, 0.8, "bench")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = sm.Search("go programming language", "", 10, 0.0)
	}
}
