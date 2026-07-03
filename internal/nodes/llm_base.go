package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/alib8b8/llm-box/internal/config"
)

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMRequest struct {
	Model       string      `json:"model"`
	Messages    []LLMMessage `json:"messages"`
	Temperature float64     `json:"temperature,omitempty"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Stream      bool        `json:"stream"`
}

type LLMChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type LLMResponse struct {
	Choices []LLMChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type LLMNodeConfig struct {
	Name           string
	DefaultModel   string
	DefaultEndpoint string
	EnvAPIKey      string
	ProviderName   string
}

type OpenAICompatibleNode struct {
	config LLMNodeConfig
}

func NewOpenAICompatibleNode(config LLMNodeConfig) *OpenAICompatibleNode {
	return &OpenAICompatibleNode{config: config}
}

func (n *OpenAICompatibleNode) Name() string {
	return n.config.Name
}

func (n *OpenAICompatibleNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	model, ok := params["model"]
	if !ok || model == "" {
		model = config.GetDefaultModel(n.config.Name, n.config.EnvAPIKey+"_MODEL", n.config.DefaultModel)
	}

	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = config.GetAPIKey(n.config.Name, n.config.EnvAPIKey)
	}
	if apiKey == "" {
		return "", fmt.Errorf("%s API key required. Set %s env var, add to config file, or pass api_key param",
			n.config.ProviderName, n.config.EnvAPIKey)
	}

	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = config.GetEndpoint(n.config.Name, n.config.EnvAPIKey+"_ENDPOINT", n.config.DefaultEndpoint)
	}

	generateURL := fmt.Sprintf("%s/chat/completions", endpoint)

	systemPrompt, _ := params["system"]
	messages := []LLMMessage{}
	if systemPrompt != "" {
		messages = append(messages, LLMMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, LLMMessage{Role: "user", Content: input})

	reqBody := LLMRequest{
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
		return "", fmt.Errorf("failed to call %s API: %w", n.config.ProviderName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp LLMResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("%s API error (%d): %s", n.config.ProviderName, resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("%s API returned status %d", n.config.ProviderName, resp.StatusCode)
	}

	var llmResp LLMResponse
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return "", fmt.Errorf("failed to parse %s response: %w", n.config.ProviderName, err)
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in %s response", n.config.ProviderName)
	}

	return llmResp.Choices[0].Message.Content, nil
}
