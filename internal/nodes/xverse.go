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

type XVERSENode struct{}

func init() {
	Register(&XVERSENode{})
}

func (n *XVERSENode) Name() string {
	return "xverse"
}

type xverseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type xverseRequest struct {
	Model       string         `json:"model"`
	Messages    []xverseMessage `json:"messages"`
	Temperature float64        `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Stream      bool           `json:"stream"`
}

type xverseChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type xverseResponse struct {
	Choices []xverseChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (n *XVERSENode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = "XVERSE-7B-Chat"
	}

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("XVERSE_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("XVERSE API key required. Set XVERSE_API_KEY env var or pass api_key param")
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://api.xverse.cn/v1"
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []xverseMessage{}
	if systemPrompt != "" {
		messages = append(messages, xverseMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, xverseMessage{Role: "user", Content: input})

	reqBody := xverseRequest{
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
		return "", fmt.Errorf("failed to call XVERSE API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp xverseResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("XVERSE API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("XVERSE API returned status %d", resp.StatusCode)
	}

	var xverseResp xverseResponse
	if err := json.NewDecoder(resp.Body).Decode(&xverseResp); err != nil {
		return "", fmt.Errorf("failed to parse XVERSE response: %w", err)
	}

	if len(xverseResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in XVERSE response")
	}

	return xverseResp.Choices[0].Message.Content, nil
}
