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

package nodes

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/alib8b8/llm-box/internal/config"
)

const maxStreamResponseSize = 10 * 1024 * 1024 // 10MB max stream content

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMRequest struct {
	Model       string       `json:"model"`
	Messages    []LLMMessage `json:"messages"`
	Temperature float64      `json:"temperature,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Stream      bool         `json:"stream"`
}

type LLMChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
}

type LLMResponse struct {
	Choices []LLMChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type LLMNodeConfig struct {
	Name            string
	DefaultModel    string
	DefaultEndpoint string
	EnvAPIKey       string
	ProviderName    string
}

type OpenAICompatibleNode struct {
	config LLMNodeConfig
}

func NewOpenAICompatibleNode(config LLMNodeConfig) *OpenAICompatibleNode {
	return &OpenAICompatibleNode{config: config}
}

func (n *OpenAICompatibleNode) Name() string {
	return n.config.Name
}

func (n *OpenAICompatibleNode) Description() string {
	return fmt.Sprintf("Call %s LLM API", n.config.ProviderName)
}

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
		},
	}
}

func (n *OpenAICompatibleNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return n.execute(ctx, input, params, false, nil)
}

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
	if err := validateLMLEndpoint(endpoint); err != nil {
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
		Transport:     safeLLMHTTPClient.Transport,
		CheckRedirect: httpRedirectValidator(validateLMLEndpoint),
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call %s API: %w", n.config.ProviderName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp LLMResponse
		_ = json.NewDecoder(io.LimitReader(resp.Body, maxHTTPResponseSize)).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("%s API error (%d): %s", n.config.ProviderName, resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("%s API returned status %d", n.config.ProviderName, resp.StatusCode)
	}

	if stream {
		return n.readStreamResponse(resp, onChunk)
	}

	var llmResp LLMResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxHTTPResponseSize)).Decode(&llmResp); err != nil {
		return "", fmt.Errorf("failed to parse %s response: %w", n.config.ProviderName, err)
	}

	if len(llmResp.Choices) == 0 {
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
