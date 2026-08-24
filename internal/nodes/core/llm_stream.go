// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌‌‌​​‌​​‌​‌​‌​‌​​​‌​‌​​​‌‌‌​​‌‌​​​​​​‌​​‌​‌​​​‌‌‌‌​​‌​​​‌‌​​​‌​​​​​​​​​​​​​​​​‌​​‌​‌​​​‌​​​‌‌‌⁠
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
	"sort"
	"strings"
	"sync"

	"github.com/alib8b8/aflare/internal/config"
	aferrors "github.com/alib8b8/aflare/internal/errors"
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
