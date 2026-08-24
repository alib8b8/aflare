// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌‌‌​‌​‌​​‌‌​​​​​​​​‌​‌​‌​​‌​‌‌‌​​‌​‌‌‌‌​​​​​‌‌​​​​​​​​​​​​​​​​​‌​‌‌​‌‌‌‌​​‌‌‌‌⁠
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
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes"
)

// ── Benchmark 1: Workflow Parsing ──

// benchmarkWorkflowYAML returns a YAML string with n steps.
func benchmarkWorkflowYAML(n int) string {
	var sb strings.Builder
	sb.WriteString("name: Benchmark Workflow\n")
	sb.WriteString("steps:\n")
	for i := 0; i < n; i++ {
		sb.WriteString(fmt.Sprintf("  - node: step_%d\n", i))
		sb.WriteString(fmt.Sprintf("    name: step_%d\n", i))
		sb.WriteString("    params:\n")
		sb.WriteString(fmt.Sprintf("      url: https://example.com/%d\n", i))
		sb.WriteString(fmt.Sprintf("      key: val_%d\n", i))
	}
	return sb.String()
}

// BenchmarkWorkflowParse measures parsing YAML of different sizes.
// Sub-benchmarks cover small (10 steps), medium (100 steps), and large (500 steps).
func BenchmarkWorkflowParse(b *testing.B) {
	sizes := []struct {
		name  string
		steps int
	}{
		{"Small_10steps", 10},
		{"Medium_100steps", 100},
		{"Large_500steps", 500},
	}

	for _, sz := range sizes {
		yamlContent := benchmarkWorkflowYAML(sz.steps)
		b.Run(sz.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := ParseWorkflowFromContent(yamlContent)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ── Benchmark 2: Template Resolution ──

// BenchmarkTemplateResolution measures resolving templates with various
// expression patterns. Separates simple (single {{var.x}}) and composite
// (multiple expressions, prefix/suffix) templates.
func BenchmarkTemplateResolution(b *testing.B) {
	engine := NewExpressionEngine()
	engine.SetVariable("name", "world")
	engine.SetVariable("api_key", "sk-secret123")
	engine.SetVariable("model", "gpt-4o")
	engine.SetStepOutput(0, "fetch", `{"data":"fetched"}`)
	engine.SetStepOutput(1, "process", "processed-result")

	cases := []struct {
		name  string
		expr  string
		input string
	}{
		{"LiteralOnly", "no expressions here at all", ""},
		{"SingleVar", "{{var.name}}", ""},
		{"SingleStep", "{{step.0}}", ""},
		{"SingleInput", "{{input}}", "hello-input"},
		{"PrefixSuffix", "prefix_{{var.name}}_suffix", ""},
		{"MultiExpr", "{{var.name}}_{{step.0}}_{{var.api_key}}", ""},
		{"MixedLiteral", "Model: {{var.model}}, Key: {{var.api_key}}, Step: {{step.1}}", ""},
		{"LongComposite", "first={{var.name}}&second={{step.0}}&third={{var.api_key}}&fourth={{step.1}}&fifth={{var.model}}", ""},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = engine.Evaluate(c.expr, c.input)
			}
		})
	}
}

// BenchmarkTemplateResolution_ParamsMap measures batch template resolution
// using EvaluateParamsVectorized (the hot path in executor).
func BenchmarkTemplateResolution_ParamsMap(b *testing.B) {
	engine := NewExpressionEngine()
	engine.SetVariable("name", "world")
	engine.SetVariable("api_key", "sk-secret123")
	engine.SetVariable("api_url", "https://api.example.com")
	engine.SetStepOutput(0, "fetch", "step-output")

	params := map[string]string{
		"url":     "{{var.api_url}}/v1/chat",
		"auth":    "Bearer {{var.api_key}}",
		"prompt":  "Hello {{var.name}}, process: {{input}}",
		"context": "prev={{step.0}}",
		"plain":   "static-value",
	}

	b.Run("Serial", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.EvaluateParams(params, "user-input")
		}
	})

	b.Run("Vectorized", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.EvaluateParamsVectorized(params, "user-input")
		}
	})
}

// ── Benchmark 3: Expression Engine ──

