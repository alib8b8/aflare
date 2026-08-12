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

package core

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alib8b8/aflare/internal/cache"
	"github.com/alib8b8/aflare/internal/config"
	aferrors "github.com/alib8b8/aflare/internal/errors"
	"github.com/alib8b8/aflare/internal/logger"
)

const maxStreamResponseSize = 10 * 1024 * 1024 // 10MB max stream content

// streamBufPool reuses the 256KB read buffer handed to bufio.Scanner in
// readStreamResponse. Every streaming LLM call previously allocated a
// fresh 256KB buffer (make([]byte, 0, 256*1024)); under a map node
// fanning out N concurrent LLM calls that is N×256KB of allocator
// pressure per step, directly visible in GC profiles.
//
// We store *[]byte (a pointer to the slice header) rather than []byte so
// the pool item itself does not escape into an interface{}/cause the
// header to be heap-allocated — the standard sync.Pool idiom.
//
// Safety: the buffer is returned to the pool only after scanner.Scan()
// has returned false (the scan loop is over), so no reader touches it
// after Put. The scanner may internally allocate a larger buffer when a
// single SSE line exceeds 256KB (up to the 1MB cap passed to
// Scanner.Buffer); in that case our original 256KB slice is untouched
// and still returnable, and the grown buffer is GC'd. Pool reuse covers
// the common case; large-line streams simply don't benefit.
var streamBufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 256*1024)
		return &b
	},
}

// LLMMessage is a single chat message in an OpenAI-compatible request.
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ResponseFormat controls JSON-mode structured output (OpenAI-compatible).
// Most providers accept {"type":"json_object"}; some also support
// {"type":"json_schema","json_schema":{...}}. We pass the field through
// verbatim so callers can target a specific provider's capabilities.
type ResponseFormat struct {
	Type       string                 `json:"type"`
	JSONSchema map[string]interface{} `json:"json_schema,omitempty"`
}

// ToolDefinition describes a function the model may call.
type ToolDefinition struct {
	Type     string       `json:"type"` // always "function" today
	Function ToolFunction `json:"function"`
}

// ToolFunction is the schema of a callable function.
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolChoice controls whether/which tool the model must call.
// Accepts "none", "auto", or {"type":"function","function":{"name":"..."}}.
// We expose it as a raw json.RawMessage so callers can pass any shape.
type ToolChoice = json.RawMessage

// StreamOptions controls streaming-specific options for an OpenAI-compatible
// /chat/completions request. Today only IncludeUsage is supported: when true
// the provider emits a final chunk carrying token usage (prompt_tokens,
// completion_tokens, total_tokens) before the terminating [DONE] marker.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// LLMRequest is the body sent to an OpenAI-compatible /chat/completions endpoint.
//
// Field policy: fields present before B-1 (Model/Messages/Temperature/
// MaxTokens/Stream) keep their original JSON tags and omitempty behaviour
// for full backward compatibility. New fields added in B-1 are all
// omitempty so that requests from older callers serialize byte-for-byte
// identically to before.
type LLMRequest struct {
	Model       string       `json:"model"`
	Messages    []LLMMessage `json:"messages"`
	Temperature float64      `json:"temperature,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Stream      bool         `json:"stream"`

	// B-1 additions — all omitempty for backward-compatible serialization.
	TopP             float64          `json:"top_p,omitempty"`
	FrequencyPenalty float64          `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64          `json:"presence_penalty,omitempty"`
	Stop             []string         `json:"stop,omitempty"`
	Seed             *int             `json:"seed,omitempty"`
	ResponseFormat   *ResponseFormat  `json:"response_format,omitempty"`
	Tools            []ToolDefinition `json:"tools,omitempty"`
	ToolChoice       ToolChoice       `json:"tool_choice,omitempty"`
	User             string           `json:"user,omitempty"`

	// StreamOptions is sent only when Stream is true. When nil, no
	// stream_options key is serialized, preserving the pre-B-2 wire format.
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
}

// LLMChoiceMessage is the message returned in a non-streaming choice.
// ToolCalls is populated when the model invokes a function tool.
type LLMChoiceMessage struct {
	Content   string        `json:"content"`
	ToolCalls []LLMToolCall `json:"tool_calls,omitempty"`
}

