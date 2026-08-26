// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌​​​​​‌‌​​‌‌‌​​​‌​​​‌‌‌​‌​‌‌‌‌​​‌​‌​‌‌​‌‌​‌‌‌‌‌​​​​​​​​​​​​​​​​​‌​‌‌​​​‌​​‌‌​‌​⁠
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
	"testing"

	"github.com/alib8b8/aflare/internal/nodes"
)

type testNode struct {
	name string
}

func (n *testNode) Name() string {
	return n.name
}

func (n *testNode) Description() string {
	return "test node"
}

func (n *testNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{
		Name:        n.name,
		Description: "test node",
		Input:       "string",
		Output:      "string",
		Params: []nodes.ParamSchema{
			{Name: "prefix", Type: "string", Description: "Prefix for output", Required: false},
		},
	}
}

func (n *testNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if msg, ok := params["prefix"]; ok {
		return msg + " " + input, nil
	}
	return "processed: " + input, nil
}

// TestParseWorkflowFromContent_InputField pins the documented `input:`
// step field (docs/dataflow.md): scalar template and list-of-templates
// forms both parse. Before this field existed, yaml.Unmarshal silently
// dropped it and nodes received empty input.
func TestParseWorkflowFromContent_InputField(t *testing.T) {
	wf, err := ParseWorkflowFromContent(`
name: input-parse
steps:
  - node: a
    id: first
  - node: b
    input: "fixed: {{step.first}}"
  - node: c
    input:
      - "{{step.first}}"
      - "tail"
`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if got := wf.Steps[1].Input.Parts(); len(got) != 1 || got[0] != "fixed: {{step.first}}" {
		t.Errorf("scalar input = %v, want [fixed: {{step.first}}]", got)
	}
	if got := wf.Steps[2].Input.Parts(); len(got) != 2 || got[0] != "{{step.first}}" || got[1] != "tail" {
		t.Errorf("list input = %v, want [{{step.first}} tail]", got)
	}
	if wf.Steps[0].Input != nil {
		t.Errorf("unset input should stay nil, got %v", wf.Steps[0].Input.Parts())
	}
}

// TestParseWorkflowFromContent_InputInvalidType ensures a malformed input
// (mapping instead of string/list) is a load-time error, not a silent drop.
func TestParseWorkflowFromContent_InputInvalidType(t *testing.T) {
	_, err := ParseWorkflowFromContent(`
name: bad-input
steps:
  - node: a
    input:
      template: "{{step.x}}"
`)
	if err == nil {
		t.Fatal("expected error for mapping-form input")
	}
}

// TestParseWorkflowFromContent_IDAlias pins the documented `id:` naming
// alias (docs/dataflow.md "Assign names using the id field"): it is
// promoted to the canonical Name at parse time, recursively through
// compound sub-workflows, and `name:` wins when both are present.
func TestParseWorkflowFromContent_IDAlias(t *testing.T) {
	wf, err := ParseWorkflowFromContent(`
name: id-alias
steps:
  - node: a
    id: by_id
  - node: b
    name: by_name
    id: shadowed
  - node: c
    id: container
    if:
      condition: "{{step.by_id}} != ''"
      then:
        - node: d
          id: nested_id
      else:
        - node: e
    map:
      over: "[1]"
      steps:
        - node: f
          id: map_id
`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if wf.Steps[0].Name != "by_id" {
		t.Errorf("id should promote to name: got %q", wf.Steps[0].Name)
	}
	if wf.Steps[1].Name != "by_name" {
		t.Errorf("name should win over id: got %q", wf.Steps[1].Name)
	}
	if n := wf.Steps[2].If.Then[0].Name; n != "nested_id" {
		t.Errorf("nested if-branch id should promote: got %q", n)
	}
	if n := wf.Steps[2].Map.Steps[0].Name; n != "map_id" {
		t.Errorf("map sub-step id should promote: got %q", n)
	}
}

// TestExecuteWorkflow_InputOverride verifies the runtime half of the
// documented behavior: input: replaces the default previous-step output,
// with {{step.*}}, {{var.*}} and {{input}} all resolving inside it.
func TestExecuteWorkflow_InputOverride(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Name: "input-override",
		Vars: map[string]string{"suffix": "VAR"},
		Steps: []WorkflowStep{
			{Node: "test", Name: "first"},
			{Node: "test", Name: "second", Params: map[string]string{"prefix": ""}, Input: &StepInput{parts: []string{"step1={{step.first}} var={{var.suffix}} prev={{input}}"}}},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
	// testNode: prefix + " " + input; empty prefix → " " + input
	want := " step1=processed:  var=VAR prev=processed: "
	if output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}

// TestExecuteWorkflow_InputListJoin pins the list form's join semantics:
// each template renders individually, parts join with "\n---\n".
func TestExecuteWorkflow_InputListJoin(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Name: "input-list",
		Steps: []WorkflowStep{
			{Node: "test", Name: "a", Params: map[string]string{"prefix": "A"}},
			{Node: "test", Name: "b", Params: map[string]string{"prefix": ""}, Input: &StepInput{parts: []string{"{{step.a}}", "static"}}},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
	// step a output: "A " + "" → "A "; list join: "A \n---\nstatic"; step b: " " + that
	want := " A \n---\nstatic"
	if output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}

// TestExecuteWorkflow_InputOverrideDAG verifies input: also applies in
// DAG scheduling mode (depends_on).
func TestExecuteWorkflow_InputOverrideDAG(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Name: "input-dag",
		Steps: []WorkflowStep{
			{Node: "test", Name: "src"},
			{Node: "test", Name: "sink", DependsOn: []string{"src"}, Params: map[string]string{"prefix": ""}, Input: &StepInput{parts: []string{"[{{step.src}}]"}}},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
	want := " [processed: ]"
	if output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}

// TestExecuteWorkflow_InputOverrideParamsSeesOverride pins cross-mode
// consistency: {{input}} inside params resolves to the OVERRIDDEN input in
// sequential mode, matching DAG-mode ordering (override → condition →
// params → node).
func TestExecuteWorkflow_InputOverrideParamsSeesOverride(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Name: "input-params",
		Steps: []WorkflowStep{
			{Node: "test", Name: "src", Params: map[string]string{"prefix": "S"}},
			{Node: "test", Name: "sink", Input: &StepInput{parts: []string{"OVERRIDE"}}, Params: map[string]string{"prefix": "P:{{input}}"}},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
	// src → "S "; sink input overridden to "OVERRIDE", params prefix
	// rendered against it → "P:OVERRIDE" + " " + "OVERRIDE".
	want := "P:OVERRIDE OVERRIDE"
	if output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}

// TestExecuteWorkflow_InputOverrideSubStep pins input: on a sub-step inside
// an if branch (the executeSubStep path shared by if/map/reduce/saga): the
// override applies BEFORE the sub-step's condition evaluation.
func TestExecuteWorkflow_InputOverrideSubStep(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Name: "input-substep",
		Steps: []WorkflowStep{
			{Node: "test", Name: "src", Params: map[string]string{"prefix": "S"}},
			{
				Node: "test", Name: "branch",
				If: &IfConfig{
					Condition: "not_empty",
					Then: []WorkflowStep{
						{
							Node: "test", Name: "inner",
							Input:     &StepInput{parts: []string{"GO"}},
							Condition: "equals:GO",
							Params:    map[string]string{"prefix": ""},
						},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
	// The condition "equals:GO" only passes because the override was
	// applied before condition evaluation; inner then outputs " GO".
	want := " GO"
	if output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}

// TestExecuteWorkflow_InputOverrideConditionSkipRestore pins that a
// condition-skipped step's input: override does NOT leak into the next
// step: skip still passes the original upstream value through.
func TestExecuteWorkflow_InputOverrideConditionSkipRestore(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Name: "input-skip-restore",
		Steps: []WorkflowStep{
			{Node: "test", Name: "src", Params: map[string]string{"prefix": "S"}},
			{Node: "test", Name: "skipped", Input: &StepInput{parts: []string{"LEAK"}}, Condition: "empty"},
			{Node: "test", Name: "after", Params: map[string]string{"prefix": ""}},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
	// "LEAK" is not empty → skipped; upstream "S " passes through (the
	// override is restored); after → " S ".
	want := " S "
	if output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}

func TestExecuteWorkflow_Simple(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Name: "test workflow",
		Steps: []WorkflowStep{
			{Node: "test", Params: map[string]string{"prefix": "first"}},
			{Node: "test", Params: map[string]string{"prefix": "second"}},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	expected := "second first "
	if output != expected {
		t.Errorf("expected output '%s', got '%s'", expected, output)
	}

	if results[0].Duration <= 0 {
		t.Error("first step duration should be positive")
	}
}

func TestExecuteWorkflow_NodeNotFound(t *testing.T) {
	reg := nodes.NewRegistry()

	wf := &Workflow{
		Steps: []WorkflowStep{
			{Node: "nonexistent"},
		},
	}

	_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestExecuteWorkflow_StepError(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&errorNode{name: "err"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{Node: "err"},
		},
	}

	_, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Error("expected error from error node")
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if results[0].Error == nil {
		t.Error("expected error in result")
	}
}

type errorNode struct {
	name string
}

func (n *errorNode) Name() string {
	return n.name
}

func (n *errorNode) Description() string {
	return "error node"
}

func (n *errorNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{
		Name:        n.name,
		Description: "error node",
		Input:       "string",
		Output:      "string",
		Params:      nil,
	}
}

func (n *errorNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return "", &testError{msg: "test error"}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestExecuteWorkflow_EmptyWorkflow(t *testing.T) {
	reg := nodes.NewRegistry()

	wf := &Workflow{
		Steps: []WorkflowStep{},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output != "" {
		t.Errorf("expected empty output, got '%s'", output)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParseWorkflow_InvalidFile(t *testing.T) {
	_, err := ParseWorkflow("/nonexistent/file.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// ── Error handling tests ──

func TestContinueOnError(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&errorNode{name: "err"})
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{Node: "err", ContinueOnError: true},
			{Node: "test", Params: map[string]string{"prefix": "after"}},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("expected workflow to continue, got error: %v", err)
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
	if output != "after " {
		t.Errorf("expected 'after ', got '%s'", output)
	}
}

func TestFallback(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&errorNode{name: "err"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{Node: "err", Fallback: "fallback value"},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("expected fallback to prevent error: %v", err)
	}
	if output != "fallback value" {
		t.Errorf("expected 'fallback value', got '%s'", output)
	}
	if results[0].Error != nil {
		t.Error("error should be recovered")
	}
}

func TestOnError(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&errorNode{name: "err"})
	reg.Register(&testNode{name: "handler"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Node: "err",
				OnError: &Step{
					Node:   "handler",
					Params: map[string]string{"prefix": "recovered"},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("expected on_error handler to prevent error: %v", err)
	}
	if output != "recovered " {
		t.Errorf("expected 'recovered ', got '%s'", output)
	}
}

func TestParallelMaxFailures(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "ok"})
	reg.Register(&errorNode{name: "fail"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Parallel: []Step{
					{Node: "ok"},
					{Node: "fail"},
					{Node: "ok"},
				},
				MaxFailures: 1,
			},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("expected workflow to tolerate 1 failure: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// output should contain 2 successful outputs
	if output == "" {
		t.Error("expected non-empty output from successful parallel steps")
	}
}

func TestParallelExceedsMaxFailures(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "ok"})
	reg.Register(&errorNode{name: "fail"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Parallel: []Step{
					{Node: "fail"},
					{Node: "fail"},
					{Node: "ok"},
				},
				MaxFailures: 1,
			},
		},
	}

	_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Error("expected error when failures exceed max_failures")
	}
}

// ── Loop tests ──

func TestLoopSequential(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "echo"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Node: "echo",
				Loop: &LoopConfig{
					Items: "apple\nbanana\ncherry",
				},
			},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("loop execution failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Each iteration: "processed: " + item
	expected := "processed: apple\n---\nprocessed: banana\n---\nprocessed: cherry"
	if output != expected {
		t.Errorf("expected '%s', got '%s'", expected, output)
	}
}

func TestLoopWithParams(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "echo"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Node:   "echo",
				Params: map[string]string{"prefix": "item:"},
				Loop: &LoopConfig{
					Items: "one\ntwo",
				},
			},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("loop execution failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	expected := "item: one\n---\nitem: two"
	if output != expected {
		t.Errorf("expected '%s', got '%s'", expected, output)
	}
}

func TestLoopConcurrent(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "echo"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Node: "echo",
				Loop: &LoopConfig{
					Items:       "a\nb\nc",
					Concurrency: 3,
				},
			},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("loop execution failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Results should be in order
	if results[0].Input != "a" || results[1].Input != "b" || results[2].Input != "c" {
		t.Error("results should be in original order")
	}
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestLoopStopOnError(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&failOnNode{name: "conditional"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Node: "conditional",
				Loop: &LoopConfig{
					Items: "ok1\nfail\nok2",
				},
			},
		},
	}

	_, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Error("expected error from loop with stop_on_error")
	}
	// Should have stopped after 2 iterations (ok1 + fail)
	if len(results) != 2 {
		t.Errorf("expected 2 results (stopped on error), got %d", len(results))
	}
}

func TestLoopContinueOnError(t *testing.T) {
	stopFalse := false
	reg := nodes.NewRegistry()
	reg.Register(&failOnNode{name: "conditional"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Node: "conditional",
				Loop: &LoopConfig{
					Items:       "ok1\nfail\nok2",
					StopOnError: &stopFalse,
				},
			},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("expected loop to continue on error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Only 2 successful outputs (ok1 and ok2)
	if output == "" {
		t.Error("expected non-empty output from successful iterations")
	}
}

func TestLoopEmptyItems(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "echo"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Node: "echo",
				Loop: &LoopConfig{
					Items: "",
				},
			},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if output != "" {
		t.Errorf("expected empty output, got '%s'", output)
	}
}

func TestLoopMaxIterations(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "echo"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Node: "echo",
				Loop: &LoopConfig{
					Items:         "a\nb\nc",
					MaxIterations: 2,
				},
			},
		},
	}

	_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Error("expected error when items exceed max_iterations")
	}
}

func TestLoopVariableAccess(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "echo"})

	// Use {{loop.index}} in params to verify loop variable access
	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Node:   "echo",
				Params: map[string]string{"prefix": "[{{loop.index}}]"},
				Loop: &LoopConfig{
					Items: "x\ny\nz",
				},
			},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("loop execution failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	expected := "[0] x\n---\n[1] y\n---\n[2] z"
	if output != expected {
		t.Errorf("expected '%s', got '%s'", expected, output)
	}
}

func TestLoopCustomVar(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "echo"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Node:   "echo",
				Params: map[string]string{"prefix": "val:"},
				Loop: &LoopConfig{
					Items: "one\ntwo",
					Var:   "entry",
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("loop execution failed: %v", err)
	}
	// The custom var "entry" is set as a param, but the item is also the input
	// testNode with prefix "val:" returns "val: " + input
	expected := "val: one\n---\nval: two"
	if output != expected {
		t.Errorf("expected '%s', got '%s'", expected, output)
	}
}

// failOnNode fails when input contains "fail", succeeds otherwise.
type failOnNode struct {
	name string
}

func (n *failOnNode) Name() string        { return n.name }
func (n *failOnNode) Description() string { return "fails on 'fail' input" }
func (n *failOnNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Description: "fails on 'fail'", Input: "string", Output: "string"}
}
func (n *failOnNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if strings.Contains(input, "fail") {
		return "", fmt.Errorf("intentional failure for input: %s", input)
	}
	return "ok: " + input, nil
}
