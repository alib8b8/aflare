// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌‌‌​​‌​​‌​‌​‌​‌​​‌​‌‌‌‌‌‌‌‌‌‌​​​​​‌‌‌‌‌‌‌​​‌‌​‌​​‌​​​‌‌​​‌​​​‌​​​​​​​​​​​​​​​​​‌​‌‌‌‌​​​‌​​‌‌‌⁠
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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/cache"
	"github.com/alib8b8/aflare/internal/logger"
)

// --- LLM response caching -------------------------------------------------
//
// The LLM response cache is a PERFORMANCE OPTIMIZATION, not an audit
// mechanism (M-10). It serves identical non-streaming requests from memory
// within a configurable TTL window (default 24h) so a workflow that issues
// the same prompt repeatedly does not pay for the upstream call each time.
// Financial-scenario audit/reproducibility requirements are met by the
// separate audit log + trace subsystem (executor_audit.go, llm_trace.go),
// which persists every LLM I/O with HMAC-chained, tamper-evident records
// for the regulatory retention window (1–7 years). The cache's 24h TTL is
// far shorter than the audit window and the cache is in-memory only, so it
// MUST NOT be relied on for audit — a process restart, eviction, or TTL
// expiry silently removes entries. Caching is OFF by default so existing
// tests and dev workflows are unaffected; it is opted into via the
// AFLARE_LLM_CACHE env var, or by injecting a cache instance with SetCache.
// Only the final non-streaming response string is cached; streaming
// responses are never cached (SSE chunk boundaries are not byte-reproducible).
//
// M-4: the cache key includes a hash of the API key (first 8 hex chars of
// sha256) so identical (model, prompt, params) requests made with DIFFERENT
// API keys (different accounts / tenants sharing one process) do NOT share
// a cache entry. The raw key is never stored or logged; only its hash
// participates in the key. The hash is omitted when no API key is
// configured (e.g. local ollama), so those deployments see the legacy
// shared-key behaviour.
//
// M-5: responses larger than maxCacheableResponseBytes are not cached — the
// cache write is skipped, but the response is still returned to the caller.
// This bounds memory usage: with MaxEntries=1000 and a 64KB per-entry cap
// the cache occupies at most ~64MB, rather than the ~10GB an unbounded
// per-entry size could pin.

const (
	// envLLMCacheEnable opts into the shared LLM response cache when set to
	// "1" or "true". Evaluated at node construction.
	envLLMCacheEnable = "AFLARE_LLM_CACHE"
	// envLLMCacheTTL overrides the shared cache TTL (Go duration string,
	// e.g. "24h", "30m"). Defaults to defaultLLMCacheTTL.
	envLLMCacheTTL = "AFLARE_LLM_CACHE_TTL"
	// envLLMCacheSize overrides the shared cache max entry count.
	envLLMCacheSize = "AFLARE_LLM_CACHE_SIZE"

	defaultLLMCacheTTL  = 24 * time.Hour
	defaultLLMCacheSize = 1000

	// maxCacheableResponseBytes (M-5) is the per-entry byte cap above which a
	// successful non-streaming response is NOT written to the LLM cache. The
	// cache caps entry COUNT (default 1000) but not entry SIZE; an LLM
	// response can be up to ~10MB, so without a per-entry cap a busy node
	// could pin ~10GB of response strings in memory. 64KB lets short,
	// frequently-repeated answers (the high-value cache hits) be cached
	// while excluding the long-tail large responses that would balloon
	// memory and offer little reuse. The response is still returned to the
	// caller — only the cache write is skipped.
	maxCacheableResponseBytes = 64 * 1024
)

var (
	sharedLLMCacheOnce sync.Once
	sharedLLMCacheInst *cache.Cache
)

// llmCacheEnabledFromEnv reports whether the AFLARE_LLM_CACHE env var
// opts into the shared response cache. Recognised truthy values are "1"
// and "true" (case-insensitive).
func llmCacheEnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(envLLMCacheEnable))) {
	case "1", "true":
		return true
	}
	return false
}

// sharedLLMCache returns a process-wide cache instance built from the
// AFLARE_LLM_CACHE* env vars, or nil if caching is not enabled via env.
// The instance is built lazily on first use and shared by every
// OpenAICompatibleNode that has no cache of its own. Env vars are read at
// most once (sync.Once); set them before process start.
func sharedLLMCache() *cache.Cache {
	sharedLLMCacheOnce.Do(func() {
		if !llmCacheEnabledFromEnv() {
			return
		}
		ttl := defaultLLMCacheTTL
		if raw := os.Getenv(envLLMCacheTTL); raw != "" {
			if d, err := time.ParseDuration(raw); err == nil && d > 0 {
				ttl = d
			}
		}
		size := defaultLLMCacheSize
		if raw := os.Getenv(envLLMCacheSize); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				size = n
			}
		}
		sharedLLMCacheInst = cache.New(cache.Config{
			Enabled:    true,
			MaxEntries: size,
			TTL:        ttl,
		})
	})
	return sharedLLMCacheInst
}

