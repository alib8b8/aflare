package nodes

import (
	"context"
	"os"
)

type OpenAINode struct {
	compat *OpenAICompatibleNode
}

func init() {
	Register(&OpenAINode{
		compat: NewOpenAICompatibleNode(LLMNodeConfig{
			Name:            "openai",
			DefaultModel:    "gpt-3.5-turbo",
			DefaultEndpoint: "https://api.openai.com/v1",
			EnvAPIKey:       "OPENAI_API_KEY",
			ProviderName:    "OpenAI",
		}),
	})
}

func (n *OpenAINode) Name() string {
	return "openai"
}

func (n *OpenAINode) Description() string {
	return "Call OpenAI API"
}

func (n *OpenAINode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "openai",
		Description: "Call OpenAI API",
		Input:       "string - user message content",
		Output:      "string - AI response content",
		Params: []ParamSchema{
			{Name: "model", Type: "string", Description: "Model name (default: gpt-3.5-turbo)", Required: false, Default: "gpt-3.5-turbo"},
			{Name: "api_key", Type: "string", Description: "OpenAI API key (or set OPENAI_API_KEY env var)", Required: false},
			{Name: "endpoint", Type: "string", Description: "API base URL (or set OPENAI_API_BASE env var)", Required: false, Default: "https://api.openai.com/v1"},
			{Name: "system", Type: "string", Description: "System prompt", Required: false},
		},
	}
}

func (n *OpenAINode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey != "" {
			params["api_key"] = apiKey
		}
	}
	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = os.Getenv("OPENAI_API_BASE")
		if endpoint != "" {
			params["endpoint"] = endpoint
		}
	}
	return n.compat.Execute(ctx, input, params)
}

func (n *OpenAINode) ExecuteStream(ctx context.Context, input string, params map[string]string, onChunk func(chunk string)) (string, error) {
	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey != "" {
			params["api_key"] = apiKey
		}
	}
	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = os.Getenv("OPENAI_API_BASE")
		if endpoint != "" {
			params["endpoint"] = endpoint
		}
	}
	return n.compat.ExecuteStream(ctx, input, params, onChunk)
}
