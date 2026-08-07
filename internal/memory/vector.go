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

// Vector retrieval support for the memory package.
//
// This file adds embedding-based semantic search to SessionMemory. It
// replaces the bag-of-words similarity in calculateSimilarity with cosine
// similarity over dense vectors, while keeping the legacy path as a
// fallback for entries that have no embedded vector.
//
// Two Embedder implementations are provided:
//   - HashEmbedder: a deterministic, dependency-free embedder that hashes
//     tokens into a fixed-dimensional vector. It is the default and is
//     used by tests. It produces stable, reproducible vectors so tests
//     can assert recall without network access.
//   - HTTPEmbedder: an OpenAI-compatible /embeddings client used in
//     production. When no endpoint is configured the Embedder falls back
//     to HashEmbedder so memory search always works offline.

package memory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/httpclient"
)

// Vector is a dense float32 embedding. We use float32 (not float64) because
// embeddings are typically served as float32 by providers and the memory
// savings matter at scale; cosine similarity computation converts to float64
// internally to avoid precision loss in the dot product.
type Vector = []float32

// Embedder converts a piece of text into a Vector.
//
// Implementations must be safe for concurrent use: a SessionMemory may be
// shared across goroutines and SearchVector calls Embed in parallel with
// Store calls.
type Embedder interface {
	// Embed returns the vector for text. The returned vector must have
	// the same dimensionality on every call; implementations that fail
	// to ensure this will cause SearchVector to skip the entry.
	Embed(ctx context.Context, text string) (Vector, error)

	// Dim returns the dimensionality of vectors produced by Embed.
	Dim() int
}

// HashEmbedder is a deterministic, dependency-free embedder. It tokenises
// text on whitespace, hashes each token with SHA-256, and projects the
// hash into a Dim-dimensional float32 vector with L2 normalisation.
//
// The result is NOT a meaningful semantic embedding — it cannot capture
// synonymy or topic similarity. It is provided so that memory search has
// a working default offline and tests can produce reproducible vectors.
// Two texts that share more tokens will have higher cosine similarity
// (because shared tokens add to the same dimensions), so it behaves
// correctly for the "did we retrieve the obviously-relevant entry?"
// recall tests used in C-5.
type HashEmbedder struct {
	dim int
}

// NewHashEmbedder returns a HashEmbedder with the given dimensionality.
// dim must be positive; values of 64–512 give a good trade-off between
// collision resistance and memory footprint.
func NewHashEmbedder(dim int) *HashEmbedder {
	if dim <= 0 {
		dim = 128
	}
	return &HashEmbedder{dim: dim}
}

// Dim implements Embedder.
func (e *HashEmbedder) Dim() int { return e.dim }

// Embed implements Embedder. The returned vector is L2-normalised so
// cosine similarity reduces to a dot product.
func (e *HashEmbedder) Embed(_ context.Context, text string) (Vector, error) {
	v := make(Vector, e.dim)
	if strings.TrimSpace(text) == "" {
		return v, nil
	}
	tokens := strings.Fields(strings.ToLower(text))
	for _, tok := range tokens {
		// Trim surrounding punctuation so "go." and "go" hash the same.
		tok = strings.Trim(tok, ".,;:!?\"'()[]{}<>/\\|")
		if tok == "" {
			continue
		}
		sum := sha256.Sum256([]byte(tok))
		// Project each of the first 8 uint32 of the hash into a
		// dimension, signing the contribution by a parity bit so
		// unrelated tokens don't all reinforce the same direction.
		for i := 0; i < 8; i++ {
			u := binary.BigEndian.Uint32(sum[i*4 : i*4+4])
			idx := int(u) % e.dim
			sign := 1.0
			if u&1 == 1 {
				sign = -1.0
			}
			v[idx] += float32(sign)
		}
	}
	normalizeL2(v)
	return v, nil
}

// normalizeL2 divides v by its L2 norm in place. If v is the zero vector
// it is left untouched (the caller treats zero vectors as "no embedding").
func normalizeL2(v Vector) {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return
	}
	norm := float32(math.Sqrt(sum))
	for i := range v {
		v[i] /= norm
	}
}

// HTTPEmbedder calls an OpenAI-compatible /embeddings endpoint (e.g.
// OpenAI text-embedding-3-small, DeepSeek, Qwen, GLM). It is the
// production Embedder; tests use HashEmbedder instead.
type HTTPEmbedder struct {
	endpoint string // e.g. "https://api.openai.com/v1"
	apiKey   string
	model    string
	dim      int
	client   *http.Client
}

// NewHTTPEmbedder returns an HTTPEmbedder. dim is the dimensionality the
// server is expected to return; if a response's vector has a different
// length the embedder returns an error so callers can detect config drift.
//
// The embedder talks to an OpenAI-compatible /embeddings endpoint, which
// may be a remote provider (OpenAI, DeepSeek, …) or a local server
// (Ollama, text-embeddings-inference on localhost). We therefore use the
// httpclient factory with ValidateAllowLoopback so local embedders work
// while private/link-local/reserved ranges stay blocked, and so the
// connection pool is tuned consistently with every other outbound client
// in aflare (the previous bare &http.Client{} used the stdlib default of
// MaxIdleConnsPerHost==2, which serialized concurrent embedder calls
// against the same host).
func NewHTTPEmbedder(endpoint, apiKey, model string, dim int) *HTTPEmbedder {
	if dim <= 0 {
		dim = 1536 // OpenAI text-embedding-3-small default
	}
	return &HTTPEmbedder{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		model:    model,
		dim:      dim,
		client: httpclient.NewClient(httpclient.Options{
			Timeout:   30 * time.Second,
			Validator: httpclient.ValidateAllowLoopback,
		}),
	}
}

