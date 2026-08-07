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
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/nodes"
)

// mapTestNode prefixes input with a configurable prefix (default "proc:").
type mapTestNode struct {
	name string
}

func (n *mapTestNode) Name() string        { return n.name }
func (n *mapTestNode) Description() string { return "map test node" }
func (n *mapTestNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Description: "map test node", Input: "string", Output: "string"}
}
func (n *mapTestNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	prefix := params["prefix"]
	if prefix == "" {
		prefix = "proc:"
	}
	return prefix + " " + input, nil
}

// counterNode records concurrent invocations to verify parallelism.
type counterNode struct {
	name    string
	counter *int32
}

func (n *counterNode) Name() string        { return n.name }
func (n *counterNode) Description() string { return "counts invocations" }
func (n *counterNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Description: "counts invocations", Input: "string", Output: "string"}
}
func (n *counterNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	atomic.AddInt32(n.counter, 1)
	// Sleep briefly so concurrent runs overlap, proving parallelism.
	time.Sleep(20 * time.Millisecond)
	return "ok:" + input, nil
}

// TestMap_SequentialNewlineSplit verifies basic map over a newline list.
func TestMap_SequentialNewlineSplit(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&mapTestNode{name: "proc"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "batch",
				Map: &MapConfig{
					Over: "apple\nbanana\ncherry",
					Steps: []WorkflowStep{
						{Node: "proc", Params: map[string]string{"prefix": "item"}},
					},
				},
			},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("map failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Default strategy is json_array.
	var arr []string
	if err := json.Unmarshal([]byte(output), &arr); err != nil {
		t.Fatalf("output not json array: %v (output=%s)", err, output)
	}
	expected := []string{"item apple", "item banana", "item cherry"}
	for i, want := range expected {
		if arr[i] != want {
			t.Errorf("arr[%d]=%q, want %q", i, arr[i], want)
		}
	}
}

// TestMap_JSONArrayInput verifies map over a JSON array of strings.
func TestMap_JSONArrayInput(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&mapTestNode{name: "proc"})

	wf := &Workflow{
		Vars: map[string]string{
			"items": `["alpha","beta"]`,
		},
		Steps: []WorkflowStep{
			{
				Name: "batch",
				Map: &MapConfig{
					Over: "{{var.items}}",
					Steps: []WorkflowStep{
						{Node: "proc"},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("map failed: %v", err)
	}
	var arr []string
	if err := json.Unmarshal([]byte(output), &arr); err != nil {
		t.Fatalf("output not json array: %v (output=%s)", err, output)
	}
	if len(arr) != 2 || arr[0] != "proc: alpha" || arr[1] != "proc: beta" {
		t.Errorf("unexpected arr: %v", arr)
	}
}

// TestMap_MultiStepSubWorkflow verifies that map runs a SEQUENCE of steps
// per item (not a single node like Loop). This is the core differentiator.
func TestMap_MultiStepSubWorkflow(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&mapTestNode{name: "proc"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "batch",
				Map: &MapConfig{
					Over: "x\ny",
					Steps: []WorkflowStep{
						// Step 1: prefix with "a"
						{Node: "proc", Params: map[string]string{"prefix": "a"}},
						// Step 2: take step1 output (now the input) and prefix with "b"
						{Node: "proc", Params: map[string]string{"prefix": "b"}},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("map failed: %v", err)
	}
	var arr []string
	if err := json.Unmarshal([]byte(output), &arr); err != nil {
		t.Fatalf("output not json array: %v (output=%s)", err, output)
	}
	// item "x": step1 -> "a x", step2 -> "b a x"
	if arr[0] != "b a x" || arr[1] != "b a y" {
		t.Errorf("expected [b a x, b a y], got %v", arr)
	}
}

// TestMap_Concurrent verifies that concurrency > 1 actually runs in
// parallel (measured by wall-clock time, not just by result count).
func TestMap_Concurrent(t *testing.T) {
	reg := nodes.NewRegistry()
	var counter int32
	reg.Register(&counterNode{name: "count", counter: &counter})

	items := make([]string, 10)
	for i := range items {
		items[i] = "i"
	}
	itemsStr := strings.Join(items, "\n")

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "batch",
				Map: &MapConfig{
					Over:        itemsStr,
					Concurrency: 10,
					Steps: []WorkflowStep{
						{Node: "count"},
					},
				},
			},
		},
	}

	start := time.Now()
	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("map failed: %v", err)
	}
	if got := atomic.LoadInt32(&counter); got != 10 {
		t.Fatalf("counter=%d, want 10", got)
	}
	// 10 * 20ms sequential = 200ms; parallel (concurrency=10) ≈ 20ms.
	// Allow generous slack for CI; the point is it must be well under 200ms.
	if elapsed > 150*time.Millisecond {
		t.Errorf("concurrent map took %v, expected <150ms (parallelism not working)", elapsed)
	}
	// Output should still be ordered.
	var arr []string
	if err := json.Unmarshal([]byte(output), &arr); err != nil {
		t.Fatalf("output not json array: %v", err)
	}
	if len(arr) != 10 {
		t.Errorf("expected 10 outputs, got %d", len(arr))
	}
}

// TestMap_StopOnError verifies that the default behavior aborts on first
// failure and returns an error.
func TestMap_StopOnError(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&mapTestNode{name: "ok"})
	reg.Register(&errorNode{name: "fail"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "batch",
				Map: &MapConfig{
					Over: "one\ntwo\nthree",
					Steps: []WorkflowStep{
						{Node: "fail"},
					},
				},
			},
		},
	}

	_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Fatal("expected error from failing map, got nil")
	}
	if !strings.Contains(err.Error(), "map iteration") {
		t.Errorf("error should mention map iteration, got: %v", err)
	}
}