// BenchmarkExpressionEngine measures the expression engine for {{var.*}} and
// {{step.*}} resolution, covering the common patterns in production workflows.
// It uses a pre-populated engine with variables and step outputs.
func BenchmarkExpressionEngine(b *testing.B) {
	engine := benchExprEngine()
	input := "benchmark-input"

	cases := []struct {
		name string
		expr string
	}{
		{"VarLookup", "{{var.api_key}}"},
		{"VarMissing", "{{var.missing}}"},
		{"StepByIndex", "{{step.0}}"},
		{"StepByName", "{{step.fetch}}"},
		{"StepMissing", "{{step.99}}"},
		{"InputExpr", "{{input}}"},
		{"LoopItem", "{{loop.item}}"},
		{"LoopIndex", "{{loop.index}}"},
		{"LoopCount", "{{loop.count}}"},
		{"JsonPath", "{{step.0.jsonpath:$.users[0].name}}"},
		{"CompositeVarStep", "var={{var.api_key}} step={{step.0}} loop={{loop.item}}"},
		{"AllPrefixes", "step={{step.0}} var={{var.api_key}} env={{env.HOME}} input={{input}} loop={{loop.item}}"},
	}

	for _, c := range cases {
		// Warm the template cache once.
		_, _ = engine.Evaluate(c.expr, input)

		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = engine.Evaluate(c.expr, input)
			}
		})
	}
}

// BenchmarkExpressionEngine_Concurrent measures parallel expression evaluation
// safety. The template cache is shared across goroutines; engine state is
// read-only once populated.
func BenchmarkExpressionEngine_Concurrent(b *testing.B) {
	engine := benchExprEngine()
	exprs := []string{
		"{{var.api_key}}",
		"{{step.0}}",
		"{{step.fetch}}",
		"{{input}}",
		"{{loop.item}}",
		"prefix_{{var.api_key}}_suffix",
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = engine.Evaluate(exprs[i%len(exprs)], "in")
			i++
		}
	})
}

// ── Benchmark 4: Pricing Lookup ──

// BenchmarkPricingLookup measures pricingForModel lookup time across
// common model names, including exact matches, prefix matches, and misses.
func BenchmarkPricingLookup(b *testing.B) {
	models := []struct {
		name  string
		model string
	}{
		{"ExactMatch_gpt4o", "gpt-4o"},
		{"ExactMatch_claude", "claude-3-5-sonnet"},
		{"ExactMatch_deepseek", "deepseek-chat"},
		{"ExactMatch_gemini", "gemini-1.5-flash"},
		{"PrefixMatch_dated", "gpt-4o-2024-08-06"},
		{"PrefixMatch_claude", "claude-3-haiku-20240307"},
		{"PrefixMatch_deepseek", "deepseek-chat-1234"},
		{"MixedCase", "GPT-4O-MINI"},
		{"UnknownModel", "unknown-model-9999"},
		{"EmptyModel", ""},
		{"LocalModel", "ollama/llama3.1"},
	}

	for _, m := range models {
		b.Run(m.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = pricingForModel(m.model)
			}
		})
	}
}

// ── Benchmark 5: Cost Computation ──

// BenchmarkCostCompute measures LLM cost calculation from token counts
// across different models and token usage patterns.
func BenchmarkCostCompute(b *testing.B) {
	cases := []struct {
		name             string
		model            string
		promptTokens     int
		completionTokens int
	}{
		{"Small_gpt4o", "gpt-4o", 100, 50},
		{"Medium_gpt4o", "gpt-4o", 1000, 500},
		{"Large_gpt4o", "gpt-4o", 100000, 50000},
		{"XLarge_gpt4o", "gpt-4o", 1000000, 500000},
		{"Small_claude", "claude-3-5-sonnet", 100, 50},
		{"Medium_claude", "claude-3-5-sonnet", 1000, 500},
		{"Small_deepseek", "deepseek-chat", 100, 50},
		{"PromptOnly", "gpt-4o", 1000, 0},
		{"CompletionOnly", "gpt-4o", 0, 1000},
		{"ZeroTokens", "gpt-4o", 0, 0},
		{"UnknownModel", "unknown-model", 1000, 500},
		{"PrefixMatch", "gpt-4o-2024-08-06", 1000, 500},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = computeLLMCost(c.model, c.promptTokens, c.completionTokens)
			}
		})
	}
}

