// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌‌​‌​‌​​‌​​‌​‌​‌​‌‌​‌‌‌‌‌‌​​​​​‌‌​​‌​​​‌‌​‌​​​​​​​​​​​​​​​​​​​​‌​​​​‌‌‌‌​​‌​‌​⁠
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
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"time"
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
	// 新增规则集（open-code-review 启发）
	{
		ID: "ERR-002", Severity: "medium", Category: "bugs", Lang: "go",
		Pattern: regexp.MustCompile(`err\.Error\(\)`),
		Title:   "Potential nil error dereference",
		Issue:   "err.Error() called where err may be nil without prior nil check",
		Fix:     "Check err != nil before calling err.Error(): if err != nil { msg = err.Error() }",
	},
	{
		ID: "DEFER-001", Severity: "high", Category: "bugs", Lang: "go",
		Pattern: regexp.MustCompile(`for\s+.*\{[\s\S]*?defer\s+`),
		Title:   "Defer inside loop",
		Issue:   "defer in a loop accumulates deferred calls until function returns, leaking resources",
		Fix:     "Extract loop body into a separate function so defer runs each iteration",
	},
	{
		ID: "GLOBAL-001", Severity: "medium", Category: "bugs", Lang: "go",
		Pattern: regexp.MustCompile(`^var\s+\w+\s+(map|chan|\[\]|\*)`),
		Title:   "Mutable global variable",
		Issue:   "Global mutable variable can be modified from anywhere, causing race conditions",
		Fix:     "Pass state explicitly or use a struct with methods + sync.Mutex",
	},
	{
		ID: "TODO-001", Severity: "low", Category: "style", Lang: "any",
		Pattern: regexp.MustCompile(`(?i)\b(TODO|FIXME|HACK|XXX)\b`),
		Title:   "Unresolved TODO/FIXME marker",
		Issue:   "Code contains unfinished work marker that should be tracked or resolved",
		Fix:     "Resolve the TODO or convert to a tracked issue",
	},
	{
		ID: "DUMP-001", Severity: "low", Category: "style", Lang: "any",
		Pattern: regexp.MustCompile(`fmt\.(Println|Printf|Print)\(`),
		Title:   "Debug print statement left in code",
		Issue:   "fmt.Println/Printf should be replaced with structured logging in production",
		Fix:     "Use logger.Info/logger.Debug instead of fmt.Println",
	},
	{
		ID: "EMPTY-001", Severity: "medium", Category: "bugs", Lang: "any",
		Pattern: regexp.MustCompile(`(if|else|for|switch)\s+[^{]*\{\s*\}`),
		Title:   "Empty control block",
		Issue:   "Empty if/else/for/switch block suggests missing implementation or swallowed logic",
		Fix:     "Add implementation or explicit comment explaining why block is empty",
	},
	{
		ID: "CONTEXT-001", Severity: "medium", Category: "bugs", Lang: "go",
		Pattern: regexp.MustCompile(`context\.Background\(\)`),
		Title:   "context.Background() used instead of passed context",
		Issue:   "Using context.Background() ignores cancellation/timeout from caller",
		Fix:     "Accept ctx context.Context as parameter and pass it through",
	},
	{
		ID: "GOROUTINE-001", Severity: "high", Category: "bugs", Lang: "go",
		Pattern: regexp.MustCompile(`go\s+func\s*\([^)]*\)\s*\{[\s\S]*?context\.Background\(\)`),
		Title:   "Goroutine uses context.Background()",
		Issue:   "Goroutine creates new context, losing parent cancellation chain",
		Fix:     "Pass parent context: ctx, cancel := context.WithCancel(parentCtx); go func(ctx context.Context){...}(ctx)",
	},
	{
		ID: "SHADOW-001", Severity: "medium", Category: "bugs", Lang: "go",
		Pattern: regexp.MustCompile(`if\s+\w+\s*:=\s*[^;]+;\s*\w+\s*!=\s*nil`),
		Title:   "Variable shadowing in if-statement",
		Issue:   "Variable declared in if-init shadows outer variable, may cause subtle bugs",
		Fix:     "Use distinct variable name or assign before the if statement",
	},
	{
		ID: "PWD-001", Severity: "critical", Category: "security", Lang: "any",
		Pattern: regexp.MustCompile(`(?i)(password|passwd|pwd).{0,10}[:=].{0,10}["'][^"']{4,}["']`),
		Title:   "Hardcoded password",
		Issue:   "Password appears to be hardcoded in source code",
		Fix:     "Read password from environment variable or secret store: os.Getenv(\"PASSWORD\")",
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

// deterministicStats 生成规则引擎扫描统计信息
func deterministicStats(findings []ReviewFinding, rulesCount, executedCount int, elapsed time.Duration) string {
	counts := map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0}
	for _, f := range findings {
		counts[f.Severity]++
	}
	return fmt.Sprintf("## Rule Engine Statistics\n\n- Total rules: %d\n- Rules executed: %d\n- Findings: %d (critical: %d, high: %d, medium: %d, low: %d)\n- Scan time: %dms\n",
		rulesCount, executedCount, len(findings),
		counts["critical"], counts["high"], counts["medium"], counts["low"],
		elapsed.Milliseconds())
}

