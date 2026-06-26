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

type YiNode struct{}

func init() {
	Register(&YiNode{})
}

func (n *YiNode) Name() string {
	return "yi"
}

type yiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type yiRequest struct {
	Model       string      `json:"model"`
	Messages    []yiMessage `json:"messages"`
	Temperature float64     `json:"temperature,omitempty"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Stream      bool        `json:"stream"`
}

type yiChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type yiResponse struct {
	Choices []yiChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (n *YiNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = "yi-lightning"
	}

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("YI_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("Yi API key required. Set YI_API_KEY env var or pass api_key param")
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://api.lingyiwanwu.com/v1"
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []yiMessage{}
	if systemPrompt != "" {
		messages = append(messages, yiMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, yiMessage{Role: "user", Content: input})

	reqBody := yiRequest{
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
		return "", fmt.Errorf("failed to call Yi API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp yiResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("Yi API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("Yi API returned status %d", resp.StatusCode)
	}

	var yiResp yiResponse
	if err := json.NewDecoder(resp.Body).Decode(&yiResp); err != nil {
		return "", fmt.Errorf("failed to parse Yi response: %w", err)
	}

	if len(yiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in Yi response")
	}

	return yiResp.Choices[0].Message.Content, nil
}