// BenchmarkCostCompute_Batch measures batch cost computation across many
// models, simulating a trace with many LLM calls.
func BenchmarkCostCompute_Batch(b *testing.B) {
	// Simulate a trace with 50 different LLM calls mixing known and
	// prefix-matched models.
	type call struct {
		model            string
		promptTokens     int
		completionTokens int
	}
	calls := make([]call, 50)
	models := []string{
		"gpt-4o", "gpt-4o-mini", "gpt-4-turbo",
		"claude-3-5-sonnet", "claude-3-haiku",
		"deepseek-chat", "deepseek-reasoner",
		"gpt-4o-2024-08-06", "claude-3-haiku-20240307",
		"gemini-1.5-flash",
	}
	for i := range calls {
		calls[i] = call{
			model:            models[i%len(models)],
			promptTokens:     (i + 1) * 100,
			completionTokens: (i + 1) * 50,
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var total float64
		for _, c := range calls {
			total += computeLLMCost(c.model, c.promptTokens, c.completionTokens)
		}
		_ = total
	}
}

// ── Benchmark 6: Saga Execution ──

// sagaBenchNode is a minimal node for saga benchmarks. It records calls into
// a shared trace slice and returns a deterministic output.
type sagaBenchNode struct {
	name       string
	output     string
	failOnCall int // fail on the N-th call (0-based); init with failNever
	mu         *sync.Mutex
	trace      *[]string
	callCount  *int
}

const failNever = -1

func (n *sagaBenchNode) Name() string        { return n.name }
func (n *sagaBenchNode) Description() string { return "saga benchmark node" }
func (n *sagaBenchNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{
		Name:        n.name,
		Description: "saga benchmark node",
		Input:       "string",
		Output:      "string",
	}
}

func (n *sagaBenchNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	n.mu.Lock()
	callIdx := *n.callCount
	*n.callCount++
	n.mu.Unlock()

	n.mu.Lock()
	*n.trace = append(*n.trace, fmt.Sprintf("%s(%s)", n.name, input))
	n.mu.Unlock()

	if n.failOnCall >= 0 && callIdx == n.failOnCall {
		return "", fmt.Errorf("saga: %s failed on call %d", n.name, callIdx)
	}
	if n.output != "" {
		return n.output, nil
	}
	return "ok:" + input, nil
}

// newSagaBenchRegistry creates a registry with benchmark nodes sharing trace state.
func newSagaBenchRegistry(entries []sagaBenchNode) (*nodes.Registry, *sync.Mutex, *[]string, *int) {
	var mu sync.Mutex
	var trace []string
	callCount := 0
	reg := nodes.NewRegistry()
	for i := range entries {
		e := entries[i]
		e.mu = &mu
		e.trace = &trace
		e.callCount = &callCount
		reg.Register(&e)
	}
	return reg, &mu, &trace, &callCount
}

// BenchmarkSagaExecution benchmarks the saga forward+compensate execution path.
// It covers the all-success path (no compensation) and the failure+rollback
// path with reverse compensation.
func BenchmarkSagaExecution(b *testing.B) {
	// ── All-forward-succeed path (no compensation) ──
	b.Run("AllForwardSucceed", func(b *testing.B) {
		reg, _, _, _ := newSagaBenchRegistry([]sagaBenchNode{
			{name: "debit", output: "debit-done", failOnCall: failNever},
			{name: "credit", output: "credit-done", failOnCall: failNever},
			{name: "notify", output: "notify-done", failOnCall: failNever},
		})

		wStep := WorkflowStep{
			Name: "transfer",
			Saga: &SagaConfig{
				Steps: []SagaStep{
					{Forward: WorkflowStep{Node: "debit"}},
					{Forward: WorkflowStep{Node: "credit"}, Compensate: &WorkflowStep{Node: "notify"}},
					{Forward: WorkflowStep{Node: "notify"}},
				},
			},
		}

		parentEngine := NewExpressionEngine()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, err := executeSagaStep(context.Background(), 0, wStep, "initial", parentEngine, reg, nil, nil)
			if err != nil {
				b.Fatalf("all-forward-succeed should not fail: %v", err)
			}
		}
	})

	// ── Failure triggers reverse compensation path ──
	b.Run("FailureCompensate", func(b *testing.B) {
		reg, _, _, callCount := newSagaBenchRegistry([]sagaBenchNode{
			{name: "debit", output: "debit-done", failOnCall: failNever},
			{name: "credit", failOnCall: 1}, // fails on credit
			{name: "refund_debit", output: "refunded", failOnCall: failNever},
		})

		wStep := WorkflowStep{
			Name: "transfer",
			Saga: &SagaConfig{
				Steps: []SagaStep{
					{Forward: WorkflowStep{Node: "debit"}, Compensate: &WorkflowStep{Node: "refund_debit"}},
					{Forward: WorkflowStep{Node: "credit"}},
				},
			},
		}

		parentEngine := NewExpressionEngine()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Reset call count for each iteration so failOnCall fires correctly.
			*callCount = 0

			_, _, err := executeSagaStep(context.Background(), 0, wStep, "initial", parentEngine, reg, nil, nil)
			if err == nil {
				b.Fatal("expected saga failure, got success")
			}
		}
	})

	// ── Multi-step saga with all-forward succeed ──
	b.Run("MultiStepForward", func(b *testing.B) {
		reg, _, _, _ := newSagaBenchRegistry([]sagaBenchNode{
			{name: "step1", output: "out1", failOnCall: failNever},
			{name: "step2", output: "out2", failOnCall: failNever},
			{name: "step3", output: "out3", failOnCall: failNever},
			{name: "step4", output: "out4", failOnCall: failNever},
			{name: "step5", output: "out5", failOnCall: failNever},
			{name: "comp1", output: "comp1", failOnCall: failNever},
			{name: "comp2", output: "comp2", failOnCall: failNever},
			{name: "comp3", output: "comp3", failOnCall: failNever},
			{name: "comp4", output: "comp4", failOnCall: failNever},
			{name: "comp5", output: "comp5", failOnCall: failNever},
		})

		wStep := WorkflowStep{
			Name: "multi-transfer",
			Saga: &SagaConfig{
				Steps: []SagaStep{
					{Forward: WorkflowStep{Node: "step1"}, Compensate: &WorkflowStep{Node: "comp1"}},
					{Forward: WorkflowStep{Node: "step2"}, Compensate: &WorkflowStep{Node: "comp2"}},
					{Forward: WorkflowStep{Node: "step3"}, Compensate: &WorkflowStep{Node: "comp3"}},
					{Forward: WorkflowStep{Node: "step4"}, Compensate: &WorkflowStep{Node: "comp4"}},
					{Forward: WorkflowStep{Node: "step5"}, Compensate: &WorkflowStep{Node: "comp5"}},
				},
			},
		}

		parentEngine := NewExpressionEngine()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, err := executeSagaStep(context.Background(), 0, wStep, "initial", parentEngine, reg, nil, nil)
			if err != nil {
				b.Fatalf("multi-step forward should succeed: %v", err)
			}
		}
	})

	// ── Multi-step saga with mid-failure and full compensation chain ──
	b.Run("MultiStepCompensate", benchSagaMultiStepCompensate)

	// ── Saga with no compensate steps (side-effect-free forward steps) ──
	b.Run("NoCompensate", func(b *testing.B) {
		reg, _, _, callCount := newSagaBenchRegistry([]sagaBenchNode{
			{name: "read1", output: "read1-done", failOnCall: failNever},
			{name: "read2", output: "read2-done", failOnCall: failNever},
			{name: "write", failOnCall: 2}, // fails on write
		})

		wStep := WorkflowStep{
			Name: "no-compensate-saga",
			Saga: &SagaConfig{
				Steps: []SagaStep{
					{Forward: WorkflowStep{Node: "read1"}}, // no compensate
					{Forward: WorkflowStep{Node: "read2"}}, // no compensate
					{Forward: WorkflowStep{Node: "write"}},
				},
			},
		}

		parentEngine := NewExpressionEngine()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			*callCount = 0
			_, _, err := executeSagaStep(context.Background(), 0, wStep, "initial", parentEngine, reg, nil, nil)
			if err == nil {
				b.Fatal("expected saga failure for no-compensate")
			}
		}
	})
}

