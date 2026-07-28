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
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alib8b8/llm-box/internal/nodes"
)

// ── A-6.3: step-level trace completeness ──
//
// These tests exercise the trace telemetry across every step outcome path in
// both sequential and DAG mode. For each scenario we verify that:
//   - ExecuteWorkflowWithTrace returns a non-nil trace
//   - len(trace.Steps) == len(results)
//   - StepResult.Trace points to the corresponding entry in trace.Steps
//   - the recorded fields match the expected outcome (skipped, attempts,
//     recoveries, error text, batch index, dependencies)

// traceTestNode is a configurable node: succeeds after `successOnAttempt`
// calls (1 = first call succeeds), or always fails when successOnAttempt <= 0.
type traceTestNode struct {
	name           string
	successOn      int // attempt number on which to succeed (1-based); 0 = never
	calls          int32
	overrideOutput string
}

func (n *traceTestNode) Name() string        { return n.name }
func (n *traceTestNode) Description() string { return "trace test node" }
func (n *traceTestNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "string"}
}
func (n *traceTestNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	calls := int(atomic.AddInt32(&n.calls, 1))
	if n.successOn > 0 && calls >= n.successOn {
		if n.overrideOutput != "" {
			return n.overrideOutput, nil
		}
		return n.name + "-ok", nil
	}
	return "", errors.New(n.name + " failed (attempt " + itoa(calls) + ")")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// runWithTrace is a small helper that runs a workflow and returns the trace.
func runWithTrace(t *testing.T, wf *Workflow, reg *nodes.Registry) *WorkflowTrace {
	t.Helper()
	_, results, trace, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		// Some scenarios (e.g. step failure without recovery) legitimately
		// return an error; we still want to inspect the trace.
		t.Logf("workflow returned error (may be expected): %v", err)
	}
	if trace == nil {
		t.Fatal("expected non-nil trace")
	}
	if len(trace.Steps) != len(results) {
		t.Fatalf("trace.Steps (%d) != results (%d)", len(trace.Steps), len(results))
	}
	// StepResult.Trace must point at the matching trace entry.
	for i, r := range results {
		if r.Trace == nil {
			t.Fatalf("result %d has nil Trace", i)
		}
		if r.Trace != &trace.Steps[i] {
			t.Errorf("result %d Trace does not point at trace.Steps[%d]", i, i)
		}
		if r.Trace.Index != r.StepIndex {
			t.Errorf("result %d: trace.Index=%d want %d", i, r.Trace.Index, r.StepIndex)
		}
	}
	return trace
}

func TestTrace_SequentialSuccess(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "n1", successOn: 1})
	reg.Register(&traceTestNode{name: "n2", successOn: 1})

	wf := &Workflow{
		Name: "seq-success",
		Steps: []WorkflowStep{
			{Node: "n1", Name: "first"},
			{Node: "n2", Name: "second"},
		},
	}

	trace := runWithTrace(t, wf, reg)

	if trace.Mode != "sequential" {
		t.Errorf("expected Mode=sequential, got %q", trace.Mode)
	}
	if trace.Name != "seq-success" {
		t.Errorf("expected Name=seq-success, got %q", trace.Name)
	}
	if len(trace.Batches) != 0 {
		t.Errorf("sequential trace should have no batches, got %d", len(trace.Batches))
	}
	if trace.Duration <= 0 {
		t.Error("expected positive total duration")
	}
	if trace.StartedAt.IsZero() || trace.EndedAt.IsZero() {
		t.Error("expected non-zero start/end times")
	}
	if trace.EndedAt.Before(trace.StartedAt) {
		t.Error("ended before started")
	}

	for i, st := range trace.Steps {
		if st.BatchIndex != -1 {
			t.Errorf("step %d: BatchIndex=%d want -1 (sequential)", i, st.BatchIndex)
		}
		if st.Attempts != 1 {
			t.Errorf("step %d: Attempts=%d want 1", i, st.Attempts)
		}
		if st.Skipped {
			t.Errorf("step %d: should not be skipped", i)
		}
		if !st.ConditionPassed {
			t.Errorf("step %d: ConditionPassed should be true", i)
		}
		if st.ErrorText != "" {
			t.Errorf("step %d: ErrorText=%q want empty", i, st.ErrorText)
		}
		if len(st.Recoveries) != 0 {
			t.Errorf("step %d: Recoveries=%v want empty", i, st.Recoveries)
		}
		if st.ExecuteDuration <= 0 {
			t.Errorf("step %d: ExecuteDuration should be positive", i)
		}
		if st.OutputLen <= 0 {
			t.Errorf("step %d: OutputLen should be positive for successful step", i)
		}
	}
}

