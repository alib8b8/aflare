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

type QwenNode struct{}

func init() {
	Register(&QwenNode{})
}

func (n *QwenNode) Name() string {
	return "qwen"
}

type qwenMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type qwenRequest struct {
	Model       string        `json:"model"`
	Messages    []qwenMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream"`
}

type qwenChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type qwenResponse struct {
	Choices []qwenChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (n *QwenNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = "qwen-turbo"
	}

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("QWEN_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("Qwen API key required. Set QWEN_API_KEY env var or pass api_key param")
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []qwenMessage{}
	if systemPrompt != "" {
		messages = append(messages, qwenMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, qwenMessage{Role: "user", Content: input})

	reqBody := qwenRequest{
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
		return "", fmt.Errorf("failed to call Qwen API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp qwenResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("Qwen API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("Qwen API returned status %d", resp.StatusCode)
	}

	var qwenResp qwenResponse
	if err := json.NewDecoder(resp.Body).Decode(&qwenResp); err != nil {
		return "", fmt.Errorf("failed to parse Qwen response: %w", err)
	}

	if len(qwenResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in Qwen response")
	}

	return qwenResp.Choices[0].Message.Content, nil
}
