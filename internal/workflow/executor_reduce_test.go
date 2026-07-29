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
	"encoding/json"
	"strings"
	"testing"

	"github.com/alib8b8/llm-box/internal/nodes"
)

// echoNode returns params["prefix"] + input (no separator). It is the
// workhorse for reduce/capture_error tests: the expression engine evaluates
// {{loop.acc}} / {{var.error}} in `prefix` before the node runs, so the node
// simply stitches the evaluated prefix and the current input together.
type echoNode struct {
	name string
}

func (n *echoNode) Name() string        { return n.name }
func (n *echoNode) Description() string { return "echo with optional prefix" }
func (n *echoNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Description: "echo with optional prefix", Input: "string", Output: "string"}
}
func (n *echoNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if p, ok := params["prefix"]; ok {
		return p + input, nil
	}
	return input, nil
}

// TestReduce_ConcatStrings verifies the canonical left fold: each iteration
// appends the current item to the running accumulator. over="a\nb\nc",
// initial="" should yield "abc". The sub-step uses {{loop.acc}} as the prefix
// so echoNode returns acc+item.
func TestReduce_ConcatStrings(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&echoNode{name: "echo"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "concat",
				Reduce: &ReduceConfig{
					Over:    "a\nb\nc",
					Initial: "",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"prefix": "{{loop.acc}}"}},
					},
				},
			},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("reduce failed: %v", err)
	}
	if output != "abc" {
		t.Errorf("expected 'abc', got %q", output)
	}
	// One result per item (3 items).
	if len(results) != 3 {
		t.Errorf("expected 3 iteration results, got %d", len(results))
	}
}

// TestReduce_WithInitial verifies the initial accumulator is used and visible
// via {{loop.acc}} on the first iteration.
func TestReduce_WithInitial(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&echoNode{name: "echo"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "concat",
				Reduce: &ReduceConfig{
					Over:    "x\ny\nz",
					Initial: "START-",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"prefix": "{{loop.acc}}"}},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("reduce failed: %v", err)
	}
	// iter0: acc=START-, item=x -> "START-x"
	// iter1: acc=START-x, item=y -> "START-xy"
	// iter2: acc=START-xy, item=z -> "START-xyz"
	if output != "START-xyz" {
		t.Errorf("expected 'START-xyz', got %q", output)
	}
}

// TestReduce_JSONArrayInput verifies reduce over a JSON array (the typical
// map→reduce pipeline: map emits a JSON array, reduce folds it).
func TestReduce_JSONArrayInput(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&echoNode{name: "echo"})

	wf := &Workflow{
		Vars: map[string]string{
			"nums": `["1","2","3","4"]`,
		},
		Steps: []WorkflowStep{
			{
				Name: "sum",
				Reduce: &ReduceConfig{
					Over:    "{{var.nums}}",
					Initial: "0",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"prefix": "{{loop.acc}}"}},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("reduce failed: %v", err)
	}
	if output != "01234" {
		t.Errorf("expected '01234', got %q", output)
	}
}

// TestReduce_EmptyItems verifies an empty list returns the initial accumulator.
func TestReduce_EmptyItems(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&echoNode{name: "echo"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "fold",
				Reduce: &ReduceConfig{
					Over:    "",
					Initial: "default",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"prefix": "{{loop.acc}}"}},
					},
				},
			},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("reduce failed: %v", err)
	}
	if output != "default" {
		t.Errorf("expected initial 'default' for empty list, got %q", output)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 iteration results for empty list, got %d", len(results))
	}
}

// TestReduce_MaxIterations verifies the safety cap.
func TestReduce_MaxIterations(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&echoNode{name: "echo"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "fold",
				Reduce: &ReduceConfig{
					Over:          "a\nb\nc\nd\ne",
					MaxIterations: 3,
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"prefix": "{{loop.acc}}"}},
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

// TestReduce_IndexAndCount verifies {{loop.index}} and {{loop.count}} resolve
// inside reduce iterations.
func TestReduce_IndexAndCount(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&echoNode{name: "echo"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "fold",
				Reduce: &ReduceConfig{
					Over:    "a\nb",
					Initial: "",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{
							// item is passed as the sub-step input (map semantics),
							// so the prefix only needs acc + index/count metadata.
							"prefix": "{{loop.acc}}[{{loop.index}}/{{loop.count}}]",
						}},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("reduce failed: %v", err)
	}
	// iter0: acc="", index=0, count=2, item=a -> prefix "[0/2]" + input "a" = "[0/2]a"
	// iter1: acc="[0/2]a", index=1, count=2, item=b -> "[0/2]a[1/2]" + "b" = "[0/2]a[1/2]b"
	if output != "[0/2]a[1/2]b" {
		t.Errorf("expected '[0/2]a[1/2]b', got %q", output)
	}
}

// TestReduce_InheritsParentVars verifies workflow-level vars are visible
// inside reduce iterations.
func TestReduce_InheritsParentVars(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&echoNode{name: "echo"})

	wf := &Workflow{
		Vars: map[string]string{"sep": "|"},
		Steps: []WorkflowStep{
			{
				Name: "fold",
				Reduce: &ReduceConfig{
					Over:    "a\nb",
					Initial: "",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{
							"prefix": "{{loop.acc}}{{var.sep}}",
						}},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("reduce failed: %v", err)
	}
	// iter0: acc="", sep="|", input=a -> "|a"
	// iter1: acc="|a", input=b -> "|a|b"
	if output != "|a|b" {
		t.Errorf("expected '|a|b', got %q", output)
	}
}

// TestReduce_NestedInMap verifies reduce works as a sub-step inside a map
// iteration (proves executeSubStep dispatches IsReduce). Each outer item
// folds a fixed inner list, producing a per-outer-item aggregated string.
func TestReduce_NestedInMap(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&echoNode{name: "echo"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "outer",
				Map: &MapConfig{
					Over: "P\nQ",
					Steps: []WorkflowStep{
						{
							Name: "inner_sum",
							Reduce: &ReduceConfig{
								Over:    "1\n2",
								Initial: "{{loop.item}}", // outer item as seed
								Steps: []WorkflowStep{
									{Node: "echo", Params: map[string]string{"prefix": "{{loop.acc}}"}},
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
		t.Fatalf("nested reduce failed: %v", err)
	}
	// Outer item "P": inner reduce over ["1","2"], initial="P"
	//   iter0: acc=P, item=1 -> "P1"
	//   iter1: acc=P1, item=2 -> "P12"
	// Outer item "Q": initial="Q" -> "Q12"
	// Map default strategy is json_array.
	var arr []string
	if err := json.Unmarshal([]byte(output), &arr); err != nil {
		t.Fatalf("map output not json array: %v (output=%s)", err, output)
	}
	if len(arr) != 2 || arr[0] != "P12" || arr[1] != "Q12" {
		t.Errorf("expected [P12 Q12], got %v", arr)
	}
}

// TestReduce_AbortsOnSubStepFailure verifies that a failing sub-step aborts
// the whole reduce (a missing accumulator makes subsequent iterations
// meaningless), unless the sub-step declares its own recovery.
func TestReduce_AbortsOnSubStepFailure(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&errorNode{name: "fail"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "fold",
				Reduce: &ReduceConfig{
					Over:    "a\nb\nc",
					Initial: "",
					Steps: []WorkflowStep{
						{Node: "fail"},
					},
				},
			},
		},
	}

	_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Fatal("expected error from failing reduce sub-step, got nil")
	}
	if !strings.Contains(err.Error(), "reduce iteration") {
		t.Errorf("error should mention reduce iteration, got: %v", err)
	}
}
