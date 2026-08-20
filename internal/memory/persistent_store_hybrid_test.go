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

// Tests for P1-5: hybrid (keyword + vector) retrieval on the persistent
// memory store. A crafted embedder maps specific texts to specific unit
// vectors so tests can engineer exact cosine similarities and isolate
// the vector half of the merge from the keyword half.

package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

// craftedEmbedder maps known texts to fixed vectors. Entry texts arrive
// as "key value" (see StoreCtx); queries arrive verbatim. Unknown texts
// error, which also exercises the "embedding failed → keyword-only"
// degradation path.
type craftedEmbedder struct {
	vecs map[string]Vector
	dim  int
}

func (c *craftedEmbedder) Embed(_ context.Context, text string) (Vector, error) {
	if v, ok := c.vecs[text]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("craftedEmbedder: no vector for %q", text)
}

func (c *craftedEmbedder) Dim() int { return c.dim }

func newHybridTestStore(t *testing.T) *PersistentMemoryStore {
	t.Helper()
	return newPersistentMemoryStore(filepath.Join(t.TempDir(), "memory.json"))
}

// TestPersistentStore_HybridVectorRecall verifies the headline property
// of hybrid retrieval: an entry with ZERO keyword overlap with the query
// is still returned when its embedding is similar to the query embedding.
// Legacy keyword-only search would return nothing here.
func TestPersistentStore_HybridVectorRecall(t *testing.T) {
	s := newHybridTestStore(t)
	s.SetEmbedder(&craftedEmbedder{dim: 2, vecs: map[string]Vector{
		"e1 alpha one": {1, 0},
		"e2 beta two":  {0, 1},
		"gamma query":  {0.99, 0.141}, // ~unit; cos ≈ 0.99 with e1
	}})
	ctx := context.Background()
	if err := s.StoreCtx(ctx, "e1", "alpha one", "fact"); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreCtx(ctx, "e2", "beta two", "fact"); err != nil {
		t.Fatal(err)
	}

	hits := s.SearchCtx(ctx, "gamma query", 10)
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1 (vector-only recall of e1)", len(hits))
	}
	if hits[0].Key != "e1" {
		t.Fatalf("top hit key = %q, want e1", hits[0].Key)
	}
}

// TestPersistentStore_KeywordOnlyWhenNoEmbedder verifies the degradation
// path: with no embedder, hybrid search behaves like the legacy
// keyword-only ranking (monotonic saturation preserves ordering).
func TestPersistentStore_KeywordOnlyWhenNoEmbedder(t *testing.T) {
	s := newHybridTestStore(t)
	s.SetEmbedder(nil)
	ctx := context.Background()
	if err := s.StoreCtx(ctx, "go_server", "go http server", "fact"); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreCtx(ctx, "py_api", "python rest api", "fact"); err != nil {
		t.Fatal(err)
	}

	hits := s.SearchCtx(ctx, "go http", 10)
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1 (go_server only)", len(hits))
	}
	if hits[0].Key != "go_server" {
		t.Fatalf("hit key = %q, want go_server", hits[0].Key)
	}
	if got := s.SearchCtx(ctx, "nomatch anywhere", 0); got != nil {
		t.Fatalf("maxResults=0 should return nil, got %v", got)
	}
}

// TestPersistentStore_VectorNoiseFloorFiltered verifies that cosine
// similarities below hybridMinVectorScore are treated as no signal, so
// hash-collision-grade similarity cannot inject noise into results.
func TestPersistentStore_VectorNoiseFloorFiltered(t *testing.T) {
	s := newHybridTestStore(t)
	// cos([0.34,0.94],[1,0]) = 0.34 < hybridMinVectorScore (0.35).
	s.SetEmbedder(&craftedEmbedder{dim: 2, vecs: map[string]Vector{
		"e1 alpha one": {1, 0},
		"zzz query":    {0.34, 0.94},
	}})
	ctx := context.Background()
	if err := s.StoreCtx(ctx, "e1", "alpha one", "fact"); err != nil {
		t.Fatal(err)
	}

	if hits := s.SearchCtx(ctx, "zzz query", 10); len(hits) != 0 {
		t.Fatalf("sub-floor vector similarity should be filtered, got %d hits", len(hits))
	}
}

