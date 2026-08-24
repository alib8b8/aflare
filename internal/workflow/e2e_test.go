// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌‌​​​​​‌​‌​​​‌​‌‌‌​​​​​‌​​‌​‌​‌​‌​‌‌​​‌​‌​‌​‌​​​​​​​​​​​​​​​​‌‌​‌‌‌‌‌‌​‌‌‌‌‌​⁠
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

// ── E2E Test Helpers ──

// captureNode records every Execute call (name, input, resolved params) into a
// shared slice so tests can assert template resolution and variable flow.
type captureNode struct {
	name   string
	output string      // returned on success
	fail   bool        // always return an error
	mu     *sync.Mutex // shared mutex for trace slice
	trace  *[]string   // shared trace slice
}

func (n *captureNode) Name() string        { return n.name }
func (n *captureNode) Description() string { return "e2e capture node" }
func (n *captureNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{
		Name:        n.name,
		Description: "e2e capture node",
		Input:       "string",
		Output:      "string",
	}
}

func (n *captureNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	entry := fmt.Sprintf("%s(in=%q", n.name, input)
	for k, v := range params {
		entry += fmt.Sprintf(", %s=%q", k, v)
	}
	entry += ")"
	n.mu.Lock()
	*n.trace = append(*n.trace, entry)
	n.mu.Unlock()

	if n.fail {
		return "", fmt.Errorf("e2e: %s forced failure", n.name)
	}
	if n.output != "" {
		return n.output, nil
	}
	return "ok:" + input, nil
}

func newE2ERegistry(entries []captureNode) (*nodes.Registry, *sync.Mutex, *[]string) {
	var mu sync.Mutex
	var trace []string
	reg := nodes.NewRegistry()
	for i := range entries {
		e := entries[i]
		e.mu = &mu
		e.trace = &trace
		reg.Register(&e)
	}
	return reg, &mu, &trace
}

// ── Test 1: Workflow Parse ──

func TestE2E_WorkflowParse(t *testing.T) {
	// Parse the real saga-transfer workflow YAML from examples/
	wf, err := ParseWorkflow("../../examples/finance/saga-transfer/workflow.yaml")
	if err != nil {
		t.Fatalf("failed to parse real workflow YAML: %v", err)
	}

	// Verify workflow metadata
	if wf.Name != "saga-transfer" {
		t.Errorf("expected name 'saga-transfer', got %q", wf.Name)
	}
	if wf.Description == "" {
		t.Error("expected non-empty description")
	}

	// Verify workflow-level vars
	if len(wf.Vars) == 0 {
		t.Error("expected workflow-level vars")
	}
	if v, ok := wf.Vars["amount"]; !ok || v != "9999" {
		t.Errorf("expected var 'amount' = '9999', got %q", v)
	}
	if v, ok := wf.Vars["from_account"]; !ok || v != "ACC0001" {
		t.Errorf("expected var 'from_account' = 'ACC0001', got %q", v)
	}

	// Verify steps exist
	if len(wf.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(wf.Steps))
	}

	// First step should be a saga
	if !wf.Steps[0].IsSaga() {
		t.Fatal("first step must be a saga")
	}
	saga := wf.Steps[0].Saga
	if saga == nil {
		t.Fatal("saga config is nil")
	}
	if len(saga.Steps) != 3 {
		t.Fatalf("expected 3 saga steps, got %d", len(saga.Steps))
	}

	// Verify saga forward steps have correct names
	expectedForwards := []string{"debit", "credit", "notify"}
	for i, want := range expectedForwards {
		got := saga.Steps[i].Forward.Name
		if got != want {
			t.Errorf("saga step %d forward name: expected %q, got %q", i, want, got)
		}
	}
	// All forward steps use the http_request node
	for i, ss := range saga.Steps {
		if ss.Forward.Node != "http_request" {
			t.Errorf("saga step %d forward node: expected 'http_request', got %q", i, ss.Forward.Node)
		}
	}

	// Verify compensations
	if saga.Steps[0].Compensate == nil {
		t.Error("debit should have a compensate step")
	} else if saga.Steps[0].Compensate.Name != "refund-debit" {
		t.Errorf("debit compensate name: expected 'refund-debit', got %q", saga.Steps[0].Compensate.Name)
	}
	if saga.Steps[1].Compensate == nil {
		t.Error("credit should have a compensate step")
	} else if saga.Steps[1].Compensate.Name != "reverse-credit" {
		t.Errorf("credit compensate name: expected 'reverse-credit', got %q", saga.Steps[1].Compensate.Name)
	}
	if saga.Steps[2].Compensate != nil {
		t.Error("notify should NOT have a compensate step")
	}

	// Second step should be a regular node
	if wf.Steps[1].IsSaga() {
		t.Error("second step should not be a saga")
	}
	if wf.Steps[1].Node != "file_write" {
		t.Errorf("second step node: expected 'file_write', got %q", wf.Steps[1].Node)
	}

	// Also test ParseWorkflowFromContent with inline YAML
	inlineYAML := `
name: inline-test
description: Test inline parsing
vars:
  foo: bar
  num: "42"
steps:
  - name: step1
    node: test_node
    params:
      key: "{{var.foo}}"
  - name: step2
    node: test_node
    params:
      key: "{{step.step1}}"
`
	wf2, err := ParseWorkflowFromContent(inlineYAML)
	if err != nil {
		t.Fatalf("failed to parse inline YAML: %v", err)
	}
	if wf2.Name != "inline-test" {
		t.Errorf("expected name 'inline-test', got %q", wf2.Name)
	}
	if len(wf2.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(wf2.Steps))
	}
	if v, ok := wf2.Vars["foo"]; !ok || v != "bar" {
		t.Errorf("expected var 'foo' = 'bar', got %q", v)
	}
}

