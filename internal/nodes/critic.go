package nodes

import (
	"context"
	"fmt"
)

type CriticNode struct{}

func init() {
	Register(&CriticNode{})
}

func (n *CriticNode) Name() string {
	return "critic"
}

func (n *CriticNode) Description() string {
	return "Critic agent that reviews and improves output quality"
}

func (n *CriticNode) Schema() NodeSchema {
	params := baseAgentParams()
	params = append(params,
		ParamSchema{Name: "role", Type: "string", Description: "Critic role: general, code, writing, security, design (default: general)", Required: false, Default: "general"},
		ParamSchema{Name: "criteria", Type: "string", Description: "Custom evaluation criteria (comma-separated)", Required: false},
		ParamSchema{Name: "output_format", Type: "string", Description: "Output format: markdown, json, bullet_points (default: markdown)", Required: false, Default: "markdown"},
		ParamSchema{Name: "suggest_improvements", Type: "string", Description: "Whether to suggest improvements: true/false (default: true)", Required: false, Default: "true"},
	)
	return NodeSchema{
		Name:        "critic",
		Description: "Critic agent that reviews output, identifies issues, and suggests improvements",
		Input:       "string - the content to be reviewed and critiqued",
		Output:      "string - structured critique with issues and improvement suggestions",
		Params:      params,
	}
}

func (n *CriticNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	role := getParam(params, "role", "general")
	criteria := getParam(params, "criteria", "")
	outputFormat := getParam(params, "output_format", "markdown")
	suggestImprovements := getParam(params, "suggest_improvements", "true")

	rolePrompts := map[string]string{
		"general":  "You are a sharp, constructive critic. Review the content thoroughly.",
		"code":     "You are a senior code reviewer. Critique for bugs, edge cases, performance, readability, and best practices.",
		"writing":  "You are an editor and writing coach. Critique for clarity, structure, tone, grammar, and impact.",
		"security": "You are a security auditor. Critique for vulnerabilities, injection risks, insecure defaults, and data exposure.",
		"design":   "You are a design critic. Critique for user experience, visual hierarchy, accessibility, and consistency.",
	}

	rolePrompt, ok := rolePrompts[role]
	if !ok {
		rolePrompt = rolePrompts["general"]
	}

	systemPrompt := fmt.Sprintf(`%s

Be specific and constructive. Point out both strengths and weaknesses.
%s

%s

Output format: %s

Respond with ONLY your critique, no extra chatter.`, rolePrompt,
		func() string {
			if criteria != "" {
				return fmt.Sprintf("Evaluation criteria: %s", criteria)
			}
			return ""
		}(),
		func() string {
			if suggestImprovements == "true" {
				return "For each issue found, include a concrete suggestion for improvement."
			}
			return "Do not suggest improvements — only identify and describe issues."
		}(),
		outputFormat)

	result, err := runAgentLLM(ctx, provider, model, apiKey, endpoint, systemPrompt, input)
	if err != nil {
		return "", fmt.Errorf("critic agent failed: %w", err)
	}

	return result, nil
}
