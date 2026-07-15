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
