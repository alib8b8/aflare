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

package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alib8b8/llm-box/internal/cache"
)

// newCountingMockLLM stands up an OpenAI-compatible /chat/completions
// endpoint that always responds with `response` and counts how many times
// it was hit. The returned *atomic.Int64 lets a test assert cache hits by
// verifying the upstream call count did not increase.
func newCountingMockLLM(t *testing.T, response string) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		// Drain the request body so the server can detect protocol issues;
		// the body content is not asserted here.
		_, _ = io.ReadAll(r.Body)
		resp := LLMResponse{Choices: []LLMChoice{{Message: LLMChoiceMessage{Content: response}}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// newCacheTestNode builds an OpenAICompatibleNode wired to srvURL with the
// given cache injected via SetCache. It uses a unique, always-unset env var
// name so the missing-API-key guard is satisfied by the api_key param. If
// c is nil a default enabled cache (1h TTL) is created.
func newCacheTestNode(t *testing.T, srvURL string, c *cache.Cache) *OpenAICompatibleNode {
	t.Helper()
	if c == nil {
		c = cache.New(cache.Config{Enabled: true, MaxEntries: 1000, TTL: time.Hour})
	}
	n := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "cachetest",
		DefaultModel:    "test-model",
		DefaultEndpoint: srvURL,
		EnvAPIKey:       "LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET",
		ProviderName:    "TestProvider",
	})
	n.SetCache(c)
	return n
}

// copyStringParams returns a shallow copy of m so callers can mutate one
// variant without affecting siblings that share a base param set.
func copyStringParams(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func TestLLMCache_HitAfterMiss(t *testing.T) {
	srv, calls := newCountingMockLLM(t, "cached-response")
	c := cache.New(cache.Config{Enabled: true, MaxEntries: 100, TTL: time.Hour})
	n := newCacheTestNode(t, srv.URL, c)
	params := map[string]string{"api_key": "sk-test", "endpoint": srv.URL}

	// First call: cache miss -> upstream is hit and the response is cached.
	out1, err := n.Execute(context.Background(), "hello", params)
	if err != nil {
		t.Fatalf("first Execute failed: %v", err)
	}
	if out1 != "cached-response" {
		t.Fatalf("first output = %q, want cached-response", out1)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 upstream call after miss, got %d", got)
	}

	// Second call: identical inputs -> cache hit, upstream NOT called again.
	out2, err := n.Execute(context.Background(), "hello", params)
	if err != nil {
		t.Fatalf("second Execute failed: %v", err)
	}
	if out2 != "cached-response" {
		t.Fatalf("second output = %q, want cached-response", out2)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected still 1 upstream call after hit, got %d", got)
	}
}

func TestLLMCache_DifferentPromptDifferentKey(t *testing.T) {
	srv, calls := newCountingMockLLM(t, "r")
	n := newCacheTestNode(t, srv.URL, nil)
	params := map[string]string{"api_key": "sk-test", "endpoint": srv.URL}

	if _, err := n.Execute(context.Background(), "prompt-A", params); err != nil {
		t.Fatalf("prompt-A failed: %v", err)
	}
	// Different prompt -> different cache key -> miss -> second upstream call.
	if _, err := n.Execute(context.Background(), "prompt-B", params); err != nil {
		t.Fatalf("prompt-B failed: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 upstream calls for two distinct prompts, got %d", got)
	}
	// prompt-A was cached earlier -> hit, no new upstream call.
	if _, err := n.Execute(context.Background(), "prompt-A", params); err != nil {
		t.Fatalf("prompt-A retry failed: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected still 2 upstream calls after prompt-A hit, got %d", got)
	}
}

func TestLLMCache_DifferentSeedDifferentKey(t *testing.T) {
	srv, calls := newCountingMockLLM(t, "r")
	n := newCacheTestNode(t, srv.URL, nil)
	base := map[string]string{"api_key": "sk-test", "endpoint": srv.URL}

	p1 := copyStringParams(base)
	p1["seed"] = "1"
	p2 := copyStringParams(base)
	p2["seed"] = "2"

	if _, err := n.Execute(context.Background(), "hello", p1); err != nil {
		t.Fatalf("seed=1 failed: %v", err)
	}
	// Same prompt but a different seed -> different cache key -> miss.
	if _, err := n.Execute(context.Background(), "hello", p2); err != nil {
		t.Fatalf("seed=2 failed: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 upstream calls for two seeds, got %d", got)
	}
	// seed=1 was cached earlier -> hit.
	if _, err := n.Execute(context.Background(), "hello", p1); err != nil {
		t.Fatalf("seed=1 retry failed: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected still 2 upstream calls after seed=1 hit, got %d", got)
	}
}

func TestLLMCache_TTLExpiry(t *testing.T) {
	srv, calls := newCountingMockLLM(t, "r")
	// Short TTL so expiry can be exercised without slowing the suite.
	c := cache.New(cache.Config{Enabled: true, MaxEntries: 100, TTL: 50 * time.Millisecond})
	n := newCacheTestNode(t, srv.URL, c)
	params := map[string]string{"api_key": "sk-test", "endpoint": srv.URL}

	if _, err := n.Execute(context.Background(), "hello", params); err != nil {
		t.Fatalf("first Execute failed: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 upstream call after miss, got %d", got)
	}
	// Immediate retry hits the cache.
	if _, err := n.Execute(context.Background(), "hello", params); err != nil {
		t.Fatalf("second Execute failed: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected still 1 upstream call after hit, got %d", got)
	}

	// Wait past TTL; the entry expires and the next call misses again.
	time.Sleep(100 * time.Millisecond)
	if _, err := n.Execute(context.Background(), "hello", params); err != nil {
		t.Fatalf("post-expiry Execute failed: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 upstream calls after TTL expiry, got %d", got)
	}
}

func TestLLMCache_DisabledByDefault(t *testing.T) {
	// Ensure the env opt-in is unset so the node is in its default
	// (caching disabled) state regardless of ambient environment.
	t.Setenv("LLM_BOX_LLM_CACHE", "")

	srv, calls := newCountingMockLLM(t, "r")
	n := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "cachetest",
		DefaultModel:    "test-model",
		DefaultEndpoint: srv.URL,
		EnvAPIKey:       "LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET",
		ProviderName:    "TestProvider",
	})
	params := map[string]string{"api_key": "sk-test", "endpoint": srv.URL}

	if _, err := n.Execute(context.Background(), "hello", params); err != nil {
		t.Fatalf("first Execute failed: %v", err)
	}
	if _, err := n.Execute(context.Background(), "hello", params); err != nil {
		t.Fatalf("second Execute failed: %v", err)
	}
	// Caching off -> every call hits upstream; nothing is stored.
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 upstream calls with caching disabled, got %d", got)
	}
}

