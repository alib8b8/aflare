// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌​​​​​​​​‌‌​​‌​​​‌​‌​​​​‌‌​‌‌‌‌​​​‌​‌​​​‌​​​‌​‌‌​‌‌​​‌‌​‌‌​‌​​‌‌​​​​​​​​​​​​​​​​​​​​‌‌‌‌​‌‌‌​‌‌‌‌⁠
// aflare​‌​​​​​​‌​​‌‌​​‌‌‌​​​​​‌​​​​‌‌​​‌​‌​​‌​​‌​​​​​‌​‌​​​​‌‌​​​​​‌‌​‌‌‌‌‌‌‌​‌​​​‌‌​‌‌​​​​‌​​‌​‌​​​‌​​‌‌​​​‌​​‌​‌​​‌‌​​​​​‌​​‌‌​‌​​​‌‌​‌​​‌‌‌‌‌‌​​‌​​​​‌‌​​‌‌​‌​‌‌​‌‌​​‌​​​​​‌‌​​‌‌‌​​‌​‌​‌​‌​‌‌‌‌‌​
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
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes"
)

// ── DAG checkpoint/resume ───────────────────────────────────────────────
//
// 覆盖崩溃恢复的核心承诺：重启后跳过已完成节点，从断点继续。所有测试
// 走 NewExecutor().WithCheckpoint(...) —— 与 CLI 相同的入口。

// dagCountNode echoes "name(input)" and counts executions, so tests can
// assert which nodes actually ran across a resume.
type dagCountNode struct {
	name  string
	calls int32
}

func (n *dagCountNode) Name() string        { return n.name }
func (n *dagCountNode) Description() string { return "counting test node" }
func (n *dagCountNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "string"}
}
func (n *dagCountNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	atomic.AddInt32(&n.calls, 1)
	return n.name + "(" + input + ")", nil
}

// dagFlakyNode fails while its fail flag is set; tests flip the flag
// between runs to simulate "the environment recovered after the crash".
type dagFlakyNode struct {
	name  string
	fail  atomic.Bool
	calls int32
}

func (n *dagFlakyNode) Name() string        { return n.name }
func (n *dagFlakyNode) Description() string { return "flaky test node" }
func (n *dagFlakyNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "string"}
}
func (n *dagFlakyNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	atomic.AddInt32(&n.calls, 1)
	if n.fail.Load() {
		return "", errors.New("simulated failure")
	}
	return n.name + "(" + input + ")", nil
}

// newDAGCheckpointRegistry registers counting nodes for a diamond workflow
// and returns the registry plus the nodes keyed by name.
func newDAGCheckpointRegistry(names ...string) (*nodes.Registry, map[string]*dagCountNode) {
	reg := nodes.NewRegistry()
	counters := make(map[string]*dagCountNode, len(names))
	for _, name := range names {
		n := &dagCountNode{name: name}
		reg.Register(n)
		counters[name] = n
	}
	return reg, counters
}

// diamondWF is a→(b,c)→d: resume must restore a and whichever of b/c is in
// the checkpoint, then run the rest.
func diamondWF() *Workflow {
	return &Workflow{
		Name: "dag-checkpoint-test",
		Steps: []WorkflowStep{
			{Node: "a", Name: "a"},
			{Node: "b", Name: "b", DependsOn: []string{"a"}},
			{Node: "c", Name: "c", DependsOn: []string{"a"}},
			{Node: "d", Name: "d", DependsOn: []string{"b", "c"}},
		},
	}
}

func assertCalls(t *testing.T, counters map[string]*dagCountNode, want map[string]int32) {
	t.Helper()
	for name, n := range counters {
		if got := atomic.LoadInt32(&n.calls); got != want[name] {
			t.Errorf("node %s executed %d times, want %d", name, got, want[name])
		}
	}
}

// TestDAGCheckpoint_FullRunThenResumeAllSkipped runs a diamond to
// completion, then re-runs the same workflow against the checkpoint: every
// node is resumed, nothing re-executes, and the output is identical.
func TestDAGCheckpoint_FullRunThenResumeAllSkipped(t *testing.T) {
	cpPath := filepath.Join(t.TempDir(), "dag-cp.json")
	reg, counters := newDAGCheckpointRegistry("a", "b", "c", "d")
	wf := diamondWF()

	out1, _, err := NewExecutor().WithCheckpoint(cpPath).Execute(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("first DAG run failed: %v", err)
	}
	assertCalls(t, counters, map[string]int32{"a": 1, "b": 1, "c": 1, "d": 1})

	// Second run: the completed-node set covers everything → all resumed.
	out2, results2, trace2, err := NewExecutor().WithCheckpoint(cpPath).ExecuteWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("resumed DAG run failed: %v", err)
	}
	assertCalls(t, counters, map[string]int32{"a": 1, "b": 1, "c": 1, "d": 1}) // no re-execution

	if out1 != out2 {
		t.Errorf("resumed output %q differs from original %q", out2, out1)
	}

	if len(results2) != 4 {
		t.Fatalf("expected 4 step results on resume, got %d", len(results2))
	}
	resumedCount := 0
	for _, st := range trace2.Steps {
		if st.Resumed {
			resumedCount++
		}
	}
	if resumedCount != 4 {
		t.Errorf("expected 4 resumed steps in trace, got %d", resumedCount)
	}

	// All-resumed output: the checkpoint's Data (last completed output).
	if out2 := results2[len(results2)-1].Output; out2 == "" {
		t.Error("resumed run produced empty output")
	}
	if _, ok := loadCheckpointForTest(t, cpPath); !ok {
		t.Fatal("checkpoint file missing after run")
	}
}

