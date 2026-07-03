package nodes

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type OllamaNode struct{}

func init() {
	Register(&OllamaNode{})
}

func (n *OllamaNode) Name() string {
	return "ollama"
}

func (n *OllamaNode) Description() string {
	return "Call Ollama local LLM server"
}

func (n *OllamaNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "ollama",
		Description: "Call Ollama local LLM server",
		Input:       "string - user prompt content",
		Output:      "string - model response content",
		Params: []ParamSchema{
			{Name: "model", Type: "string", Description: "Model name (default: llama3)", Required: false, Default: "llama3"},
			{Name: "endpoint", Type: "string", Description: "Ollama server URL (default: http://localhost:11434)", Required: false, Default: "http://localhost:11434"},
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

	// Validate endpoint URL to prevent SSRF (localhost is allowed for Ollama)
	if err := validateLMLEndpoint(endpoint); err != nil {
		return "", fmt.Errorf("endpoint URL validation failed: %w", err)
	}

	generateURL := fmt.Sprintf("%s/api/generate", endpoint)

	reqBody := ollamaRequest{
		Model:  model,
		Prompt: input,
		Stream: stream,
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

	client := &http.Client{
		Timeout:       120 * time.Second,
		CheckRedirect: httpRedirectValidator(validateLMLEndpoint),
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama not running, please start it first at %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	if stream {
		return n.readStreamResponse(resp, onChunk)
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to parse ollama response: %w", err)
	}

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