// LLMToolCall is a single tool/function invocation returned by the model.
// In streaming mode the Index field is populated to identify the tool call
// across incremental delta chunks.
type LLMToolCall struct {
	Index    int             `json:"index,omitempty"`
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function LLMToolCallFunc `json:"function"`
}

// LLMToolCallFunc carries the function name and serialized arguments.
type LLMToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // raw JSON string, per OpenAI spec
}

// LLMChoiceDelta is the incremental payload in a streaming choice.
type LLMChoiceDelta struct {
	Content   string        `json:"content"`
	ToolCalls []LLMToolCall `json:"tool_calls,omitempty"`
}

// LLMChoice is a single choice in an OpenAI-compatible response.
// Message is non-streaming; Delta is streaming. ToolCalls are exposed on
// both so callers can react to function invocations in either mode.
type LLMChoice struct {
	Message LLMChoiceMessage `json:"message"`
	Delta   LLMChoiceDelta   `json:"delta"`
}

// LLMUsage reports token accounting from the provider. Most providers
// return prompt_tokens + completion_tokens; some also include
// total_tokens. We expose all three for cost calculation (B-2).
type LLMUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

// LLMResponse is the response body returned by an OpenAI-compatible endpoint.
type LLMResponse struct {
	Choices []LLMChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Usage *LLMUsage `json:"usage,omitempty"`
}

// LLMNodeConfig describes how to construct an OpenAI-compatible node:
// the node name, default model/endpoint, and the env vars used to look
// up the API key and endpoint if not supplied via params.
//
// EnvAPIBase is an optional override for the endpoint env var. When set,
// it is consulted before the default "{EnvAPIKey}_ENDPOINT" lookup so
// that providers whose users conventionally export a differently-named
// base URL (e.g. OPENAI_API_BASE, IMA_API_BASE) keep working.
type LLMNodeConfig struct {
	Name            string
	DefaultModel    string
	DefaultEndpoint string
	EnvAPIKey       string
	EnvAPIBase      string
	ProviderName    string
}

// OpenAICompatibleNode is a Node that talks to any OpenAI-compatible
// /chat/completions endpoint (DeepSeek, Qwen, Kimi, GLM, etc.).
type OpenAICompatibleNode struct {
	config LLMNodeConfig

	// cacheEnabled is the master switch for LLM response caching. It is set
	// at construction from the AFLARE_LLM_CACHE env var, or explicitly via
	// SetCache. When false (the default) Execute bypasses the cache
	// entirely, preserving the pre-cache behaviour so existing tests and
	// dev workflows are unaffected.
	//
	// L-1: stored as an atomic so SetCache (which may run concurrently with
	// Execute on a different goroutine) does not race with the read in
	// effectiveCache. The node is always constructed via NewOpenAICompatible
	// Node (which returns a pointer) and is never copied, so the
	// no-copy discipline of atomic.Bool is preserved.
	cacheEnabled atomic.Bool
	// cache is an optional per-node cache instance. When non-nil it takes
	// precedence over the env-var-driven shared cache. nil means "fall back
	// to the shared cache" (which is itself nil unless AFLARE_LLM_CACHE=1).
	//
	// L-1: stored as an atomic.Pointer so SetCache is safe to call
	// concurrently with Execute — a concurrent Execute observes either the
	// old or the new cache, never a torn pointer.
	cachePtr atomic.Pointer[cache.Cache]
}

// NewOpenAICompatibleNode constructs an OpenAICompatibleNode from config.
// Caching is disabled unless the AFLARE_LLM_CACHE env var opts in.
func NewOpenAICompatibleNode(config LLMNodeConfig) *OpenAICompatibleNode {
	n := &OpenAICompatibleNode{config: config}
	n.cacheEnabled.Store(llmCacheEnabledFromEnv())
	return n
}

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

// Name implements the Node interface.
func (n *OpenAICompatibleNode) Name() string {
	return n.config.Name
}

// Description implements the Node interface.
func (n *OpenAICompatibleNode) Description() string {
	return fmt.Sprintf("Call %s LLM API", n.config.ProviderName)
}

