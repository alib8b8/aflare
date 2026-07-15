package nodes

import (
	"context"
	"fmt"
	"strings"
)

type CodeReviewNode struct{}

func init() {
	Register(&CodeReviewNode{})
}

func (n *CodeReviewNode) Name() string {
	return "code_review"
}

func (n *CodeReviewNode) Description() string {
	return "AI code review agent that analyzes code for issues"
}

func (n *CodeReviewNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "code_review",
		Description: "Specialized agent that reviews code for bugs, security issues, style, and best practices",
		Input:       "string - the code to review",
		Output:      "string - structured code review with findings and suggestions",
		Params: []ParamSchema{
			{Name: "provider", Type: "string", Description: "LLM provider (default: ollama)", Required: false, Default: "ollama"},
			{Name: "model", Type: "string", Description: "Model name (default: llama3)", Required: false, Default: "llama3"},
			{Name: "api_key", Type: "string", Description: "API key", Required: false},
			{Name: "endpoint", Type: "string", Description: "API endpoint URL", Required: false},
			{Name: "language", Type: "string", Description: "Programming language (auto-detected if not specified)", Required: false},
			{Name: "focus", Type: "string", Description: "Review focus: all, bugs, security, style, performance (default: all)", Required: false, Default: "all"},
			{Name: "severity", Type: "string", Description: "Minimum severity: low, medium, high, critical (default: medium)", Required: false, Default: "medium"},
		},
	}
}

func (n *CodeReviewNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	language := getParam(params, "language", "")
	focus := getParam(params, "focus", "all")
	severity := getParam(params, "severity", "medium")

	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("code input is required for code review")
	}

	backtick := "```"
	langInfo := "Language: auto-detect"
	if language != "" {
		langInfo = fmt.Sprintf("Language: %s", language)
	}
	systemPrompt := fmt.Sprintf(`You are a senior software engineer performing code review.

Review focus: %s
Minimum severity: %s
%s

Review categories to check:
1. **Bugs**: Logic errors, null pointer, off-by-one, race conditions
2. **Security**: Injection vulnerabilities, hardcoded secrets, insecure defaults
3. **Performance**: Inefficient algorithms, memory leaks, N+1 queries
4. **Style & Best Practices**: Naming, readability, consistency, error handling
5. **Maintainability**: Complexity, coupling, testability

Output format (Markdown):
## Code Review Summary
Overall assessment: [good/needs attention/requires changes]

## Findings
### [Severity] Category - Brief title
- **Location**: line X or function name
- **Issue**: description of the problem
- **Suggestion**: how to fix it
- **Code example**:
  %s
  suggested fix
  %s

## Positive Notes
- Things done well

## Action Items
- Prioritized list of things to fix

Only include findings at %s severity or higher.
Be specific and actionable. Avoid generic advice.`, focus, severity, langInfo, backtick, backtick, severity)

	result, err := runAgentLLM(ctx, provider, model, apiKey, endpoint, systemPrompt, input)
	if err != nil {
		return "", fmt.Errorf("code review agent failed: %w", err)
	}

	return result, nil
}
