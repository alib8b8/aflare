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

type FastGPTNode struct{}

func init() {
	Register(&FastGPTNode{})
}

func (n *FastGPTNode) Name() string {
	return "fastgpt"
}

type fastGPTMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type fastGPTRequest struct {
	AppId    string           `json:"appId,omitempty"`
	ChatId   string           `json:"chatId,omitempty"`
	Messages []fastGPTMessage `json:"messages"`
	Stream   bool             `json:"stream"`
	Detail   bool             `json:"detail,omitempty"`
}

type fastGPTChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type fastGPTResponse struct {
	Choices []fastGPTChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (n *FastGPTNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("FASTGPT_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("FastGPT API key required. Set FASTGPT_API_KEY env var or pass api_key param")
	}

	appId, _ := params["app_id"]
	chatId, _ := params["chat_id"]

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = os.Getenv("FASTGPT_BASE_URL")
	}
	if endpoint == "" {
		endpoint = "https://fastgpt.in/api/v1"
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []fastGPTMessage{}
	if systemPrompt != "" {
		messages = append(messages, fastGPTMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, fastGPTMessage{Role: "user", Content: input})

	reqBody := fastGPTRequest{
		AppId:    appId,
		ChatId:   chatId,
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
		return "", fmt.Errorf("failed to call FastGPT API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp fastGPTResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("FastGPT API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("FastGPT API returned status %d", resp.StatusCode)
	}

	var fgResp fastGPTResponse
	if err := json.NewDecoder(resp.Body).Decode(&fgResp); err != nil {
		return "", fmt.Errorf("failed to parse FastGPT response: %w", err)
	}

	if len(fgResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in FastGPT response")
	}

	return fgResp.Choices[0].Message.Content, nil
}
