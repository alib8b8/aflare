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

package providers

import (
	"context"
	"os"

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

type IMANode struct {
	compat *core.OpenAICompatibleNode
}

func init() {
	core.Register(&IMANode{
		compat: core.NewOpenAICompatibleNode(core.LLMNodeConfig{
			Name:            "ima",
			DefaultModel:    "",
			DefaultEndpoint: "",
			EnvAPIKey:       "IMA_API_KEY", // #nosec G101 -- env var name, not a credential value
			ProviderName:    "IMA Copilot",
		}),
	})
}

func (n *IMANode) Name() string {
	return "ima"
}

func (n *IMANode) Description() string {
	return "Call IMA Copilot API"
}

func (n *IMANode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        "ima",
		Description: "Call IMA Copilot API",
		Input:       "string - user message content",
		Output:      "string - AI response content",
		Params: []core.ParamSchema{
			{Name: "model", Type: "string", Description: "Model name (required)", Required: true},
			{Name: "api_key", Type: "string", Description: "IMA API key (or set IMA_API_KEY env var)", Required: false},
			{Name: "endpoint", Type: "string", Description: "API base URL (or set IMA_API_BASE env var)", Required: true},
			{Name: "system", Type: "string", Description: "System prompt", Required: false},
		},
	}
}

func (n *IMANode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("IMA_API_KEY")
		if apiKey != "" {
			params["api_key"] = apiKey
		}
	}
	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = os.Getenv("IMA_API_BASE")
		if endpoint != "" {
			params["endpoint"] = endpoint
		}
	}
	return n.compat.Execute(ctx, input, params)
}

func (n *IMANode) ExecuteStream(ctx context.Context, input string, params map[string]string, onChunk func(chunk string)) (string, error) {
	apiKey, ok := params["api_key"]
	if !ok || apiKey == "" {
		apiKey = os.Getenv("IMA_API_KEY")
		if apiKey != "" {
			params["api_key"] = apiKey
		}
	}
	endpoint, ok := params["endpoint"]
	if !ok || endpoint == "" {
		endpoint = os.Getenv("IMA_API_BASE")
		if endpoint != "" {
			params["endpoint"] = endpoint
		}
	}
	return n.compat.ExecuteStream(ctx, input, params, onChunk)
}