func TestTrace_SequentialRetrySuccess(t *testing.T) {
	reg := nodes.NewRegistry()
	// Succeeds on 3rd attempt. Retry=2 → maxAttempts=3.
	reg.Register(&traceTestNode{name: "flaky", successOn: 3})

	wf := &Workflow{
		Name: "seq-retry",
		Steps: []WorkflowStep{
			{Node: "flaky", Name: "f", Retry: 2, Delay: "1ms"},
		},
	}

	trace := runWithTrace(t, wf, reg)

	st := trace.Steps[0]
	if st.Attempts != 3 {
		t.Errorf("Attempts=%d want 3", st.Attempts)
	}
	if st.ErrorText != "" {
		t.Errorf("ErrorText=%q want empty (recovered via retry)", st.ErrorText)
	}
	if len(st.Recoveries) != 0 {
		t.Errorf("Recoveries=%v want empty (retry is not a recovery)", st.Recoveries)
	}
}

func TestTrace_SequentialRetryExhausted(t *testing.T) {
	reg := nodes.NewRegistry()
	// Never succeeds. Retry=1 → maxAttempts=2.
	reg.Register(&traceTestNode{name: "bad", successOn: 0})

	wf := &Workflow{
		Name: "seq-retry-exhausted",
		Steps: []WorkflowStep{
			{Node: "bad", Name: "b", Retry: 1, Delay: "1ms"},
		},
	}

	_, _, trace, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	if err == nil {
		t.Fatal("expected error from exhausted retries")
	}
	if trace == nil {
		t.Fatal("expected non-nil trace even on failure")
	}

	st := trace.Steps[0]
	if st.Attempts != 2 {
		t.Errorf("Attempts=%d want 2", st.Attempts)
	}
	if st.ErrorText == "" {
		t.Error("ErrorText should be non-empty after exhausted retries")
	}
	if len(st.Recoveries) != 0 {
		t.Errorf("Recoveries=%v want empty", st.Recoveries)
	}
}

func TestTrace_SequentialFallbackRecovery(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "failer", successOn: 0})

	wf := &Workflow{
		Name: "seq-fallback",
		Steps: []WorkflowStep{
			{Node: "failer", Name: "f", Fallback: "saved"},
		},
	}

	trace := runWithTrace(t, wf, reg)

	st := trace.Steps[0]
	if st.ErrorText != "" {
		t.Errorf("ErrorText=%q want empty (recovered)", st.ErrorText)
	}
	if len(st.Recoveries) != 1 || st.Recoveries[0] != "fallback" {
		t.Errorf("Recoveries=%v want [fallback]", st.Recoveries)
	}
}

func TestTrace_SequentialOnErrorRecovery(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "failer", successOn: 0})
	reg.Register(&traceTestNode{name: "handler", successOn: 1, overrideOutput: "handled"})

	wf := &Workflow{
		Name: "seq-onerror",
		Steps: []WorkflowStep{
			{
				Node: "failer", Name: "f",
				OnError: &Step{Node: "handler"},
			},
		},
	}

	trace := runWithTrace(t, wf, reg)

	st := trace.Steps[0]
	if st.ErrorText != "" {
		t.Errorf("ErrorText=%q want empty (recovered)", st.ErrorText)
	}
	if len(st.Recoveries) != 1 || st.Recoveries[0] != "on_error" {
		t.Errorf("Recoveries=%v want [on_error]", st.Recoveries)
	}
}