// TestPersistentStore_HybridAgreementBonus verifies the merge arithmetic:
// an entry confirmed by BOTH keyword and vector signals outranks an entry
// confirmed by the vector signal alone at the same similarity, which in
// turn outranks a keyword-only match.
func TestPersistentStore_HybridAgreementBonus(t *testing.T) {
	s := newHybridTestStore(t)
	// Query "cpp lint" embeds to [1,0].
	//   a_both: keyword score 4 (value exact matches cpp+lint), vec 1.0
	//           → combined = 1.0 + 0.2*(4/10) = 1.08
	//   b_vec:  no keyword overlap, vec 1.0 → combined = 1.0
	//   d_kw:   keyword score 5, vec 0 (orthogonal) → combined = 5/11 ≈ 0.45
	s.SetEmbedder(&craftedEmbedder{dim: 2, vecs: map[string]Vector{
		"a_key cpp lint":         {1, 0},
		"b_key unrelated words":  {1, 0},
		"d_key cpp lint strict":  {0, 1},
		"cpp lint":               {1, 0},
	}})
	ctx := context.Background()
	for _, e := range []struct{ key, value string }{
		{"a_key", "cpp lint"},
		{"b_key", "unrelated words"},
		{"d_key", "cpp lint strict"},
	} {
		if err := s.StoreCtx(ctx, e.key, e.value, "fact"); err != nil {
			t.Fatal(err)
		}
	}

	hits := s.SearchCtx(ctx, "cpp lint", 10)
	if len(hits) != 3 {
		t.Fatalf("got %d hits, want 3", len(hits))
	}
	wantOrder := []string{"a_key", "b_key", "d_key"}
	for i, want := range wantOrder {
		if hits[i].Key != want {
			t.Fatalf("hits[%d].Key = %q, want %q (full order: %v)", i, hits[i].Key, want, hybridKeysOf(hits))
		}
	}
}

func hybridKeysOf(hits []*PersistentMemoryEntry) []string {
	keys := make([]string, len(hits))
	for i, h := range hits {
		keys[i] = h.Key
	}
	return keys
}

// TestPersistentStore_ReindexVectorsBackfills verifies that entries
// stored while no embedder was configured (hence no vectors) become
// vector-searchable after SetEmbedder + ReindexVectors.
func TestPersistentStore_ReindexVectorsBackfills(t *testing.T) {
	s := newHybridTestStore(t)
	ctx := context.Background()

	s.SetEmbedder(nil)
	if err := s.StoreCtx(ctx, "e1", "alpha one", "fact"); err != nil {
		t.Fatal(err)
	}
	if hits := s.SearchCtx(ctx, "gamma query", 10); len(hits) != 0 {
		t.Fatalf("expected no hits before reindex, got %v", hybridKeysOf(hits))
	}

	s.SetEmbedder(&craftedEmbedder{dim: 2, vecs: map[string]Vector{
		"e1 alpha one": {1, 0},
		"gamma query":  {0.99, 0.141},
	}})
	if err := s.ReindexVectors(ctx); err != nil {
		t.Fatal(err)
	}
	hits := s.SearchCtx(ctx, "gamma query", 10)
	if len(hits) != 1 || hits[0].Key != "e1" {
		t.Fatalf("after reindex want [e1], got %v", hybridKeysOf(hits))
	}
}

