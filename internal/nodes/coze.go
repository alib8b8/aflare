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

type CozeNode struct{}

func init() {
	Register(&CozeNode{})
}

func (n *CozeNode) Name() string {
	return "coze"
}

type cozeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type cozeRequest struct {
	Model       string         `json:"model"`
	Messages    []cozeMessage  `json:"messages"`
	Temperature float64        `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Stream      bool           `json:"stream"`
}

type cozeChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type cozeResponse struct {
	Choices []cozeChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (n *CozeNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = "glm-4"
	}

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("COZE_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("Coze API key required. Set COZE_API_KEY env var or pass api_key param")
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://api.coze.cn/v1"
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []cozeMessage{}
	if systemPrompt != "" {
		messages = append(messages, cozeMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, cozeMessage{Role: "user", Content: input})

	reqBody := cozeRequest{
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
		return "", fmt.Errorf("failed to call Coze API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp cozeResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("Coze API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("Coze API returned status %d", resp.StatusCode)
	}

	var cozeResp cozeResponse
	if err := json.NewDecoder(resp.Body).Decode(&cozeResp); err != nil {
		return "", fmt.Errorf("failed to parse Coze response: %w", err)
	}

	if len(cozeResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in Coze response")
	}

	return cozeResp.Choices[0].Message.Content, nil
}