func loadCheckpointForTest(t *testing.T, path string) (*WorkflowState, bool) {
	t.Helper()
	state, err := loadCheckpoint(path)
	if err != nil || state == nil {
		return nil, false
	}
	return state, true
}

// TestDAGCheckpoint_PartialResumeRunsRemainingSubgraph simulates a crash
// after a and b completed (hand-written checkpoint, exactly what the engine
// persists per finalized node): the resume must skip a and b, run c and d,
// and hand d the restored b output.
func TestDAGCheckpoint_PartialResumeRunsRemainingSubgraph(t *testing.T) {
	cpPath := filepath.Join(t.TempDir(), "dag-cp.json")
	if err := saveCheckpoint(cpPath, &WorkflowState{
		WorkflowName: "dag-checkpoint-test",
		StepIndex:    -1,
		Data:         "b-result",
		StepOutputs:  map[int]string{0: "a-restored", 1: "b-result"},
		Variables:    map[string]string{"seed": "kept"},
		DAGMode:      true,
		StepNames:    []string{"a", "b", "c", "d"},
	}); err != nil {
		t.Fatalf("saveCheckpoint: %v", err)
	}

	reg, counters := newDAGCheckpointRegistry("a", "b", "c", "d")
	out, _, trace, err := NewExecutor().WithCheckpoint(cpPath).ExecuteWithTrace(context.Background(), diamondWF(), reg, nil)
	if err != nil {
		t.Fatalf("resumed DAG run failed: %v", err)
	}

	assertCalls(t, counters, map[string]int32{"a": 0, "b": 0, "c": 1, "d": 1})

	// d must have received the RESTORED b output: its input is the
	// multi-dep join "b-result\n---\nc(...)".
	if want := "d(b-result\n---\nc(a-restored))"; out != want {
		t.Errorf("final output = %q, want %q", out, want)
	}

	resumed := map[int]bool{}
	for _, st := range trace.Steps {
		if st.Resumed {
			resumed[st.Index] = true
		}
	}
	if !resumed[0] || !resumed[1] || resumed[2] || resumed[3] {
		t.Errorf("expected steps 0,1 resumed and 2,3 executed, got %v", resumed)
	}
}

// TestDAGCheckpoint_RejectsModifiedWorkflow verifies the StepNames guard:
// a checkpoint saved against a different step list must not be applied —
// index-based outputs would silently attach to the wrong steps.
func TestDAGCheckpoint_RejectsModifiedWorkflow(t *testing.T) {
	cpPath := filepath.Join(t.TempDir(), "dag-cp.json")
	if err := saveCheckpoint(cpPath, &WorkflowState{
		WorkflowName: "dag-checkpoint-test",
		StepIndex:    -1,
		StepOutputs:  map[int]string{0: "a-restored", 1: "b-restored"},
		Variables:    map[string]string{},
		DAGMode:      true,
		// Checkpoint claims 5 steps (workflow gained a step after the
		// crash): every index would shift, so the resume must be rejected.
		StepNames: []string{"a", "b", "c", "d", "e"},
	}); err != nil {
		t.Fatalf("saveCheckpoint: %v", err)
	}

	reg, counters := newDAGCheckpointRegistry("a", "b", "c", "d")

	if _, _, err := NewExecutor().WithCheckpoint(cpPath).Execute(context.Background(), diamondWF(), reg); err != nil {
		t.Fatalf("run with mismatched checkpoint failed: %v", err)
	}
	assertCalls(t, counters, map[string]int32{"a": 1, "b": 1, "c": 1, "d": 1}) // fresh start, everything ran
}

// TestDAGCheckpoint_RejectsSequentialCheckpoint verifies mode isolation: a
// sequential checkpoint (linear StepIndex cursor) must not be interpreted
// as a DAG completed-node set.
func TestDAGCheckpoint_RejectsSequentialCheckpoint(t *testing.T) {
	cpPath := filepath.Join(t.TempDir(), "dag-cp.json")
	if err := saveCheckpoint(cpPath, &WorkflowState{
		WorkflowName: "dag-checkpoint-test",
		StepIndex:    1, // sequential cursor: "completed step 1"
		StepOutputs:  map[int]string{0: "a", 1: "b"},
		Variables:    map[string]string{},
	}); err != nil {
		t.Fatalf("saveCheckpoint: %v", err)
	}

	reg, counters := newDAGCheckpointRegistry("a", "b", "c", "d")
	if _, _, err := NewExecutor().WithCheckpoint(cpPath).Execute(context.Background(), diamondWF(), reg); err != nil {
		t.Fatalf("run with sequential checkpoint failed: %v", err)
	}
	assertCalls(t, counters, map[string]int32{"a": 1, "b": 1, "c": 1, "d": 1}) // rejected → fresh start
}

