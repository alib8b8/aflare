package nodes

import (
	"context"
	"fmt"
)

func runAgentLLM(ctx context.Context, provider, model, apiKey, endpoint, systemPrompt, userInput string) (string, error) {
	llmParams := map[string]string{
		"model":    model,
		"api_key":  apiKey,
		"endpoint": endpoint,
		"system":   systemPrompt,
	}

	if provider == "ollama" {
		node := &OllamaNode{}
		return node.Execute(ctx, userInput, llmParams)
	}

	compatNode := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            provider,
		DefaultModel:    model,
		DefaultEndpoint: endpoint,
		EnvAPIKey:       fmt.Sprintf("%s_API_KEY", provider),
		ProviderName:    provider,
	})
	return compatNode.Execute(ctx, userInput, llmParams)
}

func baseAgentParams() []ParamSchema {
	return []ParamSchema{
		{Name: "provider", Type: "string", Description: "LLM provider (default: ollama)", Required: false, Default: "ollama"},
		{Name: "model", Type: "string", Description: "Model name (default: llama3)", Required: false, Default: "llama3"},
		{Name: "api_key", Type: "string", Description: "API key", Required: false},
		{Name: "endpoint", Type: "string", Description: "API endpoint URL", Required: false},
	}
}
