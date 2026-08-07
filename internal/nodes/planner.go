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

type PlannerNode struct{}

func init() {
	Register(&PlannerNode{})
}

func (n *PlannerNode) Name() string {
	return "planner"
}

func (n *PlannerNode) Description() string {
	return "Break a complex task into structured steps"
}

func (n *PlannerNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "planner",
		Description: "Task decomposition agent that breaks complex goals into actionable steps",
		Input:       "string - the complex task or goal to plan for",
		Output:      "string - JSON array of planned steps with descriptions and dependencies",
		Params: []ParamSchema{
			{Name: "provider", Type: "string", Description: "LLM provider (default: ollama)", Required: false, Default: "ollama"},
			{Name: "model", Type: "string", Description: "Model name (default: llama3)", Required: false, Default: "llama3"},
			{Name: "api_key", Type: "string", Description: "API key", Required: false},
			{Name: "endpoint", Type: "string", Description: "API endpoint URL", Required: false},
			{Name: "max_steps", Type: "string", Description: "Maximum number of steps to plan (default: 10)", Required: false, Default: "10"},
			{Name: "context", Type: "string", Description: "Additional context or constraints for the plan", Required: false},
			{Name: "auto_clarify", Type: "string", Description: "Run ACQUIRE-style clarification before planning (default: false). If task is ambiguous, ask clarifying questions first.", Required: false, Default: "false"},
			{Name: "clarify_threshold", Type: "string", Description: "Confidence threshold for auto-clarification 0-100 (default: 70)", Required: false, Default: "70"},
			{Name: "clarify_max_questions", Type: "string", Description: "Max clarification questions (default: 3)", Required: false, Default: "3"},
		},
	}
}

func (n *PlannerNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	maxSteps := getParam(params, "max_steps", "10")
	contextInfo := getParam(params, "context", "")
	autoClarify := getParam(params, "auto_clarify", "false") == "true"

	if autoClarify {
		clarifyNode := &ClarifyNode{}
		clarifyParams := map[string]string{
			"provider":      provider,
			"model":         model,
			"api_key":       apiKey,
			"endpoint":      endpoint,
			"threshold":     getParam(params, "clarify_threshold", "70"),
			"max_questions": getParam(params, "clarify_max_questions", "3"),
			"context":       contextInfo,
		}
		clarifyResult, err := clarifyNode.Execute(ctx, input, clarifyParams)
		if err != nil {
			return "", fmt.Errorf("auto-clarify failed: %w", err)
		}
		return fmt.Sprintf(`{"auto_clarify": true, "clarification_result": %s}`, clarifyResult), nil
	}

	systemPrompt := fmt.Sprintf(`You are an expert task planner. Break down the given goal into clear, actionable steps.

Constraints:
- Maximum %s steps
- Each step should be specific and actionable
- Steps should be ordered logically
- Include dependencies between steps where relevant

Output format (MUST be valid JSON):
{
  "goal": "the original goal",
  "steps": [
    {
      "step": 1,
      "title": "brief title",
      "description": "detailed description of what to do",
      "depends_on": [],
      "estimated_complexity": "low|medium|high"
    }
  ],
  "total_steps": N
}

Respond with ONLY the JSON object, no extra text.`, maxSteps)

	if contextInfo != "" {
		systemPrompt += "\n\nAdditional context:\n" + contextInfo
	}

	result, err := runAgentLLM(ctx, provider, model, apiKey, endpoint, systemPrompt, input)
	if err != nil {
		return "", fmt.Errorf("planner agent failed: %w", err)
	}

	return cleanJSONResponse(result), nil
}

func cleanJSONResponse(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
