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

type BaichuanNode struct{}

func init() {
	Register(&BaichuanNode{})
}

func (n *BaichuanNode) Name() string {
	return "baichuan"
}

type baichuanMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type baichuanRequest struct {
	Model       string            `json:"model"`
	Messages    []baichuanMessage `json:"messages"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Stream      bool              `json:"stream"`
}

type baichuanChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type baichuanResponse struct {
	Choices []baichuanChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (n *BaichuanNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = "Baichuan4"
	}

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("BAICHUAN_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("Baichuan API key required. Set BAICHUAN_API_KEY env var or pass api_key param")
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://api.baichuan-ai.com/v1"
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []baichuanMessage{}
	if systemPrompt != "" {
		messages = append(messages, baichuanMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, baichuanMessage{Role: "user", Content: input})

	reqBody := baichuanRequest{
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
		return "", fmt.Errorf("failed to call Baichuan API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp baichuanResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("Baichuan API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("Baichuan API returned status %d", resp.StatusCode)
	}

	var bcResp baichuanResponse
	if err := json.NewDecoder(resp.Body).Decode(&bcResp); err != nil {
		return "", fmt.Errorf("failed to parse Baichuan response: %w", err)
	}

	if len(bcResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in Baichuan response")
	}

	return bcResp.Choices[0].Message.Content, nil
}
