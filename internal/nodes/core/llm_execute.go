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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/alib8b8/aflare/internal/cache"
	"github.com/alib8b8/aflare/internal/config"
	aferrors "github.com/alib8b8/aflare/internal/errors"
	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/metrics"
)

func (n *OpenAICompatibleNode) execute(ctx context.Context, input string, params map[string]string, stream bool, onChunk func(chunk string)) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = config.GetDefaultModel(n.config.Name, n.config.EnvAPIKey+"_MODEL", n.config.DefaultModel)
	}

	// Resolve endpoint BEFORE the api_key check so we can decide whether
	// the mandatory-key requirement applies. Local LLM servers (vLLM,
	// LM Studio, Ollama's OpenAI-compatible port, text-embeddings-inference)
	// typically don't require an API key, but the compat node used to
	// reject them outright unless the user supplied a dummy key. With
	// AFLARE_LLM_ALLOW_NO_KEY=1 we skip the check for loopback endpoints.
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

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = config.GetAPIKey(n.config.Name, n.config.EnvAPIKey)
	}
	if apiKey == "" {
		if llmAllowNoKey() && isLoopbackEndpoint(endpoint) {
			// Local LLM server (vLLM / LM Studio / Ollama /v1 / TEI): no
			// real key needed. Send a placeholder bearer token so the
			// downstream HTTP request conforms to the OpenAI-compatible
			// Authorization header shape.
			apiKey = "aflare-local-placeholder"
		} else {
			return "", aferrors.Newf(aferrors.CodeLLMAPIAuthError, "%s API key required. Set %s env var, add to config file, or pass api_key param. For local LLM servers without a key, set AFLARE_LLM_ALLOW_NO_KEY=1 and point endpoint at a loopback address.",
				n.config.ProviderName, n.config.EnvAPIKey)
		}
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	// Opt-in LLM 出口密钥脱敏（AFLARE_LLM_REDACT_SECRETS=1，默认关闭）。
	// 在构造 messages 之前对 input 与 systemPrompt 脱敏，确保 prompt 里的
	// secret 不会原样发给 LLM 服务商。脱敏后的文本即为发送/缓存/trace 的内容。
	input, systemPrompt = MaybeRedactLLMSecrets(n.config.ProviderName, input, systemPrompt)
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

	// B-2: telemetry capture. We stamp the call start now (after all
	// validation / marshalling) so Latency reflects actual server round
	// trip. tel accumulates status / usage / error on each path; the
	// deferred publish hands it to the workflow trace sink attached to
	// ctx (no-op if none). Repeated Execute calls within one retry loop
	// each publish their own record with Attempt = 0 — the executor or
	// router is responsible for stamping the retry index if it cares.
	// Every caller publishes its own record, including singleflight
	// followers, whose record reflects the shared flight's outcome.
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

	// P0-3 (singleflight): non-streaming requests go through
	// doUpstreamCallDeduped, which collapses concurrent identical requests
	// into a single upstream call. Streaming requests cannot be shared —
	// each caller owns its SSE body and onChunk callback — so they keep
	// the direct path below.
	if !stream {
		res := n.doUpstreamCallDeduped(ctx, generateURL, jsonBody, apiKey, cacheKey, llmCache, model)
		tel.StatusCode = res.statusCode
		tel.Response = res.content
		tel.Usage = res.usage
		tel.ErrText = res.errText
		return res.content, res.err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, generateURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		tel.ErrText = err.Error()
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{
		Timeout:       DefaultLLMTimeout,
		Transport:     SafeLLMHTTPClient.Transport,
		CheckRedirect: HTTPRedirectValidator(ValidateLMLEndpoint),
	}

	resp, err := client.Do(req) // codeql[go/request-forgery] -- LLM endpoint comes from trusted provider config and is pre-validated by ValidateLMLEndpoint (scheme/userinfo/host + resolved-IP checks); client uses SafeLLMHTTPClient.Transport (dial-time SSRF/IP validation, DNS-rebinding protection), HTTPRedirectValidator(ValidateLMLEndpoint) re-checks redirects, DefaultLLMTimeout; response body capped by MaxHTTPResponseSize
	if err != nil {
		tel.ErrText = err.Error()
		return "", aferrors.Wrapf(err, aferrors.CodeLLMProviderFailed, "failed to call %s API", n.config.ProviderName)
	}
	defer resp.Body.Close()
	tel.StatusCode = resp.StatusCode

	if resp.StatusCode != http.StatusOK {
		var errResp LLMResponse
		_ = json.NewDecoder(io.LimitReader(resp.Body, MaxHTTPResponseSize)).Decode(&errResp) // best-effort: parse error body if present
		if errResp.Error != nil && errResp.Error.Message != "" {
			tel.ErrText = errResp.Error.Message
			return "", aferrors.Newf(aferrors.CodeLLMProviderFailed, "%s API error (%d): %s", n.config.ProviderName, resp.StatusCode, errResp.Error.Message)
		}
		tel.ErrText = fmt.Sprintf("status %d", resp.StatusCode)
		return "", aferrors.Newf(aferrors.CodeLLMProviderFailed, "%s API returned status %d", n.config.ProviderName, resp.StatusCode)
	}

	out, usage, err := n.readStreamResponse(resp, onChunk)
	if err != nil {
		tel.ErrText = err.Error()
	} else {
		// 成功返回 response 前上报出站字节（best-effort，监控器为 nil 时 no-op）
		RecordOutbound(len(out))
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

// llmUpstreamResult is the outcome of one non-streaming upstream round
// trip. It is what a singleflight flight produces: the leader runs the
// round trip once and every follower of the same key receives a copy of
// this struct, from which each caller fills its own telemetry record.
type llmUpstreamResult struct {
	content    string
	err        error
	errText    string // telemetry short text: provider message or "status N"
	statusCode int    // HTTP status, 0 if the request never got a response
	usage      *LLMUsage
}

// doUpstreamCallDeduped performs the non-streaming upstream round trip for
// one Execute call, deduplicating concurrent identical requests via
// singleflight (P0-3): while one request for a given cache key is in
// flight, later identical requests wait for it instead of issuing their
// own upstream call, closing the thundering-herd window between a cache
// miss and the cache write.
//
// Dedup is active only when the LLM response cache is active
// (llmCache != nil, i.e. AFLARE_LLM_CACHE=1 or an injected SetCache
// instance): the cache key already identifies the exact request (model +
// messages + params + hashed API key), and cache users have already opted
// into "identical request → shared response" semantics. With the cache
// off the round trip is executed directly, byte-for-byte matching the
// pre-singleflight behaviour — a best-of-N fan-out sending the same
// prompt N times for independent samples keeps working.
//
// Follower cancellation: a caller whose ctx is cancelled while waiting
// for the shared flight returns ctx.Err() immediately — the same result a
// direct, un-deduped call would have produced — and the flight keeps
// running for its remaining callers. If the leader's request fails, every
// concurrent waiter receives the error; a later retry starts a fresh
// flight (errors are never cached).
func (n *OpenAICompatibleNode) doUpstreamCallDeduped(ctx context.Context, generateURL string, jsonBody []byte, apiKey string, cacheKey string, llmCache *cache.Cache, model string) llmUpstreamResult {
	if llmCache == nil || cacheKey == "" {
		return n.doUpstreamCall(ctx, generateURL, jsonBody, apiKey, cacheKey, llmCache, model)
	}
	ch := n.sf.DoChan(cacheKey, func() (interface{}, error) {
		res := n.doUpstreamCall(ctx, generateURL, jsonBody, apiKey, cacheKey, llmCache, model)
		return res, res.err
	})
	select {
	case r := <-ch:
		res, ok := r.Val.(llmUpstreamResult)
		if !ok {
			// Only reachable when the flight function panicked
			// (r.Val == nil, r.Err carries the recovered panic).
			res = llmUpstreamResult{err: r.Err}
			if r.Err != nil {
				res.errText = r.Err.Error()
			}
		}
		if r.Shared {
			metrics.IncLLMSingleflightShared()
			logger.Debug("[dedup] concurrent identical LLM requests shared one upstream call",
				"node", n.config.Name,
				"provider", n.config.ProviderName,
				"model", model,
			)
		}
		return res
	case <-ctx.Done():
		return llmUpstreamResult{err: ctx.Err(), errText: ctx.Err().Error()}
	}
}

// doUpstreamCall performs exactly one non-streaming HTTP round trip to the
// provider: request construction, status validation, response parsing and
// the LLM cache write (M-4/M-5 semantics preserved). It is the
// singleflight flight body — the leader of a deduplicated request runs
// it and its result is shared with all followers — but it is also called
// directly when dedup is inactive.
func (n *OpenAICompatibleNode) doUpstreamCall(ctx context.Context, generateURL string, jsonBody []byte, apiKey string, cacheKey string, llmCache *cache.Cache, model string) llmUpstreamResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, generateURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		err = fmt.Errorf("failed to create request: %w", err)
		return llmUpstreamResult{err: err, errText: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{
		Timeout:       DefaultLLMTimeout,
		Transport:     SafeLLMHTTPClient.Transport,
		CheckRedirect: HTTPRedirectValidator(ValidateLMLEndpoint),
	}

	resp, err := client.Do(req) // codeql[go/request-forgery] -- LLM endpoint comes from trusted provider config and is pre-validated by ValidateLMLEndpoint (scheme/userinfo/host + resolved-IP checks); client uses SafeLLMHTTPClient.Transport (dial-time SSRF/IP validation, DNS-rebinding protection), HTTPRedirectValidator(ValidateLMLEndpoint) re-checks redirects, DefaultLLMTimeout; response body capped by MaxHTTPResponseSize
	if err != nil {
		err = aferrors.Wrapf(err, aferrors.CodeLLMProviderFailed, "failed to call %s API", n.config.ProviderName)
		return llmUpstreamResult{err: err, errText: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp LLMResponse
		_ = json.NewDecoder(io.LimitReader(resp.Body, MaxHTTPResponseSize)).Decode(&errResp) // best-effort: parse error body if present
		if errResp.Error != nil && errResp.Error.Message != "" {
			return llmUpstreamResult{
				statusCode: resp.StatusCode,
				err:        aferrors.Newf(aferrors.CodeLLMProviderFailed, "%s API error (%d): %s", n.config.ProviderName, resp.StatusCode, errResp.Error.Message),
				errText:    errResp.Error.Message,
			}
		}
		return llmUpstreamResult{
			statusCode: resp.StatusCode,
			err:        aferrors.Newf(aferrors.CodeLLMProviderFailed, "%s API returned status %d", n.config.ProviderName, resp.StatusCode),
			errText:    fmt.Sprintf("status %d", resp.StatusCode),
		}
	}

	var llmResp LLMResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, MaxHTTPResponseSize)).Decode(&llmResp); err != nil {
		err = aferrors.Wrapf(err, aferrors.CodeLLMProviderFailed, "failed to parse %s response", n.config.ProviderName)
		return llmUpstreamResult{statusCode: resp.StatusCode, err: err, errText: err.Error()}
	}

	if len(llmResp.Choices) == 0 {
		err := aferrors.Newf(aferrors.CodeLLMProviderFailed, "no choices in %s response", n.config.ProviderName)
		return llmUpstreamResult{statusCode: resp.StatusCode, err: err, errText: err.Error(), usage: llmResp.Usage}
	}

	content := llmResp.Choices[0].Message.Content

	// M-5: responses larger than maxCacheableResponseBytes are not cached —
	// the cache write is skipped, but the response is still returned.
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
	// 成功返回 response 前上报出站字节（best-effort，监控器为 nil 时 no-op）。
	// 对于被去重的请求这恰好只计一次：真实的上游出站确实只有一份响应。
	RecordOutbound(len(content))
	return llmUpstreamResult{content: content, statusCode: resp.StatusCode, usage: llmResp.Usage}
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

	resp, err := client.Do(req) // codeql[go/request-forgery] -- LLM endpoint comes from trusted provider config and is pre-validated by ValidateLMLEndpoint (scheme/userinfo/host + resolved-IP checks); client uses SafeLLMHTTPClient.Transport (dial-time SSRF/IP validation, DNS-rebinding protection), HTTPRedirectValidator(ValidateLMLEndpoint) re-checks redirects, DefaultLLMTimeout; response body capped by MaxHTTPResponseSize
	if err != nil {
		return nil, aferrors.Wrapf(err, aferrors.CodeLLMProviderFailed, "failed to call %s API", n.config.ProviderName)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp LLMResponse
		_ = json.NewDecoder(io.LimitReader(resp.Body, MaxHTTPResponseSize)).Decode(&errResp) // best-effort: parse error body if present
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
