package nodes

import (
	"context"
	"os"
)

type CozeNode struct {
	compat *OpenAICompatibleNode
}

func init() {
	Register(&CozeNode{
		compat: NewOpenAICompatibleNode(LLMNodeConfig{
			Name:            "coze",
			DefaultModel:    "",
			DefaultEndpoint: "https://api.coze.cn/v1",
			EnvAPIKey:       "COZE_API_KEY",
			ProviderName:    "Coze",
		}),
	})
}

func (n *CozeNode) Name() string {
	return "coze"
}

func (n *CozeNode) Description() string {
	return "Call ByteDance Coze API"
}

func (n *CozeNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "coze",
		Description: "Call ByteDance Coze API",
		Input:       "string - user message content",
		Output:      "string - AI response content",
		Params: []ParamSchema{
			{Name: "model", Type: "string", Description: "Model name (required)", Required: true},
			{Name: "api_key", Type: "string", Description: "Coze API key (or set COZE_API_KEY env var)", Required: false},
			{Name: "endpoint", Type: "string", Description: "API base URL (default: https://api.coze.cn/v1)", Required: false, Default: "https://api.coze.cn/v1"},
			{Name: "system", Type: "string", Description: "System prompt", Required: false},
		},
	}
}

func (n *CozeNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("COZE_API_KEY")
		if apiKey != "" {
			params["api_key"] = apiKey
		}
	}
	return n.compat.Execute(ctx, input, params)
}

func (n *CozeNode) ExecuteStream(ctx context.Context, input string, params map[string]string, onChunk func(chunk string)) (string, error) {
	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("COZE_API_KEY")
		if apiKey != "" {
			params["api_key"] = apiKey
		}
	}
	return n.compat.ExecuteStream(ctx, input, params, onChunk)
}
