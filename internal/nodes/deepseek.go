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

type DeepSeekNode struct{}

func init() {
	Register(&DeepSeekNode{})
}

func (n *DeepSeekNode) Name() string {
	return "deepseek"
}

type deepseekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepseekRequest struct {
	Model       string            `json:"model"`
	Messages    []deepseekMessage `json:"messages"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Stream      bool              `json:"stream"`
}

type deepseekChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type deepseekResponse struct {
	Choices []deepseekChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (n *DeepSeekNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = "deepseek-chat"
	}

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("DEEPSEEK_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("DeepSeek API key required. Set DEEPSEEK_API_KEY env var or pass api_key param")
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://api.deepseek.com/v1"
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []deepseekMessage{}
	if systemPrompt != "" {
		messages = append(messages, deepseekMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, deepseekMessage{Role: "user", Content: input})

	reqBody := deepseekRequest{
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
		return "", fmt.Errorf("failed to call DeepSeek API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp deepseekResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("DeepSeek API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("DeepSeek API returned status %d", resp.StatusCode)
	}

	var dsResp deepseekResponse
	if err := json.NewDecoder(resp.Body).Decode(&dsResp); err != nil {
		return "", fmt.Errorf("failed to parse DeepSeek response: %w", err)
	}

	if len(dsResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in DeepSeek response")
	}

	return dsResp.Choices[0].Message.Content, nil
}
