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
	"regexp"
	"strings"
)

type ReviewFinding struct {
	Severity   string
	Category   string
	Title      string
	Location   string
	Issue      string
	Suggestion string
	CodeFix    string
	RuleID     string
}

type DeterministicRule struct {
	ID       string
	Severity string
	Category string
	Pattern  *regexp.Regexp
	Title    string
	Issue    string
	Fix      string
	Lang     string
}

var codeReviewRules = []DeterministicRule{
	{
		ID: "NPE-001", Severity: "high", Category: "bugs", Lang: "go",
		Pattern: regexp.MustCompile(`\w+\.\w+`),
		Title:   "Potential nil pointer dereference",
		Issue:   "Variable may be nil before field/method access without nil check",
		Fix:     "Add nil check before dereferencing: if var != nil { var.Field }",
	},
	{
		ID: "SEC-001", Severity: "critical", Category: "security", Lang: "any",
		Pattern: regexp.MustCompile(`(?i)(api[_-]?key|secret|password|token).{0,10}[:=].{0,10}["'][A-Za-z0-9_-]{10,}["']`),
		Title:   "Hardcoded secret detected",
		Issue:   "API keys, passwords, or tokens should not be hardcoded in source code",
		Fix:     "Use environment variables or secret management instead: os.Getenv(\"API_KEY\")",
	},
	{
		ID: "SEC-002", Severity: "high", Category: "security", Lang: "any",
		Pattern: regexp.MustCompile(`(?i)exec[. (]["']`),
		Title:   "Potential command injection",
		Issue:   "User input directly passed to shell execution without sanitization",
		Fix:     "Use parameterized execution or sanitize input with allowlist validation",
	},
	{
		ID: "RACE-001", Severity: "high", Category: "bugs", Lang: "go",
		Pattern: regexp.MustCompile(`go\s+func`),
		Title:   "Potential data race without mutex",
		Issue:   "Goroutine modifies shared variable without synchronization primitive",
		Fix:     "Use sync.Mutex or atomic operations for shared state: mu.Lock(); x++; mu.Unlock()",
	},
	{
		ID: "PERF-001", Severity: "medium", Category: "performance", Lang: "go",
		Pattern: regexp.MustCompile(`for\s+.*range`),
		Title:   "Inefficient slice append in loop",
		Issue:   "Appending to slice inside loop without pre-allocation causes reallocations",
		Fix:     "Pre-allocate slice capacity: result := make([]T, 0, estimatedSize)",
	},
	{
		ID: "ERR-001", Severity: "medium", Category: "bugs", Lang: "go",
		Pattern: regexp.MustCompile(`if\s+err\s*!=\s*nil\s*\{\s*\}`),
		Title:   "Error swallowed silently",
		Issue:   "Error is caught but no action taken (empty error handler)",
		Fix:     "Log or return the error: if err != nil { return fmt.Errorf(\"failed: %w\", err) }",
	},
	{
		ID: "SQL-001", Severity: "critical", Category: "security", Lang: "any",
		Pattern: regexp.MustCompile(`(?i)(SELECT|INSERT|UPDATE|DELETE).*\+`),
		Title:   "Potential SQL injection via string concatenation",
		Issue:   "SQL query built with string concatenation instead of parameterized queries",
		Fix:     "Use prepared statements with parameters: db.Query(\"SELECT * FROM t WHERE id = ?\", id)",
	},
	{
		ID: "STYLE-001", Severity: "low", Category: "style", Lang: "go",
		Pattern: regexp.MustCompile(`^func\s+\w+\s*\(\s*\w+\s+\*?\w+\s*\)`),
		Title:   "Consider shorter receiver name",
		Issue:   "Receiver name longer than 2 characters reduces readability",
		Fix:     "Use 1-2 character receiver name matching type first letter",
	},
}

type CodeReviewNode struct{}

func init() {
	Register(&CodeReviewNode{})
}

func (n *CodeReviewNode) Name() string {
	return "code_review"
}

func (n *CodeReviewNode) Description() string {
	return "Hybrid-architecture code review: deterministic rules + LLM deep analysis (open-code-review inspired)"
}

func (n *CodeReviewNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "code_review",
		Description: "Hybrid code review combining deterministic rule engine (NPE, thread-safety, security) with LLM deep analysis. Inspired by alibaba/open-code-review.",
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
			{Name: "use_rules", Type: "string", Description: "Run deterministic rule engine before LLM (default: true)", Required: false, Default: "true"},
			{Name: "use_llm", Type: "string", Description: "Run LLM deep analysis (default: true)", Required: false, Default: "true"},
			{Name: "auto_clarify", Type: "string", Description: "Run ACQUIRE-style clarification before review (default: false)", Required: false, Default: "false"},
			{Name: "clarify_threshold", Type: "string", Description: "Confidence threshold for auto-clarification 0-100 (default: 70)", Required: false, Default: "70"},
		},
	}
}

