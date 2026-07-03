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

type GLMNode struct{}

func init() {
	Register(&GLMNode{})
}

func (n *GLMNode) Name() string {
	return "glm"
}

type glmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type glmRequest struct {
	Model       string       `json:"model"`
	Messages    []glmMessage `json:"messages"`
	Temperature float64      `json:"temperature,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Stream      bool         `json:"stream"`
}

type glmChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type glmResponse struct {
	Choices []glmChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (n *GLMNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = "glm-4"
	}

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("GLM_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("GLM API key required. Set GLM_API_KEY env var or pass api_key param")
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://open.bigmodel.cn/api/paas/v4"
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []glmMessage{}
	if systemPrompt != "" {
		messages = append(messages, glmMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, glmMessage{Role: "user", Content: input})

	reqBody := glmRequest{
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
		return "", fmt.Errorf("failed to call GLM API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp glmResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("GLM API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("GLM API returned status %d", resp.StatusCode)
	}

	var glmResp glmResponse
	if err := json.NewDecoder(resp.Body).Decode(&glmResp); err != nil {
		return "", fmt.Errorf("failed to parse GLM response: %w", err)
	}

	if len(glmResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in GLM response")
	}

	return glmResp.Choices[0].Message.Content, nil
}
