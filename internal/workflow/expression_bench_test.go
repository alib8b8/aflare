// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌‌​‌‌​​‌​​‌​‌​‌​‌‌‌‌‌​​‌‌​‌‌‌‌​‌​​‌‌‌​‌‌‌​​​‌​‌​​​​​​​​​​​​​​​​​‌‌​‌‌‌‌‌‌‌​​​​​⁠
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
	// to a freshly-parsed template (bypassing the cache). Delegates to
	// evalBytecode (same inline-switch dispatch as production Evaluate).
	evalCompiled := func(tmpl *compiledTemplate, input string) (string, error) {
		return evalBytecode(engine, tmpl, input)
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

// ── Bytecode (switch dispatch) vs AST (interface dispatch) ──
//
// BenchmarkBytecode_vs_AST isolates the dispatch-mechanism overhead. Both
// paths consume a PRE-COMPILED template (no per-call cache lookup) and build
// the result with strings.Builder (no sync.Pool). The only difference is the
// dispatch: Bytecode calls evalInstr (switch on opcode), AST calls
// node.eval (interface dispatch). This isolates exactly what the bytecode
// optimisation improves.

// astTemplate mirrors the pre-bytecode compiledTemplate shape: a slice of
// templatePart where each expression carries an exprNode. Built once per
// expression (outside the timed loop) so the benchmark measures dispatch cost,
// not parse cost.
type astTemplate struct {
	literal string
	hasExpr bool
	parts   []templatePart
}

// compileASTTemplate builds the AST form of a template by reusing the bytecode
// parse (for literal/expr splitting) and converting each expression instruction
// into an exprNode via the retained compileExpr oracle.
func compileASTTemplate(expr string) *astTemplate {
	bc := compileTemplate(expr)
	if !bc.hasExpr {
		return &astTemplate{literal: bc.literal}
	}
	parts := make([]templatePart, 0, len(bc.instrs))
	for i := range bc.instrs {
		ins := &bc.instrs[i]
		if ins.op == opLiteral {
			parts = append(parts, templatePart{isLiteral: true, literal: ins.strArg})
			continue
		}
		parts = append(parts, templatePart{
			node:      compileExpr(ins.inner),
			fullMatch: ins.fullMatch,
			inner:     ins.inner,
		})
	}
	return &astTemplate{hasExpr: true, parts: parts}
}

// evalAST walks an astTemplate dispatching each expression through the
// exprNode interface — exactly the pre-bytecode Evaluate hot loop. Uses a
// raw []byte buffer (matching evalBytecode) so the only difference is the
// dispatch mechanism.
func evalAST(e *ExpressionEngine, tmpl *astTemplate, input string) (string, error) {
	if !tmpl.hasExpr {
		return tmpl.literal, nil
	}
	var buf []byte
	var firstErr error
	for i := range tmpl.parts {
		p := &tmpl.parts[i]
		if p.isLiteral {
			buf = append(buf, p.literal...)
			continue
		}
		val, err := p.node.eval(e, input)
		if err != nil {
			if p.node.knownPrefix() && firstErr == nil {
				firstErr = fmt.Errorf("expression '{{%s}}': %w", p.inner, err)
			}
			buf = append(buf, p.fullMatch...)
			continue
		}
		buf = append(buf, val...)
	}
	return string(buf), firstErr
}

// evalBytecode walks a pre-compiled bytecode template with the SAME inline
// switch dispatch as production Evaluate, but uses strings.Builder (no pool,
// no cache lookup) so the only difference from evalAST is the dispatch
// mechanism. This function must mirror Evaluate's switch exactly to keep the
// benchmark comparison fair.
func evalBytecode(e *ExpressionEngine, tmpl *compiledTemplate, input string) (string, error) {
	if !tmpl.hasExpr {
		return tmpl.literal, nil
	}
	var sb strings.Builder
	var firstErr error
	for i := range tmpl.instrs {
		ins := &tmpl.instrs[i]
		switch ins.op {
		case opLiteral:
			sb.WriteString(ins.strArg)
		case opInput:
			sb.WriteString(input)
		case opStep:
			if v, err := e.evalStepRef(ins.strArg); err != nil {
				setFirstExprError(ins, &firstErr, err)
				sb.WriteString(ins.fullMatch)
			} else {
				sb.WriteString(v)
			}
		case opVar:
			if v, ok := e.variables[ins.strArg]; ok {
				sb.WriteString(v)
			} else {
				setFirstExprError(ins, &firstErr, fmt.Errorf("variable not found: %s", ins.strArg))
				sb.WriteString(ins.fullMatch)
			}
		case opEnv:
			if v, err := evalEnvVar(ins.strArg); err != nil {
				setFirstExprError(ins, &firstErr, err)
				sb.WriteString(ins.fullMatch)
			} else {
				sb.WriteString(v)
			}
		case opFile:
			if v, err := evalFileContents(ins.strArg); err != nil {
				setFirstExprError(ins, &firstErr, err)
				sb.WriteString(ins.fullMatch)
			} else {
				sb.WriteString(v)
			}
		case opLoop:
			if e.loopVars == nil {
				setFirstExprError(ins, &firstErr, fmt.Errorf("not in a loop context"))
				sb.WriteString(ins.fullMatch)
			} else if v, ok := e.loopVars[ins.strArg]; ok {
				sb.WriteString(v)
			} else {
				setFirstExprError(ins, &firstErr, fmt.Errorf("loop variable not found: %s", ins.strArg))
				sb.WriteString(ins.fullMatch)
			}
		case opSecret:
			if v, err := evalSecretRef(e, ins.strArg); err != nil {
				setFirstExprError(ins, &firstErr, err)
				sb.WriteString(ins.fullMatch)
			} else {
				sb.WriteString(v)
			}
		case opBareName:
			if v, ok := e.variables[ins.strArg]; ok {
				sb.WriteString(v)
			} else if ins.strArg == "input" {
				sb.WriteString(input)
			} else {
				setFirstExprError(ins, &firstErr, fmt.Errorf("unknown expression: %s", ins.strArg))
				sb.WriteString(ins.fullMatch)
			}
		case opJSONPath:
			if rv, rerr := ins.refNode.eval(e, input); rerr != nil {
				setFirstExprError(ins, &firstErr, rerr)
				sb.WriteString(ins.fullMatch)
			} else if v, err := extractJSONPath(rv, ins.strArg2); err != nil {
				setFirstExprError(ins, &firstErr, err)
				sb.WriteString(ins.fullMatch)
			} else {
				sb.WriteString(v)
			}
		case opUnknown:
			setFirstExprError(ins, &firstErr, fmt.Errorf("unknown expression: %s", ins.inner))
			sb.WriteString(ins.fullMatch)
		}
	}
	return sb.String(), firstErr
}

func BenchmarkBytecode_vs_AST(b *testing.B) {
	engine := benchExprEngine()

	cases := []struct {
		name string
		expr string
	}{
		{"SingleInput", "{{input}}"},
		{"SingleStep", "{{step.0}}"},
		{"SingleVar", "{{var.api_key}}"},
		{"SingleLoop", "{{loop.item}}"},
		{"PrefixSuffix", "prefix-{{input}}-suffix"},
		{"MultiExpr", "first={{step.0}}&second={{step.1}}&key={{var.api_key}}"},
		{"RepeatedExpr", "{{step.0}} {{step.0}} {{var.api_key}}"},
		{"SingleJsonpath", "{{step.0.jsonpath:$.users[0].name}}"},
		{"ManyExpr", "{{step.0}}{{var.api_key}}{{loop.item}}{{input}}{{var.api_url}}{{step.1}}{{loop.index}}{{var.model}}{{step.2}}{{loop.count}}"},
	}

	const callsPerIter = 1000
	input := "benchmark-input"

	for _, c := range cases {
		// Pre-compile both forms once so the inner loop measures dispatch
		// cost only — neither path does cache lookup or parsing per call.
		bcTmpl := compileTemplate(c.expr)
		astTmpl := compileASTTemplate(c.expr)

		b.Run(c.name+"/Bytecode", func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				for k := 0; k < callsPerIter; k++ {
					_, _ = evalBytecode(engine, bcTmpl, input)
				}
			}
		})
		b.Run(c.name+"/AST", func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				for k := 0; k < callsPerIter; k++ {
					_, _ = evalAST(engine, astTmpl, input)
				}
			}
		})
	}
}

// BenchmarkEvaluateParams_Vectorized_vs_Serial compares the serial
// EvaluateParams (one Evaluate call — and one pooled-buffer round-trip — per
// param) against EvaluateParamsVectorized, which compiles all templates up
// front and evaluates them into a single shared buffer in one pass. The
// vectorised path amortises buffer growth and per-expression pool Get/Put
// overhead across the whole batch, so it should show fewer allocations.
func BenchmarkEvaluateParams_Vectorized_vs_Serial(b *testing.B) {
	engine := NewExpressionEngine()
	engine.SetVariable("name", "world")
	engine.SetVariable("count", "42")
	engine.SetStepOutput(0, "step0", "step-output")

	params := map[string]string{
		"a": "{{var.name}}",
		"b": "{{step.0}}",
		"c": "{{input}}",
		"d": "literal text",
		"e": "{{var.count}} and {{var.name}}",
	}
	input := "test-input"

	b.Run("Serial", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = engine.EvaluateParams(params, input)
		}
	})

	b.Run("Vectorized", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = engine.EvaluateParamsVectorized(params, input)
		}
	})
}
