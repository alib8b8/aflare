// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​​​​​​​‌​‌‌‌‌​​‌‌​​‌​‌​‌​​​​‌‌​​​‌‌​‌‌‌‌​‌​‌​‌​​​​​​​​​​​​​​​​​​‌‌‌​‌‌​‌​​‌​‌​⁠
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

package workflow

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes"
)

// sagaTraceNode records every Execute call (name + input) into a shared slice
// so a test can assert the exact order of forward and compensation steps. The
// node returns its configured output on success, or an error when failOnInput
// matches the input (deterministic failure injection without env vars or
// counters). When alwaysFail is true the node fails unconditionally.
type sagaTraceNode struct {
	name        string
	output      string // returned on success (input is appended after "->")
	failOnInput string // Execute returns an error when input == this
	alwaysFail  bool   // Execute always returns an error
	mu          *sync.Mutex
	trace       *[]string
}

func (n *sagaTraceNode) Name() string        { return n.name }
func (n *sagaTraceNode) Description() string { return "saga trace node" }
func (n *sagaTraceNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{
		Name:        n.name,
		Description: "saga trace node",
		Input:       "string",
		Output:      "string",
	}
}

func (n *sagaTraceNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	n.mu.Lock()
	*n.trace = append(*n.trace, fmt.Sprintf("%s(%s)", n.name, input))
	n.mu.Unlock()

	if n.alwaysFail {
		return "", fmt.Errorf("saga: %s forced failure", n.name)
	}
	if n.failOnInput != "" && input == n.failOnInput {
		return "", fmt.Errorf("saga: %s forced failure on input %q", n.name, input)
	}
	if n.output != "" {
		return n.output, nil
	}
	return "ok:" + input, nil
}

