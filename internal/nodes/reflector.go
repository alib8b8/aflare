package nodes

import (
	"context"
	"fmt"
	"strings"
)

type ReflectorNode struct{}

func init() {
	Register(&ReflectorNode{})
}

func (n *ReflectorNode) Name() string {
	return "reflector"
}

func (n *ReflectorNode) Description() string {
	return "Self-reflection agent that critiques and improves its own output"
}

func (n *ReflectorNode) Schema() NodeSchema {
	params := baseAgentParams()
	params = append(params,
		ParamSchema{Name: "iterations", Type: "string", Description: "Number of reflection iterations (1-5, default: 2)", Required: false, Default: "2"},
		ParamSchema{Name: "goal", Type: "string", Description: "The original goal/task the output was trying to achieve", Required: false},
		ParamSchema{Name: "reflection_focus", Type: "string", Description: "What to reflect on: accuracy, completeness, quality, all (default: all)", Required: false, Default: "all"},
	)
	return NodeSchema{
		Name:        "reflector",
		Description: "Self-reflection agent that critiques output and iteratively improves it (Reflexion pattern)",
		Input:       "string - the initial output to reflect on and improve",
		Output:      "string - the improved final output",
		Params:      params,
	}
}

func (n *ReflectorNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	iterationsStr := getParam(params, "iterations", "2")
	goal := getParam(params, "goal", "")
	focus := getParam(params, "reflection_focus", "all")

	iterations := 2
	fmt.Sscanf(iterationsStr, "%d", &iterations)
	if iterations < 1 {
		iterations = 1
	}
	if iterations > 5 {
		iterations = 5
	}

	focusPrompt := map[string]string{
		"accuracy":     "Focus on factual accuracy, logical errors, and correctness.",
		"completeness": "Focus on whether all aspects of the task are addressed and nothing important is missing.",
		"quality":      "Focus on quality: clarity, structure, depth, and impact.",
		"all":          "Reflect on accuracy, completeness, quality, and any other relevant dimensions.",
	}[focus]
	if focusPrompt == "" {
		focusPrompt = "Reflect on all relevant dimensions."
	}

	currentOutput := input

	for i := 0; i < iterations; i++ {
		reflectionPrompt := fmt.Sprintf(`You are a self-reflection agent. Look at the current output and the original goal, then identify what needs improvement.

Original goal: %s

Current output:
%s

%s

List specific problems you find. Then produce a revised, improved version of the output.

Format your response as:
REFLECTION:
[list of issues you found]

IMPROVED OUTPUT:
[the improved version]`,
			func() string {
				if goal != "" {
					return goal
				}
				return "not specified"
			}(),
			currentOutput,
			focusPrompt)

		result, err := runAgentLLM(ctx, provider, model, apiKey, endpoint,
			"You are a self-reflective AI. Critique and improve the given output.", reflectionPrompt)
		if err != nil {
			if i == 0 {
				return "", fmt.Errorf("reflector agent failed: %w", err)
			}
			break
		}

		improved := extractImprovedOutput(result)
		if improved != "" && len(improved) > 20 {
			currentOutput = improved
		}
	}

	return currentOutput, nil
}

func extractImprovedOutput(response string) string {
	markers := []string{
		"IMPROVED OUTPUT:",
		"Improved Output:",
		"improved output:",
		"Final version:",
		"FINAL VERSION:",
		"Revised:",
		"REVISED:",
	}

	for _, marker := range markers {
		idx := strings.LastIndex(response, marker)
		if idx != -1 {
			result := strings.TrimSpace(response[idx+len(marker):])
			result = strings.TrimPrefix(result, "```")
			result = strings.TrimSuffix(result, "```")
			return strings.TrimSpace(result)
		}
	}

	return ""
}