// Schema implements the Node interface.
func (n *OpenAICompatibleNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.config.Name,
		Description: fmt.Sprintf("Call %s LLM API", n.config.ProviderName),
		Input:       "string - user message content",
		Output:      "string - AI response content",
		Params: []ParamSchema{
			{Name: "model", Type: "string", Description: fmt.Sprintf("Model name (default: %s)", n.config.DefaultModel), Required: false, Default: n.config.DefaultModel},
			{Name: "api_key", Type: "string", Description: fmt.Sprintf("%s API key (or set %s env var)", n.config.ProviderName, n.config.EnvAPIKey), Required: false},
			{Name: "endpoint", Type: "string", Description: fmt.Sprintf("API base URL (default: %s)", n.config.DefaultEndpoint), Required: false, Default: n.config.DefaultEndpoint},
			{Name: "system", Type: "string", Description: "System prompt", Required: false},
			{Name: "temperature", Type: "string", Description: "Sampling temperature 0.0-2.0 (default: provider default)", Required: false},
			{Name: "max_tokens", Type: "string", Description: "Max tokens to generate", Required: false},
			{Name: "top_p", Type: "string", Description: "Nucleus sampling probability mass 0.0-1.0", Required: false},
			{Name: "frequency_penalty", Type: "string", Description: "Penalty for repeated tokens -2.0 to 2.0", Required: false},
			{Name: "presence_penalty", Type: "string", Description: "Penalty for new tokens -2.0 to 2.0", Required: false},
			{Name: "stop", Type: "string", Description: "Stop sequences (comma-separated, e.g. '\\n,END')", Required: false},
			{Name: "seed", Type: "string", Description: "Random seed for deterministic sampling (int)", Required: false},
			{Name: "response_format", Type: "string", Description: "Structured output: 'json_object' or 'json_schema:<schema_json>'", Required: false},
			{Name: "tools", Type: "string", Description: "JSON array of tool definitions for function calling", Required: false},
			{Name: "tool_choice", Type: "string", Description: "Tool selection: 'none', 'auto', or JSON object", Required: false},
			{Name: "user", Type: "string", Description: "End-user identifier for provider-side abuse monitoring", Required: false},
		},
	}
}

// Execute implements the Node interface.
func (n *OpenAICompatibleNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return n.execute(ctx, input, params, false, nil)
}

// ExecuteStream streams the response, calling onChunk for each token chunk.
func (n *OpenAICompatibleNode) ExecuteStream(ctx context.Context, input string, params map[string]string, onChunk func(chunk string)) (string, error) {
	return n.execute(ctx, input, params, true, onChunk)
}

