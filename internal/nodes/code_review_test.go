// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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
	"strings"
	"testing"
)

// TestCodeReviewNode_Registration verifies the code_review node is registered.
func TestCodeReviewNode_Registration(t *testing.T) {
	node, ok := Get("code_review")
	if !ok {
		t.Fatal("code_review node not found in registry")
	}
	if node.Name() != "code_review" {
		t.Errorf("expected node name 'code_review', got '%s'", node.Name())
	}
}

// TestCodeReviewNode_Description ensures Description returns a non-empty string.
func TestCodeReviewNode_Description(t *testing.T) {
	node := &CodeReviewNode{}
	if desc := node.Description(); desc == "" {
		t.Error("Description() returned empty string")
	}
}

// TestCodeReviewNode_Schema verifies the schema name, description, and params.
func TestCodeReviewNode_Schema(t *testing.T) {
	node := &CodeReviewNode{}
	schema := node.Schema()

	if schema.Name != "code_review" {
		t.Errorf("Schema().Name = %q, want %q", schema.Name, "code_review")
	}
	if schema.Description == "" {
		t.Error("Schema().Description is empty")
	}
	if schema.Input == "" {
		t.Error("Schema().Input is empty")
	}
	if schema.Output == "" {
		t.Error("Schema().Output is empty")
	}

	expectedParams := []string{
		"provider", "model", "api_key", "endpoint", "language",
		"focus", "severity", "use_rules", "use_llm",
		"auto_clarify", "clarify_threshold",
	}
	for _, name := range expectedParams {
		found := false
		for _, p := range schema.Params {
			if p.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected param %q in schema", name)
		}
	}
}

// TestCodeReviewNode_SchemaDefaults verifies the documented default values.
func TestCodeReviewNode_SchemaDefaults(t *testing.T) {
	node := &CodeReviewNode{}
	schema := node.Schema()
	defaults := map[string]string{
		"provider":  "ollama",
		"model":     "llama3",
		"focus":     "all",
		"severity":  "medium",
		"use_rules": "true",
		"use_llm":   "true",
	}
	for _, p := range schema.Params {
		if want, ok := defaults[p.Name]; ok && p.Default != want {
			t.Errorf("param %q default = %q, want %q", p.Name, p.Default, want)
		}
	}
}

