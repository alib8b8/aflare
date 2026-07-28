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

package workflow

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func BenchmarkExpressionEvaluate(b *testing.B) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch", `{"users":[{"name":"Alice"}]}`)
	engine.SetVariable("api_key", "secret123")
	engine.SetLoopVars("item1", 0, 3)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	exprs := []string{
		"{{input}}",
		"{{step.0}}",
		"{{var.api_key}}",
		"{{loop.item}}",
		"{{loop.index}}",
		"static text",
		"prefix-{{input}}-suffix",
		"{{step.0.jsonpath:$.users[0].name}}",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			expr := exprs[rng.Intn(len(exprs))]
			input := fmt.Sprintf("test-%d", rng.Intn(10000))
			_, _ = engine.Evaluate(expr, input)
		}
	}
}

// ── A-6.2: AST vs legacy regex engine head-to-head ──
//
// These benchmarks pit the compiled-AST engine (production path: Evaluate) against
// legacyEvaluate, a verbatim re-implementation of the pre-AST regex evaluator
// (see expression_ast_test.go). Both share the same ExpressionEngine state, so
// the only difference is the parsing/evaluation strategy.
//
// Realistic conditions: the AST engine's package-level templateCache is warm
// after the first iteration, mirroring production where the same templates are
// evaluated many times (loops, parallel branches, repeated step refs). The
// legacy engine has no cache and re-runs its regex on every call.

// benchExprEngine returns a fully-populated ExpressionEngine for benchmark use.
func benchExprEngine() *ExpressionEngine {
	e := NewExpressionEngine()
	e.SetStepOutput(0, "fetch", `{"users":[{"name":"Alice"},{"name":"Bob"}],"meta":{"ok":true}}`)
	e.SetStepOutput(1, "process", "processed-data")
	e.SetStepOutput(2, "transform", `{"result":"done","count":42}`)
	e.SetVariable("api_key", "secret123")
	e.SetVariable("api_url", "https://api.example.com")
	e.SetVariable("model", "gpt-4")
	e.SetLoopVars("apple", 2, 5)
	return e
}

// BenchmarkAST_vs_Legacy compares the two engines across representative
// template shapes. Each sub-benchmark runs the same template 1000 times per
// iteration to amplify per-call overhead above measurement noise.
func BenchmarkAST_vs_Legacy(b *testing.B) {
	engine := benchExprEngine()

	cases := []struct {
		name string
		expr string
	}{
		{"StaticText", "the quick brown fox jumps over the lazy dog"},
		{"NoBracesLiteral", "no expressions here at all, just plain text"},
		{"SingleInput", "{{input}}"},
		{"SingleStep", "{{step.0}}"},
		{"SingleVar", "{{var.api_key}}"},
		{"SingleLoop", "{{loop.item}}"},
		{"SingleJsonpath", "{{step.0.jsonpath:$.users[0].name}}"},
		{"PrefixSuffix", "prefix-{{input}}-suffix"},
		{"MultiExpr", "first={{step.0}}&second={{step.1}}&key={{var.api_key}}"},
		{"RepeatedExpr", "{{step.0}} {{step.0}} {{var.api_key}}"},
		{"UrlComposite", "POST {{var.api_url}}/v1/models with key={{var.api_key}}"},
		{"UnknownVerbatim", "Hello {{.Name}} - {{unknown.thing}}"},
		{"ErrorFallback", "a={{step.99}} b={{var.missing}}"},
	}

	const callsPerIter = 1000
	input := "benchmark-input"

	for _, c := range cases {
		b.Run(c.name+"/AST", func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				for k := 0; k < callsPerIter; k++ {
					_, _ = engine.Evaluate(c.expr, input)
				}
			}
		})
		b.Run(c.name+"/Legacy", func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				for k := 0; k < callsPerIter; k++ {
					_, _ = legacyEvaluate(engine, c.expr, input)
				}
			}
		})
	}
}

