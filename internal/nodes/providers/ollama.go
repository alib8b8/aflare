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

package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

type OllamaNode struct{}

func init() {
	core.Register(&OllamaNode{})
}

func (n *OllamaNode) Name() string {
	return "ollama"
}

func (n *OllamaNode) Description() string {
	return "Call Ollama local LLM server"
}

func (n *OllamaNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        "ollama",
		Description: "Call Ollama local LLM server",
		Input:       "string - user prompt content (used when prompt param is not provided)",
		Output:      "string - model response content",
		Params: []core.ParamSchema{
			{Name: "model", Type: "string", Description: "Model name (default: llama3)", Required: false, Default: "llama3"},
			{Name: "endpoint", Type: "string", Description: "Ollama server URL (default: http://localhost:11434)", Required: false, Default: "http://localhost:11434"},
			{Name: "prompt", Type: "string", Description: "Prompt to send to Ollama (if not provided, uses input)", Required: false},
		},
	}
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func (n *OllamaNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return n.execute(ctx, input, params, false, nil)
}

func (n *OllamaNode) ExecuteStream(ctx context.Context, input string, params map[string]string, onChunk func(chunk string)) (string, error) {
	return n.execute(ctx, input, params, true, onChunk)
}

func (n *OllamaNode) execute(ctx context.Context, input string, params map[string]string, stream bool, onChunk func(chunk string)) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = "llama3"
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "http://localhost:11434"
	}

	prompt, ok := params["prompt"]
	if !ok || prompt == "" {
		prompt = input
	}

	// Opt-in LLM 出口密钥脱敏（AFLARE_LLM_REDACT_SECRETS=1，默认关闭）。
	// Ollama 无 system 概念，仅对 prompt 脱敏。
	prompt, _ = core.MaybeRedactLLMSecrets("Ollama", prompt, "")

	// Validate endpoint URL to prevent SSRF (localhost is allowed for Ollama)
	if err := core.ValidateLMLEndpoint(endpoint); err != nil {
		return "", fmt.Errorf("endpoint URL validation failed: %w", err)
	}

	generateURL := fmt.Sprintf("%s/api/generate", endpoint)

	reqBody := ollamaRequest{
		Model:  model,
		Prompt: prompt,
		Stream: stream,
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

	client := &http.Client{
		Timeout:       core.DefaultLLMTimeout,
		Transport:     core.SafeLLMHTTPClient.Transport,
		CheckRedirect: core.HTTPRedirectValidator(core.ValidateLMLEndpoint),
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama not running, please start it first (check endpoint configuration): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	if stream {
		out, err := n.readStreamResponse(resp, onChunk)
		if err == nil {
			core.RecordOutbound(len(out))
		}
		return out, err
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to parse ollama response: %w", err)
	}

	core.RecordOutbound(len(ollamaResp.Response))
	return ollamaResp.Response, nil
}

func (n *OllamaNode) readStreamResponse(resp *http.Response, onChunk func(chunk string)) (string, error) {
	scanner := bufio.NewScanner(resp.Body)
	var fullContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var streamResp ollamaResponse
		if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
			continue
		}

		if streamResp.Response != "" {
			fullContent.WriteString(streamResp.Response)
			if onChunk != nil {
				onChunk(streamResp.Response)
			}
		}

		if streamResp.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return fullContent.String(), fmt.Errorf("error reading stream: %w", err)
	}

	return fullContent.String(), nil
}