// TestDetectCodeLanguage verifies language auto-detection for several languages.
func TestDetectCodeLanguage(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{"go_with_package", "package main\n\nfunc main() {}", "go"},
		{"go_with_import", "import (\n\"fmt\"\n)\n\nfunc main() {}", "go"},
		{"go_func_without_package_or_import", "func hello() {}", "unknown"},
		{"python", "import os\ndef hello():\n    pass", "python"},
		{"javascript_function", "function foo() { return 1; }", "javascript"},
		{"javascript_arrow", "const f = () => 1", "javascript"},
		{"cpp", "#include <iostream>\nint main() { return 0; }", "cpp"},
		{"java", "public class Foo { }", "java"},
		{"unknown", "hello world", "unknown"},
		{"empty", "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectCodeLanguage(tt.code)
			if got != tt.want {
				t.Errorf("detectCodeLanguage() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRunDeterministicRules verifies that specific code patterns trigger the
// expected deterministic rules. Tests use focus filters to isolate rules.
func TestRunDeterministicRules(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		language    string
		focus       string
		wantRuleIDs []string // empty means expect no findings
	}{
		{
			name:        "hardcoded_secret",
			code:        `apiKey := "sk-1234567890abcdef"`,
			language:    "go",
			focus:       "security",
			wantRuleIDs: []string{"SEC-001"},
		},
		{
			name:        "hardcoded_secret_password",
			code:        `password := "supersecret12345"`,
			language:    "go",
			focus:       "security",
			wantRuleIDs: []string{"SEC-001"},
		},
		{
			name:        "sql_injection_concat",
			code:        `query := "SELECT * FROM users WHERE id=" + userID`,
			language:    "go",
			focus:       "security",
			wantRuleIDs: []string{"SQL-001"},
		},
		{
			name:        "empty_error_handler",
			code:        "if err != nil {}",
			language:    "go",
			focus:       "bugs",
			wantRuleIDs: []string{"ERR-001"},
		},
		{
			name:        "command_injection",
			code:        `out := exec("ls " + userInput)`,
			language:    "go",
			focus:       "security",
			wantRuleIDs: []string{"SEC-002"},
		},
		{
			name:        "goroutine_without_mutex",
			code:        "go func() {\n    counter++\n}()",
			language:    "go",
			focus:       "bugs",
			wantRuleIDs: []string{"RACE-001"},
		},
		{
			name:        "range_loop_no_prealloc",
			code:        "for _, item := range items {\n    result = append(result, item)\n}",
			language:    "go",
			focus:       "performance",
			wantRuleIDs: []string{"PERF-001"},
		},
		{
			name:        "style_long_receiver",
			code:        "func foo(x int) int { return x }",
			language:    "go",
			focus:       "style",
			wantRuleIDs: []string{"STYLE-001"},
		},
		{
			name:        "clean_code_no_findings",
			code:        "x := 1\ny := x + 2",
			language:    "go",
			focus:       "all",
			wantRuleIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := runDeterministicRules(tt.code, tt.language, tt.focus)

			if len(tt.wantRuleIDs) == 0 {
				if len(findings) > 0 {
					var ids []string
					for _, f := range findings {
						ids = append(ids, f.RuleID)
					}
					t.Errorf("expected no findings, got %d: %v", len(findings), ids)
				}
				return
			}

			gotIDs := make(map[string]bool)
			for _, f := range findings {
				gotIDs[f.RuleID] = true
			}
			for _, wantID := range tt.wantRuleIDs {
				if !gotIDs[wantID] {
					var ids []string
					for id := range gotIDs {
						ids = append(ids, id)
					}
					t.Errorf("expected rule %q to fire, got findings: %v", wantID, ids)
				}
			}
		})
	}
}

// TestRunDeterministicRules_FocusFilter verifies that the focus parameter
// restricts which rules run.
func TestRunDeterministicRules_FocusFilter(t *testing.T) {
	// Code triggers both a security rule (SEC-001) and a bugs rule (ERR-001).
	code := `apiKey := "sk-1234567890abcdef"
if err != nil {}`

	t.Run("only_security", func(t *testing.T) {
		findings := runDeterministicRules(code, "go", "security")
		for _, f := range findings {
			if f.Category != "security" {
				t.Errorf("focus=security but got finding in category %q (%s)", f.Category, f.RuleID)
			}
		}
	})

	t.Run("only_bugs", func(t *testing.T) {
		findings := runDeterministicRules(code, "go", "bugs")
		for _, f := range findings {
			if f.Category != "bugs" {
				t.Errorf("focus=bugs but got finding in category %q (%s)", f.Category, f.RuleID)
			}
		}
	})

	t.Run("all_includes_both", func(t *testing.T) {
		findings := runDeterministicRules(code, "go", "all")
		categories := make(map[string]bool)
		for _, f := range findings {
			categories[f.Category] = true
		}
		if !categories["security"] {
			t.Error("focus=all should include security findings")
		}
		if !categories["bugs"] {
			t.Error("focus=all should include bugs findings")
		}
	})
}

// TestCodeReview_Execute_RulesOnly tests Execute with use_llm disabled, so only
// the deterministic rule engine runs. No network access is required.
func TestCodeReview_Execute_RulesOnly(t *testing.T) {
	ctx := context.Background()
	code := `package main

import "fmt"

func main() {
    apiKey := "sk-1234567890abcdef"
    if err != nil {}
    query := "SELECT * FROM users WHERE id=" + userID
    fmt.Println(apiKey)
    fmt.Println(query)
}`
	params := map[string]string{
		"use_llm":   "false",
		"use_rules": "true",
		"language":  "go",
		"focus":     "all",
		"severity":  "low",
	}
	output, err := (&CodeReviewNode{}).Execute(ctx, code, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(output, "Deterministic Rule Findings") {
		t.Errorf("expected 'Deterministic Rule Findings' header in output:\n%s", output)
	}
	for _, want := range []string{"SEC-001", "SQL-001", "ERR-001"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %s finding in output", want)
		}
	}
}

// TestCodeReview_Execute_CleanCode verifies that code without rule violations
// produces the "no violations" message.
func TestCodeReview_Execute_CleanCode(t *testing.T) {
	ctx := context.Background()
	code := `package main

func add(a, b int) int {
    return a + b
}`
	params := map[string]string{
		"use_llm":   "false",
		"use_rules": "true",
		"language":  "go",
		"focus":     "all",
		"severity":  "low",
	}
	output, err := (&CodeReviewNode{}).Execute(ctx, code, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(output, "No deterministic rule violations found") {
		t.Errorf("expected no violations for clean code, got:\n%s", output)
	}
}

// TestCodeReview_Execute_EmptyInput verifies Execute errors on empty input.
func TestCodeReview_Execute_EmptyInput(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"whitespace_only", "   \n\t  "},
		{"tabs_only", "\t\t\t"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&CodeReviewNode{}).Execute(ctx, tt.input, nil)
			if err == nil {
				t.Fatal("expected error for empty input, got nil")
			}
			if !strings.Contains(err.Error(), "code input is required") {
				t.Errorf("expected 'code input is required' error, got: %v", err)
			}
		})
	}
}