// BenchmarkAST_CacheBenefit measures the AST engine with a warm cache (production
// steady state) against the same engine forced to re-parse on every call. The
// gap quantifies the value of templateCache for hot templates.
//
// To make the comparison fair, both paths walk the compiled template and resolve
// every node — only the source of the *compiledTemplate differs: cache hit vs
// fresh parseTemplate.
func BenchmarkAST_CacheBenefit(b *testing.B) {
	engine := benchExprEngine()
	expr := "result={{step.0.jsonpath:$.users[0].name}} key={{var.api_key}} url={{var.api_url}}"

	// Warm the cache once before the timed loop (production steady state).
	_, _ = engine.Evaluate(expr, "in")

	// evalCompiled walks a compiled template against the engine, mirroring the
	// post-compile half of Evaluate. Defined here so the benchmark can apply it
	// to a freshly-parsed template (bypassing the cache).
	evalCompiled := func(tmpl *compiledTemplate, input string) (string, error) {
		if !tmpl.hasExpr {
			return tmpl.literal, nil
		}
		var sb strings.Builder
		var firstErr error
		for i := range tmpl.parts {
			p := &tmpl.parts[i]
			if p.isLiteral {
				sb.WriteString(p.literal)
				continue
			}
			val, err := p.node.eval(engine, input)
			if err != nil {
				if p.node.knownPrefix() && firstErr == nil {
					firstErr = fmt.Errorf("expression '{{%s}}': %w", p.inner, err)
				}
				sb.WriteString(p.fullMatch)
				continue
			}
			sb.WriteString(val)
		}
		return sb.String(), firstErr
	}

	b.Run("WarmCache", func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			_, _ = engine.Evaluate(expr, "in")
		}
	})

	// "ColdCache" simulates a cache-less engine: parse + walk on every call.
	b.Run("ColdCache", func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			tmpl := parseTemplate(expr)
			_, _ = evalCompiled(tmpl, "in")
		}
	})
}

// BenchmarkAST_ParallelEvaluate verifies the engine is safe under parallel
// access (templateCache is a sync.Map; ExpressionEngine state is read-only during
// Evaluate once populated). Run with -race to confirm.
func BenchmarkAST_ParallelEvaluate(b *testing.B) {
	engine := benchExprEngine()
	exprs := []string{
		"{{input}}",
		"{{step.0}}",
		"{{var.api_key}}",
		"{{step.0.jsonpath:$.users[0].name}}",
		"prefix-{{input}}-suffix",
		"{{step.0}} {{var.api_key}} {{loop.item}}",
	}

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = engine.Evaluate(exprs[i%len(exprs)], "in")
			i++
		}
	})
}

// BenchmarkEvaluateParams mirrors the hot path inside executor: a params map
// with a handful of mixed templates is evaluated for every step.
func BenchmarkEvaluateParams(b *testing.B) {
	engine := benchExprEngine()
	params := map[string]string{
		"url":     "{{var.api_url}}/v1/chat",
		"auth":    "Bearer {{var.api_key}}",
		"model":   "{{var.model}}",
		"prompt":  "Process: {{input}}",
		"context": "prev={{step.0}}",
		"plain":   "static-value",
	}

	b.Run("AST", func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			_, _ = engine.EvaluateParams(params, "user-input")
		}
	})
	b.Run("Legacy", func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			legacyEvaluateParams(engine, params, "user-input")
		}
	})
}

// legacyEvaluateParams mirrors EvaluateParams using legacyEvaluate, for a fair
// apples-to-apples comparison at the params-map level.
func legacyEvaluateParams(e *ExpressionEngine, params map[string]string, input string) (map[string]string, error) {
	if params == nil {
		return nil, nil
	}
	out := make(map[string]string, len(params))
	for k, v := range params {
		resolved, err := legacyEvaluate(e, v, input)
		if err != nil {
			return nil, fmt.Errorf("param %q: %w", k, err)
		}
		out[k] = resolved
	}
	return out, nil
}