func TestTrace_SequentialContinueOnError(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "failer", successOn: 0})
	reg.Register(&traceTestNode{name: "ok", successOn: 1})

	wf := &Workflow{
		Name: "seq-continue",
		Steps: []WorkflowStep{
			{Node: "failer", Name: "f", ContinueOnError: true},
			{Node: "ok", Name: "o"},
		},
	}

	trace := runWithTrace(t, wf, reg)

	st := trace.Steps[0]
	if len(st.Recoveries) != 1 || st.Recoveries[0] != "continue_on_error" {
		t.Errorf("Recoveries=%v want [continue_on_error]", st.Recoveries)
	}
	// continue_on_error preserves the error in StepResult (resultErr is not cleared).
	if st.ErrorText == "" {
		t.Error("ErrorText should be non-empty (continue_on_error preserves error)")
	}
}

func TestTrace_SequentialConditionSkip(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "ok", successOn: 1})

	wf := &Workflow{
		Name: "seq-skip",
		Vars:  map[string]string{"flag": "false"},
		Steps: []WorkflowStep{
			{Node: "ok", Name: "skipped", Condition: "{{var.flag}}"},
		},
	}

	trace := runWithTrace(t, wf, reg)

	st := trace.Steps[0]
	if !st.Skipped {
		t.Error("expected Skipped=true")
	}
	if st.ConditionPassed {
		t.Error("expected ConditionPassed=false")
	}
	if st.ConditionExpr == "" {
		t.Error("expected non-empty ConditionExpr")
	}
	if st.ExecuteDuration != 0 {
		t.Errorf("ExecuteDuration=%v want 0 (skipped, no execution)", st.ExecuteDuration)
	}
}

func TestTrace_SequentialConditionEvalFailure(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "ok", successOn: 1})

	wf := &Workflow{
		Name: "seq-cond-fail",
		Steps: []WorkflowStep{
			{Node: "ok", Name: "o", Condition: "{{var.missing}} == true"},
		},
	}

	_, _, trace, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	if err == nil {
		t.Fatal("expected error from condition evaluation failure")
	}
	if trace == nil {
		t.Fatal("expected non-nil trace")
	}
	if len(trace.Steps) != 1 {
		t.Fatalf("expected 1 step in trace, got %d", len(trace.Steps))
	}

	st := trace.Steps[0]
	if st.ErrorText == "" {
		t.Error("ErrorText should describe the condition evaluation failure")
	}
	if st.ConditionPassed {
		t.Error("ConditionPassed should be false on condition eval failure")
	}
	if st.EvalDuration <= 0 {
		t.Error("EvalDuration should be positive (condition was attempted)")
	}
}

func TestTrace_SequentialEvalFailure(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "ok", successOn: 1})

	wf := &Workflow{
		Name: "seq-eval-fail",
		Steps: []WorkflowStep{
			{Node: "ok", Name: "o", Params: map[string]string{"k": "{{var.missing}}"}},
		},
	}

	_, _, trace, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	if err == nil {
		t.Fatal("expected error from param evaluation failure")
	}
	if trace == nil || len(trace.Steps) != 1 {
		t.Fatal("expected 1 step in trace")
	}

	st := trace.Steps[0]
	if st.ErrorText == "" {
		t.Error("ErrorText should describe the evaluation failure")
	}
	if st.EvalDuration <= 0 {
		t.Error("EvalDuration should be positive")
	}
}

func TestTrace_SequentialNodeNotFound(t *testing.T) {
	reg := nodes.NewRegistry()

	wf := &Workflow{
		Name: "seq-node-missing",
		Steps: []WorkflowStep{
			{Node: "nonexistent", Name: "x"},
		},
	}

	_, _, trace, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	if err == nil {
		t.Fatal("expected error from missing node")
	}
	if trace == nil || len(trace.Steps) != 1 {
		t.Fatal("expected 1 step in trace")
	}

	st := trace.Steps[0]
	if st.ErrorText == "" {
		t.Error("ErrorText should describe the missing node")
	}
}

// ── DAG path ──

