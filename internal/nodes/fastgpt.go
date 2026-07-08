package nodes

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type FastGPTNode struct{}

func init() {
	Register(&FastGPTNode{})
}

func (n *FastGPTNode) Name() string {
	return "fastgpt"
}

func (n *FastGPTNode) Description() string {
	return "Call FastGPT API"
}

func (n *FastGPTNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "fastgpt",
		Description: "Call FastGPT API",
		Input:       "string - user message content",
		Output:      "string - AI response content",
		Params: []ParamSchema{
			{Name: "api_key", Type: "string", Description: "FastGPT API key (or set FASTGPT_API_KEY env var)", Required: false},
			{Name: "app_id", Type: "string", Description: "FastGPT app ID", Required: false},
			{Name: "chat_id", Type: "string", Description: "Chat ID for conversation continuity", Required: false},
			{Name: "endpoint", Type: "string", Description: "API base URL (or set FASTGPT_BASE_URL env var)", Required: false, Default: "https://fastgpt.in/api/v1"},
			{Name: "system", Type: "string", Description: "System prompt", Required: false},
		},
	}
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
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
}

type fastGPTResponse struct {
	Choices []fastGPTChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (n *FastGPTNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return n.execute(ctx, input, params, false, nil)
}

func (n *FastGPTNode) ExecuteStream(ctx context.Context, input string, params map[string]string, onChunk func(chunk string)) (string, error) {
	return n.execute(ctx, input, params, true, onChunk)
}

func (n *FastGPTNode) execute(ctx context.Context, input string, params map[string]string, stream bool, onChunk func(chunk string)) (string, error) {
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

	// Validate endpoint URL to prevent SSRF + API key leakage
	if err := validateLMLEndpoint(endpoint); err != nil {
		return "", fmt.Errorf("endpoint URL validation failed: %w", err)
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	// Validate the full URL to prevent SSRF
	if err := validateLMLEndpoint(generateURL); err != nil {
		return "", fmt.Errorf("URL validation failed: %w", err)
	}

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
		Stream:   stream,
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
		Timeout:       DefaultLLMTimeout,
		CheckRedirect: httpRedirectValidator(validateLMLEndpoint),
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call FastGPT API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp fastGPTResponse
		_ = json.NewDecoder(io.LimitReader(resp.Body, maxHTTPResponseSize)).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("FastGPT API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("FastGPT API returned status %d", resp.StatusCode)
	}

	if stream {
		return n.readStreamResponse(resp, onChunk)
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

func (n *FastGPTNode) readStreamResponse(resp *http.Response, onChunk func(chunk string)) (string, error) {
	scanner := bufio.NewScanner(resp.Body)
	var fullContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var streamResp fastGPTResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue
		}

		if len(streamResp.Choices) > 0 {
			chunk := streamResp.Choices[0].Delta.Content
			if chunk != "" {
				fullContent.WriteString(chunk)
				if onChunk != nil {
					onChunk(chunk)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fullContent.String(), fmt.Errorf("error reading stream: %w", err)
	}

	return fullContent.String(), nil
}