// TestPersistentStore_DeleteRemovesVector verifies the vector index stays
// in sync with entries across deletes.
func TestPersistentStore_DeleteRemovesVector(t *testing.T) {
	s := newHybridTestStore(t)
	s.SetEmbedder(&craftedEmbedder{dim: 2, vecs: map[string]Vector{
		"e1 alpha one": {1, 0},
		"gamma query":  {1, 0},
	}})
	ctx := context.Background()
	if err := s.StoreCtx(ctx, "e1", "alpha one", "fact"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("e1"); err != nil {
		t.Fatal(err)
	}
	if hits := s.SearchCtx(ctx, "gamma query", 10); len(hits) != 0 {
		t.Fatalf("vector should be gone after delete, got %v", hybridKeysOf(hits))
	}
}

// TestPersistentStore_StoreUpdateRefreshesVector verifies that updating
// an entry's value replaces its embedding, so the vector index never
// serves a stale vector for the current value.
func TestPersistentStore_StoreUpdateRefreshesVector(t *testing.T) {
	s := newHybridTestStore(t)
	s.SetEmbedder(&craftedEmbedder{dim: 2, vecs: map[string]Vector{
		"e1 alpha one": {1, 0},
		"e1 beta two":  {0, 1},
		"gamma query":  {1, 0},
	}})
	ctx := context.Background()
	if err := s.StoreCtx(ctx, "e1", "alpha one", "fact"); err != nil {
		t.Fatal(err)
	}
	// Update the value; the new embedding is orthogonal to the query.
	if err := s.StoreCtx(ctx, "e1", "beta two", "fact"); err != nil {
		t.Fatal(err)
	}
	if hits := s.SearchCtx(ctx, "gamma query", 10); len(hits) != 0 {
		t.Fatalf("stale vector should not match after value update, got %v", hybridKeysOf(hits))
	}
	// The entry is still findable by its new content.
	if hits := s.SearchCtx(ctx, "beta", 10); len(hits) != 1 || hits[0].Key != "e1" {
		t.Fatalf("want keyword hit [e1] for new value, got %v", hybridKeysOf(hits))
	}
}

// TestPersistentStore_DefaultHashEmbedderSmoke verifies the shipped
// default: a fresh store has the offline HashEmbedder active, so hybrid
// search works with no configuration and no network.
func TestPersistentStore_DefaultHashEmbedderSmoke(t *testing.T) {
	s := newHybridTestStore(t)
	if s.GetEmbedder() == nil {
		t.Fatal("default embedder should be set on a new store")
	}
	ctx := context.Background()
	if err := s.StoreCtx(ctx, "srv", "go http server", "fact"); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreCtx(ctx, "db", "postgres migrations", "fact"); err != nil {
		t.Fatal(err)
	}
	hits := s.SearchCtx(ctx, "go http server", 10)
	if len(hits) == 0 || hits[0].Key != "srv" {
		t.Fatalf("want top hit srv, got %v", hybridKeysOf(hits))
	}
}

// TestPersistentStore_PersistenceRoundtripKeepsHybrid verifies that a
// store reloaded from disk rebuilds embeddings at load time, so hybrid
// search covers persisted entries in a fresh process.
func TestPersistentStore_PersistenceRoundtripKeepsHybrid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	ctx := context.Background()

	s1 := newPersistentMemoryStore(path)
	if err := s1.StoreCtx(ctx, "srv", "go http server", "fact"); err != nil {
		t.Fatal(err)
	}

	s2 := newPersistentMemoryStore(path)
	hits := s2.SearchCtx(ctx, "go http", 10)
	if len(hits) != 1 || hits[0].Key != "srv" {
		t.Fatalf("reloaded store should find srv, got %v", hybridKeysOf(hits))
	}
}

// TestPersistentStore_HybridConcurrentAccess exercises concurrent
// StoreCtx/SearchCtx/Delete mixes for the race detector: the keyword
// phase runs under the store read lock, the vector phase under the
// index's own lock, and the embed call runs lock-free.
func TestPersistentStore_HybridConcurrentAccess(t *testing.T) {
	s := newHybridTestStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(3)
		go func(i int) {
			defer wg.Done()
			_ = s.StoreCtx(ctx, fmt.Sprintf("k%d", i), fmt.Sprintf("value %d shared words here", i), "fact")
		}(i)
		go func() {
			defer wg.Done()
			_ = s.SearchCtx(ctx, "shared words", 5)
		}()
		go func(i int) {
			defer wg.Done()
			_ = s.Delete(fmt.Sprintf("k%d", i)) // best-effort
		}(i)
	}
	wg.Wait()
}

// TestPersistentStore_VectorHitRankingConsistency sanity-checks that an
// exact-text duplicate ranks first with the default embedder (cosine 1.0
// plus keyword agreement).
func TestPersistentStore_VectorHitRankingConsistency(t *testing.T) {
	s := newHybridTestStore(t)
	ctx := context.Background()
	if err := s.StoreCtx(ctx, "dup", "identical text body", "fact"); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreCtx(ctx, "other", "completely different content", "fact"); err != nil {
		t.Fatal(err)
	}
	hits := s.SearchCtx(ctx, "identical text body", 10)
	if len(hits) == 0 {
		t.Fatal("expected at least one hit")
	}
	if hits[0].Key != "dup" {
		t.Fatalf("top hit = %q, want dup", hits[0].Key)
	}
}