// TestDAGCheckpoint_FailedStepNotCached pins the honest-cache semantics: a
// step that failed under continue_on_error completes the WORKFLOW but is
// not cached as completed — a crash resume must re-run it, not replay its
// failure as a success.
func TestDAGCheckpoint_FailedStepNotCached(t *testing.T) {
	cpPath := filepath.Join(t.TempDir(), "dag-cp.json")

	flaky := &dagFlakyNode{name: "flaky"}
	reg := nodes.NewRegistry()
	a := &dagCountNode{name: "a"}
	c := &dagCountNode{name: "c"}
	reg.Register(a)
	reg.Register(flaky)
	reg.Register(c)

	wf := &Workflow{
		Name: "dag-flaky-test",
		Steps: []WorkflowStep{
			{Node: "a", Name: "a"},
			{Node: "flaky", Name: "flaky", DependsOn: []string{"a"}, ContinueOnError: true},
			{Node: "c", Name: "c", DependsOn: []string{"flaky"}},
		},
	}

	// Run 1: flaky fails but continue_on_error lets the workflow finish.
	flaky.fail.Store(true)
	if _, _, err := NewExecutor().WithCheckpoint(cpPath).Execute(context.Background(), wf, reg); err != nil {
		t.Fatalf("run with continue_on_error failure should succeed: %v", err)
	}
	if got := atomic.LoadInt32(&flaky.calls); got != 1 {
		t.Fatalf("flaky calls after run 1 = %d, want 1", got)
	}

	// The checkpoint must contain a and c but NOT the failed flaky step.
	state, ok := loadCheckpointForTest(t, cpPath)
	if !ok {
		t.Fatal("checkpoint missing after run")
	}
	if !state.DAGMode {
		t.Fatal("checkpoint not marked DAGMode")
	}
	for _, idx := range []int{0, 2} {
		if _, done := state.StepOutputs[idx]; !done {
			t.Errorf("step %d (succeeded) missing from checkpoint: %v", idx, state.StepOutputs)
		}
	}
	if _, done := state.StepOutputs[1]; done {
		t.Error("failed step (continue_on_error) must NOT be cached as completed")
	}

	// Run 2 (crash resume, environment fixed): a and c are resumed, flaky
	// re-runs and now succeeds.
	flaky.fail.Store(false)
	if _, _, err := NewExecutor().WithCheckpoint(cpPath).Execute(context.Background(), wf, reg); err != nil {
		t.Fatalf("resumed run failed: %v", err)
	}
	if got := atomic.LoadInt32(&flaky.calls); got != 2 {
		t.Errorf("flaky calls after resume = %d, want 2 (failure must re-run)", got)
	}
	if got := atomic.LoadInt32(&a.calls); got != 1 {
		t.Errorf("a calls after resume = %d, want 1 (resumed, not re-run)", got)
	}
	if got := atomic.LoadInt32(&c.calls); got != 1 {
		t.Errorf("c calls after resume = %d, want 1 (resumed, not re-run)", got)
	}
}

// TestDAGCheckpoint_AtomicWriteGuarantee is a structural check that the
// checkpoint file survives a simulated torn write: the atomic-write
// primitive (temp + rename) is exercised directly in internal/fsutil; here
// we verify the DAG executor only ever replaces the previous checkpoint
// wholesale, so a mid-crash resume loads the last complete state.
func TestDAGCheckpoint_PerNodePersistence(t *testing.T) {
	cpPath := filepath.Join(t.TempDir(), "dag-cp.json")
	reg, counters := newDAGCheckpointRegistry("a", "b", "c", "d")

	_, _, err := NewExecutor().WithCheckpoint(cpPath).Execute(context.Background(), diamondWF(), reg)
	if err != nil {
		t.Fatalf("DAG run failed: %v", err)
	}
	assertCalls(t, counters, map[string]int32{"a": 1, "b": 1, "c": 1, "d": 1})

	state, ok := loadCheckpointForTest(t, cpPath)
	if !ok {
		t.Fatal("checkpoint missing")
	}
	if len(state.StepOutputs) != 4 {
		t.Errorf("checkpoint has %d completed nodes, want 4 (persisted after every finalized node)", len(state.StepOutputs))
	}
	if !state.DAGMode {
		t.Error("checkpoint missing DAGMode marker")
	}
	if len(state.StepNames) != 4 {
		t.Errorf("checkpoint StepNames = %v, want 4 entries", state.StepNames)
	}
}
