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

type IMANode struct{}

func init() {
	Register(&IMANode{})
}

func (n *IMANode) Name() string {
	return "ima"
}

type imaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type imaRequest struct {
	Model       string       `json:"model"`
	Messages    []imaMessage `json:"messages"`
	Temperature float64      `json:"temperature,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Stream      bool         `json:"stream"`
}

type imaChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type imaResponse struct {
	Choices []imaChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (n *IMANode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = "gpt-4o"
	}

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("IMA_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("IMA Copilot API key required. Set IMA_API_KEY env var or pass api_key param")
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = os.Getenv("IMA_API_BASE")
	}
	if endpoint == "" {
		return "", fmt.Errorf("IMA Copilot endpoint required. Set IMA_API_BASE env var or pass endpoint param")
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []imaMessage{}
	if systemPrompt != "" {
		messages = append(messages, imaMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, imaMessage{Role: "user", Content: input})

	reqBody := imaRequest{
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
		return "", fmt.Errorf("failed to call IMA Copilot API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp imaResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("IMA Copilot API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("IMA Copilot API returned status %d", resp.StatusCode)
	}

	var imaResp imaResponse
	if err := json.NewDecoder(resp.Body).Decode(&imaResp); err != nil {
		return "", fmt.Errorf("failed to parse IMA Copilot response: %w", err)
	}

	if len(imaResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in IMA Copilot response")
	}

	return imaResp.Choices[0].Message.Content, nil
}