func (n *OpenAICompatibleNode) execute(ctx context.Context, input string, params map[string]string, stream bool, onChunk func(chunk string)) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = config.GetDefaultModel(n.config.Name, n.config.EnvAPIKey+"_MODEL", n.config.DefaultModel)
	}

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = config.GetAPIKey(n.config.Name, n.config.EnvAPIKey)
	}
	if apiKey == "" {
		return "", aferrors.Newf(aferrors.CodeLLMAPIAuthError, "%s API key required. Set %s env var, add to config file, or pass api_key param",
			n.config.ProviderName, n.config.EnvAPIKey)
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		// Prefer a provider-specific base-URL env var (e.g. OPENAI_API_BASE)
		// when configured, then fall back to the generic "{KEY}_ENDPOINT"
		// lookup and finally the static default endpoint.
		envEndpointVar := n.config.EnvAPIKey + "_ENDPOINT"
		if n.config.EnvAPIBase != "" {
			if v := os.Getenv(n.config.EnvAPIBase); v != "" {
				endpoint = v
			}
		}
		if endpoint == "" {
			endpoint = config.GetEndpoint(n.config.Name, envEndpointVar, n.config.DefaultEndpoint)
		}
	}

	// Validate endpoint URL to prevent SSRF + API key leakage
	if err := ValidateLMLEndpoint(endpoint); err != nil {
		return "", fmt.Errorf("endpoint URL validation failed: %w", err)
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []LLMMessage{}
	if systemPrompt != "" {
		messages = append(messages, LLMMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, LLMMessage{Role: "user", Content: input})

	reqBody := LLMRequest{
		Model:    model,
		Messages: messages,
		Stream:   stream,
	}
	if err := applyLLMRequestParams(&reqBody, params); err != nil {
		return "", err
	}

	// For streaming requests, ask the provider to emit a final usage chunk.
	// Most OpenAI-compatible providers honour stream_options.include_usage;
	// providers that ignore it simply omit usage, which readStreamResponse
	// tolerates (tel.Usage stays nil).
	if stream {
		reqBody.StreamOptions = &StreamOptions{IncludeUsage: true}
	}

	// LLM response cache — non-streaming only.
	cachedContent, cacheHit, cacheKey, llmCache := n.checkLLMCache(reqBody, apiKey, model, stream)
	if cacheHit {
		return cachedContent, nil
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, generateURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{
		Timeout:       DefaultLLMTimeout,
		Transport:     SafeLLMHTTPClient.Transport,
		CheckRedirect: HTTPRedirectValidator(ValidateLMLEndpoint),
	}

	// B-2: telemetry capture. We stamp the call start now (after all
	// validation / marshalling) so Latency reflects actual server round
	// trip. tel accumulates status / usage / error on each path; the
	// deferred publish hands it to the workflow trace sink attached to
	// ctx (no-op if none). Repeated Execute calls within one retry loop
	// each publish their own record with Attempt = 0 — the executor or
	// router is responsible for stamping the retry index if it cares.
	callStart := time.Now()
	sink := LLMCallSinkFrom(ctx)
	tel := LLMCallTelemetry{
		NodeName: n.config.Name,
		Provider: n.config.ProviderName,
		Model:    model,
		Endpoint: endpoint,
		Stream:   stream,
		Prompt:   input,
	}
	defer func() {
		tel.Latency = time.Since(callStart)
		sink.RecordLLMCall(tel)
	}()

	resp, err := client.Do(req)
	if err != nil {
		tel.ErrText = err.Error()
		return "", aferrors.Wrapf(err, aferrors.CodeLLMProviderFailed, "failed to call %s API", n.config.ProviderName)
	}
	defer resp.Body.Close()
	tel.StatusCode = resp.StatusCode

	if resp.StatusCode != http.StatusOK {
		var errResp LLMResponse
		_ = json.NewDecoder(io.LimitReader(resp.Body, MaxHTTPResponseSize)).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			tel.ErrText = errResp.Error.Message
			return "", aferrors.Newf(aferrors.CodeLLMProviderFailed, "%s API error (%d): %s", n.config.ProviderName, resp.StatusCode, errResp.Error.Message)
		}
		tel.ErrText = fmt.Sprintf("status %d", resp.StatusCode)
		return "", aferrors.Newf(aferrors.CodeLLMProviderFailed, "%s API returned status %d", n.config.ProviderName, resp.StatusCode)
	}

	if stream {
		out, usage, err := n.readStreamResponse(resp, onChunk)
		if err != nil {
			tel.ErrText = err.Error()
		}
		tel.Response = out
		// Streaming usage is only populated when the provider emits a
		// usage chunk (requires stream_options.include_usage). Tolerant:
		// providers that omit usage leave tel.Usage nil, mirroring the
		// non-streaming path where Usage is nil if the provider omits it.
		if usage != nil {
			tel.Usage = usage
		}
		return out, err
	}

	return n.processNonStreamingResponse(resp, &tel, cacheKey, llmCache, model)
}

func (n *OpenAICompatibleNode) processNonStreamingResponse(resp *http.Response, tel *LLMCallTelemetry, cacheKey string, llmCache *cache.Cache, model string) (string, error) {
	var llmResp LLMResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, MaxHTTPResponseSize)).Decode(&llmResp); err != nil {
		tel.ErrText = err.Error()
		return "", aferrors.Wrapf(err, aferrors.CodeLLMProviderFailed, "failed to parse %s response", n.config.ProviderName)
	}
	tel.Usage = llmResp.Usage

	if len(llmResp.Choices) == 0 {
		tel.ErrText = "no choices in response"
		return "", aferrors.Newf(aferrors.CodeLLMProviderFailed, "no choices in %s response", n.config.ProviderName)
	}

	content := llmResp.Choices[0].Message.Content
	tel.Response = content

	if cacheKey != "" && llmCache != nil && llmResp.Error == nil {
		if len(content) > maxCacheableResponseBytes {
			logger.Debug("[cache skip] LLM response exceeds per-entry size cap, not cached",
				"node", n.config.Name,
				"provider", n.config.ProviderName,
				"model", model,
				"content_bytes", len(content),
				"cap", maxCacheableResponseBytes,
			)
		} else {
			llmCache.Set(cacheKey, content)
		}
	}
	return content, nil
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