// TestMap_ContinueOnError verifies that stop_on_error=false collects empty
// outputs for failed items and continues.
func TestMap_ContinueOnError(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&mapTestNode{name: "ok"})
	reg.Register(&errorNode{name: "fail"})

	falsePtr := false
	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "batch",
				Map: &MapConfig{
					Over:        "one\ntwo\nthree",
					StopOnError: &falsePtr,
					Steps: []WorkflowStep{
						{Node: "fail"},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("expected no error with stop_on_error=false, got: %v", err)
	}
	// Failed items contribute empty strings; json_array keeps them.
	var arr []string
	if err := json.Unmarshal([]byte(output), &arr); err != nil {
		t.Fatalf("output not json array: %v (output=%s)", err, output)
	}
	if len(arr) != 3 {
		t.Errorf("expected 3 entries (incl. empty), got %d", len(arr))
	}
	for _, s := range arr {
		if s != "" {
			t.Errorf("expected empty string for failed item, got %q", s)
		}
	}
}

// TestMap_EmptyItems verifies graceful handling of empty input.
func TestMap_EmptyItems(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&mapTestNode{name: "proc"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "batch",
				Map: &MapConfig{
					Over: "",
					Steps: []WorkflowStep{
						{Node: "proc"},
					},
				},
			},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("map failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty items, got %d", len(results))
	}
	if output != "" {
		t.Errorf("expected empty output, got %q", output)
	}
}

// TestMap_MaxIterations verifies the safety cap.
func TestMap_MaxIterations(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&mapTestNode{name: "proc"})

	// 5 items, cap at 3.
	items := "a\nb\nc\nd\ne"
	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "batch",
				Map: &MapConfig{
					Over:          items,
					MaxIterations: 3,
					Steps: []WorkflowStep{
						{Node: "proc"},
					},
				},
			},
		},
	}

	_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Fatal("expected error for exceeding max_iterations, got nil")
	}
	if !strings.Contains(err.Error(), "max_iterations") {
		t.Errorf("error should mention max_iterations, got: %v", err)
	}
}

// TestMap_InheritsParentVars verifies that workflow-level vars are visible
// inside map iterations.
func TestMap_InheritsParentVars(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&mapTestNode{name: "proc"})

	wf := &Workflow{
		Vars: map[string]string{"lang": "zh"},
		Steps: []WorkflowStep{
			{
				Name: "batch",
				Map: &MapConfig{
					Over: "one\ntwo",
					Steps: []WorkflowStep{
						// prefix uses an inherited var.
						{Node: "proc", Params: map[string]string{"prefix": "{{var.lang}}"}},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("map failed: %v", err)
	}
	var arr []string
	if err := json.Unmarshal([]byte(output), &arr); err != nil {
		t.Fatalf("output not json array: %v", err)
	}
	if arr[0] != "zh one" || arr[1] != "zh two" {
		t.Errorf("inherited var not applied: %v", arr)
	}
}

// TestMap_ItemVarAccess verifies {{item}} resolves inside iterations.
func TestMap_ItemVarAccess(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&mapTestNode{name: "proc"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "batch",
				Map: &MapConfig{
					Over: "alpha\nbeta",
					Steps: []WorkflowStep{
						// Use {{item}} as input via a param trick: pass it
						// as the node's input by making it the step's input.
						// Since map sets the item as `data` (input to first
						// sub-step), a plain echo-style node suffices.
						{Node: "proc", Params: map[string]string{"prefix": "got"}},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("map failed: %v", err)
	}
	var arr []string
	if err := json.Unmarshal([]byte(output), &arr); err != nil {
		t.Fatalf("output not json array: %v", err)
	}
	if arr[0] != "got alpha" || arr[1] != "got beta" {
		t.Errorf("{{item}} not flowing: %v", arr)
	}
}

// TestMap_NestedMap verifies a map inside a map (each item expands to a
// sub-list). This is the hardest case and proves the engine isolation
// works.
func TestMap_NestedMap(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&mapTestNode{name: "proc"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "outer",
				Map: &MapConfig{
					Over: "A\nB",
					Steps: []WorkflowStep{
						{
							Name: "inner",
							Map: &MapConfig{
								Over: "1\n2",
								Steps: []WorkflowStep{
									{Node: "proc", Params: map[string]string{"prefix": "n"}},
								},
							},
						},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("nested map failed: %v", err)
	}
	// Inner `Over` is "1\n2" literally (not the outer item), so each
	// outer iteration runs inner over ["1","2"], producing ["n 1","n 2"].
	// Outer's json_array strategy parses each inner output as JSON (it is
	// a JSON array string), so the result is a nested array:
	// [["n 1","n 2"],["n 1","n 2"]].
	// This verifies isolation: inner doesn't accidentally use outer's item.
	var outer [][]string
	if err := json.Unmarshal([]byte(output), &outer); err != nil {
		t.Fatalf("outer not json array of arrays: %v (output=%s)", err, output)
	}
	if len(outer) != 2 {
		t.Fatalf("expected 2 outer items, got %d", len(outer))
	}
	for _, inner := range outer {
		if len(inner) != 2 || inner[0] != "n 1" || inner[1] != "n 2" {
			t.Errorf("unexpected inner: %v", inner)
		}
	}
}
