// Copyright (c) 2026 aflare Contributors
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

type ResearcherNode struct{}

func init() {
	Register(&ResearcherNode{})
}

func (n *ResearcherNode) Name() string {
	return "researcher"
}

func (n *ResearcherNode) Description() string {
	return "Research agent that gathers and summarizes information from web sources"
}

func (n *ResearcherNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "researcher",
		Description: "Research agent that fetches information from URLs and summarizes findings",
		Input:       "string - the research topic or question",
		Output:      "string - structured research summary with sources",
		Params: []ParamSchema{
			{Name: "provider", Type: "string", Description: "LLM provider (default: ollama)", Required: false, Default: "ollama"},
			{Name: "model", Type: "string", Description: "Model name (default: llama3)", Required: false, Default: "llama3"},
			{Name: "api_key", Type: "string", Description: "API key", Required: false},
			{Name: "endpoint", Type: "string", Description: "API endpoint URL", Required: false},
			{Name: "urls", Type: "string", Description: "Comma-separated URLs to research (if not provided, agent will use input topic)", Required: false},
			{Name: "depth", Type: "string", Description: "Research depth: basic, detailed, comprehensive (default: basic)", Required: false, Default: "basic"},
			{Name: "output_format", Type: "string", Description: "Output format: markdown, json, summary (default: markdown)", Required: false, Default: "markdown"},
		},
	}
}

func (n *ResearcherNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	urls := getParam(params, "urls", "")
	depth := getParam(params, "depth", "basic")
	outputFormat := getParam(params, "output_format", "markdown")

	var collectedContent string
	var sources []string

	if urls != "" {
		urlList := strings.Split(urls, ",")
		fetchNode := &FetchURLNode{}
		for _, u := range urlList {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			content, err := fetchNode.Execute(ctx, u, map[string]string{})
			if err != nil {
				collectedContent += fmt.Sprintf("\n--- Source: %s (failed: %v) ---\n", u, err)
				continue
			}
			if len(content) > 5000 {
				content = content[:5000] + "\n... (truncated)"
			}
			collectedContent += fmt.Sprintf("\n--- Source: %s ---\n%s\n", u, content)
			sources = append(sources, u)
		}
	}

	systemPrompt := fmt.Sprintf(`You are a research analyst. Analyze the provided information and produce a clear, well-structured %s on the topic.

Research depth: %s

Key requirements:
- Be factual and cite sources when possible
- Organize information into clear sections
- Highlight key findings
- Note any uncertainties or gaps in information`, outputFormat, depth)

	userPrompt := input
	if collectedContent != "" {
		userPrompt = fmt.Sprintf("Topic: %s\n\nCollected information:\n%s\n\nPlease synthesize this into a %s.", input, collectedContent, outputFormat)
	}

	result, err := runAgentLLM(ctx, provider, model, apiKey, endpoint, systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("researcher agent failed: %w", err)
	}

	if len(sources) > 0 && outputFormat == "markdown" {
		result += "\n\n## Sources\n"
		for i, s := range sources {
			result += fmt.Sprintf("%d. %s\n", i+1, s)
		}
	}

	return result, nil
}