// ── Test 2: Template Resolution ──

func TestE2E_TemplateResolution(t *testing.T) {
	reg, _, tracePtr := newE2ERegistry([]captureNode{
		{name: "echo", output: "echo-done"},
	})

	// Use ParseWorkflowFromContent to test template resolution through the
	// full YAML → parse → execute pipeline.
	yamlContent := `
name: template-resolution-test
vars:
  greeting: "hello"
  target: "world"
steps:
  - name: first-step
    node: echo
    params:
      greeting: "{{var.greeting}}"
      target: "{{var.target}}"
  - name: second-step
    node: echo
    params:
      prev_output: "{{step.first-step}}"
      combined: "{{var.greeting}} {{var.target}}"
`
	wf, err := ParseWorkflowFromContent(yamlContent)
	if err != nil {
		t.Fatalf("failed to parse workflow YAML: %v", err)
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if output != "echo-done" {
		t.Errorf("expected output 'echo-done', got %q", output)
	}

	// Verify template resolution via trace
	trace := *tracePtr
	if len(trace) != 2 {
		t.Fatalf("expected 2 trace entries, got %d: %v", len(trace), trace)
	}

	// First step: {{var.greeting}} and {{var.target}} should be resolved
	if !strings.Contains(trace[0], `greeting="hello"`) {
		t.Errorf("first step: expected greeting='hello', got %q", trace[0])
	}
	if !strings.Contains(trace[0], `target="world"`) {
		t.Errorf("first step: expected target='world', got %q", trace[0])
	}

	// Second step: {{step.first-step}} should resolve to first step's output
	if !strings.Contains(trace[1], `prev_output="echo-done"`) {
		t.Errorf("second step: expected prev_output='echo-done', got %q", trace[1])
	}
	if !strings.Contains(trace[1], `combined="hello world"`) {
		t.Errorf("second step: expected combined='hello world', got %q", trace[1])
	}
}

// ── Test 3: Saga Compensation ──

func TestE2E_SagaCompensation(t *testing.T) {
	t.Run("AllForwardSucceed", func(t *testing.T) {
		reg, _, tracePtr := newE2ERegistry([]captureNode{
			{name: "debit", output: "debit-ok"},
			{name: "credit", output: "credit-ok"},
			{name: "notify", output: "notify-ok"},
			{name: "refund_debit", output: "refunded"},
		})

		wf := &Workflow{
			Name: "saga-commit",
			Steps: []WorkflowStep{
				{
					Name: "transfer",
					Saga: &SagaConfig{
						Steps: []SagaStep{
							{
								Forward:    WorkflowStep{Node: "debit"},
								Compensate: &WorkflowStep{Node: "refund_debit"},
							},
							{Forward: WorkflowStep{Node: "credit"}},
							{Forward: WorkflowStep{Node: "notify"}},
						},
					},
				},
			},
		}

		output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
		if err != nil {
			t.Fatalf("saga should commit: %v", err)
		}
		if output != "notify-ok" {
			t.Errorf("expected 'notify-ok', got %q", output)
		}

		trace := *tracePtr
		if len(trace) != 3 {
			t.Fatalf("expected 3 forward calls, got %d: %v", len(trace), trace)
		}
		// Verify no compensation ran
		for _, entry := range trace {
			if strings.Contains(entry, "refund") {
				t.Errorf("compensation should not run on success, got %q", entry)
			}
		}
	})

	t.Run("FailureTriggersCompensation", func(t *testing.T) {
		// credit fails → debit gets compensated
		reg, _, tracePtr := newE2ERegistry([]captureNode{
			{name: "debit", output: "debit-ok"},
			{name: "credit", fail: true},
			{name: "refund_debit", output: "refunded"},
		})

		wf := &Workflow{
			Name: "saga-rollback",
			Steps: []WorkflowStep{
				{
					Name: "transfer",
					Saga: &SagaConfig{
						Steps: []SagaStep{
							{
								Forward:    WorkflowStep{Node: "debit"},
								Compensate: &WorkflowStep{Node: "refund_debit"},
							},
							{Forward: WorkflowStep{Node: "credit"}},
						},
					},
				},
			},
		}

		_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
		if err == nil {
			t.Fatal("expected saga failure")
		}
		if !strings.Contains(err.Error(), "forced failure") {
			t.Errorf("expected forced-failure error, got: %v", err)
		}

		trace := *tracePtr
		if len(trace) != 3 {
			t.Fatalf("expected 3 calls (debit, credit, refund_debit), got %d: %v", len(trace), trace)
		}
		// Verify order: debit forward → credit forward (fails) → refund_debit compensate
		if !strings.Contains(trace[0], "debit") {
			t.Errorf("trace[0] should be debit: %q", trace[0])
		}
		if !strings.Contains(trace[1], "credit") {
			t.Errorf("trace[1] should be credit: %q", trace[1])
		}
		if !strings.Contains(trace[2], "refund_debit") {
			t.Errorf("trace[2] should be refund_debit: %q", trace[2])
		}
	})

	t.Run("FirstStepFailureNoCompensation", func(t *testing.T) {
		// First forward step fails → no compensation
		reg, _, tracePtr := newE2ERegistry([]captureNode{
			{name: "debit", fail: true},
			{name: "credit", output: "credit-ok"},
			{name: "refund_debit", output: "refunded"},
		})

		wf := &Workflow{
			Steps: []WorkflowStep{
				{
					Saga: &SagaConfig{
						Steps: []SagaStep{
							{
								Forward:    WorkflowStep{Node: "debit"},
								Compensate: &WorkflowStep{Node: "refund_debit"},
							},
							{Forward: WorkflowStep{Node: "credit"}},
						},
					},
				},
			},
		}

		_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
		if err == nil {
			t.Fatal("expected first-step failure")
		}

		trace := *tracePtr
		if len(trace) != 1 {
			t.Fatalf("expected 1 call (debit only), got %d: %v", len(trace), trace)
		}
		if !strings.Contains(trace[0], "debit") {
			t.Errorf("only debit should have run: %q", trace[0])
		}
	})
}

// ── Test 4: Error Recovery ──

func TestE2E_ErrorRecovery(t *testing.T) {
	t.Run("ContinueOnError", func(t *testing.T) {
		reg, _, tracePtr := newE2ERegistry([]captureNode{
			{name: "failer", fail: true},
			{name: "echo", output: "recovered"},
		})

		wf := &Workflow{
			Steps: []WorkflowStep{
				{Node: "failer", ContinueOnError: true, Name: "failing-step"},
				{Node: "echo", Name: "recovery-step"},
			},
		}

		output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
		if err != nil {
			t.Fatalf("workflow should continue after error: %v", err)
		}
		if output != "recovered" {
			t.Errorf("expected 'recovered', got %q", output)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].Error == nil {
			t.Error("first step should have error")
		}
		if results[1].Error != nil {
			t.Error("second step should succeed")
		}

		// Both steps should have executed
		trace := *tracePtr
		if len(trace) != 2 {
			t.Fatalf("expected 2 trace entries, got %d: %v", len(trace), trace)
		}
	})

	t.Run("Fallback", func(t *testing.T) {
		reg, _, tracePtr := newE2ERegistry([]captureNode{
			{name: "failer", fail: true},
		})

		wf := &Workflow{
			Steps: []WorkflowStep{
				{Node: "failer", Fallback: "fallback-value"},
			},
		}

		output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
		if err != nil {
			t.Fatalf("fallback should prevent error: %v", err)
		}
		if output != "fallback-value" {
			t.Errorf("expected 'fallback-value', got %q", output)
		}
		if results[0].Error != nil {
			t.Error("error should be recovered by fallback")
		}

		// The node should still have been called
		trace := *tracePtr
		if len(trace) != 1 {
			t.Fatalf("expected 1 trace entry, got %d", len(trace))
		}
	})

	t.Run("OnError", func(t *testing.T) {
		reg, _, tracePtr := newE2ERegistry([]captureNode{
			{name: "failer", fail: true},
			{name: "handler", output: "handled"},
		})

		wf := &Workflow{
			Steps: []WorkflowStep{
				{
					Node: "failer",
					OnError: &Step{
						Node:   "handler",
						Params: map[string]string{"ctx": "error-handler"},
					},
				},
			},
		}

		output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
		if err != nil {
			t.Fatalf("on_error handler should prevent error: %v", err)
		}
		if output != "handled" {
			t.Errorf("expected 'handled', got %q", output)
		}

		trace := *tracePtr
		if len(trace) != 2 {
			t.Fatalf("expected 2 trace entries (fail + handler), got %d: %v", len(trace), trace)
		}
		if !strings.Contains(trace[0], "failer") {
			t.Errorf("trace[0] should be failer: %q", trace[0])
		}
		if !strings.Contains(trace[1], "handler") {
			t.Errorf("trace[1] should be handler: %q", trace[1])
		}
	})

	t.Run("Retry", testErrorRecoveryRetry)

	t.Run("RetryExhausted", func(t *testing.T) {
		reg, _, tracePtr := newE2ERegistry([]captureNode{
			{name: "failer", fail: true},
		})

		wf := &Workflow{
			Steps: []WorkflowStep{
				{Node: "failer", Retry: 2, Delay: "10ms"},
			},
		}

		_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
		if err == nil {
			t.Fatal("expected error after retries exhausted")
		}

		// The node should have been called 3 times (1 initial + 2 retries)
		trace := *tracePtr
		if len(trace) != 3 {
			t.Fatalf("expected 3 attempts (1 + 2 retries), got %d: %v", len(trace), trace)
		}
	})
}

func testErrorRecoveryRetry(t *testing.T) {
	// A node that succeeds on the third attempt
	attempt := 0
	var mu sync.Mutex
	reg := nodes.NewRegistry()
	reg.Register(&retryNode{
		name: "retryer",
		exec: func() (string, error) {
			mu.Lock()
			attempt++
			a := attempt
			mu.Unlock()
			if a < 3 {
				return "", fmt.Errorf("attempt %d failed", a)
			}
			return "success-after-retry", nil
		},
	})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{Node: "retryer", Retry: 3, Delay: "10ms"},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("retry should eventually succeed: %v", err)
	}
	if output != "success-after-retry" {
		t.Errorf("expected 'success-after-retry', got %q", output)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error != nil {
		t.Error("final result should have no error after retry")
	}
}

// ── Test 5: Variable Flow Between Steps ──

func TestE2E_VariableFlow(t *testing.T) {
	t.Run("VarAccessInParams", func(t *testing.T) {
		reg, _, tracePtr := newE2ERegistry([]captureNode{
			{name: "echo", output: "step-output"},
		})

		wf := &Workflow{
			Vars: map[string]string{
				"api_url": "https://api.example.com",
				"timeout": "30s",
			},
			Steps: []WorkflowStep{
				{
					Name: "call-api",
					Node: "echo",
					Params: map[string]string{
						"url":     "{{var.api_url}}",
						"timeout": "{{var.timeout}}",
					},
				},
			},
		}

		output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}
		if output != "step-output" {
			t.Errorf("expected 'step-output', got %q", output)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}

		trace := *tracePtr
		if len(trace) != 1 {
			t.Fatalf("expected 1 trace entry, got %d", len(trace))
		}
		if !strings.Contains(trace[0], `url="https://api.example.com"`) {
			t.Errorf("var.api_url not resolved: %q", trace[0])
		}
		if !strings.Contains(trace[0], `timeout="30s"`) {
			t.Errorf("var.timeout not resolved: %q", trace[0])
		}
	})

	t.Run("StepOutputFlow", func(t *testing.T) {
		reg, _, tracePtr := newE2ERegistry([]captureNode{
			{name: "producer", output: "produced-data"},
			{name: "consumer", output: "consumed"},
		})

		wf := &Workflow{
			Steps: []WorkflowStep{
				{Name: "producer", Node: "producer"},
				{
					Name: "consumer",
					Node: "consumer",
					Params: map[string]string{
						"data": "{{step.producer}}",
					},
				},
			},
		}

		output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}
		if output != "consumed" {
			t.Errorf("expected 'consumed', got %q", output)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}

		// Verify flow: step 2 received step 1's output
		trace := *tracePtr
		if len(trace) != 2 {
			t.Fatalf("expected 2 trace entries, got %d", len(trace))
		}
		// Consumer should receive "produced-data" as the flowing input
		if !strings.Contains(trace[1], `in="produced-data"`) {
			t.Errorf("consumer should receive produced-data as input: %q", trace[1])
		}
		if !strings.Contains(trace[1], `data="produced-data"`) {
			t.Errorf("consumer param 'data' should resolve to produced-data: %q", trace[1])
		}
	})

	t.Run("MultiStepDataFlow", func(t *testing.T) {
		// Three-step pipeline: each step transforms the data
		reg, _, tracePtr := newE2ERegistry([]captureNode{
			{name: "stepA", output: "alpha"},
			{name: "stepB", output: "beta"},
			{name: "stepC", output: "gamma"},
		})

		wf := &Workflow{
			Steps: []WorkflowStep{
				{Name: "stepA", Node: "stepA"},
				{Name: "stepB", Node: "stepB"},
				{Name: "stepC", Node: "stepC"},
			},
		}

		output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}
		if output != "gamma" {
			t.Errorf("expected 'gamma', got %q", output)
		}
		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}

		// Verify the data flowing through each step
		trace := *tracePtr
		if len(trace) != 3 {
			t.Fatalf("expected 3 trace entries, got %d", len(trace))
		}
		// Step A receives empty input (top-level, no initial data)
		if !strings.Contains(trace[0], `in=""`) {
			t.Errorf("stepA should receive empty input: %q", trace[0])
		}
		// Step B receives step A's output ("alpha") as flowing input
		if !strings.Contains(trace[1], `in="alpha"`) {
			t.Errorf("stepB should receive 'alpha' as input: %q", trace[1])
		}
		// Step C receives step B's output ("beta") as flowing input
		if !strings.Contains(trace[2], `in="beta"`) {
			t.Errorf("stepC should receive 'beta' as input: %q", trace[2])
		}
	})

	t.Run("ConditionBasedStepSkipping", testVariableFlowConditionBasedStepSkipping)
}