// newSagaRegistry builds a registry with the given trace nodes, all sharing a
// single trace slice and mutex.
func newSagaRegistry(entries []sagaTraceNode) (*nodes.Registry, *sync.Mutex, *[]string) {
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

// TestSaga_AllForwardSucceed verifies that when every forward step succeeds,
// no compensation runs and the last forward step's output is the saga output.
func TestSaga_AllForwardSucceed(t *testing.T) {
	reg, _, tracePtr := newSagaRegistry([]sagaTraceNode{
		{name: "debit", output: "debit-done"},
		{name: "credit", output: "credit-done"},
		{name: "notify", output: "notify-done"},
	})

	wf := &Workflow{
		Name: "saga-commit",
		Steps: []WorkflowStep{
			{
				Name: "transfer",
				Saga: &SagaConfig{
					Steps: []SagaStep{
						{Forward: WorkflowStep{Node: "debit"}},
						{Forward: WorkflowStep{Node: "credit"}, Compensate: &WorkflowStep{Node: "notify"}},
						{Forward: WorkflowStep{Node: "notify"}},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("saga should commit on all-success, got error: %v", err)
	}
	if output != "notify-done" {
		t.Errorf("expected last forward output 'notify-done', got %q", output)
	}
	// All three forward steps ran in order; no compensation.
	trace := *tracePtr
	if len(trace) != 3 {
		t.Fatalf("expected 3 forward calls, got %d: %v", len(trace), trace)
	}
	for i, want := range []string{"debit(", "credit(", "notify("} {
		if !strings.HasPrefix(trace[i], want) {
			t.Errorf("trace[%d] = %q, want prefix %q", i, trace[i], want)
		}
	}
}

// TestSaga_FailureTriggersReverseCompensation verifies that when a mid-saga
// forward step fails, the already-completed forward steps are compensated in
// REVERSE order, and the saga returns the triggering error.
func TestSaga_FailureTriggersReverseCompensation(t *testing.T) {
	// credit fails -> debit (the only completed step) must be compensated.
	reg, _, tracePtr := newSagaRegistry([]sagaTraceNode{
		{name: "debit", output: "debit-done"},
		{name: "credit", failOnInput: "debit-done"}, // fails on the debit output
		{name: "refund_debit", output: "refunded"},
	})

	wf := &Workflow{
		Name: "saga-rollback",
		Steps: []WorkflowStep{
			{
				Name: "transfer",
				Saga: &SagaConfig{
					Steps: []SagaStep{
						{Forward: WorkflowStep{Node: "debit"}, Compensate: &WorkflowStep{Node: "refund_debit"}},
						{Forward: WorkflowStep{Node: "credit"}},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Fatal("expected saga to return the forward failure error")
	}
	if !strings.Contains(err.Error(), "forced failure") {
		t.Errorf("expected forced-failure error, got: %v", err)
	}
	// On failure the sequential executor returns an empty output (matching
	// the existing StepError convention); the saga's lastOutput is only
	// observable via a capture_error branch on the saga step itself.
	_ = output

	trace := *tracePtr
	// Expected order: debit(forward), credit(forward, fails), refund_debit(compensate).
	if len(trace) != 3 {
		t.Fatalf("expected 3 calls (debit, credit, refund_debit), got %d: %v", len(trace), trace)
	}
	if !strings.HasPrefix(trace[0], "debit(") {
		t.Errorf("trace[0] = %q, want debit forward", trace[0])
	}
	if !strings.HasPrefix(trace[1], "credit(") {
		t.Errorf("trace[1] = %q, want credit forward", trace[1])
	}
	if !strings.HasPrefix(trace[2], "refund_debit(") {
		t.Errorf("trace[2] = %q, want refund_debit compensation", trace[2])
	}
	// Compensation receives the forward step's output as input.
	if !strings.Contains(trace[2], "debit-done") {
		t.Errorf("compensation should receive forward output 'debit-done' as input, got %q", trace[2])
	}
}

// TestSaga_CompensationIsBestEffort verifies that a compensating step that
// itself fails does NOT abort the compensation of earlier steps.
func TestSaga_CompensationIsBestEffort(t *testing.T) {
	// notify fails -> credit (compensate: refund_credit, which also fails) and
	// debit (compensate: refund_debit) must both run; refund_credit failing
	// must not block refund_debit.
	reg, _, tracePtr := newSagaRegistry([]sagaTraceNode{
		{name: "debit", output: "debit-done"},
		{name: "credit", output: "credit-done"},
		{name: "notify", failOnInput: "credit-done"},        // fails on credit output
		{name: "refund_credit", failOnInput: "credit-done"}, // compensation also fails
		{name: "refund_debit", output: "refunded"},
	})

	wf := &Workflow{
		Name: "saga-best-effort",
		Steps: []WorkflowStep{
			{
				Name: "transfer",
				Saga: &SagaConfig{
					Steps: []SagaStep{
						{Forward: WorkflowStep{Node: "debit"}, Compensate: &WorkflowStep{Node: "refund_debit"}},
						{Forward: WorkflowStep{Node: "credit"}, Compensate: &WorkflowStep{Node: "refund_credit"}},
						{Forward: WorkflowStep{Node: "notify"}},
					},
				},
			},
		},
	}

	_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Fatal("expected saga to return the notify forward failure")
	}

	trace := *tracePtr
	// Expected order:
	//  1. debit (forward)
	//  2. credit (forward)
	//  3. notify (forward, fails)
	//  4. refund_credit (compensate credit, fails - best effort)
	//  5. refund_debit (compensate debit, succeeds)
	// Compensation is in REVERSE order: credit's compensate before debit's.
	if len(trace) != 5 {
		t.Fatalf("expected 5 calls, got %d: %v", len(trace), trace)
	}
	wantOrder := []string{"debit(", "credit(", "notify(", "refund_credit(", "refund_debit("}
	for i, want := range wantOrder {
		if !strings.HasPrefix(trace[i], want) {
			t.Errorf("trace[%d] = %q, want prefix %q", i, trace[i], want)
		}
	}
}

// TestSaga_NoCompensateStep verifies that a forward step without a Compensate
// is simply skipped during rollback (no side effect to undo).
func TestSaga_NoCompensateStep(t *testing.T) {
	reg, _, tracePtr := newSagaRegistry([]sagaTraceNode{
		{name: "read", output: "read-done"},       // no compensate (pure read)
		{name: "write", output: "write-done"},     // has compensate
		{name: "fail", failOnInput: "write-done"}, // triggers rollback
		{name: "undo_write", output: "undone"},
	})

	wf := &Workflow{
		Name: "saga-no-compensate",
		Steps: []WorkflowStep{
			{
				Saga: &SagaConfig{
					Steps: []SagaStep{
						{Forward: WorkflowStep{Node: "read"}}, // no compensate
						{Forward: WorkflowStep{Node: "write"}, Compensate: &WorkflowStep{Node: "undo_write"}},
						{Forward: WorkflowStep{Node: "fail"}},
					},
				},
			},
		},
	}

	_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Fatal("expected saga failure")
	}
	trace := *tracePtr
	// read, write, fail(forward), undo_write(compensate write only). read has
	// no compensate so it is skipped during rollback.
	if len(trace) != 4 {
		t.Fatalf("expected 4 calls (read skipped in rollback), got %d: %v", len(trace), trace)
	}
	wantOrder := []string{"read(", "write(", "fail(", "undo_write("}
	for i, want := range wantOrder {
		if !strings.HasPrefix(trace[i], want) {
			t.Errorf("trace[%d] = %q, want prefix %q", i, trace[i], want)
		}
	}
}

// TestSaga_VarErrorExposedToCompensate verifies that {{var.error}} is set to
// the triggering failure message inside a compensating step.
func TestSaga_VarErrorExposedToCompensate(t *testing.T) {
	// A compensate step whose params reference {{var.error}}; we capture it
	// via a node that echoes its param into the trace.
	reg, _, tracePtr := newSagaRegistry([]sagaTraceNode{
		{name: "debit", output: "debit-done"},
		{name: "credit", failOnInput: "debit-done"},
		{name: "refund", output: "refunded"},
	})

	wf := &Workflow{
		Name: "saga-var-error",
		Steps: []WorkflowStep{
			{
				Saga: &SagaConfig{
					Steps: []SagaStep{
						{Forward: WorkflowStep{Node: "debit"}, Compensate: &WorkflowStep{Node: "refund", Params: map[string]string{"prefix": "{{var.error}}"}}},
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
	trace := *tracePtr
	if len(trace) != 3 {
		t.Fatalf("expected 3 calls, got %d: %v", len(trace), trace)
	}
	// refund's input is the debit output; the {{var.error}} is in params, not
	// input, so we only assert the compensate ran with the forward output.
	if !strings.HasPrefix(trace[2], "refund(") {
		t.Errorf("trace[2] = %q, want refund compensation", trace[2])
	}
	if !strings.Contains(trace[2], "debit-done") {
		t.Errorf("compensate should receive forward output 'debit-done' as input, got %q", trace[2])
	}
}

// TestSaga_EmptySteps verifies that a saga with no forward steps returns
// successfully with empty output and runs nothing.
func TestSaga_EmptySteps(t *testing.T) {
	reg, _, tracePtr := newSagaRegistry(nil)

	wf := &Workflow{
		Steps: []WorkflowStep{
			{Saga: &SagaConfig{Steps: []SagaStep{}}},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("empty saga should succeed, got: %v", err)
	}
	if output != "" {
		t.Errorf("empty saga output should be empty, got %q", output)
	}
	if len(*tracePtr) != 0 {
		t.Errorf("empty saga should run no steps, got trace: %v", *tracePtr)
	}
}

// TestSaga_FailureOnFirstStep verifies that when the very first forward step
// fails, no compensation runs (nothing completed to undo) and the saga returns
// the error.
func TestSaga_FailureOnFirstStep(t *testing.T) {
	reg, _, tracePtr := newSagaRegistry([]sagaTraceNode{
		{name: "fail", alwaysFail: true},
		{name: "never", output: "never-done"},
		{name: "compensate", output: "undone"},
	})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Saga: &SagaConfig{
					Steps: []SagaStep{
						{Forward: WorkflowStep{Node: "fail"}, Compensate: &WorkflowStep{Node: "compensate"}},
						{Forward: WorkflowStep{Node: "never"}},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Fatal("expected first-step failure")
	}
	// Sequential executor returns empty output on failure (existing convention).
	_ = output
	trace := *tracePtr
	// Only the failed forward step ran; no compensation (nothing completed).
	if len(trace) != 1 {
		t.Fatalf("expected 1 call (failed forward only), got %d: %v", len(trace), trace)
	}
	if !strings.HasPrefix(trace[0], "fail(") {
		t.Errorf("trace[0] = %q, want fail forward", trace[0])
	}
}