func (n *OpenAICompatibleNode) readStreamResponse(resp *http.Response, onChunk func(chunk string)) (string, *LLMUsage, error) {
	// Pull a 256KB read buffer from the pool instead of allocating one
	// per call. bufPtr is a *[]byte so the slice header itself is reused
	// rather than heap-allocated by sync.Pool's interface{} boxing. We
	// reset to length 0 (preserving capacity) so the scanner starts with
	// a clean working slice; bufio.Scanner.Buffer then takes ownership
	// for the duration of the scan loop.
	bufPtr := streamBufPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]
	// Return the buffer to the pool when we exit, whichever path taken.
	// By this point scanner.Scan() has returned false (loop over) so the
	// scanner no longer touches buf; the pooled slice is safe to hand to
	// another goroutine. We reset to length 0 again before Put to avoid
	// retaining references to SSE payload bytes that could otherwise
	// delay their collection.
	defer func() {
		*bufPtr = buf[:0]
		streamBufPool.Put(bufPtr)
	}()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(buf, 1024*1024) // 1MB max buffer
	var fullContent strings.Builder
	var parseErrors int
	var usage *LLMUsage

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var streamResp LLMResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			parseErrors++
			if parseErrors > 10 {
				return fullContent.String(), usage, fmt.Errorf("too many stream JSON parse errors (last error: %w)", err)
			}
			continue
		}

		// OpenAI-compatible providers emit a final chunk carrying token
		// usage when stream_options.include_usage is set. The chunk may
		// arrive with an empty choices array (OpenAI) or alongside the
		// last delta (some providers), so capture usage on every chunk
		// and let the last non-nil value win. Providers that omit usage
		// entirely leave usage nil — tolerant by design.
		if streamResp.Usage != nil {
			usage = streamResp.Usage
		}

		if len(streamResp.Choices) > 0 {
			chunk := streamResp.Choices[0].Delta.Content
			if chunk != "" {
				if fullContent.Len()+len(chunk) > maxStreamResponseSize {
					return fullContent.String(), usage, fmt.Errorf("stream response exceeded max size %d bytes", maxStreamResponseSize)
				}
				fullContent.WriteString(chunk)
				if onChunk != nil {
					onChunk(chunk)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fullContent.String(), usage, fmt.Errorf("error reading stream: %w", err)
	}

	return fullContent.String(), usage, nil
}

// applyLLMRequestParams populates the B-1 extended fields of req from the
// params map. It is strict: malformed numeric/JSON values return an error
// rather than being silently dropped, so callers can trust that a
// populated field reached the provider.
//
// Recognised param keys (all string-valued, consistent with the Node API):
//
//	temperature        - float in [0, 2]
//	max_tokens         - positive int
//	top_p              - float in (0, 1]
//	frequency_penalty  - float in [-2, 2]
//	presence_penalty   - float in [-2, 2]
//	stop               - comma-separated list, e.g. "\n,END"
//	seed               - int
//	response_format    - "json_object" or "json_schema:<schema_json>"
//	tools              - JSON array of ToolDefinition
//	tool_choice        - "none", "auto", or a JSON object
//	user               - end-user identifier string
func applyLLMRequestParams(req *LLMRequest, params map[string]string) error {
	if v, ok := params["temperature"]; ok && v != "" {
		f, err := parseFloatParam(v, "temperature", 0, 2)
		if err != nil {
			return err
		}
		req.Temperature = f
	}
	if v, ok := params["max_tokens"]; ok && v != "" {
		n, err := parseIntParam(v, "max_tokens")
		if err != nil {
			return err
		}
		if n <= 0 {
			return fmt.Errorf("max_tokens must be positive, got %d", n)
		}
		// Cap at a sane upper bound to prevent abuse; providers reject
		// extreme values anyway, but we want a friendly error rather
		// than an opaque 4xx from upstream.
		if n > 128000 {
			return fmt.Errorf("max_tokens %d exceeds upper bound 128000", n)
		}
		req.MaxTokens = n
	}
	if v, ok := params["top_p"]; ok && v != "" {
		f, err := parseFloatParam(v, "top_p", 0, 1)
		if err != nil {
			return err
		}
		req.TopP = f
	}
	if v, ok := params["frequency_penalty"]; ok && v != "" {
		f, err := parseFloatParam(v, "frequency_penalty", -2, 2)
		if err != nil {
			return err
		}
		req.FrequencyPenalty = f
	}
	if v, ok := params["presence_penalty"]; ok && v != "" {
		f, err := parseFloatParam(v, "presence_penalty", -2, 2)
		if err != nil {
			return err
		}
		req.PresencePenalty = f
	}
	if v, ok := params["stop"]; ok && v != "" {
		// Split on comma; each entry used verbatim. An empty entry (e.g.
		// from a trailing comma) is dropped to avoid sending an empty
		// stop string, which some providers reject.
		parts := strings.Split(v, ",")
		for _, p := range parts {
			if p != "" {
				req.Stop = append(req.Stop, p)
			}
		}
		// OpenAI allows at most 4 stop sequences; other providers have
		// similar limits. Cap at a small bound to avoid an opaque 4xx.
		if len(req.Stop) > 16 {
			return fmt.Errorf("too many stop sequences: %d (max 16)", len(req.Stop))
		}
	}
	if v, ok := params["seed"]; ok && v != "" {
		n, err := parseIntParam(v, "seed")
		if err != nil {
			return err
		}
		req.Seed = &n
	}
	if v, ok := params["response_format"]; ok && v != "" {
		rf, err := parseResponseFormatParam(v)
		if err != nil {
			return err
		}
		req.ResponseFormat = rf
	}
	if v, ok := params["tools"]; ok && v != "" {
		var tools []ToolDefinition
		if err := json.Unmarshal([]byte(v), &tools); err != nil {
			return fmt.Errorf("tools must be a JSON array of tool definitions: %w", err)
		}
		if len(tools) == 0 {
			return fmt.Errorf("tools array must not be empty")
		}
		req.Tools = tools
	}
	if v, ok := params["tool_choice"]; ok && v != "" {
		// Accept bare "none"/"auto" strings or a JSON object.
		if v == "none" || v == "auto" {
			req.ToolChoice = json.RawMessage(`"` + v + `"`)
		} else {
			// Validate it parses as JSON; store raw.
			var probe json.RawMessage
			if err := json.Unmarshal([]byte(v), &probe); err != nil {
				return fmt.Errorf("tool_choice must be 'none', 'auto', or a JSON object: %w", err)
			}
			req.ToolChoice = json.RawMessage(v)
		}
	}
	if v, ok := params["user"]; ok && v != "" {
		req.User = v
	}
	return nil
}

// parseResponseFormatParam parses the response_format param value.
// Accepted forms:
//
//	json_object                            -> {"type":"json_object"}
//	json_schema:{"name":"...","schema":{}} -> {"type":"json_schema","json_schema":{...}}
//	{"type":"json_object"}                  (raw JSON passed through)
func parseResponseFormatParam(v string) (*ResponseFormat, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	// Raw JSON object form.
	if strings.HasPrefix(v, "{") {
		var rf ResponseFormat
		if err := json.Unmarshal([]byte(v), &rf); err != nil {
			return nil, fmt.Errorf("response_format JSON invalid: %w", err)
		}
		if rf.Type == "" {
			return nil, fmt.Errorf("response_format JSON missing 'type'")
		}
		return &rf, nil
	}
	// Keyword form.
	if v == "json_object" {
		return &ResponseFormat{Type: "json_object"}, nil
	}
	if strings.HasPrefix(v, "json_schema:") {
		rest := strings.TrimSpace(v[len("json_schema:"):])
		var schema map[string]interface{}
		if err := json.Unmarshal([]byte(rest), &schema); err != nil {
			return nil, fmt.Errorf("response_format json_schema payload invalid: %w", err)
		}
		return &ResponseFormat{Type: "json_schema", JSONSchema: schema}, nil
	}
	return nil, fmt.Errorf("response_format must be 'json_object', 'json_schema:<json>', or a raw JSON object")
}

func parseFloatParam(v, name string, min, max float64) (float64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number, got %q: %w", name, v, err)
	}
	if f < min || f > max {
		return 0, fmt.Errorf("%s must be in [%g, %g], got %g", name, min, max, f)
	}
	return f, nil
}

