// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌​​​‌‌‌‌‌‌​​‌​‌​‌‌‌‌​‌‌‌‌‌‌‌​‌​​‌​​‌‌​​​‌‌‌​​‌​​​​​​​​​​​​​​​​‌‌​​‌​​​‌‌‌‌‌​‌​⁠
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
	"os"
	"path/filepath"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes"
)

// TestExecutor_WAL_EndToEndResume verifies the full WAL persistence lifecycle
// through the Executor public API:
//  1. Run a 4-step workflow that fails at step 3 (WAL records steps 1 and 2).
//  2. Re-run with the same WAL path: execution resumes from step 3, skipping
//     steps 1-2. The final output reflects only the post-resume steps.
//  3. A second successful run overwrites the WAL with the new tail records.
//
// This exercises the integration between Executor, ExpressionEngine (step
// output restore), WAL (append + replay), and SaveStateWAL/LoadStateWAL.
func TestExecutor_WAL_EndToEndResume(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	walPath := filepath.Join(tmp, "wf.wal")

	// Use a registry local to this test with stateful nodes that track
	// invocation counts, so we can assert exactly which steps ran in each run.
	reg := nodes.NewRegistry()
	var echoCalls, failCalls int
	reg.Register(&statefulEcho{name: "echo", counter: &echoCalls})
	reg.Register(&statefulFail{name: "fail", counter: &failCalls, failUntil: 1})

	wf := &Workflow{
		Name: "wal-resume-test",
		Steps: []WorkflowStep{
			{Node: "echo", Params: map[string]string{"prefix": "s1"}},
			{Node: "echo", Params: map[string]string{"prefix": "s2"}},
			{Node: "fail", Params: map[string]string{"prefix": "s3"}},
			{Node: "echo", Params: map[string]string{"prefix": "s4"}},
		},
	}

	exec := func() (string, []StepResult, error) {
		return NewExecutor().WithWAL(walPath).Execute(context.Background(), wf, reg)
	}

	// Run 1: should fail at step 3 (first fail invocation).
	_, _, err := exec()
	if err == nil {
		t.Fatalf("run1: expected failure at step 3, got nil error")
	}
	if echoCalls != 2 {
		t.Errorf("run1: expected 2 echo calls (s1+s2), got %d", echoCalls)
	}
	if failCalls != 1 {
		t.Errorf("run1: expected 1 fail call (s3), got %d", failCalls)
	}

	// Verify the WAL has records for steps 0 and 1 (s1, s2).
	state, err := LoadStateWAL(walPath)
	if err != nil {
		t.Fatalf("LoadStateWAL after run1: %v", err)
	}
	if state == nil {
		t.Fatalf("LoadStateWAL returned nil state, expected at least one record")
	}
	if state.StepIndex != 1 {
		t.Errorf("after run1: WAL StepIndex = %d, want 1 (last completed step)", state.StepIndex)
	}

	// Run 2: should resume from step 3 (index 2). failUntil was 1, so the
	// first call already consumed the failure budget; this call succeeds.
	output, _, err := exec()
	if err != nil {
		t.Fatalf("run2: expected success after resume, got: %v", err)
	}
	// Only step 3 (fail, now succeeds) + step 4 (echo) should have run.
	// echoCalls should be 2 (run1) + 1 (s4 in run2) = 3.
	if echoCalls != 3 {
		t.Errorf("run2: expected 3 total echo calls (2 from run1 + 1 from s4), got %d", echoCalls)
	}
	if failCalls != 2 {
		t.Errorf("run2: expected 2 total fail calls (1 from run1 + 1 from s3 retry), got %d", failCalls)
	}

	// Final output should come from s4 (the last step).
	if output == "" {
		t.Errorf("run2: expected non-empty output from s4")
	}

	// WAL should now record up to step 3 (index 3) after successful run2.
	state2, err := LoadStateWAL(walPath)
	if err != nil {
		t.Fatalf("LoadStateWAL after run2: %v", err)
	}
	if state2 == nil {
		t.Fatalf("LoadStateWAL returned nil state after run2")
	}
	if state2.StepIndex != 3 {
		t.Errorf("after run2: WAL StepIndex = %d, want 3 (all steps completed)", state2.StepIndex)
	}

	// Run 3: with a fully-completed WAL, resume should be a no-op (resumeFromStep
	// clamps to len(steps)=4, loop body never executes).
	echoCallsBefore := echoCalls
	failCallsBefore := failCalls
	if _, _, err := exec(); err != nil {
		t.Fatalf("run3: expected nil error on already-complete WAL, got: %v", err)
	}
	if echoCalls != echoCallsBefore || failCalls != failCallsBefore {
		t.Errorf("run3: should not execute any steps on a complete WAL; echoCalls=%d→%d failCalls=%d→%d",
			echoCallsBefore, echoCalls, failCallsBefore, failCalls)
	}

	// Cleanup
	if err := os.Remove(walPath); err != nil && !os.IsNotExist(err) {
		t.Logf("cleanup: %v", err)
	}
}

// statefulEcho is a counter node that appends its prefix to the input.
type statefulEcho struct {
	name    string
	counter *int
}

func (n *statefulEcho) Name() string        { return n.name }
func (n *statefulEcho) Description() string { return "echo node for test" }
func (n *statefulEcho) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "string"}
}
func (n *statefulEcho) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	*n.counter++
	if p, ok := params["prefix"]; ok {
		return p + ":" + input, nil
	}
	return input, nil
}

// statefulFail fails the first failUntil invocations, then succeeds.
type statefulFail struct {
	name      string
	counter   *int
	failUntil int
}

func (n *statefulFail) Name() string        { return n.name }
func (n *statefulFail) Description() string { return "fail node for test" }
func (n *statefulFail) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "string"}
}
func (n *statefulFail) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	*n.counter++
	if *n.counter <= n.failUntil {
		return "", errTestCrash
	}
	if p, ok := params["prefix"]; ok {
		return p + ":" + input, nil
	}
	return input, nil
}

// register is a no-op placeholder kept for documentation; real test nodes
// are statefulEcho and statefulFail above.

var errTestCrash = &testCrashError{}

type testCrashError struct{}

func (e *testCrashError) Error() string { return "simulated crash at step 3" }