func TestTrace_DAGSuccess(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "a", successOn: 1})
	reg.Register(&traceTestNode{name: "b", successOn: 1})
	reg.Register(&traceTestNode{name: "c", successOn: 1})

	wf := &Workflow{
		Name: "dag-success",
		Steps: []WorkflowStep{
			{Node: "a", Name: "a"},
			{Node: "b", Name: "b", DependsOn: []string{"a"}},
			{Node: "c", Name: "c", DependsOn: []string{"a"}},
		},
	}

	trace := runWithTrace(t, wf, reg)

	if trace.Mode != "dag" {
		t.Errorf("expected Mode=dag, got %q", trace.Mode)
	}
	// Diamond: batch 0 = {a}, batch 1 = {b, c}.
	if len(trace.Batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(trace.Batches))
	}
	if len(trace.Batches[0].StepIndices) != 1 || trace.Batches[0].StepIndices[0] != 0 {
		t.Errorf("batch 0 should be [0], got %v", trace.Batches[0].StepIndices)
	}
	if len(trace.Batches[1].StepIndices) != 2 {
		t.Errorf("batch 1 should have 2 steps, got %d", len(trace.Batches[1].StepIndices))
	}
	if trace.Batches[0].Duration <= 0 {
		t.Error("batch 0 duration should be positive")
	}

	// Step 0: batch 0, no deps.
	if trace.Steps[0].BatchIndex != 0 {
		t.Errorf("step 0: BatchIndex=%d want 0", trace.Steps[0].BatchIndex)
	}
	if len(trace.Steps[0].Dependencies) != 0 {
		t.Errorf("step 0: Dependencies=%v want empty", trace.Steps[0].Dependencies)
	}
	// Steps 1 and 2: batch 1, depend on step 0.
	for _, idx := range []int{1, 2} {
		if trace.Steps[idx].BatchIndex != 1 {
			t.Errorf("step %d: BatchIndex=%d want 1", idx, trace.Steps[idx].BatchIndex)
		}
		if len(trace.Steps[idx].Dependencies) != 1 || trace.Steps[idx].Dependencies[0] != 0 {
			t.Errorf("step %d: Dependencies=%v want [0]", idx, trace.Steps[idx].Dependencies)
		}
	}
}

func TestTrace_DAGFallbackRecovery(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "failer", successOn: 0})
	reg.Register(&traceTestNode{name: "ok", successOn: 1})

	wf := &Workflow{
		Name: "dag-fallback",
		Steps: []WorkflowStep{
			{Node: "failer", Name: "f", Fallback: "saved"},
			{Node: "ok", Name: "o", DependsOn: []string{"f"}},
		},
	}

	trace := runWithTrace(t, wf, reg)

	if trace.Mode != "dag" {
		t.Errorf("expected Mode=dag, got %q", trace.Mode)
	}
	// Find the failer step by name.
	var failerStep *StepTrace
	for i := range trace.Steps {
		if trace.Steps[i].NodeName == "failer" {
			failerStep = &trace.Steps[i]
			break
		}
	}
	if failerStep == nil {
		t.Fatal("failer step not found in trace")
	}
	if failerStep.ErrorText != "" {
		t.Errorf("ErrorText=%q want empty (recovered)", failerStep.ErrorText)
	}
	if len(failerStep.Recoveries) != 1 || failerStep.Recoveries[0] != "fallback" {
		t.Errorf("Recoveries=%v want [fallback]", failerStep.Recoveries)
	}
}

func TestTrace_DAGContinueOnError(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "failer", successOn: 0})
	reg.Register(&traceTestNode{name: "ok", successOn: 1})

	wf := &Workflow{
		Name: "dag-continue",
		Steps: []WorkflowStep{
			{Node: "failer", Name: "f", ContinueOnError: true},
			{Node: "ok", Name: "o", DependsOn: []string{"f"}},
		},
	}

	trace := runWithTrace(t, wf, reg)

	var failerStep *StepTrace
	for i := range trace.Steps {
		if trace.Steps[i].NodeName == "failer" {
			failerStep = &trace.Steps[i]
			break
		}
	}
	if failerStep == nil {
		t.Fatal("failer step not found in trace")
	}
	if len(failerStep.Recoveries) != 1 || failerStep.Recoveries[0] != "continue_on_error" {
		t.Errorf("Recoveries=%v want [continue_on_error]", failerStep.Recoveries)
	}
}

