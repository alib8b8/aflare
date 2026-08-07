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

package workflow

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// ── AST engine tests ──
//
// These tests pin down the compiled-AST expression engine (A-5). The central
// guarantee is TestAST_EquivalentToLegacyOracle: a faithful re-implementation
// of the old regex-based evaluator acts as an oracle, and the new engine must
// produce identical (result, error-nil-ness) output across a broad corpus.

// legacyEvaluate is a verbatim re-implementation of the pre-AST Evaluate path,
// kept here as an oracle. It must not depend on the new AST code.
func legacyEvaluate(e *ExpressionEngine, expr, input string) (string, error) {
	if expr == "" {
		return "", nil
	}
	var firstErr error
	result := varPattern.ReplaceAllStringFunc(expr, func(match string) string {
		inner := strings.TrimSpace(match[2 : len(match)-2])
		value, err := legacyEvalSingle(e, inner, input)
		if err != nil {
			if legacyIsKnown(inner) {
				if firstErr == nil {
					firstErr = fmt.Errorf("expression '{{%s}}': %w", inner, err)
				}
			}
			return match
		}
		return value
	})
	if firstErr != nil {
		return result, firstErr
	}
	return result, nil
}

func legacyIsKnown(expr string) bool {
	parts := strings.SplitN(expr, ".", 2)
	if len(parts) < 2 {
		return expr == "input"
	}
	switch strings.TrimSpace(parts[0]) {
	case "step", "var", "env", "file", "input", "loop", "secret":
		return true
	}
	return false
}

func legacyEvalSingle(e *ExpressionEngine, expr, input string) (string, error) {
	if idx := strings.Index(expr, ".jsonpath:"); idx > 0 {
		refPart := expr[:idx]
		jsonPath := expr[idx+len(".jsonpath:"):]
		refValue, err := legacyEvalSingle(e, refPart, input)
		if err != nil {
			return "", err
		}
		return extractJSONPath(refValue, jsonPath)
	}
	parts := strings.SplitN(expr, ".", 2)
	if len(parts) < 2 {
		if v, ok := e.variables[expr]; ok {
			return v, nil
		}
		if expr == "input" {
			return input, nil
		}
		return "", fmt.Errorf("unknown expression: %s", expr)
	}
	prefix := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	switch prefix {
	case "input":
		return input, nil
	case "step":
		return e.evalStepRef(name)
	case "var":
		if v, ok := e.variables[name]; ok {
			return v, nil
		}
		return "", fmt.Errorf("variable not found: %s", name)
	case "env":
		if !isAllowedEnvVar(name) {
			return "", fmt.Errorf("access to environment variable %q is not allowed", name)
		}
		if v, ok := os.LookupEnv(name); ok {
			return v, nil
		}
		return "", fmt.Errorf("environment variable not found: %s", name)
	case "file":
		safePath, err := validateExprFilePath(name)
		if err != nil {
			return "", fmt.Errorf("file path validation failed: %w", err)
		}
		info, err := os.Stat(safePath)
		if err != nil {
			return "", fmt.Errorf("failed to stat file '%s': %w", name, err)
		}
		if info.Size() > maxExprFileSize {
			return "", fmt.Errorf("file '%s' too large (max %d bytes)", name, maxExprFileSize)
		}
		content, err := os.ReadFile(safePath)
		if err != nil {
			return "", fmt.Errorf("failed to read file '%s': %w", name, err)
		}
		return string(content), nil
	case "loop":
		if e.loopVars == nil {
			return "", fmt.Errorf("not in a loop context")
		}
		if v, ok := e.loopVars[name]; ok {
			return v, nil
		}
		return "", fmt.Errorf("loop variable not found: %s", name)
	case "secret":
		if e.secretGetter == nil {
			return "", fmt.Errorf("secrets not available - use 'aflare secrets add' to store secrets first")
		}
		secretParts := strings.SplitN(name, ".", 2)
		if len(secretParts) < 2 {
			return "", fmt.Errorf("secret expression requires format: secret.GROUP.KEY")
		}
		return e.secretGetter(strings.TrimSpace(secretParts[0]), strings.TrimSpace(secretParts[1]))
	default:
		return "", fmt.Errorf("unknown expression: %s", expr)
	}
}

