// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌‌‌​​​​‌​‌‌​​‌​‌‌​‌​‌‌​​​‌​​‌​​‌​‌​​‌‌‌‌​‌​‌‌‌​​​​​​​​​​​​​​​​​​​​​‌‌‌‌​‌‌​​‌‌‌⁠
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
	"fmt"
	"sync/atomic"

	"github.com/alib8b8/aflare/internal/cache"
	"golang.org/x/sync/singleflight"
)

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
	// DescriptionOverride, when non-empty, replaces the default
	// "Call <ProviderName> LLM API" description in Schema(). Used by
	// providers whose default description would mislead users about
	// actual capabilities (e.g. anthropic, which is registered as an
	// OpenAI-compatible provider but the native Anthropic API is NOT
	// OpenAI-compatible and requires a protocol-converting proxy).
	DescriptionOverride string
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

	// sf deduplicates concurrent identical non-streaming requests (P0-3).
	// It is keyed by the LLM cache key, so dedup is active exactly when the
	// response cache is; see doUpstreamCallDeduped for the semantics. The
	// node is only ever used through a pointer and never copied, so the
	// no-copy discipline of the embedded mutex is preserved.
	sf singleflight.Group
}

// NewOpenAICompatibleNode constructs an OpenAICompatibleNode from config.
// Caching is disabled unless the AFLARE_LLM_CACHE env var opts in.
func NewOpenAICompatibleNode(config LLMNodeConfig) *OpenAICompatibleNode {
	n := &OpenAICompatibleNode{config: config}
	n.cacheEnabled.Store(llmCacheEnabledFromEnv())
	return n
}

// Name implements the Node interface.
func (n *OpenAICompatibleNode) Name() string {
	return n.config.Name
}

// Description implements the Node interface.
func (n *OpenAICompatibleNode) Description() string {
	if n.config.DescriptionOverride != "" {
		return n.config.DescriptionOverride
	}
	return fmt.Sprintf("Call %s LLM API", n.config.ProviderName)
}

// Schema implements the Node interface.
func (n *OpenAICompatibleNode) Schema() NodeSchema {
	desc := fmt.Sprintf("Call %s LLM API", n.config.ProviderName)
	if n.config.DescriptionOverride != "" {
		desc = n.config.DescriptionOverride
	}
	return NodeSchema{
		Name:        n.config.Name,
		Description: desc,
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