func benchSagaMultiStepCompensate(b *testing.B) {
	reg, _, _, callCount := newSagaBenchRegistry([]sagaBenchNode{
		{name: "step1", output: "out1", failOnCall: failNever},
		{name: "step2", output: "out2", failOnCall: failNever},
		{name: "step3", failOnCall: 2}, // fails on step3 (0-based call index)
		{name: "step4", output: "out4", failOnCall: failNever},
		{name: "step5", output: "out5", failOnCall: failNever},
		{name: "comp1", output: "comp1", failOnCall: failNever},
		{name: "comp2", output: "comp2", failOnCall: failNever},
		{name: "comp3", output: "comp3", failOnCall: failNever},
		{name: "comp4", output: "comp4", failOnCall: failNever},
		{name: "comp5", output: "comp5", failOnCall: failNever},
	})

	wStep := WorkflowStep{
		Name: "multi-rollback",
		Saga: &SagaConfig{
			Steps: []SagaStep{
				{Forward: WorkflowStep{Node: "step1"}, Compensate: &WorkflowStep{Node: "comp1"}},
				{Forward: WorkflowStep{Node: "step2"}, Compensate: &WorkflowStep{Node: "comp2"}},
				{Forward: WorkflowStep{Node: "step3"}, Compensate: &WorkflowStep{Node: "comp3"}},
				{Forward: WorkflowStep{Node: "step4"}, Compensate: &WorkflowStep{Node: "comp4"}},
				{Forward: WorkflowStep{Node: "step5"}, Compensate: &WorkflowStep{Node: "comp5"}},
			},
		},
	}

	parentEngine := NewExpressionEngine()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		*callCount = 0
		_, _, err := executeSagaStep(context.Background(), 0, wStep, "initial", parentEngine, reg, nil, nil)
		if err == nil {
			b.Fatal("expected saga failure for multi-step compensate")
		}
	}
}
