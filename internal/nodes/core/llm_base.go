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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alib8b8/llm-box/internal/config"
)

const maxStreamResponseSize = 10 * 1024 * 1024 // 10MB max stream content

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
}

// LLMChoiceMessage is the message returned in a non-streaming choice.
// ToolCalls is populated when the model invokes a function tool.
type LLMChoiceMessage struct {
	Content   string        `json:"content"`
	ToolCalls []LLMToolCall `json:"tool_calls,omitempty"`
}

// LLMToolCall is a single tool/function invocation returned by the model.
type LLMToolCall struct {
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
type LLMNodeConfig struct {
	Name            string
	DefaultModel    string
	DefaultEndpoint string
	EnvAPIKey       string
	ProviderName    string
}

// OpenAICompatibleNode is a Node that talks to any OpenAI-compatible
// /chat/completions endpoint (DeepSeek, Qwen, Kimi, GLM, etc.).
type OpenAICompatibleNode struct {
	config LLMNodeConfig
}

// NewOpenAICompatibleNode constructs an OpenAICompatibleNode from config.
func NewOpenAICompatibleNode(config LLMNodeConfig) *OpenAICompatibleNode {
	return &OpenAICompatibleNode{config: config}
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
		return "", fmt.Errorf("%s API key required. Set %s env var, add to config file, or pass api_key param",
			n.config.ProviderName, n.config.EnvAPIKey)
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = config.GetEndpoint(n.config.Name, n.config.EnvAPIKey+"_ENDPOINT", n.config.DefaultEndpoint)
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

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", generateURL, bytes.NewBuffer(jsonBody))
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
	}
	defer func() {
		tel.Latency = time.Since(callStart)
		sink.RecordLLMCall(tel)
	}()

	resp, err := client.Do(req)
	if err != nil {
		tel.ErrText = err.Error()
		return "", fmt.Errorf("failed to call %s API: %w", n.config.ProviderName, err)
	}
	defer resp.Body.Close()
	tel.StatusCode = resp.StatusCode

	if resp.StatusCode != http.StatusOK {
		var errResp LLMResponse
		_ = json.NewDecoder(io.LimitReader(resp.Body, MaxHTTPResponseSize)).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			tel.ErrText = errResp.Error.Message
			return "", fmt.Errorf("%s API error (%d): %s", n.config.ProviderName, resp.StatusCode, errResp.Error.Message)
		}
		tel.ErrText = fmt.Sprintf("status %d", resp.StatusCode)
		return "", fmt.Errorf("%s API returned status %d", n.config.ProviderName, resp.StatusCode)
	}

	if stream {
		out, err := n.readStreamResponse(resp, onChunk)
		if err != nil {
			tel.ErrText = err.Error()
		}
		// Streaming responses do not surface token usage in our current
		// SSE parser; tel.Usage stays nil. Future work: parse the final
		// usage chunk when stream_options.include_usage is set.
		return out, err
	}

	var llmResp LLMResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, MaxHTTPResponseSize)).Decode(&llmResp); err != nil {
		tel.ErrText = err.Error()
		return "", fmt.Errorf("failed to parse %s response: %w", n.config.ProviderName, err)
	}
	tel.Usage = llmResp.Usage

	if len(llmResp.Choices) == 0 {
		tel.ErrText = "no choices in response"
		return "", fmt.Errorf("no choices in %s response", n.config.ProviderName)
	}

	return llmResp.Choices[0].Message.Content, nil
}

func (n *OpenAICompatibleNode) readStreamResponse(resp *http.Response, onChunk func(chunk string)) (string, error) {
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 256*1024) // 256KB initial buffer
	scanner.Buffer(buf, 1024*1024)   // 1MB max buffer
	var fullContent strings.Builder
	var parseErrors int

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
				return fullContent.String(), fmt.Errorf("too many stream JSON parse errors (last error: %w)", err)
			}
			continue
		}

		if len(streamResp.Choices) > 0 {
			chunk := streamResp.Choices[0].Delta.Content
			if chunk != "" {
				if fullContent.Len()+len(chunk) > maxStreamResponseSize {
					return fullContent.String(), fmt.Errorf("stream response exceeded max size %d bytes", maxStreamResponseSize)
				}
				fullContent.WriteString(chunk)
				if onChunk != nil {
					onChunk(chunk)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fullContent.String(), fmt.Errorf("error reading stream: %w", err)
	}

	return fullContent.String(), nil
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
