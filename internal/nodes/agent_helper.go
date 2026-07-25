// Copyright (c) 2026 llm-box Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
