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
			{Name: "auto_clarify", Type: "string", Description: "Run ACQUIRE-style clarification before review (default: false). Ask clarifying questions if review scope is ambiguous.", Required: false, Default: "false"},
			{Name: "clarify_threshold", Type: "string", Description: "Confidence threshold for auto-clarification 0-100 (default: 70)", Required: false, Default: "70"},
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
	autoClarify := getParam(params, "auto_clarify", "false") == "true"

	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("code input is required for code review")
	}

	if autoClarify {
		clarifyNode := &ClarifyNode{}
		clarifyParams := map[string]string{
			"provider":  provider,
			"model":     model,
			"api_key":   apiKey,
			"endpoint":  endpoint,
			"threshold": getParam(params, "clarify_threshold", "70"),
			"context":   fmt.Sprintf("Code review task. Language: %s, Focus: %s, Severity: %s", language, focus, severity),
		}
		clarifyResult, err := clarifyNode.Execute(ctx, input, clarifyParams)
		if err != nil {
			return "", fmt.Errorf("auto-clarify failed: %w", err)
		}
		return fmt.Sprintf(`{"auto_clarify": true, "clarification_result": %s}`, clarifyResult), nil
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