func parseIntParam(v, name string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q: %w", name, v, err)
	}
	return n, nil
}

// CallWithTools sends a chat completion request with structured messages and
// tool definitions, returning the full LLMResponse so the caller can inspect
// tool_calls for native function calling. This is the low-level API used by
// the ReAct agent; the higher-level Execute method flattens the response to a
// string for non-agent callers.
//
// When tools is non-empty, the request includes the tools array and
// tool_choice: "auto" so the model can choose to invoke functions.
func (n *OpenAICompatibleNode) CallWithTools(ctx context.Context, messages []LLMMessage, model, apiKey, endpoint string, tools []ToolDefinition, params map[string]string) (*LLMResponse, error) {
	if apiKey == "" {
		apiKey = config.GetAPIKey(n.config.Name, n.config.EnvAPIKey)
	}
	if apiKey == "" {
		return nil, aferrors.Newf(aferrors.CodeLLMAPIAuthError, "%s API key required. Set %s env var, add to config file, or pass api_key param",
			n.config.ProviderName, n.config.EnvAPIKey)
	}
	if endpoint == "" {
		endpoint = config.GetEndpoint(n.config.Name, n.config.EnvAPIKey+"_ENDPOINT", n.config.DefaultEndpoint)
	}
	if model == "" {
		model = n.config.DefaultModel
	}
	if err := ValidateLMLEndpoint(endpoint); err != nil {
		return nil, aferrors.Wrap(err, aferrors.CodeLLMProviderFailed, "endpoint URL validation failed")
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	reqBody := LLMRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}
	if len(tools) > 0 {
		reqBody.Tools = tools
		reqBody.ToolChoice = json.RawMessage(`"auto"`)
	}
	if params != nil {
		if err := applyLLMRequestParams(&reqBody, params); err != nil {
			return nil, err
		}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, aferrors.Wrap(err, aferrors.CodeLLMProviderFailed, "failed to marshal request")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, generateURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, aferrors.Wrap(err, aferrors.CodeLLMProviderFailed, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{
		Timeout:       DefaultLLMTimeout,
		Transport:     SafeLLMHTTPClient.Transport,
		CheckRedirect: HTTPRedirectValidator(ValidateLMLEndpoint),
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, aferrors.Wrapf(err, aferrors.CodeLLMProviderFailed, "failed to call %s API", n.config.ProviderName)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp LLMResponse
		_ = json.NewDecoder(io.LimitReader(resp.Body, MaxHTTPResponseSize)).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return nil, aferrors.Newf(aferrors.CodeLLMProviderFailed, "%s API error (%d): %s", n.config.ProviderName, resp.StatusCode, errResp.Error.Message)
		}
		return nil, aferrors.Newf(aferrors.CodeLLMProviderFailed, "%s API returned status %d", n.config.ProviderName, resp.StatusCode)
	}

	var llmResp LLMResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, MaxHTTPResponseSize)).Decode(&llmResp); err != nil {
		return nil, aferrors.Wrapf(err, aferrors.CodeLLMProviderFailed, "failed to parse %s response", n.config.ProviderName)
	}
	if len(llmResp.Choices) == 0 {
		return nil, aferrors.Newf(aferrors.CodeLLMProviderFailed, "no choices in %s response", n.config.ProviderName)
	}

	return &llmResp, nil
}