func detectCodeLanguage(code string) string {
	if strings.Contains(code, "func ") && (strings.Contains(code, "package ") || strings.Contains(code, "import (")) {
		return "go"
	}
	if strings.Contains(code, "def ") && strings.Contains(code, "import ") {
		return "python"
	}
	if strings.Contains(code, "function ") || strings.Contains(code, "=>") {
		return "javascript"
	}
	if strings.Contains(code, "#include") || strings.Contains(code, "int main(") {
		return "cpp"
	}
	if strings.Contains(code, "public class ") || strings.Contains(code, "System.out") {
		return "java"
	}
	return "unknown"
}

func runDeterministicRules(code, language, focus string) []ReviewFinding {
	var findings []ReviewFinding
	lines := strings.Split(code, "\n")

	for _, rule := range codeReviewRules {
		if focus != "all" && focus != rule.Category {
			continue
		}
		if rule.Lang != "any" && rule.Lang != language && language != "unknown" {
			continue
		}

		matches := rule.Pattern.FindAllStringSubmatchIndex(code, -1)
		for _, match := range matches {
			lineNum := 1
			pos := 0
			for i, line := range lines {
				if match[0] >= pos && match[0] < pos+len(line)+1 {
					lineNum = i + 1
					break
				}
				pos += len(line) + 1
			}

			varName := ""
			if len(match) > 2 {
				start := match[2]
				end := match[3]
				if end > start && end <= len(code) {
					varName = code[start:end]
				}
			}

			fix := rule.Fix
			if varName != "" {
				fix = fmt.Sprintf(rule.Fix, varName, varName)
			}

			findings = append(findings, ReviewFinding{
				Severity:   rule.Severity,
				Category:   rule.Category,
				Title:      rule.Title,
				Location:   fmt.Sprintf("line %d", lineNum),
				Issue:      rule.Issue,
				Suggestion: fix,
				RuleID:     rule.ID,
			})
		}
	}

	return findings
}

func severityRank(sev string) int {
	switch sev {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func formatFindings(findings []ReviewFinding, minSeverity string) string {
	minRank := severityRank(minSeverity)
	var sb strings.Builder

	filtered := make([]ReviewFinding, 0, len(findings))
	for _, f := range findings {
		if severityRank(f.Severity) >= minRank {
			filtered = append(filtered, f)
		}
	}

	if len(filtered) == 0 {
		return "No deterministic rule violations found.\n"
	}

	sb.WriteString(fmt.Sprintf("## Deterministic Rule Findings (%d issues)\n\n", len(filtered)))
	for i, f := range filtered {
		sb.WriteString(fmt.Sprintf("### [%s] %s - %s\n",
			strings.ToUpper(f.Severity), f.Category, f.Title))
		sb.WriteString(fmt.Sprintf("- **Rule ID**: %s\n", f.RuleID))
		sb.WriteString(fmt.Sprintf("- **Location**: %s\n", f.Location))
		sb.WriteString(fmt.Sprintf("- **Issue**: %s\n", f.Issue))
		sb.WriteString(fmt.Sprintf("- **Suggestion**: %s\n", f.Suggestion))
		if i < len(filtered)-1 {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func (n *CodeReviewNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	language := getParam(params, "language", "")
	focus := getParam(params, "focus", "all")
	severity := getParam(params, "severity", "medium")
	useRules := getParam(params, "use_rules", "true") == "true"
	useLLM := getParam(params, "use_llm", "true") == "true"
	autoClarify := getParam(params, "auto_clarify", "false") == "true"

	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("code input is required for code review")
	}

	if language == "" {
		language = detectCodeLanguage(input)
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

	var resultParts []string

	if useRules {
		findings := runDeterministicRules(input, language, focus)
		resultParts = append(resultParts, formatFindings(findings, severity))
	}

	if useLLM {
		backtick := "```"
		langInfo := fmt.Sprintf("Language: %s", language)

		ruleContext := ""
		if useRules {
			ruleContext = fmt.Sprintf("Deterministic rule engine has already scanned and found issues. Use those as a starting point and add deeper analysis beyond pattern matching.")
		}

		systemPrompt := fmt.Sprintf(`You are a senior software engineer performing code review.
%s

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
## LLM Deep Analysis
Overall assessment: [good/needs attention/requires changes]

## Additional Findings
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
Be specific and actionable. Avoid generic advice.`, ruleContext, focus, severity, langInfo, backtick, backtick, severity)

		llmResult, err := runAgentLLM(ctx, provider, model, apiKey, endpoint, systemPrompt, input)
		if err != nil {
			return "", fmt.Errorf("LLM code review failed: %w", err)
		}
		resultParts = append(resultParts, llmResult)
	}

	header := fmt.Sprintf("# Hybrid Code Review Report\n\n**Language**: %s | **Focus**: %s | **Min Severity**: %s\n\n---\n\n", language, focus, severity)
	return header + strings.Join(resultParts, "\n---\n\n"), nil
}
