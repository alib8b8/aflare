package nodes

import (
	"context"
	"fmt"
	"strings"
)

type SupervisorNode struct{}

func init() {
	Register(&SupervisorNode{})
}

func (n *SupervisorNode) Name() string {
	return "supervisor"
}

func (n *SupervisorNode) Description() string {
	return "Supervisor agent that decomposes tasks and routes to specialists"
}

func (n *SupervisorNode) Schema() NodeSchema {
	params := baseAgentParams()
	params = append(params,
		ParamSchema{Name: "specialists", Type: "string", Description: "Comma-separated list of specialist agent names available (default: planner,researcher,critic,code_review)", Required: false, Default: "planner,researcher,critic,code_review"},
		ParamSchema{Name: "strategy", Type: "string", Description: "Supervision strategy: sequential, parallel, hierarchical (default: sequential)", Required: false, Default: "sequential"},
		ParamSchema{Name: "max_depth", Type: "string", Description: "Maximum delegation depth (default: 3)", Required: false, Default: "3"},
		ParamSchema{Name: "output_format", Type: "string", Description: "Output format: json, markdown, summary (default: json)", Required: false, Default: "json"},
	)
	return NodeSchema{
		Name:        "supervisor",
		Description: "Supervisor agent that breaks down tasks, delegates to specialists, and synthesizes results",
		Input:       "string - the overall goal or task to supervise",
		Output:      "string - structured task plan with delegation and synthesis",
		Params:      params,
	}
}

func (n *SupervisorNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	specialists := getParam(params, "specialists", "planner,researcher,critic,code_review")
	strategy := getParam(params, "strategy", "sequential")
	outputFormat := getParam(params, "output_format", "json")

	specialistList := strings.Split(specialists, ",")
	for i := range specialistList {
		specialistList[i] = strings.TrimSpace(specialistList[i])
	}

	specialistDescs := map[string]string{
		"planner":     "planner — breaks tasks into structured steps",
		"researcher":  "researcher — gathers and synthesizes information",
		"critic":      "critic — reviews quality and suggests improvements",
		"code_review": "code_review — audits code for bugs, security, style",
		"evaluator":   "evaluator — scores output against rubrics",
		"reflector":   "reflector — self-improves output iteratively",
		"router":      "router — classifies and routes inputs",
	}

	specDescs := ""
	for i, s := range specialistList {
		if desc, ok := specialistDescs[s]; ok {
			if i > 0 {
				specDescs += "\n"
			}
			specDescs += fmt.Sprintf("- %s", desc)
		}
	}

	systemPrompt := fmt.Sprintf(`You are a supervisor agent. Your job is to:
1. Analyze the given task
2. Break it into subtasks
3. Assign each subtask to the most appropriate specialist
4. Define the order of execution
5. Specify how results should be synthesized

Available specialists:
%s

Supervision strategy: %s

Output format (MUST be valid JSON):
{
  "task": "the original task",
  "subtasks": [
    {
      "id": 1,
      "description": "what this subtask does",
      "assigned_to": "specialist_name",
      "depends_on": [],
      "input": "what input to pass to this specialist",
      "expected_output": "what output to expect"
    }
  ],
  "synthesis_plan": "how to combine the results from all subtasks",
  "total_subtasks": N,
  "strategy": "%s"
}

Respond with ONLY the JSON object, no extra text.`, specDescs, strategy, strategy)

	result, err := runAgentLLM(ctx, provider, model, apiKey, endpoint, systemPrompt, input)
	if err != nil {
		return "", fmt.Errorf("supervisor agent failed: %w", err)
	}

	if outputFormat == "json" {
		return cleanJSONResponse(result), nil
	}

	return result, nil
}