// CallWithToolsStream sends a streaming chat completion request with tool
// definitions. It accumulates content and tool_calls from the SSE stream,
// calling onChunk for each content delta. Returns the final LLMResponse
// with the accumulated content and tool_calls.
func (n *OpenAICompatibleNode) CallWithToolsStream(ctx context.Context, messages []LLMMessage, model, apiKey, endpoint string, tools []ToolDefinition, params map[string]string, onChunk func(chunk string)) (*LLMResponse, error) {
	if apiKey == "" {
		apiKey = config.GetAPIKey(n.config.Name, n.config.EnvAPIKey)
	}
	if apiKey == "" {
		return nil, aferrors.Newf(aferrors.CodeLLMAPIAuthError, "%s API key required. Set %s env var, add to config file, or pass api_key param",
			n.config.ProviderName, n.config.EnvAPIKey)
	}
	if endpoint == "" {
		endpoint = config.GetEndpoint(n.config.Name, n.config.EnvAPIKey+"_ENDPOINT", n.config.DefaultEndpoint)
	}
	if model == "" {
		model = n.config.DefaultModel
	}
	if err := ValidateLMLEndpoint(endpoint); err != nil {
		return nil, aferrors.Wrap(err, aferrors.CodeLLMProviderFailed, "endpoint URL validation failed")
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	reqBody := LLMRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}
	if len(tools) > 0 {
		reqBody.Tools = tools
		reqBody.ToolChoice = json.RawMessage(`"auto"`)
	}
	if params != nil {
		if err := applyLLMRequestParams(&reqBody, params); err != nil {
			return nil, err
		}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, aferrors.Wrap(err, aferrors.CodeLLMProviderFailed, "failed to marshal request")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, generateURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, aferrors.Wrap(err, aferrors.CodeLLMProviderFailed, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{
		Timeout:       DefaultLLMTimeout,
		Transport:     SafeLLMHTTPClient.Transport,
		CheckRedirect: HTTPRedirectValidator(ValidateLMLEndpoint),
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, aferrors.Wrapf(err, aferrors.CodeLLMProviderFailed, "failed to call %s API", n.config.ProviderName)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp LLMResponse
		_ = json.NewDecoder(io.LimitReader(resp.Body, MaxHTTPResponseSize)).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return nil, aferrors.Newf(aferrors.CodeLLMProviderFailed, "%s API error (%d): %s", n.config.ProviderName, resp.StatusCode, errResp.Error.Message)
		}
		return nil, aferrors.Newf(aferrors.CodeLLMProviderFailed, "%s API returned status %d", n.config.ProviderName, resp.StatusCode)
	}

	// Parse SSE stream: accumulate content and tool_calls
	var fullContent strings.Builder
	var toolCalls []LLMToolCall
	// toolCallAccum tracks in-progress tool_calls by index (streaming tool_calls arrive incrementally)
	toolCallAccum := make(map[int]*LLMToolCall)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var streamResp LLMResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue
		}

		if len(streamResp.Choices) == 0 {
			continue
		}

		delta := streamResp.Choices[0].Delta

		// Content streaming
		if delta.Content != "" {
			fullContent.WriteString(delta.Content)
			if onChunk != nil {
				onChunk(delta.Content)
			}
		}

		// Tool call accumulation (streaming tool_calls arrive as incremental chunks)
		for _, tc := range delta.ToolCalls {
			idx := tc.Index
			if existing, ok := toolCallAccum[idx]; ok {
				// Update ID/Name if the new chunk carries them (some providers
				// split ID/Name and arguments across separate chunks).
				if tc.ID != "" {
					existing.ID = tc.ID
				}
				if tc.Function.Name != "" {
					existing.Function.Name = tc.Function.Name
				}
				existing.Function.Arguments += tc.Function.Arguments
			} else {
				cp := tc
				toolCallAccum[idx] = &cp
			}
		}
	}

	// Convert accumulated tool_calls to slice, sorted by index
	if len(toolCallAccum) > 0 {
		indices := make([]int, 0, len(toolCallAccum))
		for i := range toolCallAccum {
			indices = append(indices, i)
		}
		sort.Ints(indices)
		for _, i := range indices {
			toolCalls = append(toolCalls, *toolCallAccum[i])
		}
	}

	return &LLMResponse{
		Choices: []LLMChoice{
			{
				Message: LLMChoiceMessage{
					Content:   fullContent.String(),
					ToolCalls: toolCalls,
				},
			},
		},
	}, nil
}