// TestLLMCache_DifferentAPIKeyDifferentKey (M-4) verifies that two requests
// with the SAME (model, prompt, params) but DIFFERENT API keys do NOT share
// a cache entry. Without the API-key hash in the cache key, tenant A's
// response would be served to tenant B from the shared cache — a cross-
// tenant data leak in multi-tenant SaaS deployments that share one process
// and one sharedLLMCache.
//
// The mock server returns a tenant-tagged response so the test can also
// assert the right tenant's answer is delivered to the right caller (not
// just that the upstream was hit twice).
func TestLLMCache_DifferentAPIKeyDifferentKey(t *testing.T) {
	// One shared cache instance simulates the process-wide sharedLLMCache.
	c := cache.New(cache.Config{Enabled: true, MaxEntries: 100, TTL: time.Hour})

	// The server returns the Authorization header (minus the "Bearer "
	// prefix) as the response content, so each caller can verify it got
	// ITS OWN API key's response and not the other tenant's cached entry.
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.ReadAll(r.Body)
		auth := r.Header.Get("Authorization")
		resp := LLMResponse{Choices: []LLMChoice{{Message: LLMChoiceMessage{Content: strings.TrimPrefix(auth, "Bearer ")}}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	n := newCacheTestNode(t, srv.URL, c)

	// Two identical requests differing ONLY in the API key.
	pA := map[string]string{"api_key": "sk-tenant-A", "endpoint": srv.URL}
	pB := map[string]string{"api_key": "sk-tenant-B", "endpoint": srv.URL}

	outA, err := n.Execute(context.Background(), "hello", pA)
	if err != nil {
		t.Fatalf("tenant A Execute failed: %v", err)
	}
	if outA != "sk-tenant-A" {
		t.Fatalf("tenant A got %q, want sk-tenant-A", outA)
	}

	outB, err := n.Execute(context.Background(), "hello", pB)
	if err != nil {
		t.Fatalf("tenant B Execute failed: %v", err)
	}
	if outB != "sk-tenant-B" {
		t.Fatalf("tenant B got %q, want sk-tenant-B (cross-tenant cache leak via shared API key)", outB)
	}

	// Both calls must have hit upstream — the cache key includes the API
	// key hash (M-4), so tenant B's request is a cache MISS, not a hit on
	// tenant A's entry. If the key did NOT include the API key, tenant B
	// would have been served tenant A's cached "sk-tenant-A" response and
	// the upstream call count would be 1.
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 upstream calls (one per API key), got %d — cache key not isolated by API key (M-4 regression)", got)
	}

	// Repeating tenant A's request must now HIT the cache (no third
	// upstream call) and return tenant A's response, proving the entries
	// coexist in the cache without colliding.
	outA2, err := n.Execute(context.Background(), "hello", pA)
	if err != nil {
		t.Fatalf("tenant A retry failed: %v", err)
	}
	if outA2 != "sk-tenant-A" {
		t.Fatalf("tenant A retry got %q, want sk-tenant-A", outA2)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected still 2 upstream calls after tenant A cache hit, got %d", got)
	}
}

// TestLLMCache_EmptyAPIKeySharedKey (M-4) verifies the legacy fall-back: when
// NO API key is supplied (e.g. local ollama), apiKeyHash returns the empty
// string, so the api_key_hash component of the cache key is "" for every such
// caller and two identical empty-key requests collapse to the SAME cache key
// (preserving the pre-M-4 shared-key behaviour for deployments with no API
// key).
//
// This is tested at the llmCacheKey/apiKeyHash level rather than through
// Execute because the node REQUIRES a non-empty API key (see the
// "API key required" guard in Execute) — the empty-key fall-back exists for
// hypothetical no-key deployments and is not reachable through the public
// Execute path in this codebase. Testing the key-derivation functions
// directly is the faithful way to cover the M-4 fall-back property.
func TestLLMCache_EmptyAPIKeySharedKey(t *testing.T) {
	// The fall-back: an empty API key hashes to the empty string (not a
	// sha256 of "").
	if h := apiKeyHash(""); h != "" {
		t.Fatalf("apiKeyHash(\"\") = %q, want \"\" (empty-key fall-back)", h)
	}

	// Two identical requests with NO API key must produce the SAME cache
	// key — this is the legacy shared-key behaviour the fall-back preserves.
	req := LLMRequest{
		Model:    "test-model",
		Messages: []LLMMessage{{Role: "user", Content: "hello"}},
	}
	keyEmptyA := llmCacheKey(req, "")
	keyEmptyB := llmCacheKey(req, "")
	if keyEmptyA == "" {
		t.Fatalf("llmCacheKey returned empty key for a valid request")
	}
	if keyEmptyA != keyEmptyB {
		t.Fatalf("two identical empty-key requests produced different cache keys: %q != %q (empty-key fall-back broken)", keyEmptyA, keyEmptyB)
	}

	// Sanity: the empty-key key must DIFFER from a non-empty-key key, so the
	// assertion above is meaningful (not vacuously true) and so a non-empty
	// API key still isolates from the empty-key pool. This also re-checks the
	// core M-4 isolation at the key-derivation level.
	keyReal := llmCacheKey(req, "sk-real")
	if keyEmptyA == keyReal {
		t.Fatalf("empty-key cache key == non-empty-key cache key: the API-key hash is not mixed into the key (M-4 regression)")
	}
}

// TestLLMCache_LargeResponseNotCached (M-5) verifies that a response larger
// than maxCacheableResponseBytes is NOT written to the cache. The response
// is still returned to the caller (correctness preserved), but a follow-up
// identical request misses the cache and hits upstream again — the
// per-entry size cap is what bounds memory usage.
func TestLLMCache_LargeResponseNotCached(t *testing.T) {
	// Build a response strictly larger than maxCacheableResponseBytes (64KB).
	// 64KB + 1KB leaves no ambiguity at the boundary.
	large := strings.Repeat("x", maxCacheableResponseBytes+1024)

	c := cache.New(cache.Config{Enabled: true, MaxEntries: 100, TTL: time.Hour})
	srv, calls := newCountingMockLLM(t, large)
	n := newCacheTestNode(t, srv.URL, c)
	params := map[string]string{"api_key": "sk-test", "endpoint": srv.URL}

	out1, err := n.Execute(context.Background(), "hello", params)
	if err != nil {
		t.Fatalf("first Execute failed: %v", err)
	}
	if len(out1) != len(large) {
		t.Fatalf("first output len = %d, want %d (large response must still be returned)", len(out1), len(large))
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 upstream call after first miss, got %d", got)
	}

	// Second identical call: the large response was NOT cached, so this is
	// another miss (upstream called again) — not a hit. This is the M-5
	// property: per-entry size cap prevents large responses from pinning
	// memory at the cost of re-fetching them.
	out2, err := n.Execute(context.Background(), "hello", params)
	if err != nil {
		t.Fatalf("second Execute failed: %v", err)
	}
	if len(out2) != len(large) {
		t.Fatalf("second output len = %d, want %d", len(out2), len(large))
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 upstream calls (large response not cached), got %d — M-5 per-entry size cap regression", got)
	}

	// The cache must be empty: the only entry attempted was skipped.
	if c.Len() != 0 {
		t.Errorf("cache should be empty after skipping large response, got Len=%d", c.Len())
	}
}

// TestLLMCache_SmallResponseCachedAtSizeCap (M-5) verifies the boundary: a
// response exactly at maxCacheableResponseBytes IS cached (the check is
// strict greater-than), so the cut-off does not accidentally exclude the
// common ~64KB case.
func TestLLMCache_SmallResponseCachedAtSizeCap(t *testing.T) {
	atCap := strings.Repeat("y", maxCacheableResponseBytes)

	c := cache.New(cache.Config{Enabled: true, MaxEntries: 100, TTL: time.Hour})
	srv, calls := newCountingMockLLM(t, atCap)
	n := newCacheTestNode(t, srv.URL, c)
	params := map[string]string{"api_key": "sk-test", "endpoint": srv.URL}

	if _, err := n.Execute(context.Background(), "hello", params); err != nil {
		t.Fatalf("first Execute failed: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 upstream call after first miss, got %d", got)
	}
	// Second call: response is at the cap (not above), so it WAS cached —
	// this must be a hit (no second upstream call).
	if _, err := n.Execute(context.Background(), "hello", params); err != nil {
		t.Fatalf("second Execute failed: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 upstream call (at-cap response cached), got %d", got)
	}
}