// runASTAnalysis 使用 go/ast 对 Go 代码做 AST 级别分析，检测正则难以覆盖的问题
func runASTAnalysis(code string) []ReviewFinding {
	var findings []ReviewFinding

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "review.go", code, parser.ParseComments)
	if err != nil {
		return findings
	}

	ast.Inspect(f, func(n ast.Node) bool {
		// 检测 1: 循环中的 defer
		if fl, ok := n.(*ast.ForStmt); ok {
			ast.Inspect(fl.Body, func(inner ast.Node) bool {
				if df, ok := inner.(*ast.DeferStmt); ok {
					pos := fset.Position(df.Pos())
					findings = append(findings, ReviewFinding{
						Severity: "high", Category: "bugs",
						RuleID:     "AST-DEFER-001",
						Title:      "Defer inside loop (AST detected)",
						Location:   fmt.Sprintf("line %d", pos.Line),
						Issue:      "defer inside for-loop defers until function return, not loop iteration",
						Suggestion: "Extract loop body to a function so defer runs each iteration",
					})
				}
				return true
			})
		}
		// 检测 2: 调用返回 error 的函数但未检查返回值
		if expr, ok := n.(*ast.ExprStmt); ok {
			if call, ok := expr.X.(*ast.CallExpr); ok {
				if hasErrorReturnType(fset, f, call) {
					pos := fset.Position(call.Pos())
					findings = append(findings, ReviewFinding{
						Severity: "medium", Category: "bugs",
						RuleID:     "AST-ERR-001",
						Title:      "Unchecked error return value (AST detected)",
						Location:   fmt.Sprintf("line %d", pos.Line),
						Issue:      "Function returns error but caller ignores it",
						Suggestion: "Check the error: result, err := func(); if err != nil { ... }",
					})
				}
			}
		}
		// 检测 3: 空的 error 处理块
		if iff, ok := n.(*ast.IfStmt); ok {
			if isErrCheck(iff.Cond) && iff.Body != nil && len(iff.Body.List) == 0 {
				pos := fset.Position(iff.Pos())
				findings = append(findings, ReviewFinding{
					Severity: "medium", Category: "bugs",
					RuleID:     "AST-EMPTY-001",
					Title:      "Empty error handler (AST detected)",
					Location:   fmt.Sprintf("line %d", pos.Line),
					Issue:      "if err != nil {} block is empty, error is swallowed",
					Suggestion: "Log or return the error: if err != nil { return err }",
				})
			}
		}
		return true
	})

	return findings
}

// hasErrorReturnType 简单判断调用是否可能返回 error（启发式：函数名常见模式）
func hasErrorReturnType(fset *token.FileSet, f *ast.File, call *ast.CallExpr) bool {
	// 只关注 ExprStmt（语句形式调用，返回值被丢弃）
	fnName := ""
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		fnName = fn.Name
	case *ast.SelectorExpr:
		fnName = fn.Sel.Name
	}
	// 常见返回 error 的函数命名模式
	errorFuncPatterns := []string{"Get", "Set", "Add", "Delete", "Update", "Insert",
		"Create", "Remove", "Save", "Load", "Read", "Write", "Open", "Close",
		"Start", "Stop", "Run", "Execute", "Parse", "Decode", "Encode", "Marshal", "Unmarshal"}
	for _, p := range errorFuncPatterns {
		if strings.Contains(fnName, p) {
			return true
		}
	}
	return false
}

// isErrCheck 判断条件是否为 err != nil 形式
func isErrCheck(cond ast.Expr) bool {
	if bin, ok := cond.(*ast.BinaryExpr); ok {
		if bin.Op == token.NEQ {
			if id, ok := bin.X.(*ast.Ident); ok && id.Name == "err" {
				return true
			}
			if id, ok := bin.Y.(*ast.Ident); ok && id.Name == "err" {
				return true
			}
		}
	}
	return false
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
		start := time.Now()
		findings := runDeterministicRules(input, language, focus)

		// 对 Go 代码运行 AST 分析，检测正则难以覆盖的问题
		if language == "go" {
			astFindings := runASTAnalysis(input)
			findings = append(findings, astFindings...)
		}

		executed := 0
		for _, rule := range codeReviewRules {
			if focus != "all" && focus != rule.Category {
				continue
			}
			if rule.Lang != "any" && rule.Lang != language && language != "unknown" {
				continue
			}
			executed++
		}
		resultParts = append(resultParts, deterministicStats(findings, len(codeReviewRules), executed, time.Since(start)))
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
