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
	"strings"
)

type RouterNode struct{}

func init() {
	Register(&RouterNode{})
}

func (n *RouterNode) Name() string {
	return "router"
}

func (n *RouterNode) Description() string {
	return "Router agent that classifies input and routes to the appropriate path"
}

func (n *RouterNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "router",
		Description: "Classification agent that analyzes input and decides which processing path to take",
		Input:       "string - the input text to classify and route",
		Output:      "string - JSON with classification result and routing decision",
		Params: []ParamSchema{
			{Name: "provider", Type: "string", Description: "LLM provider (default: ollama)", Required: false, Default: "ollama"},
			{Name: "model", Type: "string", Description: "Model name (default: llama3)", Required: false, Default: "llama3"},
			{Name: "api_key", Type: "string", Description: "API key", Required: false},
			{Name: "endpoint", Type: "string", Description: "API endpoint URL", Required: false},
			{Name: "categories", Type: "string", Description: "Comma-separated list of routing categories (e.g. bug,feature,question,spam)", Required: true},
			{Name: "instructions", Type: "string", Description: "Additional routing instructions or classification criteria", Required: false},
		},
	}
}

func (n *RouterNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	categories := getParam(params, "categories", "")
	instructions := getParam(params, "instructions", "")

	if categories == "" {
		return "", fmt.Errorf("categories parameter is required for router node")
	}

	catList := strings.Split(categories, ",")
	for i := range catList {
		catList[i] = strings.TrimSpace(catList[i])
	}

	systemPrompt := fmt.Sprintf(`You are an intelligent router. Your job is to classify the input into exactly one of the predefined categories.

Available categories: %s

Rules:
1. Choose the BEST matching category from the list
2. If input is ambiguous, choose the closest match
3. Provide a brief confidence score and reason

%s

Output format (MUST be valid JSON):
{
  "category": "chosen_category",
  "confidence": 0.0-1.0,
  "reason": "brief explanation of why this category was chosen",
  "suggested_next_step": "what to do with this input"
}

Respond with ONLY the JSON object, no extra text.`,
		strings.Join(catList, ", "),
		func() string {
			if instructions != "" {
				return "\nAdditional instructions:\n" + instructions
			}
			return ""
		}())

	result, err := runAgentLLM(ctx, provider, model, apiKey, endpoint, systemPrompt, input)
	if err != nil {
		return "", fmt.Errorf("router agent failed: %w", err)
	}

	return cleanJSONResponse(result), nil
}
