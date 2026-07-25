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
	"fmt"
	"strings"
	"testing"

	"github.com/alib8b8/llm-box/internal/nodes"
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
