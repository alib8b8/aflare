package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OllamaNode implements the Node interface for calling local Ollama models
type OllamaNode struct{}

func init() {
	Register(&OllamaNode{})
}

// Name returns the node name
func (n *OllamaNode) Name() string {
	return "ollama"
}

// ollamaRequest represents the request body for Ollama API
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// ollamaResponse represents the response from Ollama API
type ollamaResponse struct {
	Response string `json:"response"`
}

// Execute implements the Node interface
func (n *OllamaNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	// Get model from params, default to "llama3"
	model, ok := params["model"]
	if !ok || model == "" {
		model = "llama3"
	}

	// Get endpoint from params, default to "http://localhost:11434"
	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "http://localhost:11434"
	}

	// Build the request URL
	generateURL := fmt.Sprintf("%s/api/generate", endpoint)

	// Prepare request body
	reqBody := ollamaRequest{
		Model:  model,
		Prompt: input,
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", generateURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Set up HTTP client with timeout
	client := &http.Client{
		Timeout: 120 * time.Second,
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama not running, please start it first at %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	// Parse response
	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to parse ollama response: %w", err)
	}

	return ollamaResp.Response, nil
}