func testVariableFlowConditionBasedStepSkipping(t *testing.T) {
	reg, _, tracePtr := newE2ERegistry([]captureNode{
		{name: "stepA", output: "true"},
		{name: "stepB", output: "skipped-anyway"},
		{name: "stepC", output: "final"},
	})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{Name: "stepA", Node: "stepA"},
			{
				Name:      "stepB",
				Node:      "stepB",
				Condition: "equals:true",
			},
			{Name: "stepC", Node: "stepC"},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if output != "final" {
		t.Errorf("expected 'final', got %q", output)
	}

	// All 3 steps should have results (stepB with skipped flag)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	trace := *tracePtr
	if len(trace) != 3 {
		t.Fatalf("expected 3 trace entries, got %d: %v", len(trace), trace)
	}
	// stepC should receive stepB's output (which is "true" from stepA)
	// When condition is true, the node runs and its output flows
	if !strings.Contains(trace[2], `in="skipped-anyway"`) {
		t.Errorf("stepC should receive stepB's output: %q", trace[2])
	}
}

// ── Helper: retryNode for testing retry behavior ──

type retryNode struct {
	name string
	exec func() (string, error)
}

func (n *retryNode) Name() string        { return n.name }
func (n *retryNode) Description() string { return "retry test node" }
func (n *retryNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{
		Name:        n.name,
		Description: "retry test node",
		Input:       "string",
		Output:      "string",
	}
}
func (n *retryNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return n.exec()
}