func TestAST_EquivalentToLegacyOracle(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch", `{"users":[{"name":"Alice"},{"name":"Bob"}],"meta":{"ok":true}}`)
	engine.SetStepOutput(1, "process", "processed-data")
	engine.SetStepOutput(2, "numeric", "42")
	engine.SetVariable("api_key", "secret123")
	engine.SetVariable("api_url", "https://api.example.com")
	engine.SetVariable("input", "shadowed-input-var") // shadows the {{input}} token
	engine.SetLoopVars("apple", 2, 5)
	engine.SetSecretGetter(func(g, k string) (string, error) {
		return g + "/" + k, nil
	})

	cases := []string{
		"",
		"static text",
		"{{input}}",
		"{{input.anything}}",
		"prefix-{{input}}-suffix",
		"{{step.0}}",
		"{{step.fetch}}",
		"{{step.1}}",
		"{{step.process}}",
		"{{step.99}}",
		"{{step.nonexistent}}",
		"{{var.api_key}}",
		"{{var.missing}}",
		"{{var.api_url}}/path",
		"{{loop.item}}",
		"{{loop.index}}",
		"{{loop.count}}",
		"{{loop.missing}}",
		"{{secret.llm.openai}}",
		"{{secret.missing}}",
		"{{env.PATH}}",
		"{{env.SECRET}}",
		"{{env.AFLARE_NONEXISTENT_XYZ}}",
		"{{file.nonexistent.txt}}",
		"{{unknown.value}}",
		"{{.foo}}",
		"{{.Name}}",
		"Hello {{.input}} - {{.Name}}",
		"{{ }}",
		"{{}}",
		"{{x}",
		"{{x}y}}",
		"{{{a}}}",
		"{{a}}{{b}}",
		"{{a}}b{{c}}",
		"{{step.0.jsonpath:$.users[0].name}}",
		"{{step.0.jsonpath:$.users[*].name}}",
		"{{step.0.jsonpath:$.meta.ok}}",
		"{{step.fetch.jsonpath:$.users[1].name}}",
		"{{var.api_key.jsonpath:$.x}}",
		"{{step.0.jsonpath:$..name}}",
		"{{step.0.jsonpath:$.nonexistent}}",
		"first={{step.0}}&second={{step.1}}&key={{var.api_key}}",
		"{{step.0}} {{step.0}} {{var.api_key}}",
		"no expressions here at all",
		"{{ input }}",
		"{{  step.0  }}",
		"{{step.2}}",
		"{{foo}}", // bare name, not a variable
		"{{api_key}}",
	}

	for _, c := range cases {
		gotRes, gotErr := engine.Evaluate(c, "raw-input")
		wantRes, wantErr := legacyEvaluate(engine, c, "raw-input")
		if gotRes != wantRes {
			t.Errorf("expr=%q: result mismatch\n  new=%q\n  old=%q", c, gotRes, wantRes)
		}
		if (gotErr == nil) != (wantErr == nil) {
			t.Errorf("expr=%q: error-nil mismatch (new=%v, old=%v)", c, gotErr, wantErr)
		}
		// Note: error text is not compared word-for-word; error *presence* and
		// result text (which includes verbatim fallback) are the observable
		// contract. Both engines format errors identically by construction.
	}
}

func TestAST_CompileCacheReuse(t *testing.T) {
	// Same expression string must yield the same compiled template pointer.
	a := compileTemplate("hello {{step.0}} world")
	b := compileTemplate("hello {{step.0}} world")
	if a != b {
		t.Error("expected compileTemplate to return the same cached pointer for identical input")
	}
	// Different expressions yield different templates.
	c := compileTemplate("hello {{step.1}} world")
	if a == c {
		t.Error("expected different templates for different expressions")
	}
}

func TestAST_StaticTextFastPath(t *testing.T) {
	tmpl := compileTemplate("plain text without expressions")
	if tmpl.hasExpr {
		t.Error("expected hasExpr=false for static text")
	}
	if tmpl.literal != "plain text without expressions" {
		t.Errorf("expected literal preserved, got %q", tmpl.literal)
	}
	if len(tmpl.instrs) != 0 {
		t.Errorf("expected no instrs for static text, got %d", len(tmpl.instrs))
	}
}