func TestTrace_DAGRetrySuccess(t *testing.T) {
	reg := nodes.NewRegistry()
	// Register two independent flaky nodes so each has its own call counter.
	reg.Register(&traceTestNode{name: "flaky1", successOn: 3})
	reg.Register(&traceTestNode{name: "flaky2", successOn: 3})

	wf := &Workflow{
		Name: "dag-retry",
		Steps: []WorkflowStep{
			{Node: "flaky1", Name: "f", Retry: 2, Delay: "1ms"},
			{Node: "flaky2", Name: "g", Retry: 2, Delay: "1ms", DependsOn: []string{"f"}},
		},
	}

	trace := runWithTrace(t, wf, reg)

	// Both flaky nodes should have made 3 attempts.
	for i := range trace.Steps {
		if trace.Steps[i].Attempts != 3 {
			t.Errorf("step %d: Attempts=%d want 3", i, trace.Steps[i].Attempts)
		}
		if trace.Steps[i].ErrorText != "" {
			t.Errorf("step %d: ErrorText=%q want empty (recovered via retry)", i, trace.Steps[i].ErrorText)
		}
	}
}

func TestTrace_DAGConditionSkip(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "ok", successOn: 1})

	wf := &Workflow{
		Name: "dag-skip",
		Vars:  map[string]string{"flag": "false"},
		Steps: []WorkflowStep{
			{Node: "ok", Name: "skipped", Condition: "{{var.flag}}"},
			{Node: "ok", Name: "runs", DependsOn: []string{"skipped"}},
		},
	}

	trace := runWithTrace(t, wf, reg)

	var skippedStep *StepTrace
	for i := range trace.Steps {
		if trace.Steps[i].StepName == "skipped" {
			skippedStep = &trace.Steps[i]
			break
		}
	}
	if skippedStep == nil {
		t.Fatal("skipped step not found in trace")
	}
	if !skippedStep.Skipped {
		t.Error("expected Skipped=true")
	}
	if skippedStep.ConditionPassed {
		t.Error("expected ConditionPassed=false")
	}
	if skippedStep.ExecuteDuration != 0 {
		t.Errorf("ExecuteDuration=%v want 0 (skipped)", skippedStep.ExecuteDuration)
	}
}

// TestTrace_DAGBatchesRecorded verifies the BatchTrace timeline is complete
// and ordered, and that batch durations sum to roughly the workflow duration.
func TestTrace_DAGBatchesRecorded(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&dagSlowNode{name: "s1", delay: 30 * time.Millisecond, callCount: new(int32)})
	reg.Register(&dagSlowNode{name: "s2", delay: 30 * time.Millisecond, callCount: new(int32)})

	wf := &Workflow{
		Name: "dag-batches",
		Steps: []WorkflowStep{
			{Node: "s1", Name: "a"},
			{Node: "s2", Name: "b", DependsOn: []string{"a"}},
		},
	}

	trace := runWithTrace(t, wf, reg)

	if len(trace.Batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(trace.Batches))
	}
	for i, bt := range trace.Batches {
		if bt.Index != i {
			t.Errorf("batch %d: Index=%d want %d", i, bt.Index, i)
		}
		if bt.StartedAt.IsZero() {
			t.Errorf("batch %d: StartedAt is zero", i)
		}
		if bt.Duration <= 0 {
			t.Errorf("batch %d: Duration should be positive", i)
		}
		if len(bt.StepIndices) == 0 {
			t.Errorf("batch %d: StepIndices empty", i)
		}
	}
	// Batches should be temporally ordered.
	if !trace.Batches[1].StartedAt.After(trace.Batches[0].StartedAt) &&
		!trace.Batches[1].StartedAt.Equal(trace.Batches[0].StartedAt) {
		t.Error("batch 1 should start at or after batch 0")
	}
	// Total batch duration should not exceed workflow duration by much.
	var batchSum time.Duration
	for _, bt := range trace.Batches {
		batchSum += bt.Duration
	}
	if batchSum > trace.Duration+50*time.Millisecond {
		t.Errorf("batch sum %v exceeds workflow duration %v by too much", batchSum, trace.Duration)
	}
}

