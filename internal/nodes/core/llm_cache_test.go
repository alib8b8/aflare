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