// SetCache attaches a cache instance to this node, enabling LLM response
// caching for non-streaming Execute calls. Passing nil disables caching on
// this node and overrides the AFLARE_LLM_CACHE env var.
//
// L-1: SetCache stores the cache and the enabled flag via atomic operations,
// so it is safe to call concurrently with Execute — a concurrent Execute
// observes either the old or the new cache, never a torn pointer. The
// recommended pattern remains "set once at construction before first
// Execute": the atomicity is a safety net against accidental races, not a
// license to swap caches mid-flight (doing so would let an in-flight
// Execute read from one cache and write back to another, which is benign
// for correctness but would surprise observability and stats).
func (n *OpenAICompatibleNode) SetCache(c *cache.Cache) {
	n.cachePtr.Store(c)
	n.cacheEnabled.Store(c != nil)
}

// effectiveCache returns the cache to consult for this call, or nil when
// caching is disabled. An explicitly injected cache (SetCache) takes
// precedence; otherwise the env-var-driven shared cache is used.
//
// L-1: reads the atomic cacheEnabled flag and cachePtr, so it is safe to
// call concurrently with SetCache.
func (n *OpenAICompatibleNode) effectiveCache() *cache.Cache {
	if !n.cacheEnabled.Load() {
		return nil
	}
	if c := n.cachePtr.Load(); c != nil {
		return c
	}
	return sharedLLMCache()
}

// llmCacheKey derives a stable SHA256 cache key for a non-streaming LLM
// request. The key covers every request field that influences the model's
// output: model, messages (system + user), temperature, max_tokens, top_p,
// frequency_penalty, presence_penalty, stop sequences, seed,
// response_format, tools, tool_choice, and user.
//
// M-4: a hash of the API key (apiKeyHash — first 8 hex chars of sha256) is
// also mixed into the key. Two callers with the SAME (model, prompt,
// params) but DIFFERENT API keys (different accounts / tenants sharing one
// process and one sharedLLMCache) MUST NOT share a cache entry, otherwise
// tenant A's response could be served to tenant B. The raw key is never
// stored or logged; only its short hash participates in the cache key, so
// the cache cannot leak the key. When apiKey is empty (e.g. local ollama),
// the hash is the empty string and the legacy shared-key behaviour is
// preserved.
//
// The endpoint is deliberately excluded: the same model+seed is expected to
// be reproducible regardless of which compatible endpoint served it, and
// including the endpoint would fragment the cache when a failover switches
// the upstream URL.
func llmCacheKey(reqBody LLMRequest, apiKey string) string {
	promptBytes, _ := json.Marshal(reqBody.Messages)
	params := map[string]interface{}{
		"model":             reqBody.Model,
		"temperature":       reqBody.Temperature,
		"max_tokens":        reqBody.MaxTokens,
		"top_p":             reqBody.TopP,
		"frequency_penalty": reqBody.FrequencyPenalty,
		"presence_penalty":  reqBody.PresencePenalty,
		"stop":              reqBody.Stop,
		"response_format":   reqBody.ResponseFormat,
		"tools":             reqBody.Tools,
		"tool_choice":       reqBody.ToolChoice,
		"user":              reqBody.User,
		// M-4: isolate cache entries across API keys / tenants. See
		// apiKeyHash for the rationale and the no-leak guarantee.
		"api_key_hash": apiKeyHash(apiKey),
	}
	if reqBody.Seed != nil {
		params["seed"] = *reqBody.Seed
	}
	return cache.GenerateKey(string(promptBytes), params)
}

// apiKeyHash returns the first 8 hex characters of sha256(apiKey), or the
// empty string when apiKey is empty. The hash isolates LLM cache entries
// across tenants / accounts sharing one process (M-4): two callers with
// identical (model, prompt, params) but different API keys produce
// different cache keys, so tenant A's cached response cannot be served to
// tenant B. Only the short hash participates in the cache key — the raw
// API key is never stored in the cache, logged, or persisted, so this
// cannot leak the key. The empty-string fall-back preserves the legacy
// shared-key behaviour for deployments with no API key (e.g. local
// ollama). 8 hex chars = 32 bits of entropy = ~4 billion buckets, which
// is more than enough to keep tenant collision probability negligible for
// any realistic tenant count while keeping the cache key short.
func apiKeyHash(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])[:8]
}

// checkLLMCache checks the LLM response cache for a non-streaming request.
// On a cache hit it returns the cached content with cacheHit=true.
// On a miss it returns cacheKey and llmCache for later use in cache write.
// For streaming requests (stream=true) it returns empty values immediately.
func (n *OpenAICompatibleNode) checkLLMCache(reqBody LLMRequest, apiKey string, model string, stream bool) (cachedContent string, cacheHit bool, cacheKey string, llmCache *cache.Cache) {
	if stream {
		return
	}
	llmCache = n.effectiveCache()
	if llmCache == nil {
		return
	}
	// M-4: pass apiKey so the cache key includes its hash, isolating
	// entries across tenants / accounts sharing one cache.
	cacheKey = llmCacheKey(reqBody, apiKey)
	if cached, ok := llmCache.Get(cacheKey); ok {
		logger.Debug("[cache hit] LLM response served from cache",
			"node", n.config.Name,
			"provider", n.config.ProviderName,
			"model", model,
		)
		return cached, true, cacheKey, llmCache
	}
	logger.Debug("[cache miss] LLM response not cached, calling upstream",
		"node", n.config.Name,
		"provider", n.config.ProviderName,
		"model", model,
	)
	return "", false, cacheKey, llmCache
}
