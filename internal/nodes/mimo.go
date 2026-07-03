package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type MiMoNode struct{}

func init() {
	Register(&MiMoNode{})
}

func (n *MiMoNode) Name() string {
	return "mimo"
}

type mimoMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type mimoRequest struct {
	Model       string        `json:"model"`
	Messages    []mimoMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_completion_tokens,omitempty"`
	Stream      bool          `json:"stream"`
}

type mimoChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type mimoResponse struct {
	Choices []mimoChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (n *MiMoNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = "mimo-v2.5-pro"
	}

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("MIMO_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("MiMo API key required. Set MIMO_API_KEY env var or pass api_key param")
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://api.xiaomimimo.com/v1"
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []mimoMessage{}
	if systemPrompt != "" {
		messages = append(messages, mimoMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, mimoMessage{Role: "user", Content: input})

	reqBody := mimoRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
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
		Timeout: 120 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call MiMo API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp mimoResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("MiMo API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("MiMo API returned status %d", resp.StatusCode)
	}

	var mimoResp mimoResponse
	if err := json.NewDecoder(resp.Body).Decode(&mimoResp); err != nil {
		return "", fmt.Errorf("failed to parse MiMo response: %w", err)
	}

	if len(mimoResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in MiMo response")
	}

	return mimoResp.Choices[0].Message.Content, nil
}