// Dim implements Embedder.
func (e *HTTPEmbedder) Dim() int { return e.dim }

// Embed implements Embedder.
func (e *HTTPEmbedder) Embed(ctx context.Context, text string) (Vector, error) {
	if e.endpoint == "" || e.apiKey == "" {
		return nil, fmt.Errorf("HTTPEmbedder: endpoint and api_key are required")
	}
	body, err := json.Marshal(map[string]interface{}{
		"model": e.model,
		"input": text,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Drain up to 1KB for the error message.
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("embeddings API error (status %d): %s", resp.StatusCode, string(msg))
	}

	// Streaming-style line scan is unnecessary; just decode the body.
	var parsed struct {
		Data []struct {
			Embedding Vector `json:"embedding"`
		} `json:"data"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 32*1024*1024))
	if err := dec.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode embeddings response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("embeddings response had no data")
	}
	v := parsed.Data[0].Embedding
	if len(v) != e.dim {
		return nil, fmt.Errorf("embeddings dimension mismatch: got %d, configured %d (model %s)", len(v), e.dim, e.model)
	}
	// Many providers already return normalised embeddings, but we
	// normalise defensively so cosine similarity reduces to a dot
	// product regardless of the provider.
	normalizeL2(v)
	return v, nil
}

// CosineSimilarity computes the cosine of the angle between a and b.
// Returns 0 if either vector is zero-length or dimensions don't match.
// Vectors must be L2-normalised for this to be the true cosine; we
// normalise defensively inside the calculation so callers don't have to.
func CosineSimilarity(a, b Vector) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		af := float64(a[i])
		bf := float64(b[i])
		dot += af * bf
		na += af * af
		nb += bf * bf
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// VectorHit is a single search result from VectorIndex.Search.
type VectorHit struct {
	Key      string
	Score    float64
	Vector   Vector
	Metadata map[string]string // optional caller-supplied metadata
}

// VectorIndex is an in-memory brute-force vector store. It supports
// concurrent reads and single-writer updates. Brute-force O(n*dim) is
// fine for the per-session memory sizes we expect (low thousands of
// entries); a future optimisation can swap in an ANN index behind the
// same interface.
type VectorIndex struct {
	mu      sync.RWMutex
	keys    []string
	vectors []Vector
	meta    []map[string]string
}

// NewVectorIndex returns an empty index.
func NewVectorIndex() *VectorIndex { return &VectorIndex{} }

// Add inserts (or replaces, if key already exists) a vector with optional
// metadata. Returns false if v has zero length (entry rejected).
func (idx *VectorIndex) Add(key string, v Vector, meta map[string]string) bool {
	if len(v) == 0 {
		return false
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for i, k := range idx.keys {
		if k == key {
			idx.vectors[i] = v
			idx.meta[i] = meta
			return true
		}
	}
	idx.keys = append(idx.keys, key)
	idx.vectors = append(idx.vectors, v)
	idx.meta = append(idx.meta, meta)
	return true
}

// Remove deletes the entry with the given key. Returns true if it existed.
func (idx *VectorIndex) Remove(key string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for i, k := range idx.keys {
		if k == key {
			// Swap-delete: O(1) but changes ordering. Order doesn't
			// matter for search; callers must not rely on insertion
			// order from the index.
			last := len(idx.keys) - 1
			idx.keys[i] = idx.keys[last]
			idx.vectors[i] = idx.vectors[last]
			idx.meta[i] = idx.meta[last]
			idx.keys = idx.keys[:last]
			idx.vectors = idx.vectors[:last]
			idx.meta = idx.meta[:last]
			return true
		}
	}
	return false
}

// Len returns the number of stored vectors.
func (idx *VectorIndex) Len() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.keys)
}

// Search returns the top-K entries whose cosine similarity to query is
// at least threshold, sorted by descending similarity. The returned slice
// is owned by the caller (no shared state with the index).
func (idx *VectorIndex) Search(query Vector, topK int, threshold float64) []VectorHit {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if len(idx.keys) == 0 || len(query) == 0 {
		return nil
	}
	if topK <= 0 {
		topK = 10
	}

	hits := make([]VectorHit, 0, len(idx.keys))
	for i, k := range idx.keys {
		score := CosineSimilarity(query, idx.vectors[i])
		if score >= threshold {
			hits = append(hits, VectorHit{
				Key:      k,
				Score:    score,
				Vector:   idx.vectors[i],
				Metadata: idx.meta[i],
			})
		}
	}
	// Sort by score descending, ties broken by key for determinism.
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Key < hits[j].Key
	})
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}

// StreamVectors writes each (key, vector, metadata) triple to fn. It is
// primarily used by tests to assert index contents. Stops and returns
// the error if fn returns one.
func (idx *VectorIndex) StreamVectors(fn func(key string, v Vector, meta map[string]string) error) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	for i, k := range idx.keys {
		if err := fn(k, idx.vectors[i], idx.meta[i]); err != nil {
			return err
		}
	}
	return nil
}