// TestTrace_StepsCoverAllOutcomes is a meta-test ensuring every code path that
// records a StepTrace sets the documented invariants. It runs a workflow that
// mixes success, skip, retry, and recovery in one go.
func TestTrace_StepsCoverAllOutcomes(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "ok", successOn: 1})
	reg.Register(&traceTestNode{name: "flaky", successOn: 3}) // 2 retries then success
	reg.Register(&traceTestNode{name: "failer", successOn: 0})

	wf := &Workflow{
		Name: "seq-mixed",
		Vars:  map[string]string{"skip": "false"},
		Steps: []WorkflowStep{
			{Node: "ok", Name: "s1"},
			{Node: "ok", Name: "s2-skip", Condition: "{{var.skip}}"}, // var.skip="false" → skipped
			{Node: "flaky", Name: "s3", Retry: 2, Delay: "1ms"},
			{Node: "failer", Name: "s4", Fallback: "fb"},
		},
	}

	trace := runWithTrace(t, wf, reg)

	if len(trace.Steps) != 4 {
		t.Fatalf("expected 4 steps in trace, got %d", len(trace.Steps))
	}

	// s1: success
	if trace.Steps[0].Attempts != 1 || trace.Steps[0].Skipped || trace.Steps[0].ErrorText != "" {
		t.Errorf("s1: unexpected trace: %+v", trace.Steps[0])
	}
	// s2: skipped
	if !trace.Steps[1].Skipped || trace.Steps[1].ConditionPassed {
		t.Errorf("s2: expected skipped, got %+v", trace.Steps[1])
	}
	// s3: retried then succeeded
	if trace.Steps[2].Attempts != 3 || trace.Steps[2].ErrorText != "" {
		t.Errorf("s3: expected 3 attempts and no error, got %+v", trace.Steps[2])
	}
	// s4: failed then recovered via fallback
	if len(trace.Steps[3].Recoveries) != 1 || trace.Steps[3].Recoveries[0] != "fallback" {
		t.Errorf("s4: expected [fallback] recovery, got %+v", trace.Steps[3])
	}
}

// TestTrace_AllStepTracesHaveBasicFields ensures no field is accidentally left
// at its zero value when it should be populated.
func TestTrace_AllStepTracesHaveBasicFields(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "ok", successOn: 1})

	wf := &Workflow{
		Name: "seq-fields",
		Steps: []WorkflowStep{
			{Node: "ok", Name: "first"},
		},
	}

	trace := runWithTrace(t, wf, reg)

	st := trace.Steps[0]
	if st.NodeName != "ok" {
		t.Errorf("NodeName=%q want 'ok'", st.NodeName)
	}
	if st.StepName != "first" {
		t.Errorf("StepName=%q want 'first'", st.StepName)
	}
	// Initial input is "" so InputLen may be 0 for the first step; that's acceptable.
	if st.InputLen < 0 {
		t.Error("InputLen should be >= 0")
	}
	// OutputLen must be positive for a successful step.
	if st.OutputLen <= 0 {
		t.Error("OutputLen should be > 0 for successful step")
	}
	if st.TotalDuration <= 0 {
		t.Error("TotalDuration should be > 0")
	}
	// EvalDuration includes EvaluateParams; should be >= 0 (likely 0 for trivial params).
	if st.EvalDuration < 0 {
		t.Error("EvalDuration should be >= 0")
	}
}

// TestTrace_NilSafeRecordStep verifies recordStep on a nil trace does not panic
// (defensive — executeWorkflowSequential always creates a trace, but library
// callers may construct StepTrace-handling code that tolerates nil).
func TestTrace_NilSafeRecordStep(t *testing.T) {
	var nilTrace *WorkflowTrace
	if ptr := nilTrace.recordStep(StepTrace{Index: 0}); ptr != nil {
		t.Errorf("expected nil pointer from nil trace, got %v", ptr)
	}
	nilTrace.finish(time.Now()) // must not panic
}