func TestAST_NoOpeningBracesIsLiteral(t *testing.T) {
	for _, s := range []string{"", "no braces", "single { brace", "single } brace", "}{}{"} {
		tmpl := compileTemplate(s)
		if tmpl.hasExpr {
			t.Errorf("expected %q to be literal-only (no {{), got hasExpr=true", s)
		}
		if tmpl.literal != s {
			t.Errorf("expected literal %q, got %q", s, tmpl.literal)
		}
	}
}

func TestAST_ParseTemplateEdgeCases(t *testing.T) {
	// Cases that must NOT be treated as expressions (mirror varPattern).
	for _, s := range []string{"{{}}", "{{x}", "{{x}y}}"} {
		tmpl := compileTemplate(s)
		if tmpl.hasExpr {
			t.Errorf("expected %q to have no valid expressions, got hasExpr=true instrs=%d", s, len(tmpl.instrs))
		}
	}
	// "{{ }}" is a valid match (space is a non-'}' char) but resolves to unknown.
	tmpl := compileTemplate("{{ }}")
	if !tmpl.hasExpr {
		t.Fatal("expected {{ }} to contain a (zero-content) expression")
	}
	if len(tmpl.instrs) != 1 || tmpl.instrs[0].op == opLiteral {
		t.Fatalf("expected one expr instr for {{ }}, got %+v", tmpl.instrs)
	}
	// Multiple expressions split into instrs correctly.
	tmpl = compileTemplate("{{a}}{{b}}")
	if len(tmpl.instrs) != 2 {
		t.Fatalf("expected 2 instrs for {{a}}{{b}}, got %d", len(tmpl.instrs))
	}
	tmpl = compileTemplate("{{a}}b{{c}}")
	if len(tmpl.instrs) != 3 {
		t.Fatalf("expected 3 instrs for {{a}}b{{c}}, got %d", len(tmpl.instrs))
	}
}

func TestAST_BareNameShadowsInput(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetVariable("input", "var-value")
	// A variable named "input" shadows the {{input}} token (legacy quirk).
	got, err := engine.Evaluate("{{input}}", "raw-input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "var-value" {
		t.Errorf("expected variable to shadow input token, got %q", got)
	}
}

func TestAST_JsonpathWrapsNonStepRef(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetVariable("data", `{"k":"v"}`)
	// {{var.data.jsonpath:$.k}} — jsonpath modifier on a var reference.
	got, err := engine.Evaluate("{{var.data.jsonpath:$.k}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "v" {
		t.Errorf("expected 'v', got %q", got)
	}
}

func TestAST_FirstErrorFromMultipleKnownExpressions(t *testing.T) {
	engine := NewExpressionEngine()
	// {{step.99}} errors (known prefix step), {{var.missing}} also errors.
	// The engine surfaces the first known-prefix error but still returns the
	// partially-rendered string (with verbatim fallbacks for failing exprs).
	res, err := engine.Evaluate("a={{step.99}} b={{var.missing}}", "in")
	if err == nil {
		t.Fatal("expected an error from known-prefix failures")
	}
	if res != "a={{step.99}} b={{var.missing}}" {
		t.Errorf("expected verbatim fallbacks in result, got %q", res)
	}
}

func TestAST_UnknownPrefixDoesNotError(t *testing.T) {
	engine := NewExpressionEngine()
	res, err := engine.Evaluate("{{unknown.thing}} and {{.goTemplate}}", "in")
	if err != nil {
		t.Fatalf("unexpected error for unknown prefixes: %v", err)
	}
	if res != "{{unknown.thing}} and {{.goTemplate}}" {
		t.Errorf("expected verbatim preservation, got %q", res)
	}
}

func TestAST_ConcurrentCompileIsSafe(t *testing.T) {
	// Hammer compileTemplate from many goroutines with overlapping inputs to
	// exercise the sync.Map cache under concurrency (relevant for parallel/loop
	// sub-engines sharing the package cache).
	exprs := []string{
		"{{step.0}}", "{{var.x}}", "{{input}}", "static",
		"{{step.0.jsonpath:$.a}}", "{{loop.item}}", "{{.foo}}",
		"prefix-{{input}}-suffix", "{{a}}{{b}}",
	}
	done := make(chan struct{})
	for g := 0; g < 16; g++ {
		go func(seed int) {
			for i := 0; i < 200; i++ {
				_ = compileTemplate(exprs[(seed+i)%len(exprs)])
			}
			done <- struct{}{}
		}(g)
	}
	for g := 0; g < 16; g++ {
		<-done
	}
}