// TestCodeReview_Execute_LanguageAutoDetect verifies language is auto-detected
// from the code when not specified in params.
func TestCodeReview_Execute_LanguageAutoDetect(t *testing.T) {
	ctx := context.Background()
	code := `package main

func main() {
    apiKey := "sk-1234567890abcdef"
}`
	params := map[string]string{
		"use_llm":   "false",
		"use_rules": "true",
		"severity":  "low",
	}
	output, err := (&CodeReviewNode{}).Execute(ctx, code, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(output, "Language**: go") {
		t.Errorf("expected auto-detected language 'go' in output:\n%s", output)
	}
	if !strings.Contains(output, "SEC-001") {
		t.Errorf("expected SEC-001 finding for hardcoded secret")
	}
}

// TestCodeReview_Execute_SeverityFilter verifies that the severity param
// filters out findings below the specified threshold.
func TestCodeReview_Execute_SeverityFilter(t *testing.T) {
	ctx := context.Background()
	// ERR-001 is "medium", SEC-001 is "critical".
	code := `if err != nil {}
apiKey := "sk-1234567890abcdef"`
	params := map[string]string{
		"use_llm":   "false",
		"use_rules": "true",
		"language":  "go",
		"focus":     "all",
		"severity":  "high",
	}
	output, err := (&CodeReviewNode{}).Execute(ctx, code, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(output, "SEC-001") {
		t.Error("expected SEC-001 (critical) to be present with severity=high")
	}
	if strings.Contains(output, "ERR-001") {
		t.Error("expected ERR-001 (medium) to be filtered out with severity=high")
	}
}

// TestCodeReview_Execute_FocusParam verifies the focus param restricts which
// rule categories appear in the output.
func TestCodeReview_Execute_FocusParam(t *testing.T) {
	ctx := context.Background()
	code := `apiKey := "sk-1234567890abcdef"
if err != nil {}`
	params := map[string]string{
		"use_llm":   "false",
		"use_rules": "true",
		"language":  "go",
		"focus":     "security",
		"severity":  "low",
	}
	output, err := (&CodeReviewNode{}).Execute(ctx, code, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(output, "SEC-001") {
		t.Error("expected SEC-001 (security) with focus=security")
	}
	if strings.Contains(output, "ERR-001") {
		t.Error("expected ERR-001 (bugs) to be excluded with focus=security")
	}
}

// TestSeverityRank verifies the severityRank helper function.
func TestSeverityRank(t *testing.T) {
	tests := []struct {
		sev  string
		want int
	}{
		{"critical", 4},
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"unknown", 0},
		{"", 0},
	}
	for _, tt := range tests {
		t.Run(tt.sev, func(t *testing.T) {
			if got := severityRank(tt.sev); got != tt.want {
				t.Errorf("severityRank(%q) = %d, want %d", tt.sev, got, tt.want)
			}
		})
	}
}

// TestFormatFindings verifies the formatFindings output formatting and
// severity-based filtering.
func TestFormatFindings(t *testing.T) {
	findings := []ReviewFinding{
		{Severity: "critical", Category: "security", Title: "Critical", RuleID: "SEC-001", Issue: "i", Suggestion: "s"},
		{Severity: "medium", Category: "bugs", Title: "Medium", RuleID: "ERR-001", Issue: "i", Suggestion: "s"},
		{Severity: "low", Category: "style", Title: "Low", RuleID: "STYLE-001", Issue: "i", Suggestion: "s"},
	}

	t.Run("all_severities", func(t *testing.T) {
		out := formatFindings(findings, "low")
		if !strings.Contains(out, "SEC-001") || !strings.Contains(out, "ERR-001") || !strings.Contains(out, "STYLE-001") {
			t.Errorf("expected all rule IDs in output:\n%s", out)
		}
		if !strings.Contains(out, "3 issues") {
			t.Errorf("expected '3 issues' in output:\n%s", out)
		}
	})

	t.Run("high_severity_only", func(t *testing.T) {
		out := formatFindings(findings, "high")
		if !strings.Contains(out, "SEC-001") {
			t.Error("expected SEC-001 (critical) with min severity high")
		}
		if strings.Contains(out, "ERR-001") {
			t.Error("expected ERR-001 (medium) filtered out with min severity high")
		}
		if strings.Contains(out, "STYLE-001") {
			t.Error("expected STYLE-001 (low) filtered out with min severity high")
		}
	})

	t.Run("empty_findings", func(t *testing.T) {
		out := formatFindings(nil, "low")
		if !strings.Contains(out, "No deterministic rule violations found") {
			t.Errorf("expected 'No deterministic rule violations found' for empty findings")
		}
	})
}

// TestCodeReviewRules_Count verifies the deterministic rule set is non-empty.
func TestCodeReviewRules_Count(t *testing.T) {
	if len(codeReviewRules) == 0 {
		t.Error("expected non-empty codeReviewRules slice")
	}
	// Verify each rule has required fields.
	for _, rule := range codeReviewRules {
		if rule.ID == "" {
			t.Errorf("rule has empty ID: %+v", rule)
		}
		if rule.Pattern == nil {
			t.Errorf("rule %s has nil Pattern", rule.ID)
		}
		if rule.Severity == "" {
			t.Errorf("rule %s has empty Severity", rule.ID)
		}
		if rule.Category == "" {
			t.Errorf("rule %s has empty Category", rule.ID)
		}
		if rule.Title == "" {
			t.Errorf("rule %s has empty Title", rule.ID)
		}
	}
}
