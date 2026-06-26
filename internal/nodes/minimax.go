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

type MiniMaxNode struct{}

func init() {
	Register(&MiniMaxNode{})
}

func (n *MiniMaxNode) Name() string {
	return "minimax"
}

type minimaxMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type minimaxRequest struct {
	Model       string           `json:"model"`
	Messages    []minimaxMessage `json:"messages"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream"`
}

type minimaxChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type minimaxResponse struct {
	Choices []minimaxChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (n *MiniMaxNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = "abab6.5s-chat"
	}

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("MINIMAX_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("MiniMax API key required. Set MINIMAX_API_KEY env var or pass api_key param")
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://api.minimax.chat/v1"
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []minimaxMessage{}
	if systemPrompt != "" {
		messages = append(messages, minimaxMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, minimaxMessage{Role: "user", Content: input})

	reqBody := minimaxRequest{
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
		return "", fmt.Errorf("failed to call MiniMax API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp minimaxResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("MiniMax API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("MiniMax API returned status %d", resp.StatusCode)
	}

	var mmResp minimaxResponse
	if err := json.NewDecoder(resp.Body).Decode(&mmResp); err != nil {
		return "", fmt.Errorf("failed to parse MiniMax response: %w", err)
	}

	if len(mmResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in MiniMax response")
	}

	return mmResp.Choices[0].Message.Content, nil
}
