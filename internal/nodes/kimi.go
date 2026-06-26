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

type KimiNode struct{}

func init() {
	Register(&KimiNode{})
}

func (n *KimiNode) Name() string {
	return "kimi"
}

type kimiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type kimiRequest struct {
	Model       string        `json:"model"`
	Messages    []kimiMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream"`
}

type kimiChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type kimiResponse struct {
	Choices []kimiChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (n *KimiNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = "moonshot-v1-8k"
	}

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("KIMI_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("Kimi API key required. Set KIMI_API_KEY env var or pass api_key param")
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://api.moonshot.cn/v1"
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []kimiMessage{}
	if systemPrompt != "" {
		messages = append(messages, kimiMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, kimiMessage{Role: "user", Content: input})

	reqBody := kimiRequest{
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
		return "", fmt.Errorf("failed to call Kimi API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp kimiResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("Kimi API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("Kimi API returned status %d", resp.StatusCode)
	}

	var kimiResp kimiResponse
	if err := json.NewDecoder(resp.Body).Decode(&kimiResp); err != nil {
		return "", fmt.Errorf("failed to parse Kimi response: %w", err)
	}

	if len(kimiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in Kimi response")
	}

	return kimiResp.Choices[0].Message.Content, nil
}
